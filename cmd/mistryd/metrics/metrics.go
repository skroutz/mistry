package metrics

import (
	"fmt"
	"io/ioutil"
	"log"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Recorder holds the collectors used by mistry to export data to prometheus.
type Recorder struct {
	Log *log.Logger

	BuildsHosted *prometheus.GaugeVec
}

const namespace = "mistry"

// NewRecorder initializes a Recorder and sets up the collectors.
func NewRecorder(logger *log.Logger) *Recorder {
	r := new(Recorder)
	r.Log = logger

	r.BuildsHosted = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "builds_hosted",
			Help:      "The number of finished build hosted currently in the server",
		},
		[]string{"project"},
	)

	return r
}

// RecordHostedBuilds reads the number of builds for the project by counting
// the folders under its directories.
func (r *Recorder) RecordHostedBuilds(buildPath, projectsPath string) {
	projects, err := ioutil.ReadDir(projectsPath)
	if err != nil {
		r.Log.Printf("Failed to read project directory: %s", projectsPath)

		return
	}

	for _, project := range projects {
		buildDir := fmt.Sprintf("%s/%s/ready", buildPath, project.Name())
		builds, err := ioutil.ReadDir(buildDir)
		if err != nil {
			r.Log.Printf("Failed to read data directory: %s", buildDir)

			continue
		}

		labels := prometheus.Labels{"project": project.Name()}
		r.BuildsHosted.With(labels).Set(float64(len(builds)))
	}
}
