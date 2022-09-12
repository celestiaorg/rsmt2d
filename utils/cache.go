package utils

import (
	"reflect"
	"sync"
)

type Cache[T any] interface {
	Insert(int, T)
	Query(int) (T, bool)
}

var (
	DefaultDoubleCacheCapacity = 1024 * 1024
)

type DoubleCacheOptions struct {
	Capacity int
}

func DefaultDoubleCacheOptions() DoubleCacheOptions {
	return DoubleCacheOptions{
		Capacity: DefaultDoubleCacheCapacity,
	}
}

func (opts DoubleCacheOptions) WithCapacity(capacity int) DoubleCacheOptions {
	opts.Capacity = capacity / 2
	return opts
}

type DoubleCache[T any] struct {
	opts           DoubleCacheOptions
	cacheMu        *sync.Mutex
	cacheFrontSize int
	cacheFront     map[int]T
	cacheBack      map[int]T
}

func NewDoubleCache[T any](opts DoubleCacheOptions) *DoubleCache[T] {
	return &DoubleCache[T]{
		opts:           opts,
		cacheMu:        new(sync.Mutex),
		cacheFrontSize: 0,
		cacheFront:     make(map[int]T, 0),
		cacheBack:      make(map[int]T, 0),
	}
}

func (r *DoubleCache[T]) Insert(id int, value T) {
	r.cacheMu.Lock()
	defer r.cacheMu.Unlock()

	if r.cacheFrontSize+1 > r.opts.Capacity {
		r.cacheBack = r.cacheFront
		r.cacheFrontSize = 0
		r.cacheFront = make(map[int]T, 0)
	}

	r.cacheFrontSize += int(reflect.TypeOf(value).Size())
	r.cacheFront[id] = value
}

func (r *DoubleCache[T]) Query(id int) (T, bool) {
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
