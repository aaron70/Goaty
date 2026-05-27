package repositories

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/aaron70/goaty/errors"
)

type FS[I comparable, E any] struct {
	rootDir   string
	marshal   func(E) ([]byte, error)
	unmarshal func([]byte) (E, error)
}

func defaultMarshal[E any](v E) ([]byte, error) {
	return json.Marshal(v)
}

func defaultUnmarshal[E any](data []byte) (E, error) {
	var v E
	if err := json.Unmarshal(data, &v); err != nil {
		return v, err
	}
	return v, nil
}

func NewFSRepository[I comparable, E any](rootDir string) (*FS[I, E], error) {
	return NewFSRepositoryWithSerializer[I,](rootDir, defaultMarshal[E], defaultUnmarshal[E])
}

func NewFSRepositoryWithSerializer[I comparable, E any](
	rootDir string,
	marshal func(E) ([]byte, error),
	unmarshal func([]byte) (E, error),
) (*FS[I, E], error) {
	if err := os.MkdirAll(rootDir, 0755); err != nil {
		return nil, errors.NewError(nil, err, "failed to create repository directory")
	}

	return &FS[I, E]{
		rootDir:   rootDir,
		marshal:   marshal,
		unmarshal: unmarshal,
	}, nil
}

func (r *FS[I, E]) filePath(id I) string {
	return filepath.Join(r.rootDir, fmt.Sprintf("%v.json", id))
}

func (r *FS[I, E]) fileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (r *FS[I, E]) Save(id I, entity E) (E, error) {
	path := r.filePath(id)

	exists, err := r.fileExists(path)
	if err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to check file existence")
	}
	if exists {
		var zero E
		return zero, errors.Wrap(errors.ErrConflict, errors.New("entity with id %v already exists", id))
	}

	data, err := r.marshal(entity)
	if err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to marshal entity")
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to write file")
	}

	return entity, nil
}

func (r *FS[I, E]) Update(id I, entity E) (E, error) {
	path := r.filePath(id)

	exists, err := r.fileExists(path)
	if err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to check file existence")
	}
	if !exists {
		var zero E
		return zero, errors.Wrap(errors.ErrNotFound, errors.New("entity with id %v not found", id))
	}

	data, err := r.marshal(entity)
	if err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to marshal entity")
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to write file")
	}

	return entity, nil
}

func (r *FS[I, E]) Get(id I) (E, error) {
	path := r.filePath(id)

	data, err := os.ReadFile(path)
	if err != nil {
		var zero E
		if os.IsNotExist(err) {
			return zero, errors.Wrap(errors.ErrNotFound, errors.New("entity with id %v not found", id))
		}
		return zero, errors.NewError(nil, err, "failed to read file")
	}

	entity, err := r.unmarshal(data)
	if err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to unmarshal entity")
	}

	return entity, nil
}

func (r *FS[I, E]) GetAll() ([]E, error) {
	entries, err := os.ReadDir(r.rootDir)
	if err != nil {
		return nil, errors.NewError(nil, err, "failed to read directory")
	}

	entities := make([]E, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		data, err := os.ReadFile(filepath.Join(r.rootDir, entry.Name()))
		if err != nil {
			return nil, errors.NewError(nil, err, "failed to read file %s", entry.Name())
		}

		entity, err := r.unmarshal(data)
		if err != nil {
			return nil, errors.NewError(nil, err, "failed to unmarshal file %s", entry.Name())
		}

		entities = append(entities, entity)
	}

	return entities, nil
}

func (r *FS[I, E]) Delete(id I) (E, error) {
	path := r.filePath(id)

	entity, err := r.Get(id)
	if err != nil {
		var zero E
		return zero, err
	}

	if err := os.Remove(path); err != nil {
		var zero E
		return zero, errors.NewError(nil, err, "failed to delete file")
	}

	return entity, nil
}
