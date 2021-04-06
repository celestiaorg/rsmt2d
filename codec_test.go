package rsmt2d

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	encodedDataDump [][]byte
	decodedDataDump [][]byte
)

func TestCodec_String(t *testing.T) {
	for codec := range codecs {
		assert.NotEqual(t, "", codec.String())
	}
}

func BenchmarkEncoding(b *testing.B) {
	// generate some fake data
	data := generateRandData(128)
	for _codecType := range codecs {
		b.Run(
			fmt.Sprintf("Encoding 128 shares using %s", _codecType),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					encodedData, err := Encode(data, _codecType)
					if err != nil {
						b.Error(err)
					}
					encodedDataDump = encodedData
				}
			},
		)
	}
}

func generateRandData(count int) [][]byte {
	out := make([][]byte, count)
	for i := 0; i < count; i++ {
		randData := make([]byte, count)
		_, err := rand.Read(randData)
		if err != nil {
			panic(err)
		}
		out[i] = randData
	}
	return out
}

func BenchmarkDecoding(b *testing.B) {
	// generate some fake data
	for codecType := range codecs {
		data := generateMissingData(128, codecType)
		b.Run(
			fmt.Sprintf("Decoding 128 shares using %s", codecType),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					encodedData, err := Decode(data, codecType)
					if err != nil {
						b.Error(err)
					}
					encodedDataDump = encodedData
				}
			},
		)
	}
}

func generateMissingData(count int, codecType CodecType) [][]byte {
	randData := generateRandData(count)
	encoded, err := Encode(randData, codecType)
	if err != nil {
		panic(err)
	}
	output := append(randData, encoded...)

	// remove half of the shares randomly
	for i := 0; i < (count / 2); {
		ind := rand.Intn(count)
		if len(output[ind]) == 0 {
			continue
		}
		output[ind] = []byte{}
		i++
	}

	return output
}
