package btrfs

import (
	"fmt"

	"github.com/skroutz/mistry/pkg/filesystem"
	"github.com/skroutz/mistry/pkg/utils"
)

// Btrfs implements the FileSystem interface. It is an efficient implementation
// since it uses Copy-on-Write snapshots to do the cloning. It is the
// recommended solution for production systems.
type Btrfs struct{}

func init() {
	filesystem.List["btrfs"] = Btrfs{}
}

func (fs Btrfs) Create(path string) []string {
	return []string{"btrfs", "subvolume", "create", path}
}

func (fs Btrfs) Clone(src, dst string) []string {
	return []string{"btrfs", "subvolume", "snapshot", src, dst}
}

func (fs Btrfs) Remove(path string) error {
	out, err := utils.RunCmd([]string{"btrfs", "subvolume", "delete", path})
	if err != nil {
		return fmt.Errorf("%s (%s)", err, out)
	}
	return nil
}
