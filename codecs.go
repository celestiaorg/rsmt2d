package rsmt2d

import "fmt"

const (
	Leopard = "Leopard"
	RSGF8   = "RSFG8"
)

type Codec interface {
	// Encode encodes original data, automatically extracting share size.
	// There must be no missing shares. Only returns parity shares.
	Encode(data [][]byte) ([][]byte, error)
	// Decode decodes sparse original + parity data, automatically extracting share size.
	// Missing shares must be nil. Returns original shares only.
	Decode(data [][]byte) ([][]byte, error)
	// maxChunks returns the max. number of chunks each code supports in a 2D square.
	maxChunks() int
	// name returns the name of the codec.
	name() string
}

// codecs is a global map used for keeping track of registered codecs for testing and JSON unmarshalling
var codecs = make(map[string]Codec)

func registerCodec(ct string, codec Codec) {
	if codecs[ct] != nil {
		panic(fmt.Sprintf("%v already registered", codec))
	}
	codecs[ct] = codec
}
