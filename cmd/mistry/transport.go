package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type Transport interface {
	Copy(user, host, project, src, dst string, clearDst bool) []string
}

type Scp struct{}

func (ts Scp) Copy(user, host, project, src, dst string, clearDst bool) []string {
	if clearDst {
		removeDirContents(dst)
	}
	return []string{"scp", "-r", fmt.Sprintf("%s@%s:%s", user, host, src), dst}
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

func (ts Rsync) Copy(user, host, project, src, dst string, clearDst bool) []string {
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

	return cmd
}
