module github.com/celestiaorg/rsmt2d

go 1.20

require (
	github.com/celestiaorg/celestia-app v1.0.0-rc7
	github.com/celestiaorg/merkletree v0.0.0-20230308153949-c33506a7aa26
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/klauspost/reedsolomon v1.11.8
	golang.org/x/sync v0.3.0
)

require (
	github.com/celestiaorg/nmt v0.17.0 // indirect
	github.com/petermattis/goid v0.0.0-20230518223814-80aa455d8761 // indirect
	github.com/sasha-s/go-deadlock v0.3.1 // indirect
	github.com/tendermint/tendermint v0.35.9 // indirect
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/minio/sha256-simd v1.0.1
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gitlab.com/NebulousLabs/errors v0.0.0-20200929122200-06c536cf6975 // indirect
	golang.org/x/sys v0.10.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace (
	github.com/gogo/protobuf => github.com/regen-network/protobuf v1.3.3-alpha.regen.1
	github.com/tendermint/tendermint => github.com/celestiaorg/celestia-core v1.23.0-tm-v0.34.28
)
