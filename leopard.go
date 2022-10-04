package rsmt2d

import (
	"github.com/klauspost/reedsolomon"
)

var _ Codec = leoRSCodec{}

var encoderCache = newDoubleCache[reedsolomon.Encoder](defaultDoubleCacheOptions())

func init() {
	registerCodec(Leopard, newLeoRSCodec())
}

type leoRSCodec struct{}

func (l leoRSCodec) Encode(data [][]byte) ([][]byte, error) {
	dataLen := len(data)
	enc, ok := encoderCache.query(dataLen)
	if !ok {
		var err error
		enc, err = reedsolomon.New(dataLen, dataLen, reedsolomon.WithLeopardGF(true))
		if err != nil {
			return nil, err
		}
		encoderCache.insert(dataLen, enc)
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

func (l leoRSCodec) Decode(data [][]byte) ([][]byte, error) {
	half := len(data) / 2
	enc, ok := encoderCache.query(half)
	var err error
	if !ok {
		enc, err = reedsolomon.New(half, half, reedsolomon.WithLeopardGF(true))
		if err != nil {
			return nil, err
		}
		encoderCache.insert(half, enc)
	}
	err = enc.Reconstruct(data)
	return data, err
}

func (l leoRSCodec) maxChunks() int {
	return 32768 * 32768
}

func newLeoRSCodec() leoRSCodec {
	return leoRSCodec{}
}
