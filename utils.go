package rsmt2d

func flattenChunks(chunks [][]byte) []byte {
	length := 0
	for _, chunk := range chunks {
		length += len(chunk)
	}

	flattened := make([]byte, 0, length)
	for _, chunk := range chunks {
		flattened = append(flattened, chunk...)
	}

	return flattened
}
