package btrfs

// Btrfs implements the FileSystem interface. It is an efficient implementation
// since it uses Copy-on-Write snapshots to do the cloning. It is the
// recommended solution for production systems.
type Btrfs struct{}

func (fs Btrfs) Create(path string) []string {
	return []string{"btrfs", "subvolume", "create", path}
}

func (fs Btrfs) Clone(src, dst string) []string {
	return []string{"btrfs", "subvolume", "snapshot", src, dst}
}
