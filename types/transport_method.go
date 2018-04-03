package types

type TransportMethod string

const (
	Rsync TransportMethod = "rsync"
	Scp                   = "scp"
)
