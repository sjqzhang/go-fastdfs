package main

import (
	"flag"
	"fmt"
	"github.com/astaxie/beego/httplib"
	"github.com/eventials/go-tus"
	"github.com/sjqzhang/goutil"
	"github.com/syndtr/goleveldb/leveldb"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var url *string
var dir *string
var worker *int
var queue chan string

var store tus.Store

//var filesize *int
//var filecount *int
//var retry *int
//var gen *bool
var scene *string
var done chan bool = make(chan bool, 1)
var wg sync.WaitGroup = sync.WaitGroup{}
var util goutil.Common

type LeveldbStore struct {
	db *leveldb.DB
}

func NewLeveldbStore(path string) (tus.Store, error) {
	db, err := leveldb.OpenFile(path, nil)
	if err != nil {
		return nil, err
	}

	store := &LeveldbStore{db: db}
	return store, err
}

func (s *LeveldbStore) Get(fingerprint string) (string, bool) {
	url, err := s.db.Get([]byte(fingerprint), nil)
	ok := true
	if err != nil {
		ok = false
	}
	return string(url), ok
}

func (s *LeveldbStore) Set(fingerprint, url string) {
	s.db.Put([]byte(fingerprint), []byte(url), nil)
}

func (s *LeveldbStore) Delete(fingerprint string) {
	s.db.Delete([]byte(fingerprint), nil)
}

func (s *LeveldbStore) Close() {
	s.Close()
}

func init() {
	util = goutil.Common{}
	defaultTransport := &http.Transport{
		DisableKeepAlives:   true,
		Dial:                httplib.TimeoutDialer(time.Second*15, time.Second*300),
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
	}
	settins := httplib.BeegoHTTPSettings{
		UserAgent:        "Go-FastDFS",
		ConnectTimeout:   15 * time.Second,
		ReadWriteTimeout: 15 * time.Second,
		Gzip:             true,
		DumpBody:         true,
		Transport:        defaultTransport,
	}
	httplib.SetDefaultSetting(settins)

	//store,_=NewLeveldbStore("upload.db")

}

func getDir(dir string) []string {
	var (
		paths []string
	)
	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if !info.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	return paths
}

func sendFile() {

	for {
		if len(queue) <= 0 {
			return
		}
		filePath := <-queue
		if strings.Index(*url, "/big/upload") > 0 {
			bigUpload(filePath)
		} else {
			normalUpload(filePath)
		}
		wg.Done()
	}

}

func normalUpload(filePath string) {
	defer func() {
		if re := recover(); re != nil {
		}
	}()
	req := httplib.Post(*url)
	req.PostFile("file", filePath) //注意不是全路径
	req.Param("output", "text")
	req.Param("scene", "")
	path := strings.Replace(filePath, *dir, "", 1)
	filename := filepath.Base(path)
	path = strings.Replace(filepath.Dir(path), "\\", "/", -1)
	req.Param("path", *scene+"/"+strings.TrimLeft(path, "/"))
	req.Param("filename", filename)
	req.Retries(-1)
	if s, err := req.String(); err != nil {
		fmt.Println(err, filePath)
	} else {
		fmt.Println(s, filePath)
	}

}

func bigUpload(filePath string) {
	defer func() {
		if re := recover(); re != nil {
		}
	}()
	f, err := os.Open(filePath)
	if err != nil {

		panic(err)
	}
	defer f.Close()
	cfg := tus.DefaultConfig()
	//cfg.Store=store
	//cfg.Resume=true
	client, err := tus.NewClient(*url, cfg)
	if err != nil {
		fmt.Println(err)
	}
	upload, err := tus.NewUploadFromFile(f)
	if err != nil {
		fmt.Println(err)
		return
	}
	uploader, err := client.CreateOrResumeUpload(upload)
	if err != nil {
		fmt.Println(err)
		return
	}
	url := uploader.Url()
	err = uploader.Upload()
	fmt.Println(url, filePath)

}

func startWorker() {
	defer func() {
		if re := recover(); re != nil {
		}
	}()
	for i := 0; i < *worker; i++ {
		go sendFile()
	}
}

func main() {

	url = flag.String("url", "http://127.0.0.1:8080/group1/upload", "url")
	dir = flag.String("dir", "./", "dir to upload")
	worker = flag.Int("worker", 100, "num of worker")
	scene = flag.String("scene", "default", "scene")
	//retry=flag.Int("retry", -1, "retry times when fail")
	//uploadPath=flag.String("uploadPath", "./", "upload path")
	//filesize=flag.Int("filesize", 1024*1024, "file of size")
	//filecount=flag.Int("filecount", 1000000, "file of count")
	//gen=flag.Bool("gen", false, "gen file")
	flag.Parse()
	st := time.Now()
	//if *gen {
	//	genFile()
	//	fmt.Println(time.Since(st))
	//	os.Exit(0)
	//}
	files := getDir(*dir)
	wg.Add(len(files))
	queue = make(chan string, len(files))
	for i := 0; i < len(files); i++ {
		queue <- files[i]
	}
	startWorker()
	wg.Wait()
	fmt.Println(time.Since(st))
}
