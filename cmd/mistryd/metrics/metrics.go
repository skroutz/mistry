package metrics

import (
	"fmt"
	"io/ioutil"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Recorder holds the collectors used by mistry to export data to prometheus.
type Recorder struct {
}

// NewRecorder initializes a Recorder and sets up the collectors.
func NewRecorder() *Recorder {
	r := new(Recorder)

	return r
}
