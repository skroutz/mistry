package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"strings"

	"github.com/skroutz/mistry/types"
	"github.com/skroutz/mistry/utils"
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
	)

	currentUser, err := user.Current()
	if err != nil {
		log.Fatal("Cannot fetch current user; ", err)
	}

	cli.AppHelpTemplate = fmt.Sprintf(`%s
WEBSITE: https://github.com/skroutz/mistry
`, cli.AppHelpTemplate)

	cli.CommandHelpTemplate = fmt.Sprintf(`%s
JOB PARAMETERS:
   -- dynamic options for the command

EXAMPLES:
   1. The following sequence schedules a command to be executed in http://example.org:9090,
      builds the yarn project, groups it under group_name, syncs the result to the
      directory /tmp/yarn and uses the dynamic arguments yarn.lock and foo.
      Note the @ before the yarn.lock, this indicates a referral to an actual file on the
      filesystem.

      $ mistry-cli build --host example.org --port 9090 --project yarn \
      --group group_name --target /tmp/yarn -- --lockfile=@yarn.lock --foo=bar

   2. The following sequence uses the GROUP environment variable to set the project group.

      $ GROUP=group_name mistry-cli build --host example.org --port 9090 --project yarn \
      --target /tmp/yarn -- --lockfile=@yarn.lock --foo=bar
	`, cli.CommandHelpTemplate)

	app := cli.NewApp()
	app.Name = "mistry-cli"
	app.Usage = "mistry client"
	app.HideVersion = true
	app.Commands = []cli.Command{
		{
			Name:  "build",
			Usage: "Schedule a mistry job and retrieve the results",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:        "host",
					Usage:       "host to connect to",
					Destination: &host,
				},
				cli.StringFlag{
					Name:        "port, p",
					Usage:       "port to connect to",
					Destination: &port,
					Value:       "8462",
				},
				cli.StringFlag{
					Name:        "transport-user",
					Usage:       "user to fetch the artifacts with",
					Destination: &transportUser,
					Value:       currentUser.Username,
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
				cli.StringFlag{
					Name:        "target, t",
					Usage:       "the local directory where the result will be saved",
					Destination: &target,
					Value:       ".",
				},
				cli.StringFlag{
					Name:        "transport",
					Destination: &transport,
					Value:       types.Scp,
				},
				cli.BoolFlag{
					Name:        "verbose, v",
					Destination: &verbose,
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
				ts, ok := transports[types.TransportMethod(transport)]
				if !ok {
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

				jr := types.JobRequest{Project: project, Group: group, Params: params}
				jrJSON, err := json.Marshal(jr)
				if err != nil {
					return err
				}

				request, err := http.NewRequest("POST", fmt.Sprintf("http://%s:%s/%s", host, port, JobsPath), bytes.NewBuffer(jrJSON))
				if err != nil {
					return err
				}

				if verbose {
					fmt.Printf("Scheduling %#v...\n", jr)
				}

				client := &http.Client{}
				resp, err := client.Do(request)
				if err != nil {
					return err
				}
				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					return err
				}

				if verbose {
					fmt.Printf("Server response: %#v\n", resp)
					fmt.Printf("Body: %s\n", body)
				}

				if resp.StatusCode != http.StatusCreated {
					return fmt.Errorf("Error creating job: %s, http code: %v", body, resp.StatusCode)
				}

				br := types.BuildResult{}
				err = json.Unmarshal([]byte(body), &br)
				if err != nil {
					return err
				}

				if verbose {
					fmt.Printf("Result after unmarshalling: %#v\n", br)
				}

				if br.ExitCode != 0 {
					return fmt.Errorf("Build failed with exit code %d", br.ExitCode)
				}

				out, err := utils.RunCmd(ts.Copy(transportUser, host, project, br.Path+"/*", target))
				fmt.Println(out)
				if err != nil {
					return err
				}

				return nil
			},
		},
	}

	err = app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
