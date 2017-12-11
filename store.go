package cache

import (
	"errors"
	"sync"

	"github.com/FeiniuBus/cache/singleflight"
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
		loadStore:  &singleflight.Store{},
	}
	stores[name] = s
	return s
}

// A Store is a cache store
type Store struct {
	name       string
	getter     Getter
	cacheBytes int64
	cache      cache
	loadStore  flightStore
	_          int32
	Stats      Stats
}

type flightStore interface {
	Do(key string, fn func() (interface{}, error)) (interface{}, error)
}

// Stats are store statistics.
type Stats struct {
	Gets          AtomicInt
	CacheHits     AtomicInt
	Loads         AtomicInt
	LoadsDeduped  AtomicInt
	LocalLoadErrs AtomicInt
	LocalLoads    AtomicInt
}

// Name returns the name of the store.
func (s *Store) Name() string {
	return s.name
}

// Get is
func (s *Store) Get(key string, dest Sink) error {
	s.Stats.Gets.Add(1)
	if dest == nil {
		return errors.New("store: nil dest Sink")
	}
	value, cacheHit := s.lookupCache(key)

	if cacheHit {
		s.Stats.CacheHits.Add(1)
		return setSinkView(dest, value)
	}

	destPopulated := false
	value, destPopulated, err := s.load(key, dest)
	if err != nil {
		return err
	}
	if destPopulated {
		return nil
	}
	return setSinkView(dest, value)
}

// load loads key by invoking the getter locally
func (s *Store) load(key string, dest Sink) (value ByteView, destPopulated bool, err error) {
	s.Stats.Loads.Add(1)
	viewi, err := s.loadStore.Do(key, func() (interface{}, error) {
		if value, cacheHit := s.lookupCache(key); cacheHit {
			s.Stats.CacheHits.Add(1)
			return value, nil
		}
		s.Stats.LoadsDeduped.Add(1)
		var value ByteView
		var err error
		value, err = s.getLocally(key, dest)
		if err != nil {
			s.Stats.LocalLoadErrs.Add(1)
			return nil, err
		}
		s.Stats.LocalLoads.Add(1)
		destPopulated = true
		s.populateCache(key, value)
		return value, nil
	})
	if err == nil {
		value = viewi.(ByteView)
	}
	return
}

func (s *Store) getLocally(key string, dest Sink) (ByteView, error) {
	err := s.getter.Get(key, dest)
	if err != nil {
		return ByteView{}, err
	}
	return dest.view()
}

func (s *Store) lookupCache(key string) (value ByteView, ok bool) {
	if s.cacheBytes <= 0 {
		return
	}
	value, ok = s.cache.get(key)
	return
}

func (s *Store) populateCache(key string, value ByteView) {
	if s.cacheBytes <= 0 {
		return
	}
	s.cache.add(key, value)

	for {
		cacheBytes := s.cache.bytes()
		if cacheBytes <= s.cacheBytes {
			return
		}

		victim := &s.cache
		victim.removeOldest()
	}
}
