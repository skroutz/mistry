# Changelog

Breaking changes are prefixed with a "[BREAKING]" label.

## master (unreleased)

### Added

- server: Don't delete build results on docker image build failure [[#75](https://github.com/skroutz/mistry/issues/75)]
- client: Add a `--timeout` option to specify maximum time to wait for a job [[#81](https://github.com/skroutz/mistry/pull/70)]
- server: Introduced a configuration option to limit the number of concurrent builds [[73c44ec](https://github.com/skroutz/mistry/commit/73c44ecc924260ccf61bad220eb26cd51a1f30d6)]
- server: Add `--rebuild` option to rebuild the docker images of a selection of projects ignoring the image cache [[#70](https://github.com/skroutz/mistry/pull/70)]
- client: Add `--rebuild` option to rebuild the docker image ignoring the image cache [[#70](https://github.com/skroutz/mistry/pull/70)]
- client: Add `--clear-target` option to clear target path before fetching
  artifacts [[#63](https://github.com/skroutz/mistry/pull/63)]
- client: Build logs are now displayed when in verbose mode [[#65](https://github.com/skroutz/mistry/pull/65)]
- Asynchronous job scheduling [[#61](https://github.com/skroutz/mistry/pull/61)]
- Web view [[#17](https://github.com/skroutz/mistry/pull/17)]

### Changed

- server: Job parameters are not logged, making the logs less verbose
- **[BREAKING]** Failed build results are no longer cached [[#62](https://github.com/skroutz/mistry/pull/62)]
- **[BREAKING]** client/server: Client and server binaries are renamed to "mistryd" and "mistry" respectively.
  Also project is now go-gettable. [[abbfb58](https://github.com/skroutz/mistry/commit/abbfb58d5a2aaf3eaebf9408d81ec7d459326416)]
- client: default host is now 0.0.0.0

### Fixed

- If a container with the same name exists, we remove it so that the new container
  can run [[#20](https://github.com/skroutz/mistry/issues/20)]
- Streaming log output in web view might occassionally hang [[7c07ca1](7c07ca177639cd6be7f9a860fb39c01370f35779)]

## 0.0.1 (2018-04-12)

First release!
