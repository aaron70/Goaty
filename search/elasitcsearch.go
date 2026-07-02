package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"

	"github.com/aaron70/goaty/errors"
	"github.com/aaron70/goaty/options"
	"github.com/aaron70/goaty/repositories"
	elasticsearch "github.com/elastic/go-elasticsearch/v8"
	"github.com/elastic/go-elasticsearch/v8/esapi"
)

var _ repositories.Repository[any, any] = new(esIndex[any, any])

type Elasticsearch[I comparable, E any] struct {
	client *elasticsearch.Client
}

func NewElasticsearch[I comparable, E any](cfg elasticsearch.Config) (*Elasticsearch[I, E], error) {
	client, err := elasticsearch.NewClient(cfg)
	if err != nil {
		return nil, errors.NewError(nil, err, "failed to create elasticsearch client")
	}
	return &Elasticsearch[I, E]{
		client: client,
	}, nil
}

func (e *Elasticsearch[I, E]) Index(name string, opts ...options.OptionAny) (Index[I, E], error) {
	cfg, err := options.ApplyAnyOptions(opts, indexConfig{})
	if err != nil {
		return nil, err
	}

	if cfg.CreateIndex {
		if err := e.ensureIndex(name, cfg.Body); err != nil {
			return nil, err
		}
	}

	return &esIndex[I, E]{
		client:    e.client,
		indexName: name,
	}, nil
}

func (e *Elasticsearch[I, E]) ensureIndex(name string, body json.RawMessage) error {
	res, err := e.client.Indices.Exists([]string{name})
	if err != nil {
		return errors.NewError(nil, err, "failed to check index existence")
	}
	defer res.Body.Close()

	if res.StatusCode == 200 {
		return nil
	}

	var createOpts []func(*esapi.IndicesCreateRequest)
	if body != nil {
		createOpts = append(createOpts, e.client.Indices.Create.WithBody(bytes.NewReader(body)))
	}

	res, err = e.client.Indices.Create(name, createOpts...)
	if err != nil {
		return errors.NewError(nil, err, "failed to create index")
	}
	defer res.Body.Close()

	if res.IsError() {
		resBody, _ := io.ReadAll(res.Body)
		return errors.New("elasticsearch create index failed: status=%s body=%s", res.Status(), string(resBody))
	}

	return nil
}

type esIndex[I comparable, E any] struct {
	client    *elasticsearch.Client
	indexName string
}

func (idx *esIndex[I, E]) Save(id I, entity E, opts ...options.OptionAny) (E, error) {
	var zero E

	body, err := json.Marshal(entity)
	if err != nil {
		return zero, errors.NewError(nil, err, "failed to marshal entity")
	}

	res, err := idx.client.Create(idx.indexName, fmt.Sprintf("%v", id), bytes.NewReader(body))
	if err != nil {
		return zero, errors.NewError(nil, err, "failed to create document")
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == 409 {
			return zero, errors.Wrap(errors.ErrConflict, errors.New("entity with id %v already exists", id))
		}
		resBody, _ := io.ReadAll(res.Body)
		return zero, errors.New("elasticsearch create failed: status=%s body=%s", res.Status(), string(resBody))
	}

	return entity, nil
}

func (idx *esIndex[I, E]) Update(id I, entity E, opts ...options.OptionAny) (E, error) {
	var zero E

	body, err := json.Marshal(map[string]any{"doc": entity})
	if err != nil {
		return zero, errors.NewError(nil, err, "failed to marshal entity")
	}

	res, err := idx.client.Update(idx.indexName, fmt.Sprintf("%v", id), bytes.NewReader(body))
	if err != nil {
		return zero, errors.NewError(nil, err, "failed to update document")
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == 404 {
			return zero, errors.Wrap(errors.ErrNotFound, errors.New("entity with id %v not found", id))
		}
		resBody, _ := io.ReadAll(res.Body)
		return zero, errors.New("elasticsearch update failed: status=%s body=%s", res.Status(), string(resBody))
	}

	return entity, nil
}

func (idx *esIndex[I, E]) Get(id I, opts ...options.OptionAny) (E, error) {
	var zero E

	res, err := idx.client.Get(idx.indexName, fmt.Sprintf("%v", id))
	if err != nil {
		return zero, errors.NewError(nil, err, "failed to get document")
	}
	defer res.Body.Close()

	if res.IsError() {
		if res.StatusCode == 404 {
			return zero, errors.Wrap(errors.ErrNotFound, errors.New("entity with id %v not found", id))
		}
		resBody, _ := io.ReadAll(res.Body)
		return zero, errors.New("elasticsearch get failed: status=%s body=%s", res.Status(), string(resBody))
	}

	var getResp struct {
		Source json.RawMessage `json:"_source"`
	}
	if err := json.NewDecoder(res.Body).Decode(&getResp); err != nil {
		return zero, errors.NewError(nil, err, "failed to decode get response")
	}

	var entity E
	if err := json.Unmarshal(getResp.Source, &entity); err != nil {
		return zero, errors.NewError(nil, err, "failed to unmarshal entity")
	}

	return entity, nil
}

func (idx *esIndex[I, E]) GetAll(opts ...options.OptionAny) ([]E, error) {
	body := bytes.NewReader([]byte(`{"query":{"match_all":{}}}`))

	res, err := idx.client.Search(
		idx.client.Search.WithIndex(idx.indexName),
		idx.client.Search.WithBody(body),
		idx.client.Search.WithSize(10000),
	)
	if err != nil {
		return nil, errors.NewError(nil, err, "failed to search documents")
	}
	defer res.Body.Close()

	if res.IsError() {
		resBody, _ := io.ReadAll(res.Body)
		return nil, errors.New("elasticsearch search failed: status=%s body=%s", res.Status(), string(resBody))
	}

	var searchResp struct {
		Hits struct {
			Hits []struct {
				Source json.RawMessage `json:"_source"`
			} `json:"hits"`
		} `json:"hits"`
	}
	if err := json.NewDecoder(res.Body).Decode(&searchResp); err != nil {
		return nil, errors.NewError(nil, err, "failed to decode search response")
	}

	entities := make([]E, 0, len(searchResp.Hits.Hits))
	for _, hit := range searchResp.Hits.Hits {
		var entity E
		if err := json.Unmarshal(hit.Source, &entity); err != nil {
			return nil, errors.NewError(nil, err, "failed to unmarshal entity")
		}
		entities = append(entities, entity)
	}

	return entities, nil
}

func (idx *esIndex[I, E]) Delete(id I, opts ...options.OptionAny) (E, error) {
	entity, err := idx.Get(id)
	if err != nil {
		var zero E
		return zero, err
	}

	res, err := idx.client.Delete(idx.indexName, fmt.Sprintf("%v", id))
	if err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to delete document")
	}
	defer res.Body.Close()

	if res.IsError() {
		resBody, _ := io.ReadAll(res.Body)
		var zero E
		return zero, errors.New("elasticsearch delete failed: status=%s body=%s", res.Status(), string(resBody))
	}

	return entity, nil
}
