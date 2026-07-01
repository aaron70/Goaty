package repositories

import (
	"fmt"
	"sync"

	"github.com/aaron70/goaty/errors"
)

var _ Repository[any, any] = &InMemory[any, any]{}

type InMemory[I comparable, E any] struct {
	mu sync.Mutex
	DB map[I]E
}

func NewInMemoryRepository[I comparable, E any]() (*InMemory[I, E], error) {
	repo := &InMemory[I, E]{
		DB: make(map[I]E),
	}
	return repo, nil
}

func (r *InMemory[I, E]) Save(id I, entity E, options ...SaveOption) (E, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.DB[id]; ok {
		var zero E
		return zero, errors.Wrap(errors.ErrConflict, fmt.Errorf("entity with id %v already exists", id))
	}

	r.DB[id] = entity
	return entity, nil
}

func (r *InMemory[I, E]) Update(id I, entity E) (E, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	var zero E
	if _, ok := r.DB[id]; !ok {
		return zero, errors.Wrap(errors.ErrNotFound, fmt.Errorf("entity with id %v not found", id))
	}

	r.DB[id] = entity
	return entity, nil
}

func (r *InMemory[I, E]) Get(id I) (E, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entity, ok := r.DB[id]
	if !ok {
		var zero E
		return zero, errors.Wrap(errors.ErrNotFound, fmt.Errorf("entity with id %v not found", id))
	}

	return entity, nil
}

func (r *InMemory[I, E]) GetAll() ([]E, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entities := make([]E, 0, len(r.DB))
	for _, entity := range r.DB {
		entities = append(entities, entity)
	}

	return entities, nil
}

func (r *InMemory[I, E]) Delete(id I) (E, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entity, ok := r.DB[id]
	if !ok {
		var zero E
		return zero, errors.Wrap(errors.ErrNotFound, fmt.Errorf("entity with id %v not found", id))
	}

	delete(r.DB, id)
	return entity, nil
}
