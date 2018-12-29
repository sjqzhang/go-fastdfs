package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	log "github.com/sjqzhang/seelog"
)

var staticHandler http.Handler
var util = &Common{}
var server = &Server{}
var bind = "0.0.0.0:8080"

const (
	STORE_DIR = "files"

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

type Server struct {
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

func (this *Common) FileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}

func (this *Server) Download(w http.ResponseWriter, r *http.Request) {
	log.Info("download:" + r.RequestURI)
	staticHandler.ServeHTTP(w, r)
}

func (this *Server) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		name := r.PostFormValue("name")
		md5sum := r.PostFormValue("md5")
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

		ns := strings.Split(name, "/")
		if len(ns) > 1 {
			if strings.TrimSpace(ns[len(ns)-1]) != "" {
				name = ns[len(ns)-1]
			} else {
				w.Write([]byte("(error) filename is error"))
				return
			}
		}

		folder := time.Now().Format("2006-01-02")

		folder = fmt.Sprintf(STORE_DIR+"/%s", folder)

		if !util.FileExists(folder) {
			os.Mkdir(folder, 0777)
		}

		outPath := fmt.Sprintf(folder+"/%s", name)

		log.Info(fmt.Sprintf("upload: %s", outPath))

		outFile, err := os.Create(outPath)
		if err != nil {
			log.Error(err)
			w.Write([]byte("(error)fail," + err.Error()))
			return
		}

		io.Copy(outFile, file)
		if md5sum != "" {
			outFile.Seek(0, 0)
			md5h := md5.New()
			io.Copy(md5h, outFile)
			sum := fmt.Sprintf("%x", md5h.Sum(nil))
			if sum != md5sum {
				outFile.Close()
				w.Write([]byte("(error)fail,md5sum error"))
				os.Remove(outPath)
				return

			}
		}
		defer outFile.Close()
		outFile.Sync()

		download_url := fmt.Sprintf("http://%s/%s", r.Host, outPath)
		w.Write([]byte(download_url))

	} else {
		w.Write([]byte("(error)fail,please use post method"))
		return
	}

	useragent := r.Header.Get("User-Agent")

	if useragent != "" && (strings.Contains(useragent, "curl") || strings.Contains(useragent, "wget")) {

	} else {
		http.Redirect(w, r, "/", http.StatusMovedPermanently)
	}
}

func (this *Server) Index(w http.ResponseWriter, r *http.Request) {
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

func init() {
	flag.StringVar(&bind, "b", bind, "bind")
	staticHandler = http.StripPrefix("/"+STORE_DIR, http.FileServer(http.Dir(STORE_DIR)))
}

func main() {

	flag.Parse()

	if logger, err := log.LoggerFromConfigAsBytes([]byte(logConfigStr)); err != nil {
		panic(err)

	} else {
		log.ReplaceLogger(logger)
	}

	if !util.FileExists(STORE_DIR) {
		os.Mkdir(STORE_DIR, 0777)
	}

	http.HandleFunc("/", server.Index)
	http.HandleFunc("/upload", server.Upload)
	http.HandleFunc("/"+STORE_DIR+"/", server.Download)
	fmt.Printf(fmt.Sprintf("Listen:%s\n", bind))
	panic(http.ListenAndServe(bind, nil))
}
