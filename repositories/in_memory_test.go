package repositories

import (
	stderr "errors"
	"os"
	"testing"

	customErrors "github.com/aaron70/goaty/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemory_Save(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	entity, err := repo.Save("1", "hello", FSSaveOptions.WithPermissions(os.ModePerm))
	assert.NoError(t, err)
	assert.Equal(t, "hello", entity)
}

func TestInMemory_Save_Duplicate(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	_, err = repo.Save("1", "hello")
	require.NoError(t, err)

	_, err = repo.Save("1", "world")
	assert.Error(t, err)
	assert.True(t, stderr.Is(err, customErrors.ErrConflict), "expected ErrConflict")
}

func TestInMemory_Get(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	_, err = repo.Save("1", "hello")
	require.NoError(t, err)

	entity, err := repo.Get("1")
	assert.NoError(t, err)
	assert.Equal(t, "hello", entity)
}

func TestInMemory_Get_NotFound(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	_, err = repo.Get("nonexistent")
	assert.Error(t, err)
	assert.True(t, stderr.Is(err, customErrors.ErrNotFound), "expected ErrNotFound")
}

func TestInMemory_Update(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
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

func TestInMemory_Update_NotFound(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	_, err = repo.Update("nonexistent", "value")
	assert.Error(t, err)
	assert.True(t, stderr.Is(err, customErrors.ErrNotFound), "expected ErrNotFound")
}

func TestInMemory_Delete(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	_, err = repo.Save("1", "hello")
	require.NoError(t, err)

	entity, err := repo.Delete("1")
	assert.NoError(t, err)
	assert.Equal(t, "hello", entity)

	_, err = repo.Get("1")
	assert.True(t, stderr.Is(err, customErrors.ErrNotFound), "expected ErrNotFound after delete")
}

func TestInMemory_Delete_NotFound(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	_, err = repo.Delete("nonexistent")
	assert.Error(t, err)
	assert.True(t, stderr.Is(err, customErrors.ErrNotFound), "expected ErrNotFound")
}

func TestInMemory_GetAll_Empty(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	entities, err := repo.GetAll()
	assert.NoError(t, err)
	assert.Empty(t, entities)
}

func TestInMemory_GetAll(t *testing.T) {
	repo, err := NewInMemoryRepository[string, string]()
	require.NoError(t, err)

	_, err = repo.Save("1", "a")
	require.NoError(t, err)
	_, err = repo.Save("2", "b")
	require.NoError(t, err)

	entities, err := repo.GetAll()
	assert.NoError(t, err)
	assert.ElementsMatch(t, []string{"a", "b"}, entities)
}

func TestInMemory_WithIntKey(t *testing.T) {
	repo, err := NewInMemoryRepository[int, string]()
	require.NoError(t, err)

	_, err = repo.Save(42, "answer")
	require.NoError(t, err)

	entity, err := repo.Get(42)
	assert.NoError(t, err)
	assert.Equal(t, "answer", entity)
}
