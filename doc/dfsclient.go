package doc

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego/httplib"
	"github.com/eventials/go-tus"
	"github.com/sjqzhang/goutil"
	"github.com/syndtr/goleveldb/leveldb"
)

var Url *string
var Dir *string
var Worker *int
var Scene *string

var queue chan string
var store tus.Store
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

func GetDir(dir string) []string {
	return getDir(dir)
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
		if strings.Index(*Url, "/big/upload") > 0 {
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
	req := httplib.Post(*Url)
	req.PostFile("file", filePath) //注意不是全路径
	req.Param("output", "text")
	req.Param("scene", "")
	path := strings.Replace(filePath, *Dir, "", 1)
	filename := filepath.Base(path)
	path = strings.Replace(filepath.Dir(path), "\\", "/", -1)
	req.Param("path", *Scene+"/"+strings.TrimLeft(path, "/"))
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
	client, err := tus.NewClient(*Url, cfg)
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

func StartWorker() {
	defer func() {
		if re := recover(); re != nil {
		}
	}()
	for i := 0; i < *Worker; i++ {
		go sendFile()
	}
}

func StartServer() {
	st := time.Now()
	files := getDir(*Dir)
	wg.Add(len(files))
	queue = make(chan string, len(files))
	for i := 0; i < len(files); i++ {
		queue <- files[i]
	}
	StartWorker()
	wg.Wait()
	fmt.Println(time.Since(st))
}

func init() {
	util = goutil.Common{}
	defaultTransport := &http.Transport{
		DisableKeepAlives:   true,
		Dial:                httplib.TimeoutDialer(time.Second*15, time.Second*300),
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
	}
	settings := httplib.BeegoHTTPSettings{
		UserAgent:        "Go-FastDFS",
		ConnectTimeout:   15 * time.Second,
		ReadWriteTimeout: 15 * time.Second,
		Gzip:             true,
		DumpBody:         true,
		Transport:        defaultTransport,
	}
	httplib.SetDefaultSetting(settings)
	//store,_=NewLeveldbStore("upload.db")
}
