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
	// NOTE: irrelevant if either Cached or Coalesced is true.
	ExitCode int

	// The docker error, if any.
	Err error

	// The distribution method. Only `rsync` is available for now.
	Type string
}

type JobRequest struct {
	// Contains the information for preparing the environment in which its jobs
	// are to be executed.
	Project string

	// Any dynamic parameters passed by the user.
	Params map[string]string

	// The grouping name of the job.
	Group string
}
