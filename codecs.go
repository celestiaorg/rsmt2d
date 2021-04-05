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

func (c CodecType) String() string {
	switch c {
	case RSGF8:
		return "RSGF8"
	case LeopardFF8:
		return "LeopardFF8"
	case LeopardFF16:
		return "LeopardFF16"
	default:
		return "UNSUPPORTED CODEC TYPE"
	}
}

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

func Encode(data [][]byte, codec CodecType) ([][]byte, error) {
	if codec, ok := codecs[codec]; !ok {
		return nil, errors.New("invalid codec")
	} else {
		return codec.encode(data)
	}
}

func Decode(data [][]byte, codec CodecType) ([][]byte, error) {
	if codec, ok := codecs[codec]; !ok {
		return nil, errors.New("invalid codec")
	} else {
		return codec.decode(data)
	}
}
