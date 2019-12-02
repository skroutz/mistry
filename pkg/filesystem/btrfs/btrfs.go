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

// Create creates a new subvolume named path.
func (fs Btrfs) Create(path string) error {
	return runCmd([]string{"btrfs", "subvolume", "create", path})
}

// Clone creates a Btrfs snapshot of subvolume src to a new subvolume, dst.
func (fs Btrfs) Clone(src, dst string) error {
	return runCmd([]string{"btrfs", "subvolume", "snapshot", src, dst})
}

// Remove deletes the subvolume with name path.
func (fs Btrfs) Remove(path string) error {
	_, err := os.Stat(path)
	if err != nil {
		return err
	}
	return runCmd([]string{"btrfs", "subvolume", "delete", path})
}

func runCmd(args []string) error {
	out, err := utils.RunCmd(args)
	if err != nil {
		return fmt.Errorf("%s (%s)", err, out)
	}
	return nil
}
