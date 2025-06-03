//! DataSquare implementation for managing 2D data structures

use crate::{Axis, Error, Result, TreeConstructorFn};
use std::sync::Mutex;

/// DataSquare stores all data for an original data square (ODS) or extended data square (EDS).
/// Data is duplicated in both row-major and column-major order to provide zero-allocation slices.
pub struct DataSquare {
    square_row: Vec<Vec<Vec<u8>>>,  // row-major
    square_col: Vec<Vec<Vec<u8>>>,  // col-major
    data_mutex: Mutex<()>,
    width: u32,
    share_size: u32,
    row_roots: Option<Vec<Vec<u8>>>,
    col_roots: Option<Vec<Vec<u8>>>,
    create_tree_fn: TreeConstructorFn,
}

impl DataSquare {
    /// Create a new DataSquare from flattened data
    pub fn new(
        data: Vec<Vec<u8>>,
        tree_creator: TreeConstructorFn,
        share_size: u32,
    ) -> Result<Self> {
        // Validate share sizes
        for share in &data {
            if !share.is_empty() && share.len() != share_size as usize {
                return Err(Error::UnevenChunks);
            }
        }
        
        let width = (data.len() as f64).sqrt() as u32;
        if (width * width) as usize != data.len() {
            return Err(Error::InvalidChunkSize);
        }
        
        // Initialize row-major storage
        let mut square_row = vec![vec![Vec::new(); width as usize]; width as usize];
        for (i, share) in data.iter().enumerate() {
            let row = i / width as usize;
            let col = i % width as usize;
            square_row[row][col] = share.clone();
        }
        
        // Initialize column-major storage
        let mut square_col = vec![vec![Vec::new(); width as usize]; width as usize];
        for col_idx in 0..width as usize {
            for row_idx in 0..width as usize {
                square_col[col_idx][row_idx] = square_row[row_idx][col_idx].clone();
            }
        }
        
        Ok(Self {
            square_row,
            square_col,
            data_mutex: Mutex::new(()),
            width,
            share_size,
            row_roots: None,
            col_roots: None,
            create_tree_fn: tree_creator,
        })
    }
    
    /// Get a row slice
    pub fn row(&self, row_idx: u32) -> Result<&Vec<Vec<u8>>> {
        if row_idx >= self.width {
            return Err(Error::InvalidChunkSize);
        }
        Ok(&self.square_row[row_idx as usize])
    }
    
    /// Get a column slice
    pub fn col(&self, col_idx: u32) -> Result<&Vec<Vec<u8>>> {
        if col_idx >= self.width {
            return Err(Error::InvalidChunkSize);
        }
        Ok(&self.square_col[col_idx as usize])
    }
    
    /// Set a cell in the data square
    pub fn set_cell(&mut self, row_idx: u32, col_idx: u32, data: Vec<u8>) -> Result<()> {
        if row_idx >= self.width || col_idx >= self.width {
            return Err(Error::InvalidChunkSize);
        }
        
        if !data.is_empty() && data.len() != self.share_size as usize {
            return Err(Error::UnevenChunks);
        }
        
        {
            let _guard = self.data_mutex.lock().unwrap();
            
            // Update both row-major and column-major storage
            self.square_row[row_idx as usize][col_idx as usize] = data.clone();
            self.square_col[col_idx as usize][row_idx as usize] = data;
        }
        
        // Reset cached roots
        self.reset_roots();
        
        Ok(())
    }
    
    /// Get a cell from the data square
    pub fn get_cell(&self, row_idx: u32, col_idx: u32) -> Result<&Vec<u8>> {
        if row_idx >= self.width || col_idx >= self.width {
            return Err(Error::InvalidChunkSize);
        }
        Ok(&self.square_row[row_idx as usize][col_idx as usize])
    }
    
    /// Get the width of the data square
    pub fn width(&self) -> u32 {
        self.width
    }
    
    /// Reset cached roots
    fn reset_roots(&mut self) {
        self.row_roots = None;
        self.col_roots = None;
    }
    
    /// Compute row roots
    pub fn row_roots(&mut self) -> Result<Vec<Vec<u8>>> {
        if let Some(ref roots) = self.row_roots {
            return Ok(roots.clone());
        }
        
        let mut roots = Vec::new();
        for row_idx in 0..self.width {
            let root = self.compute_row_root(row_idx)?;
            roots.push(root);
        }
        
        self.row_roots = Some(roots.clone());
        Ok(roots)
    }
    
    /// Compute column roots
    pub fn col_roots(&mut self) -> Result<Vec<Vec<u8>>> {
        if let Some(ref roots) = self.col_roots {
            return Ok(roots.clone());
        }
        
        let mut roots = Vec::new();
        for col_idx in 0..self.width {
            let root = self.compute_col_root(col_idx)?;
            roots.push(root);
        }
        
        self.col_roots = Some(roots.clone());
        Ok(roots)
    }
    
    /// Compute root for a specific row
    fn compute_row_root(&self, row_idx: u32) -> Result<Vec<u8>> {
        let mut tree = (self.create_tree_fn)(Axis::Row, row_idx);
        let row = self.row(row_idx)?;
        
        for share in row {
            if share.is_empty() {
                return Err(Error::IncompleteAxis { axis: Axis::Row });
            }
            tree.push(share)?;
        }
        
        tree.root().map_err(|e| Error::Tree(e.to_string()))
    }
    
    /// Compute root for a specific column
    fn compute_col_root(&self, col_idx: u32) -> Result<Vec<u8>> {
        let mut tree = (self.create_tree_fn)(Axis::Col, col_idx);
        let col = self.col(col_idx)?;
        
        for share in col {
            if share.is_empty() {
                return Err(Error::IncompleteAxis { axis: Axis::Col });
            }
            tree.push(share)?;
        }
        
        tree.root().map_err(|e| Error::Tree(e.to_string()))
    }
    
    /// Get flattened representation of the data square
    pub fn flattened(&self) -> Vec<Vec<u8>> {
        let mut result = Vec::new();
        for row in &self.square_row {
            for share in row {
                result.push(share.clone());
            }
        }
        result
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::tree::new_default_tree;

    #[test]
    fn test_data_square_creation() {
        let data = vec![
            vec![1, 2], vec![3, 4],
            vec![5, 6], vec![7, 8],
        ];
        let ds = DataSquare::new(data, new_default_tree, 2);
        assert!(ds.is_ok());
        
        let ds = ds.unwrap();
        assert_eq!(ds.width(), 2);
    }
    
    #[test]
    fn test_data_square_operations() {
        let data = vec![
            vec![1, 2], vec![3, 4],
            vec![5, 6], vec![7, 8],
        ];
        let mut ds = DataSquare::new(data, new_default_tree, 2).unwrap();
        
        // Test get_cell
        assert_eq!(ds.get_cell(0, 0).unwrap(), &vec![1, 2]);
        assert_eq!(ds.get_cell(1, 1).unwrap(), &vec![7, 8]);
        
        // Test set_cell
        assert!(ds.set_cell(0, 0, vec![9, 10]).is_ok());
        assert_eq!(ds.get_cell(0, 0).unwrap(), &vec![9, 10]);
        
        // Test invalid indices
        assert!(ds.get_cell(2, 0).is_err());
        assert!(ds.set_cell(0, 2, vec![1, 2]).is_err());
    }
}