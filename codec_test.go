package rsmt2d

import (
	"fmt"
	"math/rand"
	"testing"
)

var (
	encodedDataDump [][]byte
	decodedDataDump [][]byte
)

func BenchmarkEncoding(b *testing.B) {
	// generate some fake data
	data := generateRandData(128)
	for codecName, codec := range codecs {
		b.Run(
			fmt.Sprintf("%s 128 shares", codecName),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					encodedData, err := codec.Encode(data)
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
	for codecName, codec := range codecs {
		data := generateMissingData(128, codec)
		b.Run(
			fmt.Sprintf("%s 128 shares", codecName),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					encodedData, err := codec.Decode(data)
					if err != nil {
						b.Error(err)
					}
					encodedDataDump = encodedData
				}
			},
		)
	}
}

func generateMissingData(count int, codec Codec) [][]byte {
	randData := generateRandData(count)
	encoded, err := codec.Encode(randData)
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
