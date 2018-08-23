package rsmt2d

import(
    "errors"

    "github.com/vivint/infectious"
)

// Erasure codes.
const CodecRSGF8 = 0 // Reed-Solomon, Galois Field 2^8

// Max number of chunks each code supports in a 2D square.
var SupportedCodecs = map[int]int{
    CodecRSGF8: 128*128,
}

func encode(data [][]byte, codec int) ([][]byte, error) {
    switch codec {
    case CodecRSGF8:
        result, err := encode_RSGF8(data)
        return result, err
    default:
        return nil, errors.New("invalid codec")
    }
}

func encode_RSGF8(data [][]byte) ([][]byte, error) {
    flattened := flattenChunks(data)

    fec, err := infectious.NewFEC(len(data), len(data) * 2)
    if err != nil {
        return nil, err
    }

    shares := make([][]byte, len(data))
    output := func(s infectious.Share) {
        if s.Number >= len(data) {
            shareData := make([]byte, len(data[0]))
            copy(shareData, s.Data)
            shares[s.Number-len(data)] = shareData
        }
    }

    err = fec.Encode(flattened, output)

    return shares, err
}

func decode(data [][]byte, codec int) ([][]byte, error) {
    switch codec {
    case CodecRSGF8:
        result, err := decode_RSGF8(data)
        return result, err
    default:
        return nil, errors.New("invalid codec")
    }
}

func decode_RSGF8(data [][]byte) ([][]byte, error) {
    fec, err := infectious.NewFEC(len(data) / 2, len(data))
    if err != nil {
        return nil, err
    }

    rebuiltShares := make([][]byte, len(data) / 2)
    rebuiltSharesOutput := func(s infectious.Share) {
        rebuiltShares[s.Number] = s.DeepCopy().Data
    }

    shares := []infectious.Share{}
    for j := 0; j < len(data); j++ {
        if data[j] != nil {
            shares = append(shares, infectious.Share{j, data[j]})
        }
    }

    err = fec.Rebuild(shares, rebuiltSharesOutput)

    return rebuiltShares, err
}
