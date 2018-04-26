package plainfs

import (
	"fmt"
	"os"

	"github.com/skroutz/mistry/pkg/filesystem"
	"github.com/skroutz/mistry/pkg/utils"
)

// PlainFS implements the FileSystem interface. It uses plain `cp` and `mkdir`
// commands.
type PlainFS struct{}

func init() {
	filesystem.Registry["plain"] = PlainFS{}
}

// Create creates a new directory at path
func (fs PlainFS) Create(path string) error {
	return os.Mkdir(path, 0755)
}

// Clone recursively copies the contents of src to dst
func (fs PlainFS) Clone(src, dst string) error {
	out, err := utils.RunCmd([]string{"cp", "-r", src, dst})
	if err != nil {
		return fmt.Errorf("%s (%s)", err, out)
	}
	return nil
}

// Remove deletes the path and all its contents
func (fs PlainFS) Remove(path string) error {
	return os.RemoveAll(path)
}
