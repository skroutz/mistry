package types

import (
	"time"
)

const (
	// ContainerPendingExitCode is the zero value of BuildInfo.ExitCode
	// and is updated after the container finishes running.
	ContainerPendingExitCode = -999

	// ContainerSuccessExitCode indicates that the build was successful.
	ContainerSuccessExitCode = 0
)

// BuildInfo contains information regarding the outcome of an executed job.
type BuildInfo struct {
	// Params are the job build parameters
	Params Params

	// Group is the job group
	Group string

	// Path is the absolute path where the build artifacts are located.
	Path string

	// Cached is true if the build artifacts were retrieved from the cache.
	Cached bool

	// Coalesced is true if the build was returned from another pending
	// build.
	Coalesced bool

	// Incremental is true if the results of a previous build were
	// used as the base for this build (ie. build cache).
	Incremental bool

	// ExitCode is the exit code of the container command.
	//
	// It is initialized to ContainerFailureExitCode and is updated upon
	// build completion. If ExitCode is still set to ContainerFailureExitCode
	// after the build is finished, it indicates an error somewhere along
	// the path.
	//
	// It is irrelevant and should be ignored if Coalesced is true.
	ExitCode int

	// ErrBuild contains any errors that occured during the build.
	//
	// TODO: It might contain errors internal to the server, that the
	// user can do nothing about. This should be fixed
	ErrBuild string

	// ContainerStdouterr contains the stdout/stderr of the container.
	ContainerStdouterr string `json:",omitempty"`

	// ContainerStderr contains the stderr of the container.
	ContainerStderr string `json:",omitempty"`

	// TransportMethod is the method with which the build artifacts can be
	// fetched.
	TransportMethod TransportMethod

	// StartedAt is the date and time when the build started.
	StartedAt time.Time

	// Duration is how much the build took to complete. If it cannot be
	// calculated yet, the value will be -1 seconds.
	//
	// NOTE: if Cached is true, this refers to the original build.
	Duration time.Duration

	// URL is the relative URL at which the build log is available.
	URL string
}

func NewBuildInfo() *BuildInfo {
	bi := new(BuildInfo)
	bi.StartedAt = time.Now()
	bi.ExitCode = ContainerPendingExitCode
	bi.Duration = -1 * time.Second

	return bi
}
