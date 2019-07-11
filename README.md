# management-interface

This is a small web server which supports for management and
configuration of Cacophononator devices from the [The Cacophony
Project](https://cacophony.org.nz).

## Installing packr (will be done by make)

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

Building will install packr for you. Make sure $GOPATH/bin is in your $PATH. 
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

Releases are built using TravisCI. To create a release visit the
[repository on Github](https://github.com/TheCacophonyProject/management-interface/releases)
and then follow our [general instructions](https://docs.cacophony.org.nz/home/creating-releases)
for creating a release.

For more about the mechanics of how releases work, see `.travis.yml` and `.goreleaser.yml`.
