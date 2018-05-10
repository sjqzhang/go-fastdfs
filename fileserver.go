package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sjqzhang/seelog"
)

const (
	STORE_DIR    = "files"
	logConfigStr = `
<seelog type="asynctimer" asyncinterval="1000" minlevel="trace" maxlevel="error">  
	<outputs formatid="common">  
		<buffered formatid="common" size="1048576" flushperiod="1000">  
			<rollingfile type="size" filename="./log/fileserver.log" maxsize="104857600" maxrolls="10"/>  
		</buffered>
	</outputs>  	  
	 <formats>
		 <format id="common" format="%Date %Time [%LEV] [%File:%Line] [%Func] %Msg%n" />  
	 </formats>  
</seelog>
`
)

type Common struct {
}

func (this *Common) GetUUID() string {

	b := make([]byte, 48)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	id := this.MD5(base64.URLEncoding.EncodeToString(b))
	return fmt.Sprintf("%s-%s-%s-%s-%s", id[0:8], id[8:12], id[12:16], id[16:20], id[20:])

}

func (this *Common) MD5(str string) string {

	md := md5.New()
	md.Write([]byte(str))
	return fmt.Sprintf("%x", md.Sum(nil))
}

func Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		name := r.PostFormValue("name")
		file, header, err := r.FormFile("file")
		if err != nil {
			log.Error(err)
			fmt.Printf("FromFileErr")
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
			return
		}

		if name == "" {
			name = header.Filename
		}

		folder := time.Now().Format("2006-01-02")

		folder = fmt.Sprintf(STORE_DIR+"/%s", folder)

		if !FileExists(folder) {
			os.Mkdir(folder, 0777)
		}

		outPath := fmt.Sprintf(folder+"/%s", name)

		log.Info(fmt.Sprintf("upload: %s", outPath))

		outFile, err := os.Create(outPath)
		if err != nil {
			log.Error(err)
			w.Write([]byte("fail," + err.Error()))
		}
		defer outFile.Close()
		io.Copy(outFile, file)

		//		outFile.Seek(0, 0)
		//		md5h := md5.New()
		//		io.Copy(md5h, outFile)
		//		sum := fmt.Sprintf("%x", md5h.Sum(nil))

		outFile.Sync()

		download_url := fmt.Sprintf("http://%s/%s", r.Host, outPath)
		w.Write([]byte(download_url))

	} else {
		w.Write([]byte("fail,please use post method"))
		return
	}

	useragent := r.Header.Get("User-Agent")

	if useragent != "" && (strings.Contains(useragent, "curl") || strings.Contains(useragent, "wget")) {

	} else {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	}
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

func FileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}

func main() {

	if logger, err := log.LoggerFromConfigAsBytes([]byte(logConfigStr)); err != nil {
		panic(err)

	} else {
		log.ReplaceLogger(logger)
	}

	if !FileExists(STORE_DIR) {
		os.Mkdir(STORE_DIR, 0777)
	}

	http.HandleFunc("/", Index)
	http.HandleFunc("/upload", Upload)
	http.Handle("/files/", http.StripPrefix("/files/", http.FileServer(http.Dir("files/"))))
	fmt.Printf("Listen:8080\n")
	panic(http.ListenAndServe(":8080", nil))
}
