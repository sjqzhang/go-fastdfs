package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/eventials/go-tus"
)

var (
	fileName = flag.String("f", "big.bin", "file for upload")
	tusUrl   = flag.String("u", "http://127.0.0.1:8080/group1/big/upload/", "tusd upload server url")
)

func Upload() {
	var (
		f        *os.File
		err      error
		client   *tus.Client
		upload   *tus.Upload
		uploader *tus.Uploader
	)
	f, err = os.Open(*fileName)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	// create the tus client.
	client, err = tus.NewClient(*tusUrl, nil)
	if err != nil {
		log.Println(err)
		return
	}
	// create an upload from a file.
	upload, err = tus.NewUploadFromFile(f)
	if err != nil {
		log.Println(err)
		return
	}
	// create the uploader.
	uploader, err = client.CreateUpload(upload)
	if err != nil {
		log.Println(err)
		return
	}
	// start the uploading process.
	err = uploader.Upload()
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println(uploader.Url())

}
func main() {
	flag.Parse()
	if len(os.Args) < 4 {
		flag.Usage()
		return
	}
	Upload()
}
