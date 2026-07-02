package repositories

import (
	"github.com/aaron70/goaty/options"
)

type Repository[I comparable, E any] interface {
	Save(I, E, ...options.OptionAny) (E, error)
	Update(I, E, ...options.OptionAny) (E, error)
	Get(I, ...options.OptionAny) (E, error)
	GetAll(...options.OptionAny) ([]E, error)
	Delete(I, ...options.OptionAny) (E, error)
}

