package main

import (
	"fmt"
	"log"
	"strings"
)

type Transport interface {
	Copy(user, host, project, src, dst string) []string
}

type Scp struct{}

func (ts Scp) Copy(user, host, project, src, dst string) []string {
	return []string{"scp", "-r", fmt.Sprintf("%s@%s:%s", user, host, src), dst}
}

type Rsync struct{}

func (ts Rsync) Copy(user, host, project, src, dst string) []string {
	module := "mistry"

	idx := strings.Index(src, project)
	if idx == -1 {
		log.Fatalf("Expected '%s' to contain '%s'", src, project)
	}
	src = src[idx:]

	return []string{"rsync", "-rtlp", fmt.Sprintf("%s@%s::%s/%s", user, host, module, src), dst}
}
