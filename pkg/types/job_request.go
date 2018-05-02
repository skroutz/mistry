package types

// JobRequest contains the data the job was requested with
type JobRequest struct {
	Project string
	Params  Params
	Group   string
	Rebuild bool
}
