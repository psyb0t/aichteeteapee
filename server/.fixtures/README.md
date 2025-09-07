# Test Fixtures

This directory contains static test files used by both file upload tests and server tests.

## File Upload Test Fixtures
- `test-file.txt` - Standard text file with multi-line content
- `empty-file.txt` - Empty file for edge case testing  
- `large-file.txt` - Larger file for performance testing
- `binary-file.dat` - File with binary-like content
- `special-chars-filename.txt` - File with special characters in filename

## Server Cache Test Fixtures
- `static-test.txt` - Basic static file for cache testing baseline
- Note: Cache tests may create temporary files dynamically as needed for testing file system changes

## Usage
These fixtures are checked into the repository and should not be modified by tests.
For dynamic testing needs, create temporary files outside this directory.