package types

import (
	"fmt"
	"time"
)

// ContainerFailureExitCode is the exit code that signifies a failure
// before even running the container
const ContainerFailureExitCode = -999

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

	StartedAt time.Time

	// Contains the stdout and stderr as output by the container
	Log string
	// Contains the stderr output by the container
	ErrLog string
}

type ErrImageBuild struct {
	Image string
	Err   error
}

func NewBuildInfo() *BuildInfo {
	bi := new(BuildInfo)
	bi.StartedAt = time.Now()
	bi.ExitCode = ContainerFailureExitCode

	return bi
}

func (e ErrImageBuild) Error() string {
	return fmt.Sprintf("could not build docker image '%s': %s", e.Image, e.Err)
}
