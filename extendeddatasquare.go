// A two dimensional Reed-Solomon merkle tree data availability scheme.
package rsmt2d

import (
    "github.com/vivint/infectious"
)

// Represents an extended piece of data.
type ExtendedData struct {
    ChunkSize uint // the size of each chunk in the original data in bytes
    fec *infectious.FEC
}

// Loads original data as extended data.
func (ed *ExtendedData) LoadData() error {
    fec, err := infectious.NewFEC(int(ed.ChunkSize), int(ed.ChunkSize*2))
    if err != nil {
        return err
    }
    ed.fec = fec

    return nil
}
