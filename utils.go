package rsmt2d

func flattenChunks(chunks [][]byte) []byte {
	flattened := chunks[0]
	for _, chunk := range chunks[1:] {
		flattened = append(flattened, chunk...)
	}

	return flattened
}
