# Bundler

The project contains a Dockerfile and a docker-entrypoint that describe
it. We recommend reading the [source](_examples/bundler-cache) of these files as a form of
documentation for developing other projects.


1. Create the target directory for the artifacts

```sh
$ mdkir /tmp/gems
```

2. Schedule a job

```sh
$ ./mistry build --host localhost               \
--project bundler                               \
--group my-group                                \
--target /tmp/gems                              \
--transport rsync                               \
--clear-target                                  \
--verbose                                       \
--                                              \
--Gemfile=@/tmp/mistry/projects/bundler/Gemfile \
--Gemfile.lock=@/tmp/mistry/projects/bundler/Gemfile.lock
```

3. (optional) Observe the building process from the webview

Visit http://localhost:8462/job/bundler/:job_id


4. When the job has finished building the artifacts will by copied to
the target directory(`/tmp/gems` in our case)

```sh
$ tree -L 3 /tmp/gems

/tmp/gems
└── ruby
    └── 2.3.0
        ├── build_info
        ├── cache
        ├── doc
        ├── extensions
        ├── gems
        └── specifications
```
