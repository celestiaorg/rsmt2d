package rsmt2d

import (
	"github.com/celestiaorg/rsmt2d/utils"
	"github.com/klauspost/reedsolomon"
)

var _ Codec = leoRSFF8Codec{}
var _ Codec = leoRSFF16Codec{}

var encoderCache = utils.NewDoubleCache[reedsolomon.Encoder](utils.DefaultDoubleCacheOptions())

func init() {
	registerCodec(LeopardFF8, newLeoRSFF8Codec())
	registerCodec(LeopardFF16, newLeoRSFF16Codec())
}

type leoRSFF8Codec struct{}

func (l leoRSFF8Codec) Encode(data [][]byte) ([][]byte, error) {
	return encode(data)
}

func encode(data [][]byte) ([][]byte, error) {
	dataLen := len(data)
	enc, ok := encoderCache.Query(dataLen)
	if !ok {
		var err error
		enc, err = reedsolomon.New(dataLen, dataLen, reedsolomon.WithLeopardGF(true))
		if err != nil {
			return nil, err
		}
		encoderCache.Insert(dataLen, enc)
	}

	shards := make([][]byte, dataLen*2)
	copy(shards, data)
	for i := dataLen; i < len(shards); i++ {
		shards[i] = make([]byte, len(data[0]))
	}
	if err := enc.Encode(shards); err != nil {
		return nil, err
	}
	return shards[dataLen:], nil
}

func (l leoRSFF8Codec) Decode(data [][]byte) ([][]byte, error) {
	return decode(data)
}

func decode(data [][]byte) ([][]byte, error) {
	half := len(data) / 2
	enc, ok := encoderCache.Query(half)
	var err error
	if !ok {
		enc, err = reedsolomon.New(half, half, reedsolomon.WithLeopardGF(true))
		if err != nil {
			return nil, err
		}
		encoderCache.Insert(half, enc)
	}
	err = enc.Reconstruct(data)
	return data, err
}

func (l leoRSFF8Codec) maxChunks() int {
	return 128 * 128
}

func newLeoRSFF8Codec() leoRSFF8Codec {
	return leoRSFF8Codec{}
}

type leoRSFF16Codec struct{}

func (leo leoRSFF16Codec) Encode(data [][]byte) ([][]byte, error) {
	return encode(data)
}

func (leo leoRSFF16Codec) Decode(data [][]byte) ([][]byte, error) {
	return decode(data)
}

func (leo leoRSFF16Codec) maxChunks() int {
	return 32768 * 32768
}

func newLeoRSFF16Codec() leoRSFF16Codec {
	return leoRSFF16Codec{}
}
