package main

type FileSystem interface {
	// Create creates path as a directory
	Create(path string) error

	// Clone clones src to dst
	Clone(src, dst string) error
}
