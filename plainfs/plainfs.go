package plainfs

import "os"

// PlainFS implements the FileSystem interface. It uses plain `cp` and `mkdir`
// commands.
type PlainFS struct{}

func (fs PlainFS) Create(path string) []string {
	return []string{"mkdir", path}
}

func (fs PlainFS) Clone(src, dst string) []string {
	return []string{"cp", "-r", src, dst}
}

func (fs PlainFS) Remove(path string) error {
	return os.RemoveAll(path)
}
