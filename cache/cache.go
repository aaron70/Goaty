package cache

import (
	"sync"
	"time"

	"github.com/aaron70/goaty/errors"
)

type Cache[K, T any] interface {
	Get(key K) (T, error)
	Put(key K, value T) error
	Del(key K) (T, error)
	Clear() error
	Close()
}

type ttlCacheRecord[T any] struct {
	TTL             time.Duration
	LastUpdatedTime time.Time
	Data            T
}

type TTLCache[K, T any] struct {
	TTL       time.Duration
	PurgeTime time.Duration
	cache     sync.Map
	done      chan struct{}
}

func NewTTLCache[K, T any](ttl time.Duration, purge time.Duration) (*TTLCache[K,T], error) {
	cache := &TTLCache[K,T]{
		TTL:       ttl,
		PurgeTime: purge,
		cache: sync.Map{},
		done: make(chan struct{}),
	}

	go func(c *TTLCache[K,T]) {
		timer := time.NewTicker(c.PurgeTime)
		for {
			select {
			case <-c.done:
				return
			case <-timer.C:
				c.cache.Range(func(key, value any) bool {
					v := value.(ttlCacheRecord[T])
					if time.Since(v.LastUpdatedTime) >= v.TTL {
						c.cache.Delete(key)
					}
					return true
				})
			}
		}
	}(cache)

	return cache, nil
}

func (c *TTLCache[K, T]) Get(key K) (T, error) {
	var zero T
	v, found := c.cache.Load(key)
	if !found {
		return zero, errors.ErrNotFound
	}
	return v.(ttlCacheRecord[T]).Data, nil
}

func (c *TTLCache[K, T]) Put(key K, value T) error {
	record := ttlCacheRecord[T] {
		TTL: c.TTL,
		LastUpdatedTime: time.Now(),
		Data: value,
	}
	c.cache.Store(key, record)
	return nil
}

func (c *TTLCache[K, T]) Del(key K) (T, error) {
	var zero T
	value, found := c.cache.LoadAndDelete(key)
	if !found {
		return zero, nil
	}
	return value.(ttlCacheRecord[T]).Data, nil
}

func (c *TTLCache[K, T]) Clear() error {
	c.cache.Clear()
	return nil
}

func (c *TTLCache[K, T]) Close() {
	// if c.done == nil || channels.IsDone(c.done) {
	// 	return
	// }
	close(c.done)
}
