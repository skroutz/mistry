package types

import (
	"fmt"
)

type BuildInfo struct {
	// Job parameters
	Params Params

	// The path where the build artifacts are located.
	Path string

	// True if the result was returned from the result cache.
	Cached bool

	// True if the result was returned from another pending build.
	Coalesced bool

	// The exit code status of the container command.
	//
	// NOTE: irrelevant if Coalesced is true.
	ExitCode int

	Err error

	// The method by which the build artifacts can be fetched.
	TransportMethod TransportMethod
}

type ErrImageBuild struct {
	Image string
	Err   error
}

func (e ErrImageBuild) Error() string {
	return fmt.Sprintf("could not build docker image '%s': %s", e.Image, e.Err)
}
