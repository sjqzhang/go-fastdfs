package main

import (
	"fmt"
	"net/http"
	"io"
	"os"
)

func Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		file, header, err := r.FormFile("file")
		if err != nil {
			fmt.Printf("FromFileErr")
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
			return
		}

		outPath := fmt.Sprintf("files/%s", header.Filename)
		fmt.Printf("upload: %s\n", outPath)

		outFile, err := os.Create(outPath)
		if err != nil {
			panic(err)
		}
		defer outFile.Close()
		io.Copy(outFile, file)
		outFile.Sync()		
	}
	http.Redirect(w, r, "/", http.StatusMovedPermanently)
}

func Index(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, 
	`<html>
	    <head>
	        <meta charset="utf-8"></meta>
	        <title>Uploader</title>
	    </head>
	    <body>
	        <form action="/upload" method="post" enctype="multipart/form-data">
	            <input type="file" id="file" name="file">
	            <input type="submit" name="submit" value="upload">
	        </form>
	    </body>
	</html>`)
}

func FileExists(fileName string) bool{
	_, err := os.Stat(fileName)
	return err == nil
}

func main() {
    if !FileExists("files") {
        os.Mkdir("files", 0777)
    }

	http.HandleFunc("/", Index)
	http.HandleFunc("/upload", Upload)
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir("files/"))))
	fmt.Printf("Listen:8080\n")
	panic(http.ListenAndServe(":8080", nil))
}
