package types

type BuildResult struct {
	// The path where the build results are located.
	Path string

	// True if the result was returned from the cache.
	Cached bool

	// True if the result was returned from another pending build.
	Coalesced bool

	// The exit code status of the container command.
	//
	// NOTE: irrelevant if Coalesced is true.
	ExitCode int

	// The docker error, if any.
	Err error

	// The distribution method. Only `rsync` is available for now.
	Type string
}
