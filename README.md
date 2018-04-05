mistry [![Build Status](https://api.travis-ci.org/skroutz/mistry.svg?branch=master)](https://travis-ci.org/skroutz/mistry)
====================================

mistry executes commands in pre-defined, isolated environments and
makes the results available for later consumption.

It enables fast workflows by employing caching techniques and incremental
builds due to its copy-on-write snapshotting features.

Features include:

- running arbitrary commands inside isolated environments
- providing the build environments as Docker images
- incremental building (using [Btrfs snapshotting](https://en.wikipedia.org/wiki/Btrfs#Subvolumes_and_snapshots))
- caching and reusing build results
- efficient use of disk space due to copy-on-write semantics
- a simple JSON API for interacting with the service
- (future work) a web view for inspecting the progress and result of builds

For more information take a look at the [wiki](https://github.com/skroutz/mistry/wiki).






Status
-------------------------------------------------

mistry project is still in alpha and is not yet recommended for use in
production environments until we reach the 1.x series.






Setup
-------------------------------------------------

(TBA)




Configuration
-------------------------------------------------

The following settings currently exist:

| Setting        | Description           | Default  |
| ------------- |:-------------:| -----:|
| `projects_path` (string)      | The path where project folders are located | "" |
| `build_path` (string)      | The root path where artifacts will be placed       |   "" |
| `mounts` (object{string:string}) |  The paths from the host machine that should be mounted inside the execution containers     |    {} |

For a sample configuration file refer to [`config.sample.json`](config.sample.json).


Usage
--------------------------------------------------

(TBA)






Credits
-------------------------------------------------
mistry is released under the GNU General Public License version 3. See [COPYING](COPYING).
