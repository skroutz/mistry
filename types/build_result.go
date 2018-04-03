package types

type BuildResult struct {
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

	// The docker error, if any.
	Err error

	// The method by which the build artifacts can be fetched.
	TransportMethod TransportMethod
}
