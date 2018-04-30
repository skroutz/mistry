package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/skroutz/mistry/pkg/utils"
)

type Transport interface {
	// Copy copies from src to dst, clearing the destination if clearDst is true
	Copy(user, host, project, src, dst string, clearDst bool) (string, error)
}

type Scp struct{}

// Copy runs 'scp user@host:src dst'. If clearDst is set, all contents of dst will be
// removed before the scp
func (ts Scp) Copy(user, host, project, src, dst string, clearDst bool) (string, error) {
	if clearDst {
		err := removeDirContents(dst)
		if err != nil {
			return "", err
		}
	}
	return utils.RunCmd([]string{"scp", "-r", fmt.Sprintf("%s@%s:%s", user, host, src), dst})
}

func removeDirContents(dir string) error {
	items, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, item := range items {
		err = os.RemoveAll(filepath.Join(dir, item.Name()))
		if err != nil {
			return err
		}
	}
	return nil
}

type Rsync struct{}

// Copy runs 'rsync -rtlp user@host::mistry/src dst'. If clearDst is true, the --delete flag
// will be set
func (ts Rsync) Copy(user, host, project, src, dst string, clearDst bool) (string, error) {
	module := "mistry"

	idx := strings.Index(src, project)
	if idx == -1 {
		log.Fatalf("Expected '%s' to contain '%s'", src, project)
	}
	src = src[idx:]
	cmd := []string{"rsync", "-rtlp"}
	if clearDst {
		cmd = append(cmd, "--delete")
	}
	cmd = append(cmd, fmt.Sprintf("%s@%s::%s/%s", user, host, module, src), dst)

	return utils.RunCmd(cmd)
}
