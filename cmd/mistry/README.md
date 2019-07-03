mistry client
====================================================

`mistry` is a CLI for interacting with the mistry server, `mistryd` via its
HTTP API. It can schedule builds and download the resulting build artifacts.

It supports blocking and non-blocking operation mode.

For usage examples and information use `mistry build -h`.


## Development

To build the client, execute the following from the repository root:
```sh
$ make mistry
```

Likewise, to run the tests:
```sh
$ make test-cli
```


License
-------------------------------------------------
mistry is released under the GNU General Public License version 3. See [COPYING](/COPYING).


