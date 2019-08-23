# 断点续传示例

## golang版本{#go}
```go
package main

import (
    "os"
    "fmt"
    "github.com/eventials/go-tus"
)

func main() {
    f, err := os.Open("100m")
    if err != nil {
        panic(err)
    }
    defer f.Close()
    // create the tus client.
    client, err := tus.NewClient("http://10.1.5.9:8080/big/upload/", nil)
    fmt.Println(err)
    // create an upload from a file.
    upload, err := tus.NewUploadFromFile(f)
    fmt.Println(err)
    // create the uploader.
    uploader, err := client.CreateUpload(upload)
    fmt.Println(err)
    // start the uploading process.
   fmt.Println( uploader.Upload())

}

````
[更多客户端请参考](https://github.com/tus)

