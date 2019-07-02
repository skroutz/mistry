package main

import (
	"strings"
	"testing"

	"github.com/skroutz/mistry/pkg/types"
)

func TestBuildCache(t *testing.T) {
	params := types.Params{"foo": "bar"}
	group := "baz"

	result1, err := postJob(
		types.JobRequest{Project: "build-cache", Params: params, Group: group})
	if err != nil {
		t.Fatal(err)
	}

	out1, err := readOut(result1, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	cachedOut1, err := readOut(result1, CacheDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out1, cachedOut1, t)

	params["foo"] = "bar2"
	result2, err := postJob(
		types.JobRequest{Project: "build-cache", Params: params, Group: group})
	if err != nil {
		t.Fatal(err)
	}

	out2, err := readOut(result2, ArtifactsDir)
	if err != nil {
		t.Fatal(err)
	}

	cachedOut2, err := readOut(result2, CacheDir)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(cachedOut1, cachedOut2, t)
	assertNotEq(out1, out2, t)
	assertNotEq(result1.Path, result2.Path, t)
	assert(result1.ExitCode, 0, t)
	assert(result2.ExitCode, 0, t)
	assert(result1.Incremental, false, t)
	assert(result2.Incremental, true, t)
}

func TestFailedPendingBuildCleanup(t *testing.T) {
	var err error
	project := "failed-build-cleanup"
	expected := "unknown instruction: INVALIDCOMMAND"

	for i := 0; i < 3; i++ {
		_, err = postJob(
			types.JobRequest{Project: project, Params: params, Group: ""})
		if !strings.Contains(err.Error(), expected) {
			t.Fatalf("Expected '%s' to contain '%s'", err.Error(), expected)
		}
	}
}

// regression test for incremental building bug
func TestBuildCacheWhenFailed(t *testing.T) {
	group := "ppp"

	// a successful build - it'll be symlinked
	_, err := postJob(
		types.JobRequest{Project: "failed-build-link",
			Params: types.Params{"_exitcode": "0"},
			Group:  group})
	if err != nil {
		t.Fatal(err)
	}

	// a failed build - it should NOT be symlinked
	_, err = postJob(
		types.JobRequest{Project: "failed-build-link",
			Params: types.Params{"_exitcode": "1", "foo": "bar"},
			Group:  group})
	if err != nil {
		t.Fatal(err)
	}

	// repeat the previous failed build - it
	// SHOULD be incremental
	buildInfo, err := postJob(
		types.JobRequest{Project: "failed-build-link",
			Params: types.Params{"_exitcode": "1", "foo": "bar"},
			Group:  group})
	if err != nil {
		t.Fatal(err)
	}

	if !buildInfo.Incremental {
		t.Fatal("build should be incremental, but it isn't")
	}

}
