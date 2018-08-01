// Copyright 2018-present Skroutz S.A.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"os/user"
	"strings"
	"time"

	"github.com/skroutz/mistry/pkg/types"
	"github.com/urfave/cli"
)

var transports = make(map[types.TransportMethod]Transport)

func init() {
	transports[types.Scp] = Scp{}
	transports[types.Rsync] = Rsync{}
}

func main() {
	const JobsPath = "jobs"

	var (
		project       string
		group         string
		target        string
		host          string
		port          string
		transportUser string
		transport     string
		verbose       bool
		jsonResult    bool
		noWait        bool
		clearTarget   bool
		rebuild       bool
		timeout       string
	)

	currentUser, err := user.Current()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Cannot fetch current user:", err)
		os.Exit(1)
	}

	cli.AppHelpTemplate = fmt.Sprintf(`%s
WEBSITE: https://github.com/skroutz/mistry
`, cli.AppHelpTemplate)

	cli.CommandHelpTemplate = fmt.Sprintf(`%s
JOB PARAMETERS:
   -- dynamic options for the command

EXAMPLES:
	1. Schedule a job with a group and some parameters and put artifacts under
		/tmp/yarn using rsync. Prefixing a file name with @ will cause the contents
		of yarn.lock to be sent as parameters. Parameters prepended with '_' are opaque
		and do not affect the build result.

		$ {{.HelpName}} --host example.org --port 9090 --project yarn \
			--group group_name --transport rsync --target /tmp/yarn \
			-- --lockfile=@yarn.lock --foo=bar --_ignored=true

	2. Schedule a build and exit early without waiting for the result by setting
		the no-wait flag.

		$ {{.HelpName}} --host example.org --port 9090 --project yarn --no-wait
`, cli.CommandHelpTemplate)

	app := cli.NewApp()
	app.Usage = "schedule build jobs at the mistry service"
	app.HideVersion = true
	app.Commands = []cli.Command{
		{
			Name:  "build",
			Usage: "Schedule jobs and retrieve artifacts.",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "host",
					Usage:       "host to connect to",
					Destination: &host,
					Value:       "0.0.0.0",
				},
				cli.StringFlag{
					Name:        "port, p",
					Usage:       "port to connect to",
					Destination: &port,
					Value:       "8462",
				},
				cli.StringFlag{
					Name:        "project",
					Usage:       "job's project",
					Destination: &project,
				},
				cli.StringFlag{
					Name:        "group, g",
					Usage:       "group project builds (optional)",
					Destination: &group,
				},
				cli.BoolFlag{
					Name:        "verbose, v",
					Destination: &verbose,
				},
				cli.BoolFlag{
					Name:        "json-result",
					Usage:       "output the build result in JSON format to STDOUT (implies verbose: false)",
					Destination: &jsonResult,
				},
				cli.BoolFlag{
					Name:        "rebuild",
					Usage:       "rebuild the docker image",
					Destination: &rebuild,
				},
				cli.StringFlag{
					Name:        "timeout",
					Usage:       "time to wait for the build to finish, accepts values as defined at https://golang.org/pkg/time/#ParseDuration",
					Destination: &timeout,
					Value:       "60m",
				},

				// transport flags
				cli.BoolFlag{
					Name:        "no-wait",
					Usage:       "if set, schedule the job but don't fetch the artifacts",
					Destination: &noWait,
				},
				cli.StringFlag{
					Name:        "transport",
					Usage:       "the method to use for fetching artifacts",
					Destination: &transport,
					Value:       types.Scp,
				},
				cli.StringFlag{
					Name:        "transport-user",
					Usage:       "user to fetch the artifacts with",
					Destination: &transportUser,
					Value:       currentUser.Username,
				},
				cli.StringFlag{
					Name:        "target, t",
					Usage:       "the local directory where the artifacts will be saved",
					Destination: &target,
					Value:       ".",
				},
				cli.BoolFlag{
					Name:        "clear-target",
					Usage:       "remove contents of the target directory before fetching artifacts",
					Destination: &clearTarget,
				},
			},
			Action: func(c *cli.Context) error {
				// Validate existence of mandatory arguments
				if host == "" {
					return errors.New("host cannot be empty")
				}
				if project == "" {
					return errors.New("project cannot be empty")
				}
				if target == "" {
					return errors.New("target cannot be empty")
				}
				if !noWait && transport == "" {
					return errors.New("you need to either specify a transport or use the async flag")
				}

				var (
					clientTimeout time.Duration
					err           error
				)
				if timeout != "" {
					clientTimeout, err = time.ParseDuration(timeout)
					if err != nil {
						return err
					}
				}

				if jsonResult {
					verbose = false
				}

				var (
					ts       Transport
					tsExists bool
				)

				ts, tsExists = transports[types.TransportMethod(transport)]
				if !tsExists {
					return fmt.Errorf("invalid transport argument (%v)", transport)
				}

				// Normalize dynamic arguments by trimming the `--` and
				// creating the params map with the result.
				var dynamicArgs []string
				for _, v := range c.Args() {
					dynamicArgs = append(dynamicArgs, strings.TrimLeft(v, "--"))
				}
				params := make(map[string]string)
				for _, v := range dynamicArgs {
					arg := strings.Split(v, "=")
					params[arg[0]] = arg[1]
				}

				// Dynamic arguments starting with `@` are considered actual
				// files in the filesystem.
				//
				// For these arguments the params map contains the file
				// content.
				for k, v := range params {
					if strings.HasPrefix(v, "@") {
						data, err := ioutil.ReadFile(strings.TrimPrefix(v, "@"))
						if err != nil {
							return err
						}
						params[k] = string(data)
					}
				}

				baseURL := fmt.Sprintf("http://%s:%s", host, port)
				url := baseURL + "/" + JobsPath
				if noWait {
					url += "?async"
				}

				jr := types.JobRequest{Project: project, Group: group, Params: params, Rebuild: rebuild}
				jrJSON, err := json.Marshal(jr)
				if err != nil {
					return err
				}

				if verbose {
					fmt.Printf("Scheduling %#v...\n", jr)
				}

				body, err := sendRequest(url, jrJSON, verbose, clientTimeout)
				if err != nil {
					if isTimeout(err) {
						return fmt.Errorf("The build did not finish after %s, %s", clientTimeout, err)
					}
					return err
				}

				if noWait {
					if verbose {
						fmt.Println("Build scheduled successfully")
					}
					return nil
				}

				// Transfer the result
				bi := types.NewBuildInfo()
				err = json.Unmarshal([]byte(body), bi)
				if err != nil {
					return err
				}

				if !jsonResult {
					fmt.Println("Logs can be found at", baseURL+"/"+bi.URL)
				}

				if verbose {
					fmt.Printf(
						"\nResult:\nStarted at: %s ExitCode: %v Params: %s Cached: %v Coalesced: %v\n\nLogs:\n%s\n",
						bi.StartedAt, bi.ExitCode, bi.Params, bi.Cached, bi.Coalesced, bi.Log)
				}

				if jsonResult {
					fmt.Printf("%s\n", body)
				}

				if bi.ExitCode != 0 {
					if bi.ErrLog != "" {
						fmt.Fprintln(os.Stderr, "Container error logs:\n", bi.ErrLog)
					} else {
						fmt.Fprintln(os.Stderr, "There are no container error logs.")
					}
					return fmt.Errorf("Build failed with exit code %d", bi.ExitCode)
				}

				if verbose {
					fmt.Println("Copying artifacts to", target, "...")
				}
				out, err := ts.Copy(transportUser, host, project, bi.Path+"/*", target, clearTarget)
				fmt.Println(out)
				if err != nil {
					return err
				}
				if verbose {
					fmt.Println("Artifacts copied to", target)
				}

				return nil
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func sendRequest(url string, reqBody []byte, verbose bool, timeout time.Duration) ([]byte, error) {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, err
	}
	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if verbose {
		fmt.Printf("Server response: %#v\n", resp)
	}

	if resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("Error creating job: %s, http code: %v", respBody, resp.StatusCode)
	}
	return respBody, nil
}

func isTimeout(err error) bool {
	urlErr, ok := err.(*url.Error)
	return ok && urlErr.Timeout()
}
