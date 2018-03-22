package utils

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
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

	// TODO: maybe check the permissions too?
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
	// TODO: also check permissions

	return nil
}

// TODO: do we actually want the output?
func RunCmd(name string, args ...string) (string, error) {
	// TODO: log instead?
	fmt.Println("running", name, args)
	cmd := exec.Command(name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
