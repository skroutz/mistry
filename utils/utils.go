package utils

import (
	"errors"
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

// RunCmd runs the shell command denoted by args, using the first
// element as the command and the remained as its arguments.
// It returns the combined stderr/stdout output of the command.
func RunCmd(args []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}
