package utils_test

import (
	"bytes"
	"math/rand"
	"testing"
	"testing/quick"

	"github.com/celestiaorg/rsmt2d/utils"
)

func TestCacheFuzz(t *testing.T) {
	cache := utils.NewDoubleCache[[]byte](
		utils.DefaultDoubleCacheOptions(),
	)

	f := func(index int, data []byte) bool {
		cache.Insert(index, data)

		value, ok := cache.Query(index)
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
	cache := utils.NewDoubleCache[[]byte](
		utils.DefaultDoubleCacheOptions(),
	)
	for i := 0; i < 10; i++ {
		data := make([]byte, 4)
		rand.Read(data)
		cache.Insert(i, data)
	}
	for i := 10; i < 20; i++ {
		if _, ok := cache.Query(i); ok {
			t.Error("able to query non existent data")
		}
	}
}

func TestCacheTooSmall(t *testing.T) {
	cache := utils.NewDoubleCache[[]byte](
		utils.DefaultDoubleCacheOptions().WithCapacity(19),
	)
	index := 1
	data := [10]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09}
	cache.Insert(index, data[:])

	_, ok := cache.Query(index)
	if ok {
		t.Error("able to query data bigger than half cache")
	}
}
