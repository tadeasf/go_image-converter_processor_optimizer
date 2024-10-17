# Fast CLI batch image multi-converter written in GO

## Important shell commands

### Build binary optimized for production

First set  these env variables:

```sh
    export CGO_ENABLED=0
    export GOOS=linux  # or darwin for macOS, windows for Windows
    export GOARCH=amd64  # or arm64 for ARM-based systems
```

then you are ready to build your prod binary:

```sh
    go build -o go_image_converter \
    -ldflags="-s -w" \
    -trimpath \
    -tags=netgo \
    -a \
    src/main.go
```


### Docs


Let's break down used flags:

#### -ldflags="-s -w":

* -s: Omits the symbol table and debug information
* -w: Omits the DWARF symbol table

#### -trimpath: Removes all file system paths from the resulting executable

* -tags=netgo: Uses pure Go DNS resolver instead of the system's C library
* -a: Forces rebuilding of packages that are already up-to-date