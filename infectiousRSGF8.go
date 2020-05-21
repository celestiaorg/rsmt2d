package rsmt2d

import (
	"github.com/vivint/infectious"
)

var _ Codec = &rsGF8Codec{}

func init() {
	registerCodec(RSGF8, newRSGF8Codec())
}

type rsGF8Codec struct {
	infectiousCache map[int]*infectious.FEC
}

func newRSGF8Codec() *rsGF8Codec {
	return &rsGF8Codec{make(map[int]*infectious.FEC)}
}

func (c *rsGF8Codec) encode(data [][]byte) ([][]byte, error) {
	var fec *infectious.FEC
	var err error
	if value, ok := c.infectiousCache[len(data)]; ok {
		fec = value
	} else {
		fec, err = infectious.NewFEC(len(data), len(data)*2)
		if err != nil {
			return nil, err
		}

		c.infectiousCache[len(data)] = fec
	}

	shares := make([][]byte, len(data))
	output := func(s infectious.Share) {
		if s.Number >= len(data) {
			shareData := make([]byte, len(data[0]))
			copy(shareData, s.Data)
			shares[s.Number-len(data)] = shareData
		}
	}

	flattened := flattenChunks(data)
	err = fec.Encode(flattened, output)

	return shares, err
}
func (c *rsGF8Codec) decode(data [][]byte) ([][]byte, error) {
	var fec *infectious.FEC
	var err error
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
	for j := 0; j < len(data); j++ {
		if data[j] != nil {
			shares = append(shares, infectious.Share{Number: j, Data: data[j]})
		}
	}
	err = fec.Rebuild(shares, rebuiltSharesOutput)

	return rebuiltShares, err
}

func (c *rsGF8Codec) codecType() CodecType {
	return RSGF8
}

// maxChunks returns the max. number of chunks each code supports in a 2D square.
func (c *rsGF8Codec) maxChunks() int {
	return 128 * 128
}
