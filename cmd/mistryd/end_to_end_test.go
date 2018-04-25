// Tests here verify that all components (CLI <-> Server <-> Worker)
// interact together as expected.
package main

import (
	"io/ioutil"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/skroutz/mistry/pkg/types"
)

func TestSimpleBuild(t *testing.T) {
	cmdout, cmderr, err := cliBuildJob("--project", "simple")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
}

func TestNonGroupSubsequentInvocation(t *testing.T) {
	cmdout, cmderr, err := cliBuildJob("--project", "bootstrap-twice")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
	// invoke the 2nd job with different params to trigger the bug
	cmdout, cmderr, err = cliBuildJob("--project", "bootstrap-twice", "--", "--foo=zxc")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
}

func TestUnknownProject(t *testing.T) {
	expected := "Unknown project 'Idontexist'"

	_, cmderr, err := cliBuildJob("--project", "Idontexist")
	if err == nil {
		t.Fatal("Expected error")
	}
	if !strings.Contains(cmderr, expected) {
		t.Fatalf("Expected '%s' to contain '%s'", cmderr, expected)
	}
}

func TestJobParams(t *testing.T) {
	cmdout, cmderr, err := cliBuildJob("--project", "params", "--", "--foo=zxc")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}

	out, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assert(string(out), "zxc", t)
}

func TestImageBuildFailure(t *testing.T) {
	expErr := "could not build docker image"

	_, cmderr, err := cliBuildJob("--project", "image-build-failure")
	if err == nil {
		t.Fatal("expected error")
	}

	if !strings.Contains(cmderr, expErr) {
		t.Fatalf("Expected '%s' to contain '%s'", cmderr, expErr)
	}
}

func TestExitCode(t *testing.T) {
	cmdout, _, err := cliBuildJob("--json-result", "--project", "exit-code")
	if err == nil {
		t.Fatal("expected error")
	}

	br, err := parseClientJSON(cmdout)
	if err != nil {
		t.Fatal(err)
	}

	assert(br.ExitCode, 77, t)
}

func TestSameGroupDifferentParams(t *testing.T) {
	cmdout1, cmderr1, err := cliBuildJob("--project", "result-cache", "--group", "foo", "--", "--foo=bar")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout1, cmderr1, err)
	}
	out1, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	cmdout2, cmderr2, err := cliBuildJob("--project", "result-cache", "--group", "foo", "--", "--foo=bar2")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout2, cmderr2, err)
	}
	out2, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assertNotEq(out1, out2, t)
}

func TestResultCache(t *testing.T) {
	cmdout1, cmderr1, err := cliBuildJob("--json-result", "--project", "result-cache", "--group", "foo")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout1, cmderr1, err)
	}
	out1, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	br1, err := parseClientJSON(cmdout1)
	if err != nil {
		t.Fatal(err)
	}

	cmdout2, cmderr2, err := cliBuildJob("--json-result", "--project", "result-cache", "--group", "foo")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout2, cmderr2, err)
	}
	out2, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}
	br2, err := parseClientJSON(cmdout2)
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out1, out2, t)
	assert(br1.Cached, false, t)
	assert(br2.Cached, true, t)
	assert(br1.ExitCode, 0, t)
	assert(br2.ExitCode, 0, t)
}

func TestResultCacheExitCode(t *testing.T) {
	cmdout1, _, err := cliBuildJob("--json-result", "--project", "result-cache-exitcode")
	if err == nil {
		t.Fatal("Expected error")
	}
	br, err := parseClientJSON(cmdout1)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(br.ExitCode, 33, t)
	assertEq(br.Cached, false, t)

	cmdout2, _, err := cliBuildJob("--json-result", "--project", "result-cache-exitcode")
	if err == nil {
		t.Fatal("Expected error")
	}
	br, err = parseClientJSON(cmdout2)
	if err != nil {
		t.Fatal(err)
	}
	assertEq(br.ExitCode, 33, t)
	assertEq(br.Cached, true, t)
}

func TestBuildCoalescingExitCode(t *testing.T) {
	var wg sync.WaitGroup
	var br1, br2 *types.BuildResult

	wg.Add(1)
	go func() {
		defer wg.Done()
		cmdout, _, err := cliBuildJob("--json-result", "--project", "build-coalescing-exitcode")
		if err == nil {
			panic("Expected error")
		}
		br1, err = parseClientJSON(cmdout)
		if err != nil {
			panic(err)
		}
	}()

	cmdout, _, err := cliBuildJob("--json-result", "--project", "build-coalescing-exitcode")
	if err == nil {
		t.Fatal("Expected error")
	}
	br2, err = parseClientJSON(cmdout)
	if err != nil {
		t.Fatal(err)
	}

	wg.Wait()

	assert(br1.ExitCode, 35, t)
	assertEq(br1.ExitCode, br2.ExitCode, t)

	assert(br1.Cached, false, t)
	assertEq(br1.Cached, br2.Cached, t)

	assertNotEq(br1.Coalesced, br2.Coalesced, t)
}

func TestBuildCoalescing(t *testing.T) {
	var wg sync.WaitGroup
	var br1, br2 *types.BuildResult

	wg.Add(1)
	go func() {
		defer wg.Done()
		cmdout, _, err := cliBuildJob("--json-result", "--project", "build-coalescing", "--group", "foo")
		if err != nil {
			panic(err)
		}
		br1, err = parseClientJSON(cmdout)
		if err != nil {
			panic(err)
		}
	}()

	cmdout, _, err := cliBuildJob("--json-result", "--project", "build-coalescing", "--group", "foo")
	if err != nil {
		t.Fatal(err)
	}
	br2, err = parseClientJSON(cmdout)
	if err != nil {
		t.Fatal(err)
	}

	wg.Wait()

	out, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assertEq(string(out), "coalescing!\n", t)

	assertNotEq(br1.Coalesced, br2.Coalesced, t)
	assert(br1.ExitCode, 0, t)
	assertEq(br1.ExitCode, br2.ExitCode, t)
}
