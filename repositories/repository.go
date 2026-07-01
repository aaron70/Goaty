package repositories

import (
	"github.com/aaron70/goaty/options"
)

type Option = options.OptionAny
type Repository[I comparable, E any] interface {
	Save(I, E, ...Option) (E, error)
	Update(I, E, ...Option) (E, error)
	Get(I, ...Option) (E, error)
	GetAll(...Option) ([]E, error)
	Delete(I, ...Option) (E, error)
}

