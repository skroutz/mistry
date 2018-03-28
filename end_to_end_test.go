// Tests here verify that all components (CLI <-> Server <-> Worker)
// interact together as expected.
package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/skroutz/mistry/utils"
)

func TestSimpleBuild(t *testing.T) {
	cmdOut, err := cliBuildJob("--project", "simple")
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut, err)
	}
}

func TestUnknownProject(t *testing.T) {
	expected := "Unknown project"

	cmdOut, err := cliBuildJob("--project", "Idontexist")
	if !strings.Contains(cmdOut, expected) {
		t.Fatalf("Error output: %s, actual: %v, expected: %v", cmdOut, err.Error(), expected)
	}
}

func TestJobParams(t *testing.T) {
	cmdOut, err := cliBuildJob("--project", "params", "--", "--foo=zxc")
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut, err)
	}

	out, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assert(string(out), "zxc", t)
}

func TestSameGroupDifferentParams(t *testing.T) {
	cmdOut1, err := cliBuildJob("--project", "result-cache", "--group", "foo", "--", "--foo=bar")
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut1, err)
	}
	out1, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	cmdOut2, err := cliBuildJob("--project", "result-cache", "--group", "foo", "--", "--foo=bar2")
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut2, err)
	}
	out2, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assertNotEq(out1, out2, t)
}

func TestResultCache(t *testing.T) {
	cmdOut1, err := cliBuildJob("--project", "result-cache", "--group", "foo")
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut1, err)
	}
	out1, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	cmdOut2, err := cliBuildJob("--project", "result-cache", "--group", "foo")
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut2, err)
	}
	out2, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assertEq(out1, out2, t)
	// TODO Enable Cached and ExitCode assertions when the CLI JSON return is implemented
	// assert(result1.Cached, false, t)
	// assert(result2.Cached, true, t)
	// assert(result1.ExitCode, 0, t)
	// assert(result2.ExitCode, 0, t)
}

func TestResultCacheExitCode(t *testing.T) {
	cmdOut1, err := cliBuildJob("--project", "result-cache-exitcode")
	if err == nil || !strings.Contains(cmdOut1, "33") {
		fmt.Println("hi")
		t.Fatalf("Expected '%s' to contain the exit code 33", cmdOut1)
	}

	cmdOut2, err := cliBuildJob("--project", "result-cache-exitcode")
	if err == nil || !strings.Contains(cmdOut2, "33") {
		fmt.Println("yo")
		t.Fatalf("Expected '%s' to contain the exit code 33", cmdOut2)
	}
}

func TestBuildCoalescing(t *testing.T) {
	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		cmdOut, err := cliBuildJob("--project", "build-coalescing", "--group", "foo")
		if err != nil {
			log.Fatalf("Error output: %s, err: %v", cmdOut, err)
		}
	}()

	cmdOut, err := cliBuildJob("--project", "build-coalescing", "--group", "foo")
	if err != nil {
		t.Fatalf("Error output: %s, err: %v", cmdOut, err)
	}

	wg.Wait()

	// TODO Enable the Coalesced assertion when the CLI JSON return is implemented
	// if result1.Coalesced == result2.Coalesced {
	// 	t.Fatalf("Expected exactly one of both builds to be coalesced, both were %v", result1.Coalesced)
	// }

	out, err := ioutil.ReadFile(filepath.Join(target, "out.txt"))
	if err != nil {
		t.Fatal(err)
	}

	assertEq(string(out), "coalescing!\n", t)
	// TODO Enable the ExitCode assertion when the CLI JSON return is implemented
	// assert(result1.ExitCode, 0, t)
	// assert(result2.ExitCode, 0, t)
}

// cliBuildJob uses the CLI binary to issue a new job request to the server.
// It returns an error if the request could not be issued or if the job
// failed to build.
//
// NOTE: The CLI binary is expected to be present in the working
// directory where the tests are ran from.
func cliBuildJob(args ...string) (string, error) {
	args = append([]string{"./mistry-cli", "build", "--host", host, "--port", port, "--target", target, "--transport-user", username}, args...)
	out, err := utils.RunCmd(args)

	if err != nil {
		return fmt.Sprintf("out: %s, args: %v", out, args), err
	}
	return out, nil
}
