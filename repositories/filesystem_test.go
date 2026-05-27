package repositories

import (
	"fmt"
	stderr "errors"
	"strconv"
	"strings"
	"testing"

	customErrors "github.com/aaron70/goaty/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFS_Save(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	entity, err := repo.Save("1", "hello")
	assert.NoError(t, err)
	assert.Equal(t, "hello", entity)
}

func TestFS_Save_Duplicate(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Save("1", "hello")
	require.NoError(t, err)

	_, err = repo.Save("1", "world")
	assert.Error(t, err)
	assert.True(t, stderr.Is(err, customErrors.ErrConflict), "expected ErrConflict")
}

func TestFS_Get(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Save("1", "hello")
	require.NoError(t, err)

	entity, err := repo.Get("1")
	assert.NoError(t, err)
	assert.Equal(t, "hello", entity)
}

func TestFS_Get_NotFound(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Get("nonexistent")
	assert.Error(t, err)
	assert.True(t, stderr.Is(err, customErrors.ErrNotFound), "expected ErrNotFound")
}

func TestFS_Update(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Save("1", "hello")
	require.NoError(t, err)

	entity, err := repo.Update("1", "world")
	assert.NoError(t, err)
	assert.Equal(t, "world", entity)

	got, err := repo.Get("1")
	assert.NoError(t, err)
	assert.Equal(t, "world", got)
}

func TestFS_Update_NotFound(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Update("nonexistent", "value")
	assert.Error(t, err)
	assert.True(t, stderr.Is(err, customErrors.ErrNotFound), "expected ErrNotFound")
}

func TestFS_Delete(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Save("1", "hello")
	require.NoError(t, err)

	entity, err := repo.Delete("1")
	assert.NoError(t, err)
	assert.Equal(t, "hello", entity)

	_, err = repo.Get("1")
	assert.True(t, stderr.Is(err, customErrors.ErrNotFound), "expected ErrNotFound after delete")
}

func TestFS_Delete_NotFound(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Delete("nonexistent")
	assert.Error(t, err)
	assert.True(t, stderr.Is(err, customErrors.ErrNotFound), "expected ErrNotFound")
}

func TestFS_GetAll_Empty(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	entities, err := repo.GetAll()
	assert.NoError(t, err)
	assert.Empty(t, entities)
}

func TestFS_GetAll(t *testing.T) {
	repo, err := NewFSRepository[string, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Save("1", "a")
	require.NoError(t, err)
	_, err = repo.Save("2", "b")
	require.NoError(t, err)

	entities, err := repo.GetAll()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"a", "b"}, entities)
}

func TestFS_WithIntKey(t *testing.T) {
	repo, err := NewFSRepository[int, string](t.TempDir())
	require.NoError(t, err)

	_, err = repo.Save(42, "answer")
	require.NoError(t, err)

	entity, err := repo.Get(42)
	assert.NoError(t, err)
	assert.Equal(t, "answer", entity)
}

func TestFS_WithCustomSerializer(t *testing.T) {
	type entity struct {
		Name string
		Age  int
	}

	marshal := func(e entity) ([]byte, error) {
		return fmt.Appendf(nil, "%s,%d", e.Name, e.Age), nil
	}
	unmarshal := func(data []byte) (entity, error) {
		parts := strings.SplitN(string(data), ",", 2)
		if len(parts) != 2 {
			return entity{}, fmt.Errorf("invalid format: %s", string(data))
		}
		age, err := strconv.Atoi(parts[1])
		if err != nil {
			return entity{}, fmt.Errorf("invalid age: %w", err)
		}
		return entity{Name: parts[0], Age: age}, nil
	}

	repo, err := NewFSRepositoryWithSerializer[string, ](t.TempDir(), marshal, unmarshal)
	require.NoError(t, err)

	e := entity{Name: "Alice", Age: 30}
	saved, err := repo.Save("1", e)
	require.NoError(t, err)
	assert.Equal(t, e, saved)

	got, err := repo.Get("1")
	require.NoError(t, err)
	assert.Equal(t, e, got)
}

func TestFS_RepoDirIsCreated(t *testing.T) {
	dir := t.TempDir() + "/nested/subdir"
	repo, err := NewFSRepository[string, string](dir)
	require.NoError(t, err)

	_, err = repo.Save("1", "hello")
	assert.NoError(t, err)
}
