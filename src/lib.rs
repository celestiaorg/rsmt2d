//! # rsmt2d
//!
//! Rust implementation of two dimensional Reed-Solomon Merkle tree data availability scheme.
//! 
//! This is a Rust port of the Go rsmt2d library that implements the data availability
//! scheme described in https://arxiv.org/abs/1809.09044.

pub mod codec;
pub mod data_square;
pub mod extended_data_square;
pub mod tree;

pub use codec::Codec;
pub use data_square::DataSquare;
pub use extended_data_square::ExtendedDataSquare;
pub use tree::{Tree, TreeConstructorFn};

/// Represents an axis in the data square (Row or Column)
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Axis {
    Row,
    Col,
}

/// Error types for rsmt2d operations
#[derive(Debug, thiserror::Error)]
pub enum Error {
    #[error("uneven chunks: non-nil shares not all of equal size")]
    UnevenChunks,
    #[error("unrepairable data square")]
    UnrepairableDataSquare,
    #[error("byzantine data at {axis:?} {index}: {shares:?}")]
    ByzantineData {
        axis: Axis,
        index: u32,
        shares: Option<Vec<Vec<u8>>>,
    },
    #[error("number of chunks exceeds the maximum")]
    TooManyChunks,
    #[error("invalid chunk size")]
    InvalidChunkSize,
    #[error("filler chunk size does not match data square chunk size")]
    FillerChunkSizeMismatch,
    #[error("cannot compute root of incomplete {axis:?}")]
    IncompleteAxis { axis: Axis },
    #[error("codec error: {0}")]
    Codec(String),
    #[error("tree error: {0}")]
    Tree(String),
}

pub type Result<T> = std::result::Result<T, Error>;

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_axis() {
        let row = Axis::Row;
        let col = Axis::Col;
        assert_ne!(row, col);
        assert_eq!(row, Axis::Row);
        assert_eq!(col, Axis::Col);
    }
}