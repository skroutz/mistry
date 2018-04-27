// Tests here verify that all components (CLI <-> Server <-> Worker)
// interact together as expected.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestAsyncSimpleBuild(t *testing.T) {
	cmdout, cmderr, err := cliBuildJob("--json-result", "--project", "simple", "--no-wait", "--", "--test=async")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
	assertEq(cmdout, "", t)
	assertEq(cmderr, "", t)

	// wait until the build is done and verify the result

	j, err := NewJob("simple", types.Params{"test": "async"}, "", testcfg)
	if err != nil {
		t.Fatalf("%s", err)
	}

	buildInfoPath := filepath.Join(j.ReadyBuildPath, BuildInfoFname)

	err = waitUntilExists(buildInfoPath)
	if err != nil {
		t.Fatalf("failed to find job build info at %s: %s", buildInfoPath, err)
	}

	bi := types.BuildInfo{}
	biBlob, err := ioutil.ReadFile(buildInfoPath)
	if err != nil {
		t.Fatalf("%s", err)
	}
	err = json.Unmarshal(biBlob, &bi)
	if err != nil {
		t.Fatalf("%s", err)
	}
	assertEq(bi.ExitCode, 0, t)
}

func waitUntilExists(path string) error {
	maxElapsed := 10 * time.Second
	start := time.Now()
	for {
		_, err := os.Stat(path)
		if os.IsNotExist(err) {
			elapsed := time.Since(start)
			if elapsed > maxElapsed {
				return fmt.Errorf("file was not found at %s after %s", path, maxElapsed)
			}
			time.Sleep(500 * time.Millisecond)
		} else if err != nil {
			return err
		} else {
			return nil
		}
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

func TestImageBuildLogsNotJson(t *testing.T) {
	// trigger a job
	cmdout, cmderr, err := cliBuildJob("--json-result", "--project", "simple", "--", "--testing=logs")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
	// find the log file
	j, err := NewJob("simple", types.Params{"testing": "logs"}, "", testcfg)
	if err != nil {
		t.Fatalf("failed to create job: err: %#v", err)
	}
	f, err := os.Open(filepath.Join(j.ReadyBuildPath, BuildLogFname))
	if err != nil {
		t.Fatalf("failed to read job log: err: %#v", err)
	}
	defer f.Close()

	// if any line in the log file can be parsed into a JSON, fail
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Bytes()
		var v interface{}

		err := json.Unmarshal(line, &v)
		if err == nil {
			t.Fatalf("found JSON line in the logs: %s", line)
		}
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

func TestRerunFailedBuild(t *testing.T) {
	// schedule a build that fails (non zero exit code)
	cmdout, _, err := cliBuildJob("--json-result", "--project", "exit-code")
	if err == nil {
		t.Fatal("expected error")
	}

	br, err := parseClientJSON(cmdout)
	if err != nil {
		t.Fatal(err)
	}

	assertNotEq(br.ExitCode, 0, t)

	// schedule it again, verify it ran a 2nd time by checking the start timestamp
	cmdout, _, err = cliBuildJob("--json-result", "--project", "exit-code")
	if err == nil {
		t.Fatal("expected error")
	}

	br2, err := parseClientJSON(cmdout)
	if err != nil {
		t.Fatal(err)
	}

	assert(br2.ExitCode, br.ExitCode, t)
	assertNotEq(br2.StartedAt, br.StartedAt, t)
}

func TestBuildCoalescingExitCode(t *testing.T) {
	var wg sync.WaitGroup
	var bi1, bi2 *types.BuildInfo

	wg.Add(1)
	go func() {
		defer wg.Done()
		cmdout, _, err := cliBuildJob("--json-result", "--project", "build-coalescing-exitcode")
		if err == nil {
			panic("Expected error")
		}
		bi1, err = parseClientJSON(cmdout)
		if err != nil {
			panic(err)
		}
	}()

	cmdout, _, err := cliBuildJob("--json-result", "--project", "build-coalescing-exitcode")
	if err == nil {
		t.Fatal("Expected error")
	}
	bi2, err = parseClientJSON(cmdout)
	if err != nil {
		t.Fatal(err)
	}

	wg.Wait()

	assert(bi1.ExitCode, 35, t)
	assertEq(bi1.ExitCode, bi2.ExitCode, t)

	assert(bi1.Cached, false, t)
	assertEq(bi1.Cached, bi2.Cached, t)

	assertNotEq(bi1.Coalesced, bi2.Coalesced, t)
}

func TestBuildCoalescing(t *testing.T) {
	var wg sync.WaitGroup
	var bi1, bi2 *types.BuildInfo

	wg.Add(1)
	go func() {
		defer wg.Done()
		cmdout, _, err := cliBuildJob("--json-result", "--project", "build-coalescing", "--group", "foo")
		if err != nil {
			panic(err)
		}
		bi1, err = parseClientJSON(cmdout)
		if err != nil {
			panic(err)
		}
	}()

	cmdout, _, err := cliBuildJob("--json-result", "--project", "build-coalescing", "--group", "foo")
	if err != nil {
		t.Fatal(err)
	}
	bi2, err = parseClientJSON(cmdout)
	if err != nil {
		t.Fatal(err)
	}

	wg.Wait()

	out, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assertEq(string(out), "coalescing!\n", t)

	assertNotEq(bi1.Coalesced, bi2.Coalesced, t)
	assert(bi1.ExitCode, 0, t)
	assertEq(bi1.ExitCode, bi2.ExitCode, t)
}
