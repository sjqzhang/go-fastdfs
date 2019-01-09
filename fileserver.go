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
	"net/smtp"
	"net/url"
	"os/signal"
	"path"
	"path/filepath"
	"reflect"
	"runtime/debug"
	"strconv"
	"syscall"
	//	"strconv"
	"sync"

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
var statMap = &CommonMap{m: make(map[string]interface{})}

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

	CONST_STAT_FILE_NAME = DATA_DIR + "/stat.json"

	CONST_CONF_FILE_NAME = CONF_DIR + "/cfg.json"

	CONST_STAT_FILE_COUNT_KEY = "fileCount"

	CONST_STAT_FILE_TOTAL_SIZE_KEY = "totalSize"

	CONST_Md5_ERROR_FILE_NAME = "errors.md5"
	CONST_FILE_Md5_FILE_NAME  = "files.md5"

	cfgJson = `{
	"绑定端号": "端口",
	"addr": ":8080",
	"集群": "集群列表",
	"peers": ["%s"],
	"组号": "组号",
	"group": "group1",
	"refresh_interval": 120,
	"是否自动重命名": "真假",
	"rename_file": false,
	"是否支持ＷＥＢ上专": "真假",
	"enable_web_upload": true,
	"是否支持非日期路径": "真假",
	"enable_custom_path": true,
	"下载域名": "",
	"download_domain": "",
	"场景":"场景列表",
	"scenes":[],
	"默认场景":"",
	"default_scene":"default",
	"是否显示目录": "真假",
	"show_dir": true,
	"邮件配置":"",
	"mail":{
		"user":"abc@163.com",
		"password":"abc",
		"host":"smtp.163.com:25"
	},
	"告警接收邮件列表":"",
	"alram_receivers":[],
	"告警接收URL":"",
	"alarm_url":""
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
	Name   string
	ReName string
	Path   string
	Md5    string
	Size   int64
	Peers  []string
	Scene  string
}

type Status struct {
	Message string `json:"message"`
	Status  string `json:"status"`
}

type FileResult struct {
	Url string `json:"url"`
	Md5 string `json:"md5"`
}

type Mail struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
}

type StatDateFileInfo struct {
	Date      string `json:"date"`
	TotalSize int64  `json:"totalSize"`
	FileCount int64  `json:"fileCount"`
}

type GloablConfig struct {
	Addr             string   `json:"addr"`
	Peers            []string `json:"peers"`
	Group            string   `json:"group"`
	RenameFile       bool     `json:"rename_file"`
	ShowDir          bool     `json:"show_dir"`
	RefreshInterval  int      `json:"refresh_interval"`
	EnableWebUpload  bool     `json:"enable_web_upload"`
	DownloadDomain   string   `json:"download_domain"`
	EnableCustomPath bool     `json:"enable_custom_path"`
	Scenes           []string `json:"scenes"`
	AlramReceivers   []string `json:"alram_receivers"`
	DefaultScene     string   `json:"default_scene"`
	Mail             Mail     `json:"mail"`
	AlarmUrl         string   `json:"alarm_url"`
}

type CommonMap struct {
	sync.Mutex
	m map[string]interface{}
}

func (s *CommonMap) GetValue(k string) (interface{}, bool) {
	s.Lock()
	defer s.Unlock()
	v, ok := s.m[k]
	return v, ok
}

func (s *CommonMap) Put(k string, v interface{}) {
	s.Lock()
	defer s.Unlock()
	s.m[k] = v
}

func (s *CommonMap) AddCount(key string, count int) {
	s.Lock()
	defer s.Unlock()
	if _v, ok := s.m[key]; ok {
		v := _v.(int)
		v = v + count
		s.m[key] = v
	} else {
		s.m[key] = 1
	}
}

func (s *CommonMap) AddCountInt64(key string, count int64) {
	s.Lock()
	defer s.Unlock()

	if _v, ok := s.m[key]; ok {
		v := _v.(int64)
		v = v + count
		s.m[key] = v
	} else {

		s.m[key] = count
	}
}

func (s *CommonMap) Add(key string) {
	s.Lock()
	defer s.Unlock()
	if _v, ok := s.m[key]; ok {
		v := _v.(int)
		v = v + 1
		s.m[key] = v
	} else {

		s.m[key] = 1

	}
}

func (s *CommonMap) Zero() {
	s.Lock()
	defer s.Unlock()
	for k, _ := range s.m {

		s.m[k] = 0
	}
}

func (s *CommonMap) Get() map[string]interface{} {
	s.Lock()
	defer s.Unlock()
	m := make(map[string]interface{})
	for k, v := range s.m {
		m[k] = v
	}
	return m
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

func (this *Common) GetToDay() string {

	return time.Now().Format("20060102")

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

func (this *Common) IsExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func (this *Common) ReadBinFile(path string) ([]byte, error) {
	if this.IsExist(path) {
		fi, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer fi.Close()
		return ioutil.ReadAll(fi)
	} else {
		return nil, errors.New("not found")
	}
}
func (this *Common) RemoveEmptyDir(pathname string) {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("postFileToPeer")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()

	handlefunc := func(file_path string, f os.FileInfo, err error) error {

		if f.IsDir() {

			files, _ := ioutil.ReadDir(file_path)
			if len(files) == 0 && file_path != pathname {
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

func (this *Server) DownloadFromPeer(peer string, fileInfo *FileInfo) {
	var (
		err      error
		filename string
		fpath    string
		fi       os.FileInfo
	)
	if _, err = os.Stat(fileInfo.Path); err != nil {
		os.MkdirAll(fileInfo.Path, 0777)
	}

	filename = fileInfo.Name
	if fileInfo.ReName != "" {
		filename = fileInfo.ReName
	}

	p := strings.Replace(fileInfo.Path, STORE_DIR+"/", "", 1)
	req := httplib.Get(peer + "/" + Config().Group + "/" + p + "/" + filename)

	fpath = fileInfo.Path + "/" + filename

	req.SetTimeout(time.Second*5, time.Second*5)
	if err = req.ToFile(fpath); err != nil {
		log.Error(err)
	}

	if fi, err = os.Stat(fpath); err != nil {
		os.Remove(fpath)
		return
	}
	if fi.Size() == 0 {
		os.Remove(fpath)
	}

}

func (this *Server) Download(w http.ResponseWriter, r *http.Request) {

	var (
		err      error
		pathMd5  string
		info     os.FileInfo
		peer     string
		fileInfo *FileInfo
		fullpath string
		pathval  url.Values
	)

	fullpath = r.RequestURI[len(Config().Group)+2 : len(r.RequestURI)]

	fullpath = STORE_DIR + "/" + fullpath

	if pathval, err = url.ParseQuery(fullpath); err != nil {
		log.Error(err)
	} else {

		for k, _ := range pathval {
			if k != "" {
				fullpath = k
				break
			}
		}

	}

	if info, err = os.Stat(fullpath); err != nil {
		log.Error(err)
		pathMd5 = this.util.MD5(fullpath)
		for _, peer = range Config().Peers {

			if fileInfo, err = this.checkPeerFileExist(peer, pathMd5); err != nil {
				log.Error(err)
				continue
			}
			if fileInfo.Md5 != "" {

				go this.DownloadFromPeer(peer, fileInfo)

				http.Redirect(w, r, peer+r.RequestURI, 302)
				break
			}

		}
		w.WriteHeader(404)
		return
	}

	if !Config().ShowDir && info.IsDir() {
		w.Write([]byte("list dir deny"))
		return
	}

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
		filename = DATA_DIR + "/" + time.Now().Format("20060102") + "/" + CONST_Md5_ERROR_FILE_NAME
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

	var (
		err      error
		peer     string
		filename string
		info     *FileInfo
		postURL  string
		result   string
		data     []byte
		fi       os.FileInfo
	)

	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("postFileToPeer")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()

	for _, peer = range Config().Peers {

		if fileInfo.Peers == nil {
			fileInfo.Peers = []string{}
		}
		if this.util.Contains(peer, fileInfo.Peers) {
			continue
		}

		filename = fileInfo.Name

		if Config().RenameFile {
			filename = fileInfo.ReName
		}
		if !this.util.FileExists(fileInfo.Path + "/" + filename) {
			continue
		} else {
			if fileInfo.Size == 0 {
				if fi, err = os.Stat(fileInfo.Path + "/" + filename); err != nil {
					log.Error(err)
				} else {
					fileInfo.Size = fi.Size()
				}
			}
		}

		if info, _ = this.checkPeerFileExist(peer, fileInfo.Md5); info.Md5 != "" {

			continue
		}

		postURL = fmt.Sprintf("%s/%s", peer, "syncfile")
		b := httplib.Post(postURL)

		b.SetTimeout(time.Second*5, time.Second*5)
		b.Header("Sync-Path", fileInfo.Path)
		b.Param("name", filename)
		b.Param("md5", fileInfo.Md5)
		b.PostFile("file", fileInfo.Path+"/"+filename)
		result, err = b.String()
		if err != nil {
			log.Error(err, result)
		}

		if !strings.HasPrefix(result, "http://") {
			if write_log {
				this.SaveFileMd5Log(fileInfo, CONST_Md5_ERROR_FILE_NAME)
			}
		} else {

			log.Info(result)

			if !this.util.Contains(peer, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				if data, err = json.Marshal(fileInfo); err != nil {
					log.Error(err)
					return
				}
				this.db.Put([]byte(fileInfo.Md5), data, nil)
			}

		}
		if err != nil {
			log.Error(err)
		}

	}

}

func (this *Server) SaveFileMd5Log(fileInfo *FileInfo, filename string) {
	var (
		err     error
		msg     string
		tmpFile *os.File

		logpath string
		outname string
	)

	outname = fileInfo.Name

	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}

	logpath = DATA_DIR + "/" + time.Now().Format("20060102")
	if _, err = os.Stat(logpath); err != nil {
		os.MkdirAll(logpath, 0777)
	}
	msg = fmt.Sprintf("%s|%d|%s\n", fileInfo.Md5, fileInfo.Size, fileInfo.Path+"/"+outname)
	if tmpFile, err = os.OpenFile(DATA_DIR+"/"+time.Now().Format("20060102")+"/"+filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644); err != nil {
		log.Error(err)
		return
	}
	defer tmpFile.Close()
	tmpFile.WriteString(msg)
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
		} else {

			return &fileInfo, nil
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
	filename := DATA_DIR + "/" + date + "/" + CONST_Md5_ERROR_FILE_NAME

	if this.util.FileExists(filename) {

		go this.CheckFileAndSendToPeer(filename, is_force_upload)
	}
	filename = DATA_DIR + "/" + date + "/" + CONST_FILE_Md5_FILE_NAME

	if this.util.FileExists(filename) {

		go this.CheckFileAndSendToPeer(filename, is_force_upload)
	}
	w.Write([]byte("job is running"))
}

func (this *Server) GetFileInfoFromLevelDB(key string) (*FileInfo, error) {
	var (
		err  error
		data []byte

		fileInfo FileInfo
	)

	if data, err = this.db.Get([]byte(key), nil); err != nil {
		return nil, err
	}

	if err = json.Unmarshal(data, &fileInfo); err != nil {
		return nil, err
	}
	return &fileInfo, nil

}

func (this *Server) SaveStat() {

	stat := statMap.Get()
	if v, ok := stat[CONST_STAT_FILE_TOTAL_SIZE_KEY]; ok {
		switch v.(type) {
		case int64:
			if v.(int64) > 0 {

				if data, err := json.Marshal(stat); err != nil {
					log.Error(err)
				} else {
					this.util.WriteBinFile(CONST_STAT_FILE_NAME, data)
				}

			}
		}

	}

}

func (this *Server) SaveFileInfoToLevelDB(key string, fileInfo *FileInfo) (*FileInfo, error) {
	var (
		err  error
		data []byte
	)

	if data, err = json.Marshal(fileInfo); err != nil {

		return fileInfo, err

	}

	if err = this.db.Put([]byte(key), data, nil); err != nil {
		return fileInfo, err
	}

	return fileInfo, nil

}

func (this *Server) IsPeer(r *http.Request) bool {
	var (
		ip    string
		peer  string
		bflag bool
	)
	ip = this.util.GetClientIp(r)
	ip = "http://" + ip
	bflag = false

	for _, peer = range Config().Peers {
		if strings.HasPrefix(peer, ip) {
			bflag = true
			break
		}
	}
	return bflag
}

func (this *Server) SyncFile(w http.ResponseWriter, r *http.Request) {
	var (
		err        error
		outPath    string
		outname    string
		fileInfo   FileInfo
		tmpFile    *os.File
		fi         os.FileInfo
		uploadFile multipart.File
	)

	if !this.IsPeer(r) {
		log.Error(fmt.Sprintf(" not is peer,ip:%s", this.util.GetClientIp(r)))
		return
	}

	if r.Method == "POST" {
		fileInfo.Path = r.Header.Get("Sync-Path")
		fileInfo.Md5 = r.PostFormValue("md5")
		fileInfo.Name = r.PostFormValue("name")
		if uploadFile, _, err = r.FormFile("file"); err != nil {
			w.Write([]byte(err.Error()))
			log.Error(err)
			return
		}
		fileInfo.Peers = []string{}

		defer uploadFile.Close()

		if v, _ := this.GetFileInfoFromLevelDB(fileInfo.Md5); v != nil && v.Md5 != "" {
			outname = v.Name
			if v.ReName != "" {
				outname = v.ReName
			}

			p := strings.Replace(v.Path, STORE_DIR+"/", "", 1)

			download_url := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+p+"/"+outname)
			w.Write([]byte(download_url))

			return
		}

		os.MkdirAll(fileInfo.Path, 0777)

		outPath = fileInfo.Path + "/" + fileInfo.Name

		if this.util.FileExists(outPath) {
			if tmpFile, err = os.Open(outPath); err != nil {
				log.Error(err)
				w.Write([]byte(err.Error()))
				return
			}
			if this.util.GetFileMd5(tmpFile) != fileInfo.Md5 {
				tmpFile.Close()
				log.Error("md5 !=fileInfo.Md5 ")
				w.Write([]byte("md5 !=fileInfo.Md5 "))
				return
			}
		}

		if tmpFile, err = os.Create(outPath); err != nil {
			log.Error(err)
			w.Write([]byte(err.Error()))
			return
		}

		defer tmpFile.Close()

		if _, err = io.Copy(tmpFile, uploadFile); err != nil {
			w.Write([]byte(err.Error()))
			log.Error(err)
			return
		}
		if this.util.GetFileMd5(tmpFile) != fileInfo.Md5 {
			w.Write([]byte("md5 error"))
			tmpFile.Close()
			os.Remove(outPath)
			return

		}
		if fi, err = os.Stat(outPath); err != nil {
			log.Error(err)
		} else {
			fileInfo.Size = fi.Size()
			statMap.AddCountInt64(CONST_STAT_FILE_TOTAL_SIZE_KEY, fi.Size())
			statMap.AddCountInt64(CONST_STAT_FILE_COUNT_KEY, 1)
		}

		if fileInfo.Peers == nil {
			fileInfo.Peers = []string{fmt.Sprintf("http://%s", r.Host)}
		}
		if _, err = this.SaveFileInfoToLevelDB(this.util.MD5(outPath), &fileInfo); err != nil {
			log.Error(err)
		}
		if _, err = this.SaveFileInfoToLevelDB(fileInfo.Md5, &fileInfo); err != nil {
			log.Error(err)
		}

		this.SaveFileMd5Log(&fileInfo, CONST_FILE_Md5_FILE_NAME)

		p := strings.Replace(fileInfo.Path, STORE_DIR+"/", "", 1)

		download_url := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+p+"/"+fileInfo.Name)
		w.Write([]byte(download_url))

	}

}

func (this *Server) CheckScene(scene string) (bool, error) {

	if len(Config().Scenes) == 0 {
		return true, nil
	}

	if !this.util.Contains(scene, Config().Scenes) {
		return false, errors.New("not valid scene")
	}
	return true, nil

}

func (this *Server) RemoveFile(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		md5sum   string
		fileInfo *FileInfo
		fpath    string
	)

	r.ParseForm()

	md5sum = r.FormValue("md5")

	if len(md5sum) != 32 {
		w.Write([]byte("md5 unvalid"))
		return
	}
	if fileInfo, err = this.GetFileInfoByMd5(md5sum); err != nil {
		w.Write([]byte(err.Error()))
		return
	}

	if fileInfo.ReName != "" {
		fpath = fileInfo.Path + "/" + fileInfo.ReName
	} else {
		fpath = fileInfo.Path + "/" + fileInfo.Name
	}

	if fileInfo.Path != "" && this.util.FileExists(fpath) {
		if err = os.Remove(fpath); err != nil {
			w.Write([]byte(err.Error()))
			return
		} else {
			w.Write([]byte("remove success"))
			return
		}
	}
	w.Write([]byte("fail remove"))

}

func (this *Server) Upload(w http.ResponseWriter, r *http.Request) {

	var (
		err error

		//		pathname     string
		outname      string
		md5sum       string
		fileInfo     FileInfo
		uploadFile   multipart.File
		uploadHeader *multipart.FileHeader
		scene        string
		output       string
		fileResult   FileResult
		data         []byte
	)
	if r.Method == "POST" {
		//		name := r.PostFormValue("name")

		//		fileInfo.Path = r.Header.Get("Sync-Path")

		if Config().EnableCustomPath {
			fileInfo.Path = r.FormValue("path")
			fileInfo.Path = strings.Trim(fileInfo.Path, "/")
		}
		scene = r.FormValue("scene")
		md5sum = r.FormValue("md5")
		output = r.FormValue("output")

		fileInfo.Md5 = md5sum
		uploadFile, uploadHeader, err = r.FormFile("file")
		fileInfo.Peers = []string{}

		if scene == "" {
			scene = Config().DefaultScene
		}

		if output == "" {
			output = "text"
		}

		if !this.util.Contains(output, []string{"json", "text"}) {
			w.Write([]byte("output just support json or text"))
			return
		}

		fileInfo.Scene = scene

		if _, err = this.CheckScene(scene); err != nil {

			w.Write([]byte(err.Error()))

			return
		}

		if err != nil {
			log.Error(err)
			fmt.Printf("FromFileErr")
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
			return
		}

		SaveUploadFile := func(file multipart.File, header *multipart.FileHeader, fileInfo *FileInfo) (*FileInfo, error) {
			var (
				err     error
				outFile *os.File
				folder  string
			)

			defer file.Close()

			fileInfo.Name = header.Filename

			if Config().RenameFile {
				fileInfo.ReName = this.util.MD5(this.util.GetUUID()) + path.Ext(fileInfo.Name)
			}

			folder = time.Now().Format("20060102/15/04")
			if fileInfo.Scene != "" {
				folder = fmt.Sprintf(STORE_DIR+"/%s/%s", fileInfo.Scene, folder)
			} else {
				folder = fmt.Sprintf(STORE_DIR+"/%s", folder)
			}
			if fileInfo.Path != "" {
				if strings.HasPrefix(fileInfo.Path, STORE_DIR) {
					folder = fileInfo.Path
				} else {

					folder = STORE_DIR + "/" + fileInfo.Path
				}
			}

			if !util.FileExists(folder) {
				os.MkdirAll(folder, 0777)
			}

			outPath := fmt.Sprintf(folder+"/%s", fileInfo.Name)
			if Config().RenameFile {
				outPath = fmt.Sprintf(folder+"/%s", fileInfo.ReName)
			}

			if this.util.FileExists(outPath) {
				for i := 0; i < 10000; i++ {
					outPath = fmt.Sprintf(folder+"/%d_%s", i, header.Filename)
					fileInfo.Name = fmt.Sprintf("%d_%s", i, header.Filename)
					if !this.util.FileExists(outPath) {
						break
					}
				}
			}

			log.Info(fmt.Sprintf("upload: %s", outPath))

			if outFile, err = os.Create(outPath); err != nil {
				return fileInfo, err
			}

			defer outFile.Close()

			if err != nil {
				log.Error(err)
				return fileInfo, errors.New("(error)fail," + err.Error())

			}

			if _, err = io.Copy(outFile, file); err != nil {
				log.Error(err)
				return fileInfo, errors.New("(error)fail," + err.Error())
			}

			v := util.GetFileMd5(outFile)
			fileInfo.Md5 = v
			fileInfo.Path = folder

			fileInfo.Peers = append(fileInfo.Peers, fmt.Sprintf("http://%s", r.Host))

			return fileInfo, nil

		}

		SaveUploadFile(uploadFile, uploadHeader, &fileInfo)

		if v, _ := this.GetFileInfoFromLevelDB(fileInfo.Md5); v != nil && v.Md5 != "" {

			if Config().RenameFile {
				os.Remove(fileInfo.Path + "/" + fileInfo.ReName)
			} else {
				os.Remove(fileInfo.Path + "/" + fileInfo.Name)
			}
			outname = v.Name
			if v.ReName != "" {
				outname = v.ReName
			}
			p := strings.Replace(v.Path, STORE_DIR+"/", "", 1)
			download_url := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+p+"/"+outname)
			if Config().DownloadDomain != "" {
				download_url = fmt.Sprintf("http://%s/%s", Config().DownloadDomain, Config().Group+"/"+p+"/"+outname)
			}
			if output == "json" {
				fileResult.Url = download_url
				fileResult.Md5 = v.Md5

				if data, err = json.Marshal(fileResult); err != nil {
					w.Write([]byte(err.Error()))
					return
				}
				w.Write(data)

			} else {

				w.Write([]byte(download_url))
			}
			return
		}

		if fileInfo.Md5 == "" {
			log.Warn(" fileInfo.Md5 is null")
			return
		}

		if md5sum != "" && fileInfo.Md5 != md5sum {
			log.Warn(" fileInfo.Md5 and md5sum !=")
			return
		}

		UploadToPeer := func(fileInfo *FileInfo) {

			var (
				err      error
				pathMd5  string
				fullpath string
				//				data     []byte
			)

			if _, err = json.Marshal(fileInfo); err != nil {
				log.Error(err)
				log.Error(fmt.Sprintf("UploadToPeer fail: %v", fileInfo))
				return

			}

			if _, err = this.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo); err != nil {
				log.Error("SaveFileInfoToLevelDB fail", err)

			}

			fullpath = fileInfo.Path + "/" + fileInfo.Name

			if Config().RenameFile {
				fullpath = fileInfo.Path + "/" + fileInfo.ReName
			}

			pathMd5 = this.util.MD5(fullpath)

			if _, err = this.SaveFileInfoToLevelDB(pathMd5, fileInfo); err != nil {
				log.Error("SaveFileInfoToLevelDB fail", err)
			}

			go this.postFileToPeer(fileInfo, true)

		}

		UploadToPeer(&fileInfo)

		outname = fileInfo.Name

		if Config().RenameFile {
			outname = fileInfo.ReName
		}

		if fi, err := os.Stat(fileInfo.Path + "/" + outname); err != nil {
			log.Error(err)
		} else {
			fileInfo.Size = fi.Size()
			statMap.AddCountInt64(CONST_STAT_FILE_TOTAL_SIZE_KEY, fi.Size())
			statMap.AddCountInt64(CONST_STAT_FILE_COUNT_KEY, 1)
			statMap.AddCountInt64(this.util.GetToDay()+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY, fi.Size())
			statMap.AddCountInt64(this.util.GetToDay()+"_"+CONST_STAT_FILE_COUNT_KEY, 1)
		}

		this.SaveStat()

		this.SaveFileMd5Log(&fileInfo, CONST_FILE_Md5_FILE_NAME)

		p := strings.Replace(fileInfo.Path, STORE_DIR+"/", "", 1)

		download_url := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+p+"/"+outname)
		if Config().DownloadDomain != "" {
			download_url = fmt.Sprintf("http://%s/%s", Config().DownloadDomain, Config().Group+"/"+p+"/"+outname)
		}

		if output == "json" {
			fileResult.Url = download_url
			fileResult.Md5 = fileInfo.Md5

			if data, err = json.Marshal(fileResult); err != nil {
				w.Write([]byte(err.Error()))
				return
			}
			w.Write(data)

		} else {

			w.Write([]byte(download_url))
		}
		return

	} else {
		w.Write([]byte("(error)fail,please use post method"))
		return
	}

}

func (this *Server) SendToMail(to, subject, body, mailtype string) error {
	host := Config().Mail.Host
	user := Config().Mail.User
	password := Config().Mail.Password
	hp := strings.Split(host, ":")
	auth := smtp.PlainAuth("", user, password, hp[0])
	var content_type string
	if mailtype == "html" {
		content_type = "Content-Type: text/" + mailtype + "; charset=UTF-8"
	} else {
		content_type = "Content-Type: text/plain" + "; charset=UTF-8"
	}

	msg := []byte("To: " + to + "\r\nFrom: " + user + ">\r\nSubject: " + "\r\n" + content_type + "\r\n\r\n" + body)
	send_to := strings.Split(to, ";")
	err := smtp.SendMail(host, auth, user, send_to, msg)
	return err
}

func (this *Server) BenchMark(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	batch := new(leveldb.Batch)

	for i := 0; i < 100000000; i++ {
		f := FileInfo{}
		f.Peers = []string{"http://192.168.0.1", "http://192.168.2.5"}
		f.Path = "20190201/19/02"
		s := strconv.Itoa(i)
		s = util.MD5(s)
		f.Name = s
		f.Md5 = s

		//		server.SaveFileInfoToLevelDB(s, &f)

		if data, err := json.Marshal(&f); err == nil {
			batch.Put([]byte(s), data)
		}

		if i%10000 == 0 {

			if batch.Len() > 0 {
				server.db.Write(batch, nil)
				//				batch = new(leveldb.Batch)
				batch.Reset()
			}
			fmt.Println(i, time.Since(t).Seconds())

		}

		//fmt.Println(server.GetFileInfoByMd5(s))

	}

	util.WriteFile("time.txt", time.Since(t).String())
	fmt.Println(time.Since(t).String())
}
func (this *Server) Stat(w http.ResponseWriter, r *http.Request) {
	var (
		min  int64
		max  int64
		err  error
		i    int64
		data []byte
		rows []StatDateFileInfo
	)
	min = 20190101
	max = 20190101
	for k, _ := range statMap.Get() {
		ks := strings.Split(k, "_")
		if len(ks) == 2 {
			if i, err = strconv.ParseInt(ks[0], 10, 64); err != nil {
				continue
			}
			if i >= max {
				max = i
			}
			if i < min {
				min = i
			}

		}

	}

	for i := min; i <= max; i++ {

		s := fmt.Sprintf("%d", i)
		if v, ok := statMap.GetValue(s + "_" + CONST_STAT_FILE_TOTAL_SIZE_KEY); ok {
			var info StatDateFileInfo
			info.Date = s
			switch v.(type) {
			case int64:
				info.TotalSize = v.(int64)
			}

			if v, ok := statMap.GetValue(s + "_" + CONST_STAT_FILE_COUNT_KEY); ok {
				switch v.(type) {
				case int64:
					info.FileCount = v.(int64)
				}
			}

			rows = append(rows, info)

		}

	}

	if v, ok := statMap.GetValue(CONST_STAT_FILE_COUNT_KEY); ok {
		var info StatDateFileInfo
		info.Date = "all"
		info.FileCount = v.(int64)
		if v, ok := statMap.GetValue(CONST_STAT_FILE_TOTAL_SIZE_KEY); ok {
			info.TotalSize = v.(int64)
		}
		rows = append(rows, info)
	}

	if data, err = json.Marshal(rows); err != nil {
		return
	}
	w.Write(data)

}

func (this *Server) RegisterExit() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				this.SaveStat()
				log.Info("Exit", s)
				os.Exit(1)
			}
		}
	}()
}

func (this *Server) Check() {

	check := func() {

		defer func() {
			if re := recover(); re != nil {
				buffer := debug.Stack()
				log.Error("postFileToPeer")
				log.Error(re)
				log.Error(string(buffer))
			}
		}()

		var (
			status  Status
			err     error
			subject string
			body    string
			req     *httplib.BeegoHTTPRequest
		)

		for _, peer := range Config().Peers {

			req = httplib.Get(peer + "/status")
			req.SetTimeout(time.Second*5, time.Second*5)
			err = req.ToJSON(&status)

			if status.Status != "ok" {

				for _, to := range Config().AlramReceivers {
					subject = "fastdfs server error"

					if err != nil {
						body = fmt.Sprintf("%s\nserver:%s\nerror:\n%s", subject, peer, err.Error())
					} else {
						body = fmt.Sprintf("%s\nserver:%s\n", subject, peer)
					}
					if err = this.SendToMail(to, subject, body, "text"); err != nil {
						log.Error(err)
					}
				}

				if Config().AlarmUrl != "" {
					req = httplib.Post(Config().AlarmUrl)
					req.SetTimeout(time.Second*10, time.Second*10)
					req.Param("message", body)
					req.Param("subject", subject)
					if _, err = req.String(); err != nil {
						log.Error(err)
					}

				}

			}
		}

	}

	go func() {
		for {
			time.Sleep(time.Minute * 10)
			check()
		}
	}()

}

func (this *Server) Status(w http.ResponseWriter, r *http.Request) {

	var (
		status Status
		err    error
		data   []byte
	)

	status.Status = "ok"

	if data, err = json.Marshal(&status); err != nil {
		status.Status = "fail"
		status.Message = err.Error()
		w.Write(data)
		return
	}
	w.Write(data)

}

func (this *Server) HeartBeat(w http.ResponseWriter, r *http.Request) {

}

func (this *Server) Index(w http.ResponseWriter, r *http.Request) {
	if Config().EnableWebUpload {
		fmt.Fprintf(w,
			fmt.Sprintf(`<html>
	    <head>
	        <meta charset="utf-8"></meta>
	        <title>Uploader</title>
			<style>
			form {
				bargin
				
			}
			 .form-line {
				display:block;
			}
			</style>
	    </head>
	    <body>
	        <form action="/upload" method="post" enctype="multipart/form-data">
	            <span class="form-line">文件(file):<input  type="file" id="file" name="file" ></span>
				<span class="form-line">场景(scene):<input  type="text" id="scene" name="scene" value="%s"></span>
				<span class="form-line">输出(output):<input  type="text" id="output" name="output" value="json"></span>
				<span class="form-line">自定义路径(path):<input  type="text" id="path" name="path" value=""></span>
	            <input type="submit" name="submit" value="upload">
	        </form>
	    </body>
	</html>`, Config().DefaultScene))
	} else {
		w.Write([]byte("web upload deny"))
	}
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

	staticHandler = http.StripPrefix("/"+Config().Group+"/", http.FileServer(http.Dir(STORE_DIR)))

	initComponent()
}

func initComponent() {
	var (
		err   error
		db    *leveldb.DB
		ip    string
		stat  map[string]interface{}
		data  []byte
		count int64
	)
	ip = util.GetPulicIP()
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

	db, err = leveldb.OpenFile(CONST_LEVELDB_FILE_NAME, nil)
	if err != nil {
		log.Error(err)
		panic(err)
	}
	server.db = db

	if util.FileExists(CONST_STAT_FILE_NAME) {
		if data, err = util.ReadBinFile(CONST_STAT_FILE_NAME); err != nil {
			log.Error(err)
		} else {

			if err = json.Unmarshal(data, &stat); err != nil {
				log.Error(err)
			} else {
				for k, v := range stat {
					switch v.(type) {
					case float64:
						vv := strings.Split(fmt.Sprintf("%f", v), ".")[0]

						if count, err = strconv.ParseInt(vv, 10, 64); err != nil {
							log.Error(err)
						} else {
							statMap.Put(k, count)
						}

					default:
						statMap.Put(k, v)

					}

				}
			}
		}

	}

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
			time.Sleep(time.Second * time.Duration(Config().RefreshInterval))
			util.RemoveEmptyDir(STORE_DIR)
			server.SaveStat()
		}
	}()

	go server.Check()

	http.HandleFunc("/", server.Index)
	http.HandleFunc("/check_file_exist", server.CheckFileExist)
	http.HandleFunc("/upload", server.Upload)
	http.HandleFunc("/delete", server.RemoveFile)
	http.HandleFunc("/sync", server.Sync)
	http.HandleFunc("/stat", server.Stat)
	http.HandleFunc("/status", server.Status)
	http.HandleFunc("/syncfile", server.SyncFile)
	http.HandleFunc("/"+Config().Group+"/", server.Download)
	fmt.Println("Listen on " + Config().Addr)
	err := http.ListenAndServe(Config().Addr, new(HttpHandler))
	log.Error(err)
	fmt.Println(err)

}
