package metrics

import (
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Recorder holds the collectors used by mistry to export data to prometheus.
type Recorder struct {
	Log *log.Logger

	BuildsHosted                 *prometheus.GaugeVec
	BuildsStarted                *prometheus.CounterVec
	BuildsFinished               *prometheus.CounterVec
	BuildsCoalesced              *prometheus.CounterVec
	BuildsProcessedIncrementally *prometheus.CounterVec
	BuildsSucceeded              *prometheus.HistogramVec
	BuildsFailed                 *prometheus.HistogramVec
	CacheUtilization             *prometheus.CounterVec
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

	r.BuildsStarted = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "builds_started",
			Help:      "The total number builds started by the server",
		},
		[]string{"project"},
	)

	r.BuildsFinished = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "builds_finished",
			Help:      "The number of builds finished.",
		},
		[]string{"project"},
	)

	r.BuildsCoalesced = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "builds_coalesced",
			Help:      "The number of builds that coalesced and were not processed",
		},
		[]string{"project"},
	)

	r.BuildsProcessedIncrementally = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "builds_processed_incrementally",
			Help:      "The number builds processed incrementally by the server",
		},
		[]string{"project"},
	)

	// The buckets we create start at 2 minutes and we create 3 buckets of
	// 2 minute intervals.
	buildTimeBuckets := prometheus.LinearBuckets(120, 120, 3)

	r.BuildsSucceeded = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "builds_succeeded_seconds",
			Help:      "Build duration and count for successful results.",
			Buckets:   buildTimeBuckets,
		},
		[]string{"project"},
	)

	r.BuildsFailed = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "builds_failed_seconds",
			Help:      "Build duration and count for failed results.",
			Buckets:   buildTimeBuckets,
		},
		[]string{"project"},
	)

	r.CacheUtilization = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_utilization",
			Help:      "Build result cache utilization",
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

// RecordBuildStarted records a build started, independently from its outcome.
func (r *Recorder) RecordBuildStarted(project string) {
	r.BuildsStarted.With(prometheus.Labels{"project": project}).Inc()
}

// RecordBuildCoalesced records a project's build when coalesced.
func (r *Recorder) RecordBuildCoalesced(project string) {
	r.BuildsCoalesced.With(prometheus.Labels{"project": project}).Inc()
}

// RecordBuildFinished records a project's state, whether it was build incrementally
// and tis duration.
func (r *Recorder) RecordBuildFinished(
	project string,
	success bool,
	incremental bool,
	duration time.Duration,
) {
	labels := prometheus.Labels{"project": project}

	r.BuildsFinished.With(labels).Inc()

	if success {
		if incremental {
			r.BuildsProcessedIncrementally.With(labels).Inc()
		}

		r.BuildsSucceeded.With(labels).Observe(duration.Seconds())
	} else {
		r.BuildsFailed.With(labels).Observe(duration.Seconds())
	}
}

// RecordCacheUtilization records a project build's cache utilization.
func (r *Recorder) RecordCacheUtilization(project string) {
	r.CacheUtilization.With(prometheus.Labels{"project": project}).Inc()
}
