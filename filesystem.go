package main

type FileSystem interface {
	// Create returns a command followed by its arguments, that will
	// create path as a directory.
	Create(path string) []string

	// Clone returns a command followed by its arguments, that will
	// clone src to dst.
	Clone(src, dst string) []string
}
