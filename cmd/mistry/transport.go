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

// Transport is the interface that wraps the basic Copy method, facilitating
// downloading build artifacts from a mistry server.
type Transport interface {
	// Copy downloads to dst the build artifacts from src. The value of
	// src depends on the underlying implementation. dst denotes a path
	// on the local filesystem. host is the hostname of the server. user
	// is an opaque field that depends on the underlying implementation.
	//
	// If clearDst is true the contents of dst (if any) should be removed
	// before downloading artifacts.
	Copy(user, host, project, src, dst string, clearDst bool) (string, error)
}

// Scp uses scp(1) to fetch build artifacts from the server via SSH.
//
// See man 1 scp.
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

// Rsync uses rsync(1) and the rsync protocol to fetch build artifacts from
// the server. It is more efficient than Scp and the recommended transport
// for production systems.
//
// See man 1 rsync.
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
