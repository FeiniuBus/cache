package singleflight

import (
	"sync"
)

// call is an in-flight or completed Do call
type call struct {
	wg  sync.WaitGroup
	val interface{}
	err error
}

// Store represents a class of work and forms a namespace in which
// units of work can be executed with duplicate suppression.
type Store struct {
	mu sync.Mutex
	m  map[string]*call
}

// Do executes and returns the results of the given function.
func (s *Store) Do(key string, fn func() (interface{}, error)) (interface{}, error) {
	s.mu.Lock()
	if s.m == nil {
		s.m = make(map[string]*call)
	}
	if c, ok := s.m[key]; ok {
		s.mu.Unlock()
		c.wg.Wait()
		return c.val, c.err
	}
	c := new(call)
	c.wg.Add(1)
	s.m[key] = c
	s.mu.Unlock()

	c.val, c.err = fn()
	c.wg.Done()

	s.mu.Lock()
	delete(s.m, key)
	s.mu.Unlock()

	return c.val, c.err
}
