package search

 import (
 	"bytes"
 	"context"
 	"encoding/json"

 	"github.com/aaron70/goaty/errors"
 	"github.com/aaron70/goaty/utils"
 	elasticsearch "github.com/elastic/go-elasticsearch/v8"
 	"github.com/elastic/go-elasticsearch/v8/esapi"
 )

 type Elasticsearch[I, T any] struct {
 	client *elasticsearch.Client
 }

 var _ Index[any, any] = &Elasticsearch[any, any]{}

 func NewElasticsearch[I, T any](cfg elasticsearch.Config) (*Elasticsearch[I, T], error) {
 	client, err := elasticsearch.NewClient(cfg)
 	if err != nil {
 		return nil, err
 	}
 	return &Elasticsearch[I, T]{client: client}, nil
 }


 func (e *Elasticsearch[I, T]) Store(ctx context.Context, bucket string, doc T, options ...storeOption) error {
 	cfg := &storeConfig{}
 	if err := utils.ApplyOptions(cfg, options...); err != nil {
 		return err
 	}

 	body, err := json.Marshal(doc)
 	if err != nil {
 		return errors.NewError(errors.ErrSerialization, err, "Couldn't serialize the document")
 	}

 	req := esapi.IndexRequest{
 		Index:      bucket,
 		Body:       bytes.NewReader(body),
 		DocumentID: "",
 	}

 	res, err := req.Do(ctx, e.client)
 	if err != nil {
 		return errors.NewError(nil, err, "Couldn't store the document")
 	}
 	defer res.Body.Close()

 	if res.IsError() {
 		return errors.NewError(nil, nil, "%s", res.String())
 	}

 	return nil
 }
