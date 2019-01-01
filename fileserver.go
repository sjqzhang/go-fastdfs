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
	"path/filepath"
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

var FOLDERS = []string{DATA_DIR, STORE_DIR, CONF_DIR}

var (
	FileName string
	ptr      unsafe.Pointer
)

const (
	STORE_DIR = "files"

	CONF_DIR = "conf"

	DATA_DIR = "data"

	CONST_LEVELDB_FILE_NAME = DATA_DIR + "/fileserver.db"

	CONST_CONF_FILE_NAME = CONF_DIR + "/cfg.json"

	Md5_ERROR_FILE_NAME = "errors.md5"
	FILE_Md5_FILE_NAME  = "files.md5"

	cfgJson = `
{
  "addr": ":8080",
  "peers":["%s"],
  "group":"group1"
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
	Addr  string   `json:"addr"`
	Peers []string `json:"peers"`
	Group string   `json:"group"`
}

func Config() *GloablConfig {
	return (*GloablConfig)(atomic.LoadPointer(&ptr))
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

func (this *Common) WriteFile(path string, data string) bool {
	if err := ioutil.WriteFile(path, []byte(data), 0666); err == nil {
		return true
	} else {
		return false
	}
}

func (this *Common) WriteBinFile(path string, data []byte) bool {
	if err := ioutil.WriteFile(path, data, 0666); err == nil {
		return true
	} else {
		return false
	}
}
func (this *Common) RemoveEmptyDir(pathname string) {

	handlefunc := func(file_path string, f os.FileInfo, err error) error {

		if f.IsDir() {

			files, _ := ioutil.ReadDir(file_path)
			if len(files) == 0 {
				os.Remove(file_path)
			}

		}

		return nil
	}

	fi, _ := os.Stat(pathname)
	if fi.IsDir() {
		filepath.Walk(pathname, handlefunc)
	}

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

func (this *Server) CheckFileAndSendToPeer(filename string, is_force_upload bool) {

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

	if data, err := ioutil.ReadFile(filename); err == nil {
		content := string(data)

		lines := strings.Split(content, "\n")
		for _, line := range lines {
			cols := strings.Split(line, "|")

			if fileInfo, _ := this.GetFileInfoByMd5(cols[0]); fileInfo != nil && fileInfo.Md5 != "" {
				if is_force_upload {
					fileInfo.Peers = []string{}
				}
				this.postFileToPeer(fileInfo, false)
			}
		}

	}

}

func (this *Server) postFileToPeer(fileInfo *FileInfo, write_log bool) {

	for _, u := range Config().Peers {
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

		if info, _ := this.checkPeerFileExist(peer, fileInfo.Md5); info.Md5 != "" {

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
			if write_log {
				msg := fmt.Sprintf("%s|%s\n", fileInfo.Md5, fileInfo.Path+"/"+fileInfo.Name)
				fd, _ := os.OpenFile(STORE_DIR+"/"+time.Now().Format("20060102")+"/"+Md5_ERROR_FILE_NAME, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
				defer fd.Close()
				fd.WriteString(msg)
			}
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

func (this *Server) checkPeerFileExist(peer string, md5sum string) (*FileInfo, error) {

	var (
		err error
	)

	req := httplib.Get(peer + fmt.Sprintf("/check_file_exist?md5=%s", md5sum))

	req.SetTimeout(time.Second*5, time.Second*5)

	var fileInfo FileInfo

	if err = req.ToJSON(&fileInfo); err == nil {
		if fileInfo.Md5 == "" {
			return &FileInfo{}, nil
		}
	}
	return &FileInfo{}, errors.New("file not found")

}

func (this *Server) CheckFileExist(w http.ResponseWriter, r *http.Request) {
	var (
		data     []byte
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

		if data, err = json.Marshal(fileInfo); err == nil {
			w.Write(data)
			return
		}
	}

	data, _ = json.Marshal(FileInfo{})

	w.Write(data)

}

func (this *Server) Sync(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()

	date := ""

	force := ""
	is_force_upload := false

	if len(r.Form["force"]) > 0 {
		force = r.Form["force"][0]
	}

	if force != "" {
		is_force_upload = true
	}

	if len(r.Form["date"]) > 0 {
		date = r.Form["date"][0]
	} else {
		w.Write([]byte("require paramete date , date?=20181230"))
		return
	}
	date = strings.Replace(date, ".", "", -1)
	filename := STORE_DIR + "/" + date + "/" + Md5_ERROR_FILE_NAME

	if this.util.FileExists(filename) {

		go this.CheckFileAndSendToPeer(filename, is_force_upload)
	}
	filename = STORE_DIR + "/" + date + "/" + FILE_Md5_FILE_NAME

	if this.util.FileExists(filename) {

		go this.CheckFileAndSendToPeer(filename, is_force_upload)
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

		SaveUploadFile := func(file multipart.File, header *multipart.FileHeader, path string, name string) (*os.File, string, string, error) {

			defer file.Close()
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

			if path != "" && strings.HasPrefix(path, STORE_DIR) {
				folder = path
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

		defer uploadFile.Close()

		if uploadFile, folder, name, err = SaveUploadFile(file, header, path, name); uploadFile != nil {

			if md5sum == "" {

				md5sum = util.GetFileMd5(uploadFile)

				if info, _ := this.GetFileInfoByMd5(md5sum); info != nil && info.Path != "" && info.Path != folder {
					uploadFile.Close()
					os.Remove(folder + "/" + name)
					download_url := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+info.Path+"/"+info.Name)
					w.Write([]byte(download_url))
					return
				}

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
			download_url := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+info.Path+"/"+info.Name)
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

			go this.postFileToPeer(&fileInfo, true)

		}

		UploadToPeer(md5sum, name, folder)

		msg := fmt.Sprintf("%s|%s\n", md5sum, folder+"/"+name)
		fd, _ := os.OpenFile(STORE_DIR+"/"+time.Now().Format("20060102")+"/"+FILE_Md5_FILE_NAME, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
		defer fd.Close()
		fd.WriteString(msg)

		download_url := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+folder+"/"+name)
		w.Write([]byte(download_url))
		return

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
	flag.Parse()

	if !util.FileExists(CONST_CONF_FILE_NAME) {

		peer := "http://" + util.GetPulicIP() + ":8080"

		cfg := fmt.Sprintf(cfgJson, peer)

		util.WriteFile(CONST_CONF_FILE_NAME, cfg)
	}

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

	ParseConfig(CONST_CONF_FILE_NAME)

	staticHandler = http.StripPrefix("/"+Config().Group+"/"+STORE_DIR+"/", http.FileServer(http.Dir(STORE_DIR)))

	initComponent()
}

func initComponent() {
	ip := util.GetPulicIP()
	ex, _ := regexp.Compile("\\d+\\.\\d+\\.\\d+\\.\\d+")
	var peers []string
	for _, peer := range Config().Peers {
		if util.Contains(ip, ex.FindAllString(peer, -1)) {
			continue
		}
		if strings.HasPrefix(peer, "http") {
			peers = append(peers, peer)
		} else {
			peers = append(peers, "http://"+peer)
		}
	}
	Config().Peers = peers

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
			server.CheckFileAndSendToPeer("", false)
			time.Sleep(time.Second * 60)
			util.RemoveEmptyDir(STORE_DIR)
		}
	}()

	http.HandleFunc("/", server.Index)
	http.HandleFunc("/check_file_exist", server.CheckFileExist)
	http.HandleFunc("/upload", server.Upload)
	http.HandleFunc("/sync", server.Sync)
	http.HandleFunc("/"+Config().Group+"/"+STORE_DIR+"/", server.Download)
	fmt.Println("Listen on " + Config().Addr)
	panic(http.ListenAndServe(Config().Addr, new(HttpHandler)))

}
