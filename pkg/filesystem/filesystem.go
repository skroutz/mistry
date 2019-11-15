package filesystem

import (
	"fmt"
)

// Registry maps the filesystem name to its implementation
var Registry = make(map[string]FileSystem)

// FileSystem defines a few basic filesystem operations
type FileSystem interface {
	// Create creates a new directory in the given path.
	Create(path string) error

	// Clone copies the src path and its contents to dst.
	Clone(src, dst string) error

	// Remove removes path and its children.
	// Implementors should not return an error when the path does not
	// exist.
	Remove(path string) error
}

// Get returns the registered filesystem denoted by s. If it doesn't exist,
// an error is returned.
func Get(s string) (FileSystem, error) {
	fs, ok := Registry[s]
	if !ok {
		return nil, fmt.Errorf("unknown filesystem '%s' (%v)", s, Registry)
	}
	return fs, nil
}
