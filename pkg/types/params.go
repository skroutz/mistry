package types

// Params are the user-provided parameters of a particular build.
// They're submitted as part of the job, typically using the mistry CLI.
type Params map[string]string
