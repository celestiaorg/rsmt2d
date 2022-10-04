package rsmt2d

import (
	"reflect"
	"sync"
)

var (
	defaultDoubleCacheCapacity = 1024 * 1024
)

type doubleCacheOptions struct {
	capacity int
}

func defaultDoubleCacheOptions() doubleCacheOptions {
	return doubleCacheOptions{
		capacity: defaultDoubleCacheCapacity,
	}
}

func (opts doubleCacheOptions) withCapacity(capacity int) doubleCacheOptions {
	opts.capacity = capacity / 2
	return opts
}

type doubleCache[T any] struct {
	opts           doubleCacheOptions
	cacheMu        *sync.Mutex
	cacheFrontSize int
	cacheFront     map[int]T
	cacheBack      map[int]T
}

func newDoubleCache[T any](opts doubleCacheOptions) *doubleCache[T] {
	return &doubleCache[T]{
		opts:           opts,
		cacheMu:        new(sync.Mutex),
		cacheFrontSize: 0,
		cacheFront:     make(map[int]T, 0),
		cacheBack:      make(map[int]T, 0),
	}
}

func (r *doubleCache[T]) insert(id int, value T) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	size := int(reflect.TypeOf(value).Size())
	if size > r.opts.capacity {
		return
	}

	if r.cacheFrontSize+size > r.opts.capacity {
		r.cacheBack = r.cacheFront
		r.cacheFrontSize = 0
		r.cacheFront = make(map[int]T, 0)
	}

	r.cacheFrontSize += size
	r.cacheFront[id] = value
}

func (r *doubleCache[T]) query(id int) (T, bool) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	if value, ok := r.cacheFront[id]; ok {
		return value, ok
	}
	if value, ok := r.cacheBack[id]; ok {
		r.cacheFrontSize += int(reflect.TypeOf(value).Size())
		r.cacheFront[id] = value
		return value, ok
	}

	var value T
	return value, false
}
