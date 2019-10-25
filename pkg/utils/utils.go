package utils

import (
	"archive/tar"
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

// PathIsDir returns an error if p does not exist or is not a directory.
func PathIsDir(p string) error {
	fi, err := os.Stat(p)
	if err != nil {
		return err
	}

	if !fi.IsDir() {
		return errors.New("Path " + p + " is not a directory")
	}

	return nil
}

// EnsureDirExists verifies path is a directory and creates it if it doesn't
// exist.
func EnsureDirExists(path string) error {
	fi, err := os.Stat(path)
	if err == nil {
		if !fi.IsDir() {
			return errors.New(path + " is not a directory")
		}
	} else {
		if os.IsNotExist(err) {
			err = os.Mkdir(path, 0755)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// RunCmd runs the shell command denoted by args, using the first
// element as the command and the remained as its arguments.
// It returns the combined stderr/stdout output of the command.
func RunCmd(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Tar walks the file tree rooted at root, adding each file or directory in the
// tree (including root) in a tar archive. The files are walked
// in lexical order, which makes the output deterministic.
func Tar(root string) ([]byte, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}

		hdr, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}

		// Preserve directory structure when docker "untars" the build context
		hdr.Name, err = filepath.Rel(root, path)
		if err != nil {
			return err
		}

		err = tw.WriteHeader(hdr)
		if err != nil {
			return err
		}

		f, err := os.Open(path)
		if err != nil {
			return err
		}

		_, err = io.Copy(tw, f)
		if err != nil {
			return err
		}

		err = f.Close()
		if err != nil {
			return err
		}

		return nil
	}

	err := filepath.Walk(root, walkFn)
	if err != nil {
		return nil, err
	}

	err = tw.Close()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
