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

var infectiousCache map[int]*infectious.FEC

func init() {
    infectiousCache = make(map[int]*infectious.FEC)
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
    var fec *infectious.FEC
    var err error
    if value, ok := infectiousCache[len(data)]; ok {
        fec = value
    } else {
        fec, err = infectious.NewFEC(len(data), len(data) * 2)
        if err != nil {
            return nil, err
        }

        infectiousCache[len(data)] = fec
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
    var fec *infectious.FEC
    var err error
    if value, ok := infectiousCache[len(data) / 2]; ok {
        fec = value
    } else {
        fec, err = infectious.NewFEC(len(data) / 2, len(data))
        if err != nil {
            return nil, err
        }

        infectiousCache[len(data) / 2] = fec
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
