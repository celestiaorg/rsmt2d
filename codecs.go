package rsmt2d

import (
	"errors"
	"fmt"
)

// codecType type
type CodecType int

// Erasure codes enum:
const (
	// RSGF8 represents Reed-Solomon codecType with an 8-bit Finite Galois Field (2^8)
	RSGF8       CodecType = iota
	LeopardFF8  CodecType = 1
	LeopardFF16 CodecType = 2
)

type Codec interface {
	encode(data [][]byte) ([][]byte, error)
	decode(data [][]byte) ([][]byte, error)
	codecType() CodecType
	// maxChunks returns the max. number of chunks each code supports in a 2D square.
	maxChunks() int
}

var codecs = make(map[CodecType]Codec)

func registerCodec(ct CodecType, codec Codec) {
	if codecs[ct] != nil {
		panic(fmt.Sprintf("%v already registered", codec))
	}
	codecs[ct] = codec
}

func encode(data [][]byte, codec CodecType) ([][]byte, error) {
	if codec, ok := codecs[codec]; !ok {
		return nil, errors.New("invalid codec")
	} else {
		return codec.encode(data)
	}
}

func decode(data [][]byte, codec CodecType) ([][]byte, error) {
	if codec, ok := codecs[codec]; !ok {
		return nil, errors.New("invalid codec")
	} else {
		return codec.decode(data)
	}
}
