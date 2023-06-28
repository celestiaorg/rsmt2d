package rsmt2d

import (
	"errors"
	"fmt"
)

const (
	// Leopard is a codec that was originally implemented in the C++ library
	// https://github.com/catid/leopard. rsmt2d uses a Go port of the C++
	// library in https://github.com/klauspost/reedsolomon. The Leopard codec
	// uses 8-bit leopard for shards less than or equal to 256. The Leopard
	// codec uses 16-bit leopard for shards greater than 256.
	Leopard = "Leopard"
)

type Codec interface {
	// Encode encodes original data, automatically extracting share size.
	// There must be no missing shares. Only returns parity shares.
	Encode(data [][]byte) ([][]byte, error)
	// Decode attempts to reconstruct the missing shards in data. The data
	// parameter should contain all original + parity shards where missing
	// shards should be `nil`. If reconstruction is successful, the original +
	// parity shards are returned. Returns ErrTooFewShards if not enough non-nil
	// shards exist in data to reconstruct the missing shards.
	Decode(data [][]byte) ([][]byte, error)
	// MaxChunks returns the max. number of chunks each code supports in a 2D square.
	MaxChunks() int
	// Name returns the name of the codec.
	Name() string
}

// codecs is a global map used for keeping track of registered codecs for testing and JSON unmarshalling
var codecs = make(map[string]Codec)

func registerCodec(ct string, codec Codec) {
	if codecs[ct] != nil {
		panic(fmt.Sprintf("%v already registered", codec))
	}
	codecs[ct] = codec
}

// ErrTooFewShards is returned by Decode if too few shards exist in the data to
// reconstruct the `nil` shards.
var ErrTooFewShards = errors.New("too few shards given to reconstruct all the shards in data")
