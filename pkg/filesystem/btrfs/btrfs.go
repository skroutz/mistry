package btrfs

import (
	"fmt"
	"os"

	"github.com/skroutz/mistry/pkg/filesystem"
	"github.com/skroutz/mistry/pkg/utils"
)

// Btrfs implements the FileSystem interface. It is an efficient implementation
// since it uses Copy-on-Write snapshots to do the cloning. It is the
// recommended solution for production systems.
type Btrfs struct{}

func init() {
	filesystem.Registry["btrfs"] = Btrfs{}
}

func (fs Btrfs) Create(path string) error {
	return runCmd([]string{"btrfs", "subvolume", "create", path})
}

func (fs Btrfs) Clone(src, dst string) error {
	return runCmd([]string{"btrfs", "subvolume", "snapshot", src, dst})
}

func (fs Btrfs) Remove(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return runCmd([]string{"btrfs", "subvolume", "delete", path})
	}
	return nil
}

func runCmd(args []string) error {
	out, err := utils.RunCmd(args)
	if err != nil {
		return fmt.Errorf("%s (%s)", err, out)
	}
	return nil
}
