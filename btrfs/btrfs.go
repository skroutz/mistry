package btrfs

import "github.com/skroutz/mistry/utils"

// Btrfs implements the FileSystem interface. It is an efficient implementation
// since it uses Copy-on-Write snapshots to do the cloning. It is the
// recommended solution for production systems.
type Btrfs struct{}

func (fs Btrfs) Create(path string) error {
	_, err := utils.RunCmd("btrfs", "subvolume", "create", path)
	return err
}

func (fs Btrfs) Clone(src, dst string) error {
	_, err := utils.RunCmd("btrfs", "subvolume", "snapshot", src, dst)
	return err
}
