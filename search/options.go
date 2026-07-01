package search

var StoreOptions = storeOptions{}

type storeOptions struct{}

type storeOption func(*storeConfig) error
type storeConfig struct {
}

