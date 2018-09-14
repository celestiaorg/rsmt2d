package main

import(
    "hash"
    "fmt"
    "time"
    "crypto/rand"
    "crypto/sha256"

    "github.com/NebulousLabs/merkletree"
    "github.com/musalbas/rsmt2d"
)

func main() {
    widths := []uint{1, 2, 4, 8, 16, 32, 64, 128}
    chunkSizes := []uint{256}
    repeats := 10

    fmt.Println("Encoding square")
    fmt.Println("\tSquare width\t Chunk size\t Average time (s)")

    for _, width := range widths {
        for _, chunkSize := range chunkSizes {
            var runs []float64
            for i := 0; i < repeats; i++ {
                data := generateRandomSquare(width, chunkSize)
                start := time.Now()
                _, err := rsmt2d.ComputeExtendedDataSquare(data, rsmt2d.CodecRSGF8)
                runs = append(runs, time.Since(start).Seconds())
                if err != nil {
                    panic(err)
                }
            }
            fmt.Println("\t", width, "\t\t", chunkSize, "\t\t", mean(runs))
        }
    }

    fmt.Println("Generate/verify single sample response")
    fmt.Println("\tSquare width\t Average time (s)")

    for _, width := range widths {
        var runs []float64
        for i := 0; i < repeats; i++ {
            runs = append(runs, timeMerkleProofSimulation(width*width*2, sha256.New()))
        }
        fmt.Println("\t", width, "\t\t", mean(runs))
    }

    fmt.Println("Verify fraud proof")
    fmt.Println("\tSquare width\t Chunk size\t Average time (s)")

    for _, width := range widths {
        for _, chunkSize := range chunkSizes {
            var runs []float64
            for i := 0; i < repeats; i++ {
                data := generateRandomSquare(width, chunkSize)
                start := time.Now()
                rsmt2d.Encode(data[0:width], rsmt2d.CodecRSGF8)
                runs = append(runs, time.Since(start).Seconds())
            }
            fmt.Println("\t", width, "\t\t", chunkSize, "\t\t", mean(runs))
        }
    }
}

func generateRandomSquare(width uint, chunkSize uint) [][]byte {
    chunks := make([][]byte, width*width)

    for i := 0; i < len(chunks); i++ {
        chunks[i] = make([]byte, chunkSize)
        rand.Read(chunks[i])
    }

    return chunks
}

func mean(values []float64) float64 {
    sum := float64(0)
    for _, value := range values {
        sum += value
    }
    return sum/float64(len(values))
}

func timeMerkleProofSimulation(size uint, hasher hash.Hash) float64 {
    tree := merkletree.New(hasher)
	tree.SetIndex(0)
	tree.Push([]byte("another object"))
    start := time.Now()
	tree.Prove()
    return time.Since(start).Seconds()
}
