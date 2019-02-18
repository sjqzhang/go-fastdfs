# tusd

<img alt="Tus logo" src="https://github.com/tus/tus.io/blob/master/assets/img/tus1.png?raw=true" width="30%" align="right" />

> **tus** is a protocol based on HTTP for *resumable file uploads*. Resumable
> means that an upload can be interrupted at any moment and can be resumed without
> re-uploading the previous data again. An interruption may happen willingly, if
> the user wants to pause, or by accident in case of an network issue or server
> outage.

tusd is the official reference implementation of the [tus resumable upload
protocol](http://www.tus.io/protocols/resumable-upload.html). The protocol
specifies a flexible method to upload files to remote servers using HTTP.
The special feature is the ability to pause and resume uploads at any
moment allowing to continue seamlessly after e.g. network interruptions.

It is capable of accepting uploads with arbitrary sizes and storing them locally
on disk, on Google Cloud Storage or on AWS S3 (or any other S3-compatible
storage system). Due to its modularization and extensibility, support for
nearly any other cloud provider could easily be added to tusd.

**Protocol version:** 1.0.0

## Getting started

### Download pre-builts binaries (recommended)

You can download ready-to-use packages including binaries for OS X, Linux and
Windows in various formats of the
[latest release](https://github.com/tus/tusd/releases/latest).

### Compile from source

The only requirement for building tusd is [Go](http://golang.org/doc/install) 1.5 or newer.
If you meet this criteria, you can clone the git repository, install the remaining
dependencies and build the binary:

```bash
git clone git@github.com:tus/tusd.git
cd tusd

go get -u github.com/aws/aws-sdk-go/...
go get -u github.com/prometheus/client_golang/prometheus

go build -o tusd cmd/tusd/main.go
```

## Running tusd

Start the tusd upload server is as simple as invoking a single command. For example, following
snippet demonstrates how to start a tusd process which accepts tus uploads at
`http://localhost:1080/files/` and stores them locally in the `./data` directory:

```
$ tusd -dir ./data
[tusd] Using './data' as directory storage.
[tusd] Using 0.00MB as maximum size.
[tusd] Using 0.0.0.0:1080 as address to listen.
[tusd] Using /files/ as the base path.
[tusd] Using /metrics as the metrics path.
```

Alternatively, if you want to store the uploads on an AWS S3 bucket, you only have to specify
the bucket and provide the corresponding access credentials and region information using
environment variables (if you want to use a S3-compatible store, use can use the `-s3-endpoint`
option):

```
$ export AWS_ACCESS_KEY_ID=xxxxx
$ export AWS_SECRET_ACCESS_KEY=xxxxx
$ export AWS_REGION=eu-west-1
$ tusd -s3-bucket my-test-bucket.com
[tusd] Using 's3://my-test-bucket.com' as S3 bucket for storage.
[tusd] Using 0.00MB as maximum size.
[tusd] Using 0.0.0.0:1080 as address to listen.
[tusd] Using /files/ as the base path.
[tusd] Using /metrics as the metrics path.
```

tusd is also able to read the credentials automatically from a shared credentials file (~/.aws/credentials) as described in https://github.com/aws/aws-sdk-go#configuring-credentials.

Furthermore, tusd also has support for storing uploads on Google Cloud Storage. In order to
enable this feature, supply the path to your account file containing the necessary credentials:

```
$ export GCS_SERVICE_ACCOUNT_FILE=./account.json
$ tusd -gcs-bucket my-test-bucket.com
[tusd] Using 'gcs://my-test-bucket.com' as GCS bucket for storage.
[tusd] Using 0.00MB as maximum size.
[tusd] Using 0.0.0.0:1080 as address to listen.
[tusd] Using /files/ as the base path.
[tusd] Using /metrics as the metrics path.
```

Besides these simple examples, tusd can be easily configured using a variety of command line
options:

```
$ tusd -help
Usage of tusd:
  -base-path string
    	Basepath of the HTTP server (default "/files/")
  -behind-proxy
    	Respect X-Forwarded-* and similar headers which may be set by proxies
  -dir string
    	Directory to store uploads in (default "./data")
  -expose-metrics
    	Expose metrics about tusd usage (default true)
  -gcs-bucket string
    	Use Google Cloud Storage with this bucket as storage backend (requires the GCS_SERVICE_ACCOUNT_FILE environment variable to be set)
  -hooks-dir string
    	Directory to search for available hooks scripts
  -hooks-http string
    	An HTTP endpoint to which hook events will be sent to
  -hooks-http-backoff int
    	Number of seconds to wait before retrying each retry (default 1)
  -hooks-http-retry int
    	Number of times to retry on a 500 or network timeout (default 3)
  -host string
    	Host to bind HTTP server to (default "0.0.0.0")
  -max-size int
    	Maximum size of a single upload in bytes
  -metrics-path string
    	Path under which the metrics endpoint will be accessible (default "/metrics")
  -port string
    	Port to bind HTTP server to (default "1080")
  -s3-bucket string
    	Use AWS S3 with this bucket as storage backend (requires the AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY and AWS_REGION environment variables to be set)
  -s3-endpoint string
    	Endpoint to use S3 compatible implementations like minio (requires s3-bucket to be pass)
  -store-size int
    	Size of space allowed for storage
  -timeout int
    	Read timeout for connections in milliseconds.  A zero value means that reads will not timeout (default 30000)
  -version
    	Print tusd version information
```

## Monitoring tusd

tusd exposes metrics at the `/metrics` endpoint ([example](https://master.tus.io/metrics)) in the [Prometheus Text Format](https://prometheus.io/docs/instrumenting/exposition_formats/#text-based-format). This allows you to hook up Prometheus or any other compatible service to your tusd instance and let it monitor tusd. Alternatively, there are many [parsers and client libraries](https://prometheus.io/docs/instrumenting/clientlibs/) available for consuming the metrics format directly.

The endpoint contains details about Go's internals, general HTTP numbers and details about tus uploads and tus-specific errors. It can be completely disabled using the `-expose-metrics false` flag and it's path can be changed using the `-metrics-path /my/numbers` flag.

## Using tusd manually

Besides from running tusd using the provided binary, you can embed it into
your own Go program:

```go
package main

import (
	"fmt"
	"net/http"

	"github.com/tus/tusd"
	"github.com/tus/tusd/filestore"
)

func main() {
	// Create a new FileStore instance which is responsible for
	// storing the uploaded file on disk in the specified directory.
	// This path _must_ exist before tusd will store uploads in it.
	// If you want to save them on a different medium, for example
	// a remote FTP server, you can implement your own storage backend
	// by implementing the tusd.DataStore interface.
	store := filestore.FileStore{
		Path: "./uploads",
	}

	// A storage backend for tusd may consist of multiple different parts which
	// handle upload creation, locking, termination and so on. The composer is a
	// place where all those separated pieces are joined together. In this example
	// we only use the file store but you may plug in multiple.
	composer := tusd.NewStoreComposer()
	store.UseIn(composer)

	// Create a new HTTP handler for the tusd server by providing a configuration.
	// The StoreComposer property must be set to allow the handler to function.
	handler, err := tusd.NewHandler(tusd.Config{
		BasePath:      "/files/",
		StoreComposer: composer,
	})
	if err != nil {
		panic(fmt.Errorf("Unable to create handler: %s", err))
	}

	// Right now, nothing has happened since we need to start the HTTP server on
	// our own. In the end, tusd will start listening on and accept request at
	// http://localhost:8080/files
	http.Handle("/files/", http.StripPrefix("/files/", handler))
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		panic(fmt.Errorf("Unable to listen: %s", err))
	}
}

```

Please consult the [online documentation](https://godoc.org/github.com/tus/tusd)
for more details about tusd's APIs and its sub-packages.

## Implementing own storages

The tusd server is built to be as flexible as possible and to allow the use
of different upload storage mechanisms. By default the tusd binary includes
[`filestore`](https://godoc.org/github.com/tus/tusd/filestore) which will save every upload
to a specific directory on disk.

If you have different requirements, you can build your own storage backend
which will save the files to S3, a remote FTP server or similar. Doing so
is as simple as implementing the [`tusd.DataStore`](https://godoc.org/github.com/tus/tusd/#DataStore)
interface and using the new struct in the [configuration object](https://godoc.org/github.com/tus/tusd/#Config).
Please consult the documentation about detailed information about the
required methods.

## Packages

This repository does not only contain the HTTP server's code but also other
useful tools:

* [**s3store**](https://godoc.org/github.com/tus/tusd/s3store): A storage backend using AWS S3
* [**filestore**](https://godoc.org/github.com/tus/tusd/filestore): A storage backend using the local file system
* [**gcsstore**](https://godoc.org/github.com/tus/tusd/gcsstore): A storage backend using Google cloud storage
* [**memorylocker**](https://godoc.org/github.com/tus/tusd/memorylocker): An in-memory locker for handling concurrent uploads
* [**consullocker**](https://godoc.org/github.com/tus/tusd/consullocker): A locker using the distributed Consul service
* [**etcd3locker**](https://godoc.org/github.com/tus/tusd/etcd3locker): A locker using the distributed KV etcd3 store
* [**limitedstore**](https://godoc.org/github.com/tus/tusd/limitedstore): A storage wrapper limiting the total used space for uploads

## Running the testsuite

[![Build Status](https://travis-ci.org/tus/tusd.svg?branch=master)](https://travis-ci.org/tus/tusd)
[![Build status](https://ci.appveyor.com/api/projects/status/2y6fa4nyknoxmyc8/branch/master?svg=true)](https://ci.appveyor.com/project/Acconut/tusd/branch/master)

```bash
go test -v ./...
```

## FAQ

### How can I access tusd using HTTPS?

The tusd binary, once executed, listens on the provided port for only non-encrypted HTTP requests and *does not accept* HTTPS connections. This decision has been made to limit the functionality inside this repository which has to be developed, tested and maintained. If you want to send requests to tusd in a secure fashion - what we absolutely encourage, we recommend you to utilize a reverse proxy in front of tusd which accepts incoming HTTPS connections and forwards them to tusd using plain HTTP. More information about this topic, including sample configurations for Nginx and Apache, can be found in [issue #86](https://github.com/tus/tusd/issues/86#issuecomment-269569077) and in the [Apache example configuration](/docs/apache2.conf).

### Can I run tusd behind a reverse proxy?

Yes, it is absolutely possible to do so. Firstly, you should execute the tusd binary using the `-behind-proxy` flag indicating it to pay attention to special headers which are only relevant when used in conjunction with a proxy. Furthermore, there are additional details which should be kept in mind, depending on the used software:

- *Disable request buffering.* Nginx, for example, reads the entire incoming HTTP request, including its body, before sending it to the backend, by default. This behavior defeats the purpose of resumability where an upload is processed while it's being transfered. Therefore, such as feature should be disabled.

- *Adjust maximum request size.* Some proxies have default values for how big a request may be in order to protect your services. Be sure to check these settings to match the requirements of your application.

- *Forward hostname and scheme.* If the proxy rewrites the request URL, the tusd server does not know the original URL which was used to reach the proxy. This behavior can lead to situations, where tusd returns a redirect to a URL which can not be reached by the client. To avoid this confusion, you can explicitly tell tusd which hostname and scheme to use by supplying the `X-Forwarded-Host` and `X-Forwarded-Proto` headers.

Explicit examples for the above points can be found in the [Nginx configuration](/docs/nginx.conf) which is used to power the [master.tus.io](https://master.tus.io) instace.

### Can I run custom verification/authentication checks before an upload begins?

Yes, this is made possible by the [hook system](/docs/hooks.md) inside the tusd binary. It enables custom routines to be executed when certain events occurs, such as a new upload being created which can be handled by the `pre-create` hook. Inside the corresponding hook file, you can run your own validations against the provided upload metadata to determine whether the action is actually allowed or should be rejected by tusd. Please have a look at the [corresponding documentation](docs/hooks.md#pre-create) for a more detailed explanation.

### Can I run tusd inside a VM/Vagrant/VirtualBox?

Yes, you can absolutely do so without any modifications. However, there is one known problem: If you are using tusd inside VirtualBox (the default provider for Vagrant) and are storing the files inside a shared/synced folder, you might get TemporaryErrors (Lockfile created, but doesn't exist) when trying to upload. This happens because shared folders do not support symbolic links which are necessary for tusd. Please use another non-shared folder for storing files (see https://github.com/tus/tusd/issues/201).

### I am getting TemporaryErrors (Lockfile created, but doesn't exist)! What can I do?

This error should only occur when you are using tusd inside VirtualBox. Please see the answer above for more details on when this can happen and how to avoid it.

## License

This project is licensed under the MIT license, see `LICENSE.txt`.
