package main

import (
	"crypto/md5"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net"
	"net/http"
	"reflect"
	"runtime/debug"

	"os"
	"regexp"
	"strings"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/astaxie/beego/httplib"
	"github.com/syndtr/goleveldb/leveldb"

	log "github.com/sjqzhang/seelog"
)

var staticHandler http.Handler
var util = &Common{}
var server = &Server{}

var logacc log.LoggerInterface

var bind = "0.0.0.0:8080"

var peers = []string{}
var _peers = ""
var FOLDERS = []string{DATA_DIR, STORE_DIR}

var (
	FileName string
	ptr      unsafe.Pointer
)

const (
	STORE_DIR = "files"

	CONF_DIR = "conf"

	DATA_DIR = "data"

	CONST_LEVELDB_FILE_NAME = DATA_DIR + "/fileserver.db"

	Md5_ERROR_FILE_NAME = "errors.md5"
	FILE_Md5_FILE_NAME  = "files.md5"

	cfgJson = `
	{
      "addr": ":9160"
	}
	
	`

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

	logAccessConfigStr = `
<seelog type="asynctimer" asyncinterval="1000" minlevel="trace" maxlevel="error">  
	<outputs formatid="common">  
		<buffered formatid="common" size="1048576" flushperiod="1000">  
			<rollingfile type="size" filename="./log/access.log" maxsize="104857600" maxrolls="10"/>  
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
	db   *leveldb.DB
	util *Common
}

type FileInfo struct {
	Name  string
	Path  string
	Md5   string
	Peers []string
}

type GloablConfig struct {
}

func ParseConfig(filePath string) {
	var (
		data []byte
	)

	if filePath == "" {
		data = []byte(strings.TrimSpace(cfgJson))
	} else {
		file, err := os.Open(filePath)
		if err != nil {
			panic(fmt.Sprintln("open file path:", filePath, "error:", err))
		}

		defer file.Close()

		FileName = filePath

		data, err = ioutil.ReadAll(file)
		if err != nil {
			panic(fmt.Sprintln("file path:", filePath, " read all error:", err))
		}
	}

	var c GloablConfig
	if err := json.Unmarshal(data, &c); err != nil {
		panic(fmt.Sprintln("file path:", filePath, "json unmarshal error:", err))
	}

	log.Info(c)

	atomic.StorePointer(&ptr, unsafe.Pointer(&c))

	log.Info("config parse success")
}

func (this *Common) GetUUID() string {

	b := make([]byte, 48)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	id := this.MD5(base64.URLEncoding.EncodeToString(b))
	return fmt.Sprintf("%s-%s-%s-%s-%s", id[0:8], id[8:12], id[12:16], id[16:20], id[20:])

}
func (this *Common) GetPulicIP() string {
	conn, _ := net.Dial("udp", "8.8.8.8:80")
	defer conn.Close()
	localAddr := conn.LocalAddr().String()
	idx := strings.LastIndex(localAddr, ":")
	return localAddr[0:idx]
}

func (this *Common) MD5(str string) string {

	md := md5.New()
	md.Write([]byte(str))
	return fmt.Sprintf("%x", md.Sum(nil))
}

func (this *Common) GetFileMd5(file *os.File) string {
	file.Seek(0, 0)
	md5h := md5.New()
	io.Copy(md5h, file)
	sum := fmt.Sprintf("%x", md5h.Sum(nil))
	return sum
}

func (this *Common) Contains(obj interface{}, arrayobj interface{}) bool {
	targetValue := reflect.ValueOf(arrayobj)
	switch reflect.TypeOf(arrayobj).Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < targetValue.Len(); i++ {
			if targetValue.Index(i).Interface() == obj {
				return true
			}
		}
	case reflect.Map:
		if targetValue.MapIndex(reflect.ValueOf(obj)).IsValid() {
			return true
		}
	}
	return false
}

func (this *Common) FileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}

func (this *Common) GetClientIp(r *http.Request) string {

	client_ip := ""
	headers := []string{"X_Forwarded_For", "X-Forwarded-For", "X-Real-Ip",
		"X_Real_Ip", "Remote_Addr", "Remote-Addr"}
	for _, v := range headers {
		if _v, ok := r.Header[v]; ok {
			if len(_v) > 0 {
				client_ip = _v[0]
				break
			}
		}
	}
	if client_ip == "" {
		clients := strings.Split(r.RemoteAddr, ":")
		client_ip = clients[0]
	}
	return client_ip

}

func (this *Server) Download(w http.ResponseWriter, r *http.Request) {
	log.Info("download:" + r.RequestURI)
	staticHandler.ServeHTTP(w, r)
}

func (this *Server) GetServerURI(r *http.Request) string {
	return fmt.Sprintf("http://%s/", r.Host)
}

func (this *Server) CheckFileAndSendToPeer(filename string) {

	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("CheckFileAndSendToPeer")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()

	if filename == "" {
		filename = STORE_DIR + "/" + time.Now().Format("20060102") + "/" + Md5_ERROR_FILE_NAME
	}

	CheckPeer := func(fileInfo *FileInfo) {

		for _, p := range peers {

			if this.util.Contains(p, fileInfo.Peers) {
				continue
			}

			req := httplib.Get(p + fmt.Sprintf("/check_file_exist?md5=%s", fileInfo.Md5))

			req.SetTimeout(time.Second*5, time.Second*5)

			if str, err := req.String(); err == nil && !strings.HasPrefix(str, "http") {

				this.postFileToPeer(fileInfo)

			}
		}

	}

	if data, err := ioutil.ReadFile(filename); err == nil {
		content := string(data)
		lines := strings.Split(content, "\n")
		for _, line := range lines {
			cols := strings.Split(line, "|")
			if fileInfo, err := this.GetFileInfoByMd5(cols[0]); err == nil && fileInfo != nil {
				CheckPeer(fileInfo)
			}
		}

	}

}

func (this *Server) postFileToPeer(fileInfo *FileInfo) {

	for _, u := range peers {
		peer := ""
		if !strings.HasPrefix(u, "http://") {
			u = "http://" + u
		}
		peer = u
		if fileInfo.Peers == nil {
			fileInfo.Peers = []string{}
		}
		if this.util.Contains(peer, fileInfo.Peers) {
			continue
		}
		if !this.util.FileExists(fileInfo.Path + "/" + fileInfo.Name) {
			continue
		}
		u = fmt.Sprintf("%s/%s", u, "upload")
		b := httplib.Post(u)
		b.SetTimeout(time.Second*5, time.Second*5)
		b.Header("path", fileInfo.Path)
		b.Param("name", fileInfo.Name)
		b.Param("md5", fileInfo.Md5)
		b.PostFile("file", fileInfo.Path+"/"+fileInfo.Name)
		str, err := b.String()

		if !strings.HasPrefix(str, "http://") {
			msg := fmt.Sprintf("%s|%s\n", fileInfo.Md5, fileInfo.Path+"/"+fileInfo.Name)
			fd, _ := os.OpenFile(STORE_DIR+"/"+time.Now().Format("20060102")+"/"+Md5_ERROR_FILE_NAME, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
			defer fd.Close()
			fd.WriteString(msg)
		} else {

			if !this.util.Contains(peer, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				if data, err := json.Marshal(fileInfo); err == nil {
					this.db.Put([]byte(fileInfo.Md5), data, nil)
				}
			}

		}
		log.Info(str)
		if err != nil {
			fmt.Println(err)
			log.Error(err)
		}

	}

}

func (this *Server) GetFileInfoByMd5(md5sum string) (*FileInfo, error) {
	var (
		data     []byte
		err      error
		fileInfo FileInfo
	)

	if data, err = this.db.Get([]byte(md5sum), nil); err != nil {
		return nil, err
	} else {
		if err = json.Unmarshal(data, &fileInfo); err == nil {
			return &fileInfo, nil
		} else {
			return nil, err
		}
	}
}

func (this *Server) CheckFileExist(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		fileInfo *FileInfo
	)
	r.ParseForm()
	md5sum := ""
	if len(r.Form["md5"]) > 0 {
		md5sum = r.Form["md5"][0]
	} else {
		return
	}

	if fileInfo, err = this.GetFileInfoByMd5(md5sum); fileInfo != nil {
		w.Write([]byte(this.GetServerURI(r) + fileInfo.Path + "/" + fileInfo.Name))
		return
	} else {
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}

}

func (this *Server) Sync(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	date := ""

	if len(r.Form["date"]) > 0 {
		date = r.Form["date"][0]
	} else {
		w.Write([]byte("require paramete date , date?=20181230"))
		return
	}
	date = strings.Replace(date, ".", "", -1)
	filename := STORE_DIR + "/" + date + "/" + Md5_ERROR_FILE_NAME

	if this.util.FileExists(filename) {

		go this.CheckFileAndSendToPeer(filename)
	}
	filename = STORE_DIR + "/" + date + "/" + FILE_Md5_FILE_NAME

	if this.util.FileExists(filename) {

		go this.CheckFileAndSendToPeer(filename)
	}
	w.Write([]byte("job is running"))
}

func (this *Server) Upload(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		name := r.PostFormValue("name")
		path := r.Header.Get("Path")
		md5sum := r.PostFormValue("md5")
		file, header, err := r.FormFile("file")

		if err != nil {
			log.Error(err)
			fmt.Printf("FromFileErr")
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
			return
		}

		SaveUploadFile := func(file multipart.File, header *multipart.FileHeader) (*os.File, string, string, error) {

			if name == "" {
				name = header.Filename
			}

			ns := strings.Split(name, "/")
			if len(ns) > 1 {
				if strings.TrimSpace(ns[len(ns)-1]) != "" {
					name = ns[len(ns)-1]
				} else {
					return nil, "", "", errors.New("(error) filename is error")
				}
			}

			folder := time.Now().Format("20060102/15/04")

			folder = fmt.Sprintf(STORE_DIR+"/%s", folder)

			if len(path) == 14 {
				if ok, _ := regexp.MatchString("\\d{8}/\\d{2}/\\d{2}", path); ok {
					folder = path
				}
			}

			if !util.FileExists(folder) {
				os.MkdirAll(folder, 0777)
			}

			outPath := fmt.Sprintf(folder+"/%s", name)

			log.Info(fmt.Sprintf("upload: %s", outPath))

			outFile, err := os.Create(outPath)
			if err != nil {
				log.Error(err)
				return nil, "", "", errors.New("(error)fail," + err.Error())

			}

			if _, err := io.Copy(outFile, file); err != nil {
				log.Error(err)
				return nil, "", "", errors.New("(error)fail," + err.Error())
			}

			return outFile, folder, name, nil

		}

		var uploadFile *os.File
		var folder string

		if uploadFile, folder, name, err = SaveUploadFile(file, header); uploadFile != nil {

			if md5sum == "" {

				md5sum = util.GetFileMd5(uploadFile)

			} else {

				v := util.GetFileMd5(uploadFile)
				if v != md5sum {
					w.Write([]byte("(error)fail,md5sum error"))
					os.Remove(folder + "/" + name)
					return

				}

			}

		} else {
			w.Write([]byte("(error)" + err.Error()))
			log.Error(err)
			return

		}

		CheckFileExist := func(md5sum string) (*FileInfo, error) {
			if md5sum != "" {
				if data, err := this.db.Get([]byte(md5sum), nil); err == nil {
					var fileInfo FileInfo
					if err := json.Unmarshal(data, &fileInfo); err == nil {
						return &fileInfo, nil
					}
				}
			}
			return nil, errors.New("File Not found")
		}

		if info, err := CheckFileExist(md5sum); err == nil {
			download_url := fmt.Sprintf("http://%s/%s", r.Host, info.Path+"/"+info.Name)
			w.Write([]byte(download_url))
			return
		}

		UploadToPeer := func(md5sum string, name string, path string) {

			fileInfo := FileInfo{
				Name:  name,
				Md5:   md5sum,
				Path:  path,
				Peers: []string{this.GetServerURI(r)},
			}
			if v, err := json.Marshal(fileInfo); err == nil {

				this.db.Put([]byte(md5sum), v, nil)

			} else {
				log.Error(err)
			}

			go this.postFileToPeer(&fileInfo)

		}

		UploadToPeer(md5sum, name, folder)

		msg := fmt.Sprintf("%s|%s\n", md5sum, folder+"/"+name)
		fd, _ := os.OpenFile(STORE_DIR+"/"+time.Now().Format("20060102")+"/"+FILE_Md5_FILE_NAME, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		defer fd.Close()
		fd.WriteString(msg)

		download_url := fmt.Sprintf("http://%s/%s", r.Host, folder+"/"+name)
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
	server.util = util
	for _, folder := range FOLDERS {
		os.Mkdir(folder, 0777)
	}
	flag.StringVar(&bind, "b", bind, "bind")
	flag.StringVar(&_peers, "peers", _peers, "peers")

	flag.Parse()

	if logger, err := log.LoggerFromConfigAsBytes([]byte(logConfigStr)); err != nil {
		panic(err)

	} else {
		log.ReplaceLogger(logger)
	}

	if _logacc, err := log.LoggerFromConfigAsBytes([]byte(logAccessConfigStr)); err == nil {
		logacc = _logacc
		log.Info("succes init log access")

	} else {
		log.Error(err.Error())
	}

	staticHandler = http.StripPrefix("/"+STORE_DIR, http.FileServer(http.Dir(STORE_DIR)))
	initComponent()
}

func initComponent() {
	ip := util.GetPulicIP()
	ex, _ := regexp.Compile("\\d+\\.\\d+\\.\\d+\\.\\d+")
	if _peers != "" {
		for _, peer := range strings.Split(_peers, ",") {
			if util.Contains(ip, ex.FindAllString(peer, -1)) {
				continue
			}
			if strings.HasPrefix(peer, "http") {
				peers = append(peers, peer)
			} else {
				peers = append(peers, "http://"+peer)
			}
		}
	}

	db, err := leveldb.OpenFile(CONST_LEVELDB_FILE_NAME, nil)
	if err != nil {
		log.Error(err)
		panic(err)
	}
	server.db = db

}

type HttpHandler struct {
}

func (HttpHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	status_code := "200"
	defer func(t time.Time) {
		logStr := fmt.Sprintf("[Access] %s | %v | %s | %s | %s | %s |%s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			res.Header(),
			time.Since(t).String(),
			util.GetClientIp(req),
			req.Method,
			status_code,
			req.RequestURI,
		)

		logacc.Info(logStr)
	}(time.Now())

	defer func() {
		if err := recover(); err != nil {
			status_code = "500"
			res.WriteHeader(500)
			print(err)
			buff := debug.Stack()
			log.Error(err)
			log.Error(string(buff))

		}
	}()

	http.DefaultServeMux.ServeHTTP(res, req)
}

func main() {

	if !util.FileExists(STORE_DIR) {
		os.Mkdir(STORE_DIR, 0777)
	}

	go func() {
		for {
			server.CheckFileAndSendToPeer("")
			time.Sleep(time.Second * 60)
		}
	}()

	http.HandleFunc("/", server.Index)
	http.HandleFunc("/check_file_exist", server.CheckFileExist)
	http.HandleFunc("/upload", server.Upload)
	http.HandleFunc("/sync", server.Sync)
	http.HandleFunc("/"+STORE_DIR+"/", server.Download)
	fmt.Printf(fmt.Sprintf("Listen:%s\n", bind))
	fmt.Println(fmt.Sprintf("peers:%v", peers))
	panic(http.ListenAndServe(bind, new(HttpHandler)))

	//	folder := time.Now().Format("20060102/15/04")

	//	ex, _ := regexp.Compile("\\d{8}/\\d{2}/\\d{2}")

	//	ex, _ := regexp.Compile("\\d+\\.\\d+\\.\\d+\\.\\d+")
	//	ip := util.GetPulicIP()

	//	fmt.Println(ex.FindAllString(ip, -1), folder)

}
