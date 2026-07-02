package search

import (
	"github.com/aaron70/goaty/options"
	"github.com/aaron70/goaty/repositories"
)

type Index[I comparable, E any] interface {
	repositories.Repository[I, E]
}

type SearchEngine[I comparable, E any] interface {
	Index(string, ...options.OptionAny) (Index[I, E], error)
}

