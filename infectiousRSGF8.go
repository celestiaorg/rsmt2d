package rsmt2d

import (
	"sync"

	"github.com/vivint/infectious"
)

var _ Codec = &rsGF8Codec{}

func init() {
	registerCodec(RSGF8, NewRSGF8Codec())
}

type rsGF8Codec struct {
	infectiousCache      map[int]*infectious.FEC
	infectiousCacheMutex sync.Mutex
}

// NewRSGF8Codec issues a new cached RSGF8Codec
func NewRSGF8Codec() *rsGF8Codec {
	return &rsGF8Codec{infectiousCache: make(map[int]*infectious.FEC)}
}

func (c *rsGF8Codec) Encode(data [][]byte) ([][]byte, error) {
	var fec *infectious.FEC
	var err error

	// Set up caches.
	c.infectiousCacheMutex.Lock()
	if value, ok := c.infectiousCache[len(data)]; ok {
		fec = value
	} else {
		fec, err = infectious.NewFEC(len(data), len(data)*2)
		if err != nil {
			return nil, err
		}

		c.infectiousCache[len(data)] = fec
	}
	c.infectiousCacheMutex.Unlock()

	shares := make([][]byte, len(data))
	output := func(s infectious.Share) {
		if s.Number >= len(data) {
			shareData := make([]byte, len(s.Data))
			copy(shareData, s.Data)
			shares[s.Number-len(data)] = shareData
		}
	}

	flattened := flattenChunks(data)
	err = fec.Encode(flattened, output)

	return shares, err
}

func (c *rsGF8Codec) Decode(data [][]byte) ([][]byte, error) {
	var fec *infectious.FEC
	var err error

	// Set up caches.
	if value, ok := c.infectiousCache[len(data)/2]; ok {
		fec = value
	} else {
		fec, err = infectious.NewFEC(len(data)/2, len(data))
		if err != nil {
			return nil, err
		}

		c.infectiousCache[len(data)/2] = fec
	}

	rebuiltShares := make([][]byte, len(data)/2)
	rebuiltSharesOutput := func(s infectious.Share) {
		rebuiltShares[s.Number] = s.DeepCopy().Data
	}

	shares := []infectious.Share{}
	for i, d := range data {
		if d != nil {
			shares = append(shares, infectious.Share{Number: i, Data: d})
		}
	}

	err = fec.Rebuild(shares, rebuiltSharesOutput)

	return rebuiltShares, err
}

// maxChunks returns the max. number of chunks each code supports in a 2D square.
func (c *rsGF8Codec) maxChunks() int {
	return 128 * 128
}

func (c *rsGF8Codec) name() string {
	return RSGF8
}
