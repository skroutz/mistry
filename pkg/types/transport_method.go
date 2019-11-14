package types

// TransportMethod indicates the tool (binary) that the client will use to
// download the build artifacts from the server. The binary should be installed
// in the system.
type TransportMethod string

const (
	// Rsync instructs the client to use rsync(1) to download the assets,
	// either over the SSH or rsync protocol. It is the recommended choice
	// for production environments.
	Rsync TransportMethod = "rsync"

	// Scp instructs the client to use scp(1) to download the assets.
	Scp = "scp"
)
