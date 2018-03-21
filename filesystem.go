package main

type FileSystem interface {
	// Create creates path as a directory
	Create(path string) error

	// Clone clones src to dst
	Clone(src, dst string) error
}

// Btrfs implements the FileSystem interface. It is an efficient implementation
// since it uses Copy-on-Write snapshots to do the cloning. It is the
// recommended solution for production systems.
type Btrfs struct{}

func (fs Btrfs) Create(path string) error {
	_, err := RunCmd("btrfs", "subvolume", "create", path)
	return err
}

func (fs Btrfs) Clone(src, dst string) error {
	_, err := RunCmd("btrfs", "subvolume", "snapshot", src, dst)
	return err
}

// PlainFS implements the FileSystem interface. It uses plain `cp` and `mkdir`
// commands.
type PlainFS struct{}

func (fs PlainFS) Create(path string) error {
	_, err := RunCmd("mkdir", path)
	return err
}

func (fs PlainFS) Clone(src, dst string) error {
	_, err := RunCmd("cp", "-r", src, dst)
	return err
}
