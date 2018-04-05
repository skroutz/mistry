package main

import (
	"strings"
	"testing"

	"github.com/skroutz/mistry/types"
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
