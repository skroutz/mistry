![mistry logo](logo.png)

------------------------------------------------------------------------------

[![Build Status](https://api.travis-ci.org/skroutz/mistry.svg?branch=master)](https://travis-ci.org/skroutz/mistry)
[![Go report](https://goreportcard.com/badge/github.com/skroutz/mistry)](https://goreportcard.com/report/github.com/skroutz/mistry)
[![License: GPL v3](https://img.shields.io/badge/License-GPL%20v3-blue.svg)](https://www.gnu.org/licenses/gpl-3.0)

*mistry* is a general-purpose build server that enables fast workflows by
employing artifact caching and incremental building techniques.

mistry executes user-defined build steps inside isolated environments
and saves build artifacts for later consumption.

Refer to the introductory blog post *[Speeding Up Our Build Pipelines](https://engineering.skroutz.gr/blog/speeding-up-build-pipelines-with-mistry/)*
for more information.

At Skroutz we use mistry to speed our development and deployment
processes:

- Rails asset compilation (`rails assets:precompile`)
- Bundler dependency resolution and download (`bundle install`)
- Yarn dependency resolution and download (`yarn install`)

In the above use cases, mistry executes these commands once they are needed for
the first time and caches the results. Then, when anyone else executes the same
commands (i.e.  application servers, developer workstations, CI server etc.)
they instantly get the results back.




Features
------------------------------------------------------------------------------

- execute user-defined build steps in pre-defined environments, provided as Docker images
- build artifact caching
- incremental building (see [*"Build cache"*](https://github.com/skroutz/mistry/wiki/Build-cache))
- [CLI client](cmd/mistry/README.md) for interacting with the server (scheduling jobs etc.)
  via a JSON API
- a web view for inspecting the progress of builds (see [*"Web view"*](#web-view))
- efficient use of disk space due to copy-on-write semantics (using [Btrfs snapshotting](https://en.wikipedia.org/wiki/Btrfs#Subvolumes_and_snapshots))



For more information visit the [wiki](https://github.com/skroutz/mistry/wiki).











Getting started
-------------------------------------------------
You can get the binaries from the
[latest releases](https://github.com/skroutz/mistry/releases).

Alternatively, if you have Go 1.10 or later you can get the
latest development version.

NOTE: [statik](https://github.com/rakyll/statik) is a build-time dependency,
so it should be installed in your system and present in your PATH.

```shell
$ go get github.com/rakyll/statik

# server
$ go get -u github.com/skroutz/mistry/cmd/mistryd

# client
$ go get -u github.com/skroutz/mistry/cmd/mistry
```





Usage
--------------------------------------------------
To boot the server a configuration file is needed:

```shell
$ mistryd --config config.json
```

You can use the [sample config](cmd/mistryd/config.sample.json) as a starting
point.

Use `mistryd --help` for more info.



### Adding projects

Projects are essentially directories with at minimum a `Dockerfile` at their
root. Each project directory should be placed in the path denoted by
`projects_path` (see [*Configuration*](#configuration).

Refer to [*File system layout - Projects directory*](https://github.com/skroutz/mistry/wiki/File-system-layout#projects-directory)
for more info.





### API

Interacting with mistry (scheduling builds etc.) can be done in two ways:
(1) using the [client](cmd/mistry/README.md) and (2)
using the HTTP API directly (see below).

We recommended using the client whenever possible.

#### Client

Schedule a build for project *foo* and download the artifacts:

```sh
$ mistry build --project foo --target /tmp/foo
```

The above command will block until the build is complete and then download the
resulting artifacts to `/tmp/foo/`.

Schedule a build without fetching the artifacts:

```sh
$ mistry build --project foo --no-wait
```

The above will just schedule the build and return immediately - it will not
wait for it to complete and will not fetch the artifacts.

For more info refer to the client's [README](cmd/mistry/README.md).

#### HTTP Endpoints

Schedule a new build without fetching artifacts (this is equivalent to passing
`--no-wait` when using the client):

```shell
$ curl -X POST /jobs \
    -H 'Accept: application/json' \
    -H 'Content-Type: application/json' \
    -d '{"project": "foo"}'
{
    "Params": {"foo": "xzv"},
    "Path": "<artifact path>",
    "Cached": true,
    "Coalesced": false,
    "ExitCode": 0,
    "Err": null,
    "TransportMethod": "rsync"
}
```


### Web view

mistry comes with a web view where progress and logs of each build can be
inspected.

Browse to http://0.0.0.0:8462 (or whatever address the server listens to).









Configuration
-------------------------------------------------
Configuration is provided in JSON format. The following settings are currently
supported:

| Setting        | Description           | Default  |
| ------------- |:-------------:| -----:|
| `projects_path` (string) | The path where project folders are located | "" |
| `build_path` (string) | The root path where artifacts will be placed       |   "" |
| `mounts` (object{string:string}) | The paths from the host machine that should be mounted inside the execution containers     |    {} |
| `job_concurrency` (int) | Maximum number of builds that may run in parallel | (logical-cpu-count) |
| `job_backlog` (int) | Used for back-pressure - maximum number of outstanding build requests. If exceeded subsequent build requests will fail | (job_concurrency * 2) |

The paths denoted by `projects_path` and `build_path` should be
present and writable by the user running the server.

For an example refer to the [sample config](cmd/mistryd/config.sample.json).





Development
---------------------------------------------------

To run the tests, the [Docker daemon](https://docs.docker.com/install/) should
be running and SSH access to localhost should be configured.

```shell
$ make test
```

Note: the command above may take more time the first time it's run,
since some Docker images will have to be fetched from the internet.




License
-------------------------------------------------
mistry is released under the GNU General Public License version 3. See [COPYING](COPYING).

mistry [logo](logo.png) contributed by @cyfugr
