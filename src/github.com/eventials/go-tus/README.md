# go-tus [![Build Status](https://travis-ci.org/eventials/go-tus.svg?branch=master)](https://travis-ci.org/eventials/go-tus) [![Go Report Card](https://goreportcard.com/badge/github.com/eventials/go-tus)](https://goreportcard.com/report/github.com/eventials/go-tus) [![GoDoc](https://godoc.org/github.com/eventials/go-tus?status.svg)](http://godoc.org/github.com/eventials/go-tus)

A pure Go client for the [tus resumable upload protocol](http://tus.io/)

## Example

```go
package main

import (
    "os"
    "github.com/eventials/go-tus"
)

func main() {
    f, err := os.Open("my-file.txt")

    if err != nil {
        panic(err)
    }

    defer f.Close()

    // create the tus client.
    client, _ := tus.NewClient("https://tus.example.org/files", nil)

    // create an upload from a file.
    upload, _ := tus.NewUploadFromFile(f)

    // create the uploader.
    uploader, _ := client.CreateUpload(upload)

    // start the uploading process.
    uploader.Upload()
}
```

## Features

> This is not a full protocol client implementation.

Checksum, Termination and Concatenation extensions are not implemented yet.

This client allows to resume an upload if a Store is used.

## Built in Store

Store is used to map an upload's fingerprint with the corresponding upload URL.

| Name | Backend | Dependencies |
|:----:|:-------:|:------------:|
| MemoryStore  | In-Memory | None |
| LeveldbStore | LevelDB   | [goleveldb](https://github.com/syndtr/goleveldb) |

## Future Work

- [ ] SQLite store
- [ ] Redis store
- [ ] Memcached store
- [ ] Checksum extension
- [ ] Termination extension
- [ ] Concatenation extension
