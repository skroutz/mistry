package types

import "fmt"

type ErrImageBuild struct {
	Image string
	Err   error
}

func (e ErrImageBuild) Error() string {
	return fmt.Sprintf("could not build docker image '%s': %s", e.Image, e.Err)
}
