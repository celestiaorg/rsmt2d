package rsmt2d

import (
	"github.com/klauspost/reedsolomon"
)

var _ Codec = leoRSFF8Codec{}
var _ Codec = leoRSFF16Codec{}

func init() {
	registerCodec(LeopardFF8, newLeoRSFF8Codec())
	registerCodec(LeopardFF16, newLeoRSFF16Codec())
}

type leoRSFF8Codec struct{}

func (l leoRSFF8Codec) Encode(data [][]byte) ([][]byte, error) {
	return encode(data)
}

func encode(data [][]byte) ([][]byte, error) {
	enc, err := reedsolomon.New(len(data), len(data))
	if err != nil {
		return nil, err
	}
	shards := make([][]byte, len(data)*2)
	copy(shards, data)
	for i := len(data); i < len(shards); i++ {
		shards[i] = make([]byte, len(data[0]))
	}
	if err := enc.Encode(shards); err != nil {
		return nil, err
	}
	return shards[len(data):], nil
}

func (l leoRSFF8Codec) Decode(data [][]byte) ([][]byte, error) {
	return decode(data)
}

func decode(data [][]byte) ([][]byte, error) {
	half := len(data) / 2
	enc, err := reedsolomon.New(half, half)
	if err != nil {
		return nil, err
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
