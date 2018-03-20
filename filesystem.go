package main

type FileSystem interface {
	// Create creates path as a directory
	Create(path string) error

	// Clone clones src to dst
	Clone(src, dst string) error
}

// Btrfs implements the FileSystem interface.
type Btrfs struct{}

func (b Btrfs) Create(path string) error {
	_, err := RunCmd("btrfs", "subvolume", "create", path)
	return err
}

func (b Btrfs) Clone(src, dst string) error {
	_, err := RunCmd("btrfs", "subvolume", "snapshot", src, dst)
	return err
}
