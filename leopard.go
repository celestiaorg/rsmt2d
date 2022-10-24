package rsmt2d

import (
	"sync"

	"github.com/klauspost/reedsolomon"
)

var _ Codec = &leoRSCodec{}

func init() {
	registerCodec(Leopard, NewLeoRSCodec())
}

type leoRSCodec struct {
	// Cache the encoders of various sizes to not have to re-instantiate those
	// as it is costly.
	//
	// Note that past sizes are not removed from the cache at all as the various
	// data sizes are expected to relatively small and will not cause any memory issue.
	//
	// TODO: switch to a generic version of sync.Map with type reedsolomon.Encoder
	// once it made it into the standard lib
	encCache sync.Map
}

func (l *leoRSCodec) Encode(data [][]byte) ([][]byte, error) {
	dataLen := len(data)
	enc, err := l.loadOrInitEncoder(dataLen)
	if err != nil {
		return nil, err
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

func (l *leoRSCodec) Decode(data [][]byte) ([][]byte, error) {
	half := len(data) / 2
	enc, err := l.loadOrInitEncoder(half)
	if err != nil {
		return nil, err
	}
	err = enc.Reconstruct(data)
	return data, err
}

func (l *leoRSCodec) loadOrInitEncoder(dataLen int) (reedsolomon.Encoder, error) {
	enc, ok := l.encCache.Load(dataLen)
	if !ok {
		var err error
		enc, err = reedsolomon.New(dataLen, dataLen, reedsolomon.WithLeopardGF(true))
		if err != nil {
			return nil, err
		}
		l.encCache.Store(dataLen, enc)
	}
	return enc.(reedsolomon.Encoder), nil

}

func (l *leoRSCodec) maxChunks() int {
	return 32768 * 32768
}

func NewLeoRSCodec() *leoRSCodec {
	return &leoRSCodec{}
}
