package cache

import (
	"sync"
)

// A Getter loads data for a key.
type Getter interface {
	Get(key string, dest Sink) error
}

// A GetterFunc implements Getter with a function.
type GetterFunc func(key string, dest Sink) error

func (f GetterFunc) Get(key string, dest Sink) error {
	return f(key, dest)
}

var (
	mu     sync.RWMutex
	stores = make(map[string]*Store)
)

// GetStore returns the named store previously created with NewStore, or
// nil if there's no such store.
func GetStore(name string) *Store {
	mu.RLock()
	s := stores[name]
	mu.RUnlock()
	return s
}

// NewStore creates a new store
func NewStore(name string, cacheBytes int64, getter Getter) *Store {
	if getter == nil {
		panic("nil Getter")
	}
	mu.Lock()
	defer mu.Unlock()

	if _, dup := stores[name]; dup {
		panic("duplicate registration of store " + name)
	}

	s := &Store{
		name:       name,
		getter:     getter,
		cacheBytes: cacheBytes,
	}
	stores[name] = s
	return s
}

// A Store is a cache store
type Store struct {
	name       string
	getter     Getter
	cacheBytes int64
}

// Name returns the name of the store.
func (s *Store) Name() string {
	return s.name
}

// Get is
func (s *Store) Get(key string, dest Sink) error {
	return nil
}

func (s *Store) lookupCache(key string) (value []byte, ok bool) {
	if s.cacheBytes <= 0 {
		return
	}
	return nil, false
}
