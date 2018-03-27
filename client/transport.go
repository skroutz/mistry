package main

import (
	"fmt"
)

type Transport interface {
	Copy(user, host, src, dst string) []string
}

type Scp struct{}

func (ts Scp) Copy(user, host, src, dst string) []string {
	return []string{"scp", "-r", fmt.Sprintf("%s@%s:%s", user, host, src), dst}
}

type Rsync struct{}

func (ts Rsync) Copy(user, host, src, dst string) []string {
	// TODO Make the rsync module configurable
	return []string{"rsync", "-rtlp", fmt.Sprintf("%s@%s::mistry%s", user, host, src), dst}
}
