package plainfs

import (
	"os"

	"github.com/skroutz/mistry/filesystem"
)

// PlainFS implements the FileSystem interface. It uses plain `cp` and `mkdir`
// commands.
type PlainFS struct{}

func init() {
	filesystem.List["plain"] = PlainFS{}
}

func (fs PlainFS) Create(path string) []string {
	return []string{"mkdir", path}
}

func (fs PlainFS) Clone(src, dst string) []string {
	return []string{"cp", "-r", src, dst}
}

func (fs PlainFS) Remove(path string) error {
	return os.RemoveAll(path)
}
