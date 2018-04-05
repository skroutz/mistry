package filesystem

import (
	"fmt"
)

var List = make(map[string]FileSystem)

type FileSystem interface {
	// Create returns a command followed by its arguments, that will
	// create path as a directory.
	Create(path string) []string

	// Clone returns a command followed by its arguments, that will
	// clone src to dst.
	Clone(src, dst string) []string

	// Remove removes path and its children.
	Remove(path string) error
}

// Get returns the registered filesystem denoted by s. If it doesn't exist,
// an error is returned.
func Get(s string) (FileSystem, error) {
	fs, ok := List[s]
	if !ok {
		return nil, fmt.Errorf("unknown filesystem '%s' (%v)", s, List)
	}
	return fs, nil
}
