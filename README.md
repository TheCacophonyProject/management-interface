# management-interface

This is a small web server which supports for management and
configuration of Cacophononator devices from the [The Cacophony
Project](https://cacophony.org.nz).

## Installing packr

This project uses the [packr](https://github.com/gobuffalo/packr) tool
to embed static resources (e.g. CSS, JS and image files) into the
resulting Go binary.

To install packr from source, run:
```
go get -u github.com/gobuffalo/packr/packr
```

Make sure $GOPATH/bin is in your $PATH.

Alternatively download a stable [prebuilt
release](https://github.com/gobuffalo/packr/releases) of the packr
tool and install it into a directory in your $PATH.

## Building

To build the management server for ARM (to run on a Raspberry Pi):
```
make
```

To build the management server to run on your development machine:
```
make build
```

For either case the resulting executable is `managementd`.

## Running on a Cacophonator

* Build for ARM (run `make`)
* Copy to the Pi: `scp managementd pi@[host]`
* SSH to Pi: `ssh pi@[host]`
* Stop the running management server: `sudo systemctl stop cacophonator-management`
* Run the development version: `sudo ./managementd`

## Releases

Releases are built using TravisCI. To create a release:

* Ensure the desired revision to release is checked out.
* Create an annotated release tag. For example: `git tag -a "v1.4" -m "1.4 release"`
* Push the tag to Github: `git push origin v1.4`
* TravisCI will see the pushed tag, run the tests, create a release package and create a
  [Github Release](https://github.com/TheCacophonyProject/management-interface/releases).

For more about the mechanics of how releases work, see `.travis.yml` and `.goreleaser.yml`.
