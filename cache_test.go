package rsmt2d

import (
	"bytes"
	"math/rand"
	"testing"
	"testing/quick"
)

func TestCacheFuzz(t *testing.T) {
	cache := newDoubleCache[[]byte](
		defaultDoubleCacheOptions(),
	)

	f := func(index int, data []byte) bool {
		cache.insert(index, data)

		value, ok := cache.query(index)
		if !ok {
			return false
		}
		if !bytes.Equal(value, data) {
			return false
		}
		return true
	}

	if err := quick.Check(f, nil); err != nil {
		t.Error("failed to retreive stored value", err)
	}
}

func TestCacheNonExist(t *testing.T) {
	cache := newDoubleCache[[]byte](
		defaultDoubleCacheOptions(),
	)
	for i := 0; i < 10; i++ {
		data := make([]byte, 4)
		rand.Read(data)
		cache.insert(i, data)
	}
	for i := 10; i < 20; i++ {
		if _, ok := cache.query(i); ok {
			t.Error("able to query non existent data")
		}
	}
}

func TestCacheTooSmall(t *testing.T) {
	cache := newDoubleCache[[]byte](
		defaultDoubleCacheOptions().withCapacity(19),
	)
	index := 1
	data := [10]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	cache.insert(index, data[:])

	_, ok := cache.query(index)
	if ok {
		t.Error("able to query data bigger than half cache")
	}
}
