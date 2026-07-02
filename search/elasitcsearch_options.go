package search

import (
	"encoding/json"

	"github.com/aaron70/goaty/options"
)

var ElasticsearchOptions = elasticsearchOptions{}

type indexConfig struct {
	CreateIndex bool
	Body        json.RawMessage
}

type indexOption func(*indexConfig) error

func (o indexOption) Apply(v any) error {
	return options.CastOptionAny[indexConfig](o, v)
}

var _ options.OptionAny = indexOption(func(*indexConfig) error { return nil })

type elasticsearchOptions struct{}

func (o elasticsearchOptions) WithCreateIndex(body json.RawMessage) indexOption {
	return func(ic *indexConfig) error {
		ic.CreateIndex = true
		ic.Body = body
		return nil
	}
}
