package rsmt2d

import (
	"crypto/rand"
	"fmt"
	"testing"
)

var (
	encodedDataDump [][]byte
	decodedDataDump [][]byte
)

const benchmarkDivider = "-------------------------------"

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
	fmt.Println(benchmarkDivider)
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
	for _codecType := range codecs {
		data := mockDecodableData(128, _codecType)
		b.Run(
			fmt.Sprintf("Decoding 128 shares using %s", _codecType),
			func(b *testing.B) {
				for n := 0; n < b.N; n++ {
					encodedData, err := Decode(data, _codecType)
					if err != nil {
						b.Error(err)
					}
					encodedDataDump = encodedData
				}
			},
		)
	}
	fmt.Println(benchmarkDivider)
}

func mockDecodableData(count int, _codecType CodecType) [][]byte {
	randData := generateRandData(count)
	encoded, err := Encode(randData, _codecType)
	if err != nil {
		panic(err)
	}
	output := append(randData, encoded...)
	// remove every other share
	for i := range output {
		if i%2 != 0 {
			output[i] = []byte{}
		}
	}
	return output
}
