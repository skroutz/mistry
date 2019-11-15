package types

import "fmt"

// ErrImageBuild indicates an error occurred while building a Docker image.
type ErrImageBuild struct {
	Image string
	Err   error
}

func (e ErrImageBuild) Error() string {
	return fmt.Sprintf("could not build docker image '%s': %s", e.Image, e.Err)
}
