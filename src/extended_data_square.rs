//! ExtendedDataSquare implementation for erasure-coded data squares

use crate::{Codec, DataSquare, Error, Result, TreeConstructorFn};

/// ExtendedDataSquare represents an extended piece of data with Reed-Solomon encoding
pub struct ExtendedDataSquare {
    data_square: DataSquare,
    codec: Box<dyn Codec>,
    original_data_width: u32,
}

impl ExtendedDataSquare {
    /// Create a new ExtendedDataSquare by computing Reed-Solomon extension
    pub fn compute(
        data: Vec<Vec<u8>>,
        codec: Box<dyn Codec>,
        tree_creator_fn: TreeConstructorFn,
    ) -> Result<Self> {
        if data.len() > codec.max_chunks() {
            return Err(Error::TooManyChunks);
        }
        
        let share_size = get_share_size(&data);
        codec.validate_chunk_size(share_size)?;
        
        let ds = DataSquare::new(data, tree_creator_fn, share_size as u32)?;
        
        // TODO: Implement erasure extension of the square
        // This will mirror the functionality from extendeddatasquare.go
        
        let original_data_width = ds.width();
        
        Ok(Self {
            data_square: ds,
            codec,
            original_data_width,
        })
    }
    
    /// Import an existing extended data square
    pub fn import(
        data: Vec<Vec<u8>>,
        codec: Box<dyn Codec>,
        tree_creator_fn: TreeConstructorFn,
    ) -> Result<Self> {
        if data.len() > 4 * codec.max_chunks() {
            return Err(Error::TooManyChunks);
        }
        
        let share_size = get_share_size(&data);
        codec.validate_chunk_size(share_size)?;
        
        let ds = DataSquare::new(data, tree_creator_fn, share_size as u32)?;
        
        validate_eds_width(ds.width())?;
        
        let original_data_width = ds.width() / 2;
        
        Ok(Self {
            data_square: ds,
            codec,
            original_data_width,
        })
    }
    
    /// Get row roots
    pub fn row_roots(&mut self) -> Result<Vec<Vec<u8>>> {
        self.data_square.row_roots()
    }
    
    /// Get column roots
    pub fn col_roots(&mut self) -> Result<Vec<Vec<u8>>> {
        self.data_square.col_roots()
    }
    
    /// Get flattened representation
    pub fn flattened(&self) -> Vec<Vec<u8>> {
        self.data_square.flattened()
    }
    
    /// Get the width of the extended data square
    pub fn width(&self) -> u32 {
        self.data_square.width()
    }
    
    /// Get the original data width (before extension)
    pub fn original_data_width(&self) -> u32 {
        self.original_data_width
    }
    
    /// Repair the data square using provided roots
    pub fn repair(
        &mut self,
        row_roots: &[Vec<u8>],
        col_roots: &[Vec<u8>],
    ) -> Result<()> {
        // TODO: Implement crossword repair algorithm
        // This will mirror the functionality from extendeddatacrossword.go
        
        // Pre-repair sanity check
        self.pre_repair_sanity_check(row_roots, col_roots)?;
        
        // Crossword repair implementation would go here
        // For now, return an error as not implemented
        Err(Error::Codec("repair not yet implemented".to_string()))
    }
    
    /// Sanity check before repair
    fn pre_repair_sanity_check(
        &mut self,
        row_roots: &[Vec<u8>],
        col_roots: &[Vec<u8>],
    ) -> Result<()> {
        if row_roots.len() != self.width() as usize || col_roots.len() != self.width() as usize {
            return Err(Error::InvalidChunkSize);
        }
        
        // TODO: Add more sanity checks mirroring extendeddatacrossword.go
        Ok(())
    }
    
    /// Set a cell in the data square
    pub fn set_cell(&mut self, row_idx: u32, col_idx: u32, data: Vec<u8>) -> Result<()> {
        self.data_square.set_cell(row_idx, col_idx, data)
    }
    
    /// Get a cell from the data square
    pub fn get_cell(&self, row_idx: u32, col_idx: u32) -> Result<&Vec<u8>> {
        self.data_square.get_cell(row_idx, col_idx)
    }
}

/// Determine the share size from data
fn get_share_size(data: &[Vec<u8>]) -> usize {
    for share in data {
        if !share.is_empty() {
            return share.len();
        }
    }
    0
}

/// Validate extended data square width
fn validate_eds_width(width: u32) -> Result<()> {
    if width == 0 || width % 2 != 0 {
        return Err(Error::InvalidChunkSize);
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::{codec::LeoRSCodec, tree::new_default_tree};

    #[test]
    fn test_get_share_size() {
        let data = vec![
            vec![], vec![1, 2, 3], vec![4, 5, 6]
        ];
        assert_eq!(get_share_size(&data), 3);
        
        let empty_data = vec![vec![], vec![]];
        assert_eq!(get_share_size(&empty_data), 0);
    }
    
    #[test]
    fn test_validate_eds_width() {
        assert!(validate_eds_width(0).is_err());
        assert!(validate_eds_width(1).is_err());
        assert!(validate_eds_width(3).is_err());
        assert!(validate_eds_width(2).is_ok());
        assert!(validate_eds_width(4).is_ok());
    }
    
    #[test]
    fn test_extended_data_square_import() {
        let data = vec![
            vec![1, 2], vec![3, 4],
            vec![5, 6], vec![7, 8],
        ];
        let codec = Box::new(LeoRSCodec::new());
        let eds = ExtendedDataSquare::import(data, codec, new_default_tree);
        
        assert!(eds.is_ok());
        let eds = eds.unwrap();
        assert_eq!(eds.width(), 2);
        assert_eq!(eds.original_data_width(), 1);
    }
}