package main

import(
    "fmt"
    "time"
    "crypto/rand"

    "github.com/musalbas/rsmt2d"
)

func main() {
    widths := []uint{1, 2, 4, 8, 16, 32, 64, 128}
    chunkSizes := []uint{256}
    repeats := 10

    fmt.Println("Square width\t Chunk size\t Average time to encode (s)")

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
            fmt.Println(width, "\t\t", chunkSize, "\t\t", mean(runs))
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
