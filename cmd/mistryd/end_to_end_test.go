// Tests here verify that all components (CLI <-> Server <-> Worker)
// interact together as expected.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	docker "github.com/docker/docker/client"
	"github.com/skroutz/mistry/pkg/types"
)

func TestSimpleBuild(t *testing.T) {
	cmdout, cmderr, err := cliBuildJob("--project", "simple")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
}

func toCli(p types.Params) []string {
	cliParams := make([]string, len(p))
	i := 0
	for k, v := range p {
		cliParams[i] = fmt.Sprintf("--%s=%s", k, v)
		i++
	}
	return cliParams
}

func TestSimpleRebuild(t *testing.T) {
	// run a job, fetch its build time
	params := types.Params{"test": "rebuild-cli"}
	cmdout, cmderr, err := cliBuildJob("--project", "simple", "--", toCli(params)[0])
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}

	j, err := NewJob("simple", params, "", testcfg)
	if err != nil {
		t.Fatalf("%s", err)
	}

	client, err := docker.NewEnvClient()
	if err != nil {
		t.Fatal(err)
	}

	i, _, err := client.ImageInspectWithRaw(context.Background(), j.Image)
	if err != nil {
		t.Fatal(err)
	}

	// remove the build directory for the job to run again
	err = testcfg.FileSystem.Remove(j.ReadyBuildPath)
	if err != nil {
		t.Fatal(err)
	}

	cmdout, cmderr, err = cliBuildJob("--project", "simple", "--rebuild", "--", toCli(params)[0])
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
	// fetch last build time, make sure it is different
	i2, _, err := client.ImageInspectWithRaw(context.Background(), j.Image)
	if err != nil {
		t.Fatal(err)
	}
	assertNotEq(i.Created, i2.Created, t)
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

func TestBuildRemoveTarget(t *testing.T) {
	// create a new temp target dir
	target, err := ioutil.TempDir("", "test-remove-target")
	failIfError(err, t)
	defer os.RemoveAll(target)

	// create 2 files: a /target/file.txt and a /target/dir/file2.txt
	dirName := filepath.Join(target, "dir")
	fileNames := []string{filepath.Join(target, "file.txt"), filepath.Join(dirName, "file2.txt")}

	err = os.Mkdir(dirName, 0755)
	failIfError(err, t)

	for _, filepath := range fileNames {
		f, err := os.Create(filepath)
		failIfError(err, t)
		f.Close()
	}
	cliArgs := cliDefaultArgs
	cliArgs.target = target

	// run the job with remove-target
	cmdout, cmderr, err := cliBuildJobArgs(cliArgs, "--project", "simple", "--verbose", "--clear-target")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
	// verify the files and directory have been deleted
	for _, path := range append(fileNames, dirName) {
		_, err = os.Stat(path)
		if err == nil {
			t.Fatalf("unexpected file found at %s", path)
		} else if !os.IsNotExist(err) {
			t.Fatalf("error when trying to check target file %s: %s", path, err)
		}
	}
}

func failIfError(err error, t *testing.T) {
	if err != nil {
		t.Fatalf("%s", err)
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

	out, err := ioutil.ReadFile(filepath.Join(cliDefaultArgs.target, "out.txt"))
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

func TestLogs(t *testing.T) {
	// trigger a job
	cmdout, cmderr, err := cliBuildJob("--json-result", "--project", "simple", "--", "--testing=logs")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}

	br, err := parseClientJSON(cmdout)
	if err != nil {
		t.Fatal(err)
	}

	// find the log file
	j, err := NewJob("simple", types.Params{"testing": "logs"}, "", testcfg)
	if err != nil {
		t.Fatalf("failed to create job: err: %#v", err)
	}
	log, err := ReadJobLogs(j.ReadyBuildPath)
	if err != nil {
		t.Fatalf("failed to read job log: err: %#v", err)
	}

	assertEq(br.Log, string(log), t)
}

func TestLogsNotJson(t *testing.T) {
	// trigger a job and grab the logs
	cmdout, cmderr, err := cliBuildJob("--json-result", "--project", "simple", "--", "--testing=logsnotjson")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout, cmderr, err)
	}
	j, err := NewJob("simple", types.Params{"testing": "logsnotjson"}, "", testcfg)
	if err != nil {
		t.Fatalf("failed to create job: err: %#v", err)
	}
	logs, err := ReadJobLogs(j.ReadyBuildPath)
	if err != nil {
		t.Fatalf("failed to read job log: err: %#v", err)
	}

	// if any line in the log file can be parsed into a JSON, fail
	scanner := bufio.NewScanner(bytes.NewReader(logs))
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
	out1, err := ioutil.ReadFile(filepath.Join(cliDefaultArgs.target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	cmdout2, cmderr2, err := cliBuildJob("--project", "result-cache", "--group", "foo", "--", "--foo=bar2")
	if err != nil {
		t.Fatalf("mistry-cli stdout: %s, stderr: %s, err: %#v", cmdout2, cmderr2, err)
	}
	out2, err := ioutil.ReadFile(filepath.Join(cliDefaultArgs.target, "out.txt"))
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
	out1, err := ioutil.ReadFile(filepath.Join(cliDefaultArgs.target, "out.txt"))
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
	out2, err := ioutil.ReadFile(filepath.Join(cliDefaultArgs.target, "out.txt"))
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

	out, err := ioutil.ReadFile(filepath.Join(cliDefaultArgs.target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assertEq(string(out), "coalescing!\n", t)

	assertNotEq(bi1.Coalesced, bi2.Coalesced, t)
	assert(bi1.ExitCode, 0, t)
	assertEq(bi1.ExitCode, bi2.ExitCode, t)
}
