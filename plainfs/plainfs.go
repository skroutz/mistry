package plainfs

import "github.com/skroutz/mistry/utils"

// PlainFS implements the FileSystem interface. It uses plain `cp` and `mkdir`
// commands.
type PlainFS struct{}

func (fs PlainFS) Create(path string) error {
	_, err := utils.RunCmd("mkdir", path)
	return err
}

func (fs PlainFS) Clone(src, dst string) error {
	_, err := utils.RunCmd("cp", "-r", src, dst)
	return err
}
