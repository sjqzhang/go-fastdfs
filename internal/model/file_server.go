package model

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	slog "log"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/astaxie/beego/httplib"
	mapSet "github.com/deckarep/golang-set"
	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/pkg"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	levelDBUtil "github.com/syndtr/goleveldb/leveldb/util"

	"github.com/radovskyb/watcher"
	"github.com/sjqzhang/tusd"
	"github.com/sjqzhang/tusd/filestore"
)

var (
	Svr           *Server
	StaticHandler http.Handler
)

type Server struct {
	statMap        *pkg.CommonMap
	sumMap         *pkg.CommonMap
	rtMap          *pkg.CommonMap
	queueToPeers   chan FileInfo
	queueFromPeers chan FileInfo
	queueFileLog   chan *FileLog
	QueueUpload    chan WrapReqResp
	lockMap        *pkg.CommonMap
	sceneMap       *pkg.CommonMap
	searchMap      *pkg.CommonMap
	curDate        string
	host           string
}

type WrapReqResp struct {
	Ctx  *gin.Context
	Done chan bool
}

func NewServer(conf *config.Config) *Server {
	server := &Server{
		statMap:        pkg.NewCommonMap(0),
		lockMap:        pkg.NewCommonMap(0),
		rtMap:          pkg.NewCommonMap(0),
		sceneMap:       pkg.NewCommonMap(0),
		searchMap:      pkg.NewCommonMap(0),
		queueToPeers:   make(chan FileInfo, conf.QueueSize()),
		queueFromPeers: make(chan FileInfo, conf.QueueSize()),
		queueFileLog:   make(chan *FileLog, conf.QueueSize()),
		QueueUpload:    make(chan WrapReqResp, 100),
		sumMap:         pkg.NewCommonMap(365 * 3),
	}

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
	server.statMap.Put(conf.StatisticsFileCountKey(), int64(0))
	server.statMap.Put(conf.StatFileTotalSizeKey(), int64(0))
	server.statMap.Put(pkg.GetToDay()+"_"+conf.StatisticsFileCountKey(), int64(0))
	server.statMap.Put(pkg.GetToDay()+"_"+conf.StatFileTotalSizeKey(), int64(0))
	server.curDate = pkg.GetToDay()

	return server
}

//
func (svr *Server) WatchFilesChange(conf *config.Config) {
	var (
		w *watcher.Watcher
		//fileInfo FileInfo
		curDir string
		err    error
		qchan  chan *FileInfo
		isLink bool
	)
	qchan = make(chan *FileInfo, conf.WatchChanSize())
	w = watcher.New()
	w.FilterOps(watcher.Create)
	//w.FilterOps(watcher.Create, watcher.Remove)
	curDir, err = filepath.Abs(filepath.Dir(conf.StoreDir()))
	if err != nil {
		log.Error(err)
	}
	go func() {
		for {
			select {
			case event := <-w.Event:
				if event.IsDir() {
					continue
				}

				fpath := strings.Replace(event.Path, curDir+string(os.PathSeparator), "", 1)
				if isLink {
					fpath = strings.Replace(event.Path, curDir, conf.StoreDir(), 1)
				}
				fpath = strings.Replace(fpath, string(os.PathSeparator), "/", -1)
				sum := pkg.MD5(fpath)
				fileInfo := FileInfo{
					Size:      event.Size(),
					Name:      event.Name(),
					Path:      strings.TrimSuffix(fpath, "/"+event.Name()), // files/default/20190927/xxx
					Md5:       sum,
					TimeStamp: event.ModTime().Unix(),
					Peers:     []string{svr.host},
					OffSet:    -2,
					op:        event.Op.String(),
				}
				log.Info(fmt.Sprintf("WatchFilesChange op:%s path:%s", event.Op.String(), fpath))
				qchan <- &fileInfo
				//svr.AppendToQueue(&fileInfo)
			case err := <-w.Error:
				log.Error(err)
			case <-w.Closed:
				return
			}
		}
	}()
	go func() {
		for {
			c := <-qchan
			if time.Now().Unix()-c.TimeStamp < conf.SyncDelay() {
				qchan <- c
				time.Sleep(time.Second * 1)
				continue
			} else {
				//if c.op == watcher.Remove.String() {
				//	req := httplib.Post(fmt.Sprintf("%s%s?md5=%s", svr.host, svr.getRequestURI("delete"), c.Md5))
				//	req.Param("md5", c.Md5)
				//	req.SetTimeout(time.Second*5, time.Second*10)
				//	log.Infof(req.String())
				//}

				if c.op == watcher.Create.String() {
					log.Info(fmt.Sprintf("Syncfile Add to Queue path:%s", c.Path+"/"+c.Name))
					svr.AppendToQueue(c, conf)
					svr.SaveFileInfoToLevelDB(c.Md5, c, conf.LevelDB(), conf)
				}
			}
		}
	}()
	if dir, err := os.Readlink(conf.StoreDir()); err == nil {

		if strings.HasSuffix(dir, string(os.PathSeparator)) {
			dir = strings.TrimSuffix(dir, string(os.PathSeparator))
		}
		curDir = dir
		isLink = true
		if err := w.AddRecursive(dir); err != nil {
			log.Error(err)
		}
		w.Ignore(dir + "/_tmp/")
		w.Ignore(dir + "/" + conf.LargeDir() + "/")
	}
	if err := w.AddRecursive("./" + conf.StoreDir()); err != nil {
		log.Error(err)
	}
	w.Ignore("./" + conf.StoreDir() + "/_tmp/")
	w.Ignore("./" + conf.StoreDir() + "/" + conf.LargeDir() + "/")
	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Error(err)
	}
}

func ParseSmallFile(filename string, conf *config.Config) (string, int64, int, error) {
	var (
		err    error
		offset int64
		length int
	)
	err = errors.New("unvalid small file")
	if len(filename) < 3 {
		return filename, -1, -1, err
	}
	if strings.Contains(filename, "/") {
		filename = filename[strings.LastIndex(filename, "/")+1:]
	}
	pos := strings.Split(filename, ",")
	if len(pos) < 3 {
		return filename, -1, -1, err
	}
	offset, err = strconv.ParseInt(pos[1], 10, 64)
	if err != nil {
		return filename, -1, -1, err
	}
	if length, err = strconv.Atoi(pos[2]); err != nil {
		return filename, offset, -1, err
	}
	if length > conf.SmallFileSize() || offset < 0 {
		err = errors.New("invalid filesize or offset")
		return filename, -1, -1, err
	}
	return pos[0], offset, length, nil
}

//
func DownloadNormalFileByURI(ctx *gin.Context, conf *config.Config) (bool, error) {
	var (
		err        error
		isDownload bool
		imgWidth   int
		imgHeight  int
		width      string
		height     string
	)
	r := ctx.Request
	w := ctx.Writer
	r.ParseForm()
	isDownload = true
	if r.FormValue("download") == "" {
		isDownload = conf.DefaultDownload()
	}
	if r.FormValue("download") == "0" {
		isDownload = false
	}
	width = r.FormValue("width")
	height = r.FormValue("height")
	if width != "" {
		imgWidth, err = strconv.Atoi(width)
		if err != nil {
			log.Error(err)
		}
	}
	if height != "" {
		imgHeight, err = strconv.Atoi(height)
		if err != nil {
			log.Error(err)
		}
	}
	if isDownload {
		pkg.SetDownloadHeader(w, r)
	}
	fullPath, _ := GetFilePathFromRequest(ctx, conf)
	if imgWidth != 0 || imgHeight != 0 {
		pkg.ResizeImage(w, fullPath, uint(imgWidth), uint(imgHeight))
		return true, nil
	}
	StaticHandler.ServeHTTP(w, r)
	return true, nil
}

func (svr *Server) DownloadNotFound(ctx *gin.Context, conf *config.Config) {
	var (
		err        error
		fullpath   string
		smallPath  string
		isDownload bool
		pathMd5    string
		peer       string
		fileInfo   *FileInfo
	)
	r := ctx.Request
	w := ctx.Writer
	fullpath, smallPath = GetFilePathFromRequest(ctx, conf)
	isDownload = true
	if r.FormValue("download") == "" {
		isDownload = conf.DefaultDownload()
	}
	if r.FormValue("download") == "0" {
		isDownload = false
	}
	if smallPath != "" {
		pathMd5 = pkg.MD5(smallPath)
	} else {
		pathMd5 = pkg.MD5(fullpath)
	}
	for _, peer = range conf.Peers() {
		if fileInfo, err = checkPeerFileExist(peer, pathMd5, fullpath); err != nil {
			log.Error(err)
			continue
		}
		if fileInfo.Md5 != "" {
			go svr.DownloadFromPeer(peer, fileInfo, conf)
			//http.Redirect(w, r, peer+r.RequestURI, 302)
			if isDownload {
				pkg.SetDownloadHeader(w, r)
			}
			pkg.DownloadFileToResponse(peer+r.RequestURI, ctx)
			return
		}
	}
	w.WriteHeader(404)
	return
}

// GetSmallFileByURI
func GetSmallFileByURI(ctx *gin.Context, conf *config.Config) ([]byte, bool, error) {
	var (
		err      error
		data     []byte
		offset   int64
		length   int
		fullpath string
		info     os.FileInfo
	)
	r := ctx.Request
	fullpath, _ = GetFilePathFromRequest(ctx, conf)
	if _, offset, length, err = ParseSmallFile(r.RequestURI, conf); err != nil {
		return nil, false, err
	}
	if info, err = os.Stat(fullpath); err != nil {
		return nil, false, err
	}
	if info.Size() < offset+int64(length) {
		return nil, true, errors.New("noFound")
	} else {
		data, err = pkg.ReadFileByOffSet(fullpath, offset, length)
		if err != nil {
			return nil, false, err
		}
		return data, false, err
	}
}

//
func DownloadSmallFileByURI(ctx *gin.Context, conf *config.Config) (bool, error) {
	var (
		err        error
		data       []byte
		isDownload bool
		imgWidth   int
		imgHeight  int
		width      string
		height     string
		notFound   bool
	)
	r := ctx.Request
	w := ctx.Writer
	r.ParseForm()
	isDownload = true
	if r.FormValue("download") == "" {
		isDownload = conf.DefaultDownload()
	}
	if r.FormValue("download") == "0" {
		isDownload = false
	}
	width = r.FormValue("width")
	height = r.FormValue("height")
	if width != "" {
		imgWidth, err = strconv.Atoi(width)
		if err != nil {
			log.Error(err)
		}
	}
	if height != "" {
		imgHeight, err = strconv.Atoi(height)
		if err != nil {
			log.Error(err)
		}
	}
	data, notFound, err = GetSmallFileByURI(ctx, conf)
	_ = notFound
	if data != nil && string(data[0]) == "1" {
		if isDownload {
			pkg.SetDownloadHeader(w, r)
		}
		if imgWidth != 0 || imgHeight != 0 {
			pkg.ResizeImageByBytes(w, data[1:], uint(imgWidth), uint(imgHeight))
			return true, nil
		}
		w.Write(data[1:])
		return true, nil
	}
	return false, errors.New("not found")
}

func (svr *Server) SaveFileMd5Log(fileInfo *FileInfo, filename string, conf *config.Config) {
	var (
		info FileInfo
	)
	for len(svr.queueFileLog)+len(svr.queueFileLog)/10 > conf.QueueSize() {
		time.Sleep(time.Second * 1)
	}
	info = *fileInfo
	svr.queueFileLog <- &FileLog{FileInfo: &info, FileName: filename}
}

func (svr *Server) saveFileMd5Log(fileInfo *FileInfo, filename string, conf *config.Config) {
	var (
		err      error
		outname  string
		logDate  string
		ok       bool
		fullpath string
		md5Path  string
		logKey   string
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("saveFileMd5Log")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	if fileInfo == nil || fileInfo.Md5 == "" || filename == "" {
		log.Warn("saveFileMd5Log", fileInfo, filename)
		return
	}
	logDate = pkg.GetDayFromTimeStamp(fileInfo.TimeStamp)
	outname = fileInfo.Name
	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}
	fullpath = fileInfo.Path + "/" + outname
	logKey = fmt.Sprintf("%s_%s_%s", logDate, filename, fileInfo.Md5)
	if filename == conf.FileMd5() {
		//svr.searchMap.Put(fileInfo.Md5, fileInfo.Name)
		if ok, err = ExistFromLevelDB(fileInfo.Md5, conf.LevelDB()); !ok {
			svr.statMap.AddCountInt64(logDate+"_"+conf.StatisticsFileCountKey(), 1)
			svr.statMap.AddCountInt64(logDate+"_"+conf.StatFileTotalSizeKey(), fileInfo.Size)
			svr.SaveStat(conf)
		}
		if _, err = svr.SaveFileInfoToLevelDB(logKey, fileInfo, conf.LogLevelDB(), conf); err != nil {
			log.Error(err)
		}
		if _, err := svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, conf.LevelDB(), conf); err != nil {
			log.Error("saveToLevelDB", err, fileInfo)
		}
		if _, err = svr.SaveFileInfoToLevelDB(pkg.MD5(fullpath), fileInfo, conf.LevelDB(), conf); err != nil {
			log.Error("saveToLevelDB", err, fileInfo)
		}
		return
	}
	if filename == conf.RemoveMd5File() {
		//svr.searchMap.Remove(fileInfo.Md5)
		if ok, err = ExistFromLevelDB(fileInfo.Md5, conf.LevelDB()); ok {
			svr.statMap.AddCountInt64(logDate+"_"+conf.StatisticsFileCountKey(), -1)
			svr.statMap.AddCountInt64(logDate+"_"+conf.StatFileTotalSizeKey(), -fileInfo.Size)
			svr.SaveStat(conf)
		}
		RemoveKeyFromLevelDB(logKey, conf.LogLevelDB())
		md5Path = pkg.MD5(fullpath)
		if err := RemoveKeyFromLevelDB(fileInfo.Md5, conf.LevelDB()); err != nil {
			log.Error("RemoveKeyFromLevelDB", err, fileInfo)
		}
		if err = RemoveKeyFromLevelDB(md5Path, conf.LevelDB()); err != nil {
			log.Error("RemoveKeyFromLevelDB", err, fileInfo)
		}
		// remove files.md5 for stat info(repair from LogLevelDb)
		logKey = fmt.Sprintf("%s_%s_%s", logDate, conf.FileMd5(), fileInfo.Md5)
		RemoveKeyFromLevelDB(logKey, conf.LogLevelDB())
		return
	}
	svr.SaveFileInfoToLevelDB(logKey, fileInfo, conf.LogLevelDB(), conf)
}

func ExistFromLevelDB(key string, db *leveldb.DB) (bool, error) {
	return db.Has([]byte(key), nil)
}

func GetFileInfoFromLevelDB(key string, conf *config.Config) (*FileInfo, error) {
	var (
		err      error
		data     []byte
		fileInfo FileInfo
	)
	if data, err = conf.LevelDB().Get([]byte(key), nil); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &fileInfo); err != nil {
		return nil, err
	}
	return &fileInfo, nil
}

//
func RemoveKeyFromLevelDB(key string, db *leveldb.DB) error {
	return db.Delete([]byte(key), nil)
}

// Read: ReceiveMd5s get md5s from request, and append every one that exist in levelDB to queue channel
func (svr *Server) ReceiveMd5s(path string, router *gin.RouterGroup, conf *config.Config) {
	router.GET(path, func(ctx *gin.Context) {
		var (
			err      error
			md5str   string
			fileInfo *FileInfo
			md5s     []string
		)
		r := ctx.Request
		if !IsPeer(r, conf) {
			log.Warn(fmt.Sprintf("ReceiveMd5s %s", pkg.GetClientIp(r)))
			ctx.JSON(http.StatusNotFound, GetClusterNotPermitMessage(r))
			return
		}
		r.ParseForm()
		md5str = r.FormValue("md5s")
		md5s = strings.Split(md5str, ",")
		AppendFunc := func(md5s []string) {
			for _, m := range md5s {
				if m != "" {
					if fileInfo, err = GetFileInfoFromLevelDB(m, conf); err != nil {
						log.Error(err)
						continue
					}
					svr.AppendToQueue(fileInfo, conf)
				}
			}
		}
		go AppendFunc(md5s)
	})
}

// Read: GetMd5sMapByDate use given date and file name to get md5 which will uer to create a commonMap
func GetMd5sMapByDate(date string, filename string, conf *config.Config) (*pkg.CommonMap, error) {
	var (
		err      error
		result   *pkg.CommonMap
		filePath string
		content  string
		lines    []string
		line     string
		cols     []string
		data     []byte
	)
	result = pkg.NewCommonMap(0)
	if filename == "" {
		filePath = conf.DataDir() + "/" + date + "/" + conf.FileMd5()
	} else {
		filePath = conf.DataDir() + "/" + date + "/" + filename
	}
	if !pkg.FileExists(filePath) {
		return result, fmt.Errorf("fpath %s not found", filePath)
	}
	if data, err = ioutil.ReadFile(filePath); err != nil {
		return result, err
	}
	content = string(data)
	lines = strings.Split(content, "\n")
	for _, line = range lines {
		cols = strings.Split(line, "|")
		if len(cols) > 2 {
			if _, err = strconv.ParseInt(cols[1], 10, 64); err != nil {
				continue
			}
			result.Add(cols[0])
		}
	}
	return result, nil
}

//Read: ??
func GetMd5sByDate(date string, filename string, conf *config.Config) (mapSet.Set, error) {
	var (
		keyPrefix string
		md5set    mapSet.Set
		keys      []string
	)
	md5set = mapSet.NewSet()
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, filename)
	iter := conf.LogLevelDB().NewIterator(levelDBUtil.BytesPrefix([]byte(keyPrefix)), nil)
	for iter.Next() {
		keys = strings.Split(string(iter.Key()), "_")
		if len(keys) >= 3 {
			md5set.Add(keys[2])
		}
	}
	iter.Release()
	return md5set, nil
}

func GetRequestURI(action string) string {
	return "/" + action
}

func (svr *Server) upload(ctx *gin.Context, conf *config.Config) {
	var (
		err          error
		ok           bool
		md5sum       string
		fileName     string
		fileInfo     FileInfo
		uploadFile   multipart.File
		uploadHeader *multipart.FileHeader
		scene        string
		output       string
		fileResult   FileResult
		code         string
		secret       interface{}
	)
	r := ctx.Request
	w := ctx.Writer
	output = r.FormValue("output")

	if conf.AuthUrl() != "" {
		if !CheckAuth(r, conf) {
			log.Warn("auth fail", r.Form)
			pkg.NotPermit(w, r)
			ctx.JSON(http.StatusNotFound, "auth fail")
			return
		}
	}

	md5sum = r.FormValue("md5")
	fileName = r.FormValue("filename")
	output = r.FormValue("output")
	if conf.ReadOnly() {
		ctx.JSON(http.StatusNotFound, "(error) readonly")
		return
	}
	if conf.EnableCustomPath() {
		fileInfo.Path = r.FormValue("path")
		fileInfo.Path = strings.Trim(fileInfo.Path, "/")
	}
	scene = r.FormValue("scene")
	code = r.FormValue("code")
	if scene == "" {
		//Just for Compatibility
		scene = r.FormValue("scenes")
	}
	//Read: default not enable google auth
	if conf.EnableGoogleAuth() && scene != "" {
		if secret, ok = svr.sceneMap.GetValue(scene); ok {
			if !VerifyGoogleCode(secret.(string), code, int64(conf.DownloadTokenExpire()/30)) {
				pkg.NotPermit(w, r)
				ctx.JSON(http.StatusUnauthorized, "invalid request,error google code")
				return
			}
		}
	}
	fileInfo.Md5 = md5sum
	fileInfo.ReName = fileName
	fileInfo.OffSet = -1
	if uploadFile, uploadHeader, err = r.FormFile("file"); err != nil {
		log.Error(err)
		ctx.JSON(http.StatusNotFound, err.Error())
		return
	}
	fileInfo.Peers = []string{}
	fileInfo.TimeStamp = time.Now().Unix()
	if scene == "" {
		scene = conf.DefaultScene() // Read: scene="default"
	}
	if output == "" {
		output = "text" //Read: default output = "json"
	}
	if !pkg.Contains(output, []string{"json", "text"}) {
		ctx.JSON(http.StatusNotFound, "output just support json or text")
		return
	}
	fileInfo.Scene = scene
	if _, err = CheckScene(scene, conf); err != nil {
		ctx.JSON(http.StatusNotFound, err.Error())
		return
	}
	if _, err = SaveUploadFile(uploadFile, uploadHeader, &fileInfo, r, conf); err != nil {
		ctx.JSON(http.StatusNotFound, err.Error())
		return
	}
	if conf.EnableDistinctFile() {
		if v, _ := GetFileInfoFromLevelDB(fileInfo.Md5, conf); v != nil && v.Md5 != "" {
			fileResult = BuildFileResult(v, r.Host, conf)
			if conf.RenameFile() {
				os.Remove(fileInfo.Path + "/" + fileInfo.ReName)
			} else {
				os.Remove(fileInfo.Path + "/" + fileInfo.Name)
			}

			ctx.JSON(http.StatusOK, fileResult)
			return
		}
	}
	if fileInfo.Md5 == "" {
		log.Warn(" fileInfo.Md5 is null")
		ctx.JSON(http.StatusNotFound, "fileInfo.Md5 is null")
		return
	}
	if md5sum != "" && fileInfo.Md5 != md5sum {
		log.Warn(" fileInfo.Md5 and md5sum !=")
		ctx.JSON(http.StatusNotFound, "fileInfo.Md5 and md5sum !=")
		return
	}
	if !conf.EnableDistinctFile() {
		// bugfix filecount stat
		fileInfo.Md5 = pkg.MD5(GetFilePathByInfo(&fileInfo, false))
	}
	if conf.EnableMergeSmallFile() && fileInfo.Size < int64(conf.SmallFileSize()) {
		if err = svr.SaveSmallFile(&fileInfo, conf); err != nil {
			log.Error(err)
			ctx.JSON(http.StatusNotFound, err.Error())
			return
		}
	}
	svr.saveFileMd5Log(&fileInfo, conf.FileMd5(), conf) //maybe slow
	go svr.postFileToPeer(&fileInfo, conf)
	if fileInfo.Size <= 0 {
		log.Error("file size is zero")
		ctx.JSON(http.StatusNotFound, "file size is zero")
		return
	}
	fileResult = BuildFileResult(&fileInfo, r.Host, conf)
	ctx.JSON(http.StatusOK, fileResult)
}

func (svr *Server) SaveSmallFile(fileInfo *FileInfo, conf *config.Config) error {
	var (
		err      error
		filename string
		fpath    string
		srcFile  *os.File
		desFile  *os.File
		largeDir string
		destPath string
		reName   string
		fileExt  string
	)
	filename = fileInfo.Name
	fileExt = path.Ext(filename)
	if fileInfo.ReName != "" {
		filename = fileInfo.ReName
	}
	fpath = fileInfo.Path + "/" + filename
	largeDir = conf.LargeDir() + "/" + conf.PeerId()
	if !pkg.FileExists(largeDir) {
		os.MkdirAll(largeDir, 0775)
	}
	reName = fmt.Sprintf("%d", pkg.RandInt(100, 300))
	destPath = largeDir + "/" + reName
	svr.lockMap.LockKey(destPath)
	defer svr.lockMap.UnLockKey(destPath)
	if pkg.FileExists(fpath) {
		srcFile, err = os.OpenFile(fpath, os.O_CREATE|os.O_RDONLY, 06666)
		if err != nil {
			return err
		}
		defer srcFile.Close()
		desFile, err = os.OpenFile(destPath, os.O_CREATE|os.O_RDWR, 06666)
		if err != nil {
			return err
		}
		defer desFile.Close()
		fileInfo.OffSet, err = desFile.Seek(0, 2)
		if _, err = desFile.Write([]byte("1")); err != nil {
			//first byte set 1
			return err
		}
		fileInfo.OffSet, err = desFile.Seek(0, 2)
		if err != nil {
			return err
		}
		fileInfo.OffSet = fileInfo.OffSet - 1 //minus 1 byte
		fileInfo.Size = fileInfo.Size + 1
		fileInfo.ReName = fmt.Sprintf("%s,%d,%d,%s", reName, fileInfo.OffSet, fileInfo.Size, fileExt)
		if _, err = io.Copy(desFile, srcFile); err != nil {
			return err
		}
		srcFile.Close()
		os.Remove(fpath)
	}
	return nil
}

func BenchMark(ctx *gin.Context, conf *config.Config) {
	t := time.Now()
	batch := new(leveldb.Batch)
	for i := 0; i < 100000000; i++ {
		f := FileInfo{}
		f.Peers = []string{"http://192.168.0.1", "http://192.168.2.5"}
		f.Path = "20190201/19/02"
		s := strconv.Itoa(i)
		s = pkg.MD5(s)
		f.Name = s
		f.Md5 = s
		if data, err := json.Marshal(&f); err == nil {
			batch.Put([]byte(s), data)
		}
		if i%10000 == 0 {
			if batch.Len() > 0 {
				conf.LevelDB().Write(batch, nil)
				//				batch = new(leveldb.Batch)
				batch.Reset()
			}
			fmt.Println(i, time.Since(t).Seconds())
		}
		//fmt.Println(server.GetFileInfoFromLevelDB(s))
	}
	pkg.WriteFile("time.txt", time.Since(t).String())
	fmt.Println(time.Since(t).String())
}

func (svr *Server) RepairStatWeb(ctx *gin.Context, conf *config.Config) {
	var (
		result JsonResult
		date   string
		inner  string
	)
	r := ctx.Request
	if !IsPeer(r, conf) {
		result.Message = GetClusterNotPermitMessage(r)
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	date = r.FormValue("date")
	inner = r.FormValue("inner")
	if ok, err := regexp.MatchString("\\d{8}", date); err != nil || !ok {
		result.Message = "invalid date"
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	if date == "" || len(date) != 8 {
		date = pkg.GetToDay()
	}
	if inner != "1" {
		for _, peer := range conf.Peers() {
			req := httplib.Post(peer + GetRequestURI("repair_stat"))
			req.Param("inner", "1")
			req.Param("date", date)
			if _, err := req.String(); err != nil {
				log.Error(err)
			}
		}
	}
	result.Data = svr.RepairStatByDate(date, conf)
	result.Status = "ok"

	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) Stat(path string, router *gin.RouterGroup, conf *config.Config) {
	router.GET(path, func(ctx *gin.Context) {
		var (
			result   JsonResult
			inner    string
			eChart   string
			category []string
			barCount []int64
			barSize  []int64
			dataMap  map[string]interface{}
		)
		r := ctx.Request
		if !IsPeer(r, conf) {
			result.Message = GetClusterNotPermitMessage(r)
			ctx.JSON(http.StatusNotFound, result)
			return
		}
		r.ParseForm()
		inner = r.FormValue("inner")
		eChart = r.FormValue("echart")
		data := svr.GetStat(conf)
		result.Status = "ok"
		result.Data = data
		if eChart == "1" {
			dataMap = make(map[string]interface{}, 3)
			for _, v := range data {
				barCount = append(barCount, v.FileCount)
				barSize = append(barSize, v.TotalSize)
				category = append(category, v.Date)
			}
			dataMap["category"] = category
			dataMap["barCount"] = barCount
			dataMap["barSize"] = barSize
			result.Data = dataMap
		}
		if inner == "1" {
			ctx.JSON(http.StatusOK, data)
			return
		}
		ctx.JSON(http.StatusOK, result)
	})
}

func RegisterExit(conf *config.Config) {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				conf.LevelDB().Close()
				log.Info("Exit", s)
				os.Exit(1)
			}
		}
	}()
}

// Read: append the file info to queen channel, the file info will send to all peers
func (svr *Server) AppendToQueue(fileInfo *FileInfo, conf *config.Config) {

	for (len(svr.queueToPeers) + conf.QueueSize()/10) > conf.QueueSize() {
		time.Sleep(time.Millisecond * 50)
	}
	svr.queueToPeers <- *fileInfo
}

func (svr *Server) AppendToDownloadQueue(fileInfo *FileInfo, conf *config.Config) {
	for (len(svr.queueFromPeers) + conf.QueueSize()/10) > conf.QueueSize() {
		time.Sleep(time.Millisecond * 50)
	}
	svr.queueFromPeers <- *fileInfo
}

func (svr *Server) ConsumerDownLoad(conf *config.Config) {
	ConsumerFunc := func() {
		for {
			fileInfo := <-svr.queueFromPeers
			if len(fileInfo.Peers) <= 0 {
				log.Warn("Peer is null", fileInfo)
				continue
			}
			for _, peer := range fileInfo.Peers {
				if strings.Contains(peer, "127.0.0.1") {
					log.Warn("sync error with 127.0.0.1", fileInfo)
					continue
				}
				if peer != svr.host {
					svr.DownloadFromPeer(peer, &fileInfo, conf)
					break
				}
			}
		}
	}
	for i := 0; i < conf.SyncWorker(); i++ {
		go ConsumerFunc()
	}
}

func (svr *Server) RemoveDownloading(conf *config.Config) {
	go func() {
		for {
			iter := conf.LevelDB().NewIterator(levelDBUtil.BytesPrefix([]byte("downloading_")), nil)
			for iter.Next() {
				key := iter.Key()
				keys := strings.Split(string(key), "_")
				if len(keys) == 3 {
					if t, err := strconv.ParseInt(keys[1], 10, 64); err == nil && time.Now().Unix()-t > 60*10 {
						os.Remove(keys[2])
					}
				}
			}
			iter.Release()
			time.Sleep(time.Minute * 3)
		}
	}()
}

func (svr *Server) ConsumerLog(conf *config.Config) {
	go func() {
		for fileLog := range svr.queueFileLog {
			svr.saveFileMd5Log(fileLog.FileInfo, fileLog.FileName, conf)
		}
	}()
}

func (svr *Server) LoadSearchDict(conf *config.Config) {
	go func() {
		log.Info("Load search dict ....")
		f, err := os.Open(conf.SearchFile())
		if err != nil {
			log.Error(err)
			return
		}
		defer f.Close()
		r := bufio.NewReader(f)
		for {
			line, isprefix, err := r.ReadLine()
			for isprefix && err == nil {
				kvs := strings.Split(string(line), "\t")
				if len(kvs) == 2 {
					svr.searchMap.Put(kvs[0], kvs[1])
				}
			}
		}
		log.Info("finish load search dict")
	}()
}

func (svr *Server) SaveSearchDict(conf *config.Config) {
	svr.lockMap.LockKey(conf.SearchFile())
	defer svr.lockMap.UnLockKey(conf.SearchFile())

	searchDict := svr.searchMap.Get()
	searchFile, err := os.OpenFile(conf.SearchFile(), os.O_RDWR, 0755)
	if err != nil {
		log.Error(err)
		return
	}
	defer searchFile.Close()

	for k, v := range searchDict {
		searchFile.WriteString(fmt.Sprintf("%s\t%s", k, v.(string)))
	}
}

func (svr *Server) ConsumerUpload(conf *config.Config) {
	ConsumerFunc := func() {
		for wr := range svr.QueueUpload {
			svr.upload(wr.Ctx, conf)
			svr.rtMap.AddCountInt64(conf.UploadCounterKey(), wr.Ctx.Request.ContentLength)
			if v, ok := svr.rtMap.GetValue(conf.UploadCounterKey()); ok {
				if v.(int64) > 1*1024*1024*1024 {
					var _v int64
					svr.rtMap.Put(conf.UploadCounterKey(), _v)
					debug.FreeOSMemory()
				}
			}
			wr.Done <- true
		}
	}
	for i := 0; i < conf.UploadWorker(); i++ {
		go ConsumerFunc()
	}
}

// Read :  AutoRepair what?
func (svr *Server) AutoRepair(forceRepair bool, conf *config.Config) {
	if svr.lockMap.IsLock("AutoRepair") {
		log.Warn("Lock AutoRepair")
		return
	}
	svr.lockMap.LockKey("AutoRepair")
	defer svr.lockMap.UnLockKey("AutoRepair")

	AutoRepairFunc := func(forceRepair bool) {
		var (
			dateStats []StatDateFileInfo
			err       error
			countKey  string
			md5s      string
			localSet  mapSet.Set
			remoteSet mapSet.Set
			allSet    mapSet.Set
			tmpSet    mapSet.Set
			fileInfo  *FileInfo
		)
		defer func() {
			if re := recover(); re != nil {
				buffer := debug.Stack()
				log.Error("AutoRepair")
				log.Error(re)
				log.Error(string(buffer))
			}
		}()
		Update := func(peer string, dateStat StatDateFileInfo) {
			//从远端拉数据过来
			req := httplib.Get(fmt.Sprintf("%s%s?date=%s&force=%s", peer, GetRequestURI("sync"), dateStat.Date, "1"))
			req.SetTimeout(time.Second*5, time.Second*5)
			if _, err = req.String(); err != nil {
				log.Error(err)
			}
			log.Info(fmt.Sprintf("syn file from %s date %s", peer, dateStat.Date))
		}
		for _, peer := range conf.Peers() {
			req := httplib.Post(peer + GetRequestURI("stat"))
			req.Param("inner", "1")
			req.SetTimeout(time.Second*5, time.Second*15)
			if err = req.ToJSON(&dateStats); err != nil {
				log.Error(err)
				continue
			}
			for _, dateStat := range dateStats {
				if dateStat.Date == "all" {
					continue
				}
				countKey = dateStat.Date + "_" + conf.StatisticsFileCountKey()
				if v, ok := svr.statMap.GetValue(countKey); ok {
					switch v.(type) {
					case int64:
						if v.(int64) != dateStat.FileCount || forceRepair {
							//不相等,找差异
							//TODO
							req := httplib.Post(peer + GetRequestURI("get_md5s_by_date"))
							req.SetTimeout(time.Second*15, time.Second*60)
							req.Param("date", dateStat.Date)
							if md5s, err = req.String(); err != nil {
								continue
							}
							if localSet, err = GetMd5sByDate(dateStat.Date, conf.FileMd5(), conf); err != nil {
								log.Error(err)
								continue
							}
							remoteSet = pkg.StrToMapSet(md5s, ",")
							allSet = localSet.Union(remoteSet)
							md5s = pkg.MapSetToStr(allSet.Difference(localSet), ",")
							req = httplib.Post(peer + GetRequestURI("receive_md5s"))
							req.SetTimeout(time.Second*15, time.Second*60)
							req.Param("md5s", md5s)
							req.String()
							tmpSet = allSet.Difference(remoteSet)
							for v := range tmpSet.Iter() {
								if v != nil {
									if fileInfo, err = GetFileInfoFromLevelDB(v.(string), conf); err != nil {
										log.Error(err)
										continue
									}
									svr.AppendToQueue(fileInfo, conf)
								}
							}
							//Update(peer,dateStat)
						}
					}
				} else {
					Update(peer, dateStat)
				}
			}
		}
	}
	AutoRepairFunc(forceRepair)
}

func (svr *Server) CleanLogLevelDBByDate(date string, filename string, conf *config.Config) {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("CleanLogLevelDBByDate")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		err       error
		keyPrefix string
		keys      mapSet.Set
	)
	keys = mapSet.NewSet()
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, filename)
	iter := conf.LogLevelDB().NewIterator(levelDBUtil.BytesPrefix([]byte(keyPrefix)), nil)
	for iter.Next() {
		keys.Add(string(iter.Value()))
	}
	iter.Release()
	for key := range keys.Iter() {
		err = RemoveKeyFromLevelDB(key.(string), conf.LogLevelDB())
		if err != nil {
			log.Error(err)
		}
	}
}

func (svr *Server) CleanAndBackUp(conf *config.Config) {
	Clean := func() {
		if svr.curDate != pkg.GetToDay() {
			filenames := []string{conf.Md5QueueFile(), conf.Md5ErrorFile(), conf.RemoveMd5File()}
			yesterday := pkg.GetDayFromTimeStamp(time.Now().AddDate(0, 0, -1).Unix())
			for _, filename := range filenames {
				svr.CleanLogLevelDBByDate(yesterday, filename, conf)
			}
			svr.BackUpMetaDataByDate(yesterday, conf)
			svr.curDate = pkg.GetToDay()
		}
	}
	go func() {
		for {
			time.Sleep(time.Hour * 6)
			Clean()
		}
	}()
}

func (svr *Server) LoadQueueSendToPeer(conf *config.Config) {
	if queue, err := LoadFileInfoByDate(pkg.GetToDay(), conf.Md5QueueFile(), conf.LevelDB()); err != nil {
		log.Error(err)
	} else {
		for fileInfo := range queue.Iter() {
			//svr.queueFromPeers <- *fileInfo.(*FileInfo)
			svr.AppendToDownloadQueue(fileInfo.(*FileInfo), conf)
		}
	}
}

func (svr *Server) SearchDict(kw string, conf *config.Config) []FileInfo {
	var (
		fileInfos []FileInfo
		fileInfo  *FileInfo
	)
	for dict := range svr.searchMap.Iter() {
		if strings.Contains(dict.Val.(string), kw) {
			if fileInfo, _ = GetFileInfoFromLevelDB(dict.Key, conf); fileInfo != nil {
				fileInfos = append(fileInfos, *fileInfo)
			}
		}
	}
	return fileInfos
}

func (svr *Server) Status(ctx *gin.Context, conf *config.Config) {
	var (
		status   JsonResult
		sts      map[string]interface{}
		today    string
		sumset   mapSet.Set
		ok       bool
		v        interface{}
		err      error
		appDir   string
		diskInfo *disk.UsageStat
		memInfo  *mem.VirtualMemoryStat
	)
	memStat := new(runtime.MemStats)
	runtime.ReadMemStats(memStat)
	today = pkg.GetToDay()
	sts = make(map[string]interface{})
	sts["Fs.QueueFromPeers"] = len(svr.queueFromPeers)
	sts["Fs.QueueToPeers"] = len(svr.queueToPeers)
	sts["Fs.QueueFileLog"] = len(svr.queueFileLog)
	for _, k := range []string{conf.FileMd5(), conf.Md5ErrorFile(), conf.Md5QueueFile()} {
		k2 := fmt.Sprintf("%s_%s", today, k)
		if v, ok = svr.sumMap.GetValue(k2); ok {
			sumset = v.(mapSet.Set)
			if k == conf.Md5QueueFile() {
				sts["Fs.QueueSetSize"] = sumset.Cardinality()
			}
			if k == conf.Md5ErrorFile() {
				sts["Fs.ErrorSetSize"] = sumset.Cardinality()
			}
			if k == conf.FileMd5() {
				sts["Fs.FileSetSize"] = sumset.Cardinality()
			}
		}
	}
	sts["Fs.AutoRepair"] = conf.AutoRepair()
	sts["Fs.QueueUpload"] = len(svr.QueueUpload)
	sts["Fs.RefreshInterval"] = conf.RefreshInterval()
	sts["Fs.Peers"] = conf.Peers()
	sts["Fs.Local"] = svr.host
	sts["Fs.FileStats"] = svr.GetStat(conf)
	sts["Fs.ShowDir"] = conf.ShowDir()
	sts["Sys.NumGoroutine"] = runtime.NumGoroutine()
	sts["Sys.NumCpu"] = runtime.NumCPU()
	sts["Sys.Alloc"] = memStat.Alloc
	sts["Sys.TotalAlloc"] = memStat.TotalAlloc
	sts["Sys.HeapAlloc"] = memStat.HeapAlloc
	sts["Sys.Frees"] = memStat.Frees
	sts["Sys.HeapObjects"] = memStat.HeapObjects
	sts["Sys.NumGC"] = memStat.NumGC
	sts["Sys.GCCPUFraction"] = memStat.GCCPUFraction
	sts["Sys.GCSys"] = memStat.GCSys
	//sts["Sys.MemInfo"] = memStat
	appDir, err = filepath.Abs(".")
	if err != nil {
		log.Error(err)
	}
	diskInfo, err = disk.Usage(appDir)
	if err != nil {
		log.Error(err)
	}
	sts["Sys.DiskInfo"] = diskInfo
	memInfo, err = mem.VirtualMemory()
	if err != nil {
		log.Error(err)
	}
	sts["Sys.MemInfo"] = memInfo
	status.Status = "ok"
	status.Data = sts

	ctx.JSON(http.StatusOK, status)
}

func (svr *Server) HeartBeat(ctx *gin.Context) {
}

func test() {

	testLock := func() {
		wg := sync.WaitGroup{}
		tt := func(i int, wg *sync.WaitGroup) {
			//if server.lockMap.IsLock("xx") {
			//	return
			//}
			//fmt.Println("timeer len",len(server.lockMap.Get()))
			//time.Sleep(time.Nanosecond*10)
			Svr.lockMap.LockKey("xx")
			defer Svr.lockMap.UnLockKey("xx")
			//time.Sleep(time.Nanosecond*1)
			//fmt.Println("xx", i)
			wg.Done()
		}
		go func() {
			for {
				time.Sleep(time.Second * 1)
				fmt.Println("timeer len", len(Svr.lockMap.Get()), Svr.lockMap.Get())
			}
		}()
		fmt.Println(len(Svr.lockMap.Get()))
		for i := 0; i < 10000; i++ {
			wg.Add(1)
			go tt(i, &wg)
		}
		fmt.Println(len(Svr.lockMap.Get()))
		fmt.Println(len(Svr.lockMap.Get()))
		Svr.lockMap.LockKey("abc")
		fmt.Println("lock")
		time.Sleep(time.Second * 5)
		Svr.lockMap.UnLockKey("abc")
		Svr.lockMap.LockKey("abc")
		Svr.lockMap.UnLockKey("abc")
	}
	_ = testLock
	testFile := func() {
		var (
			err error
			f   *os.File
		)
		f, err = os.OpenFile("tt", os.O_CREATE|os.O_RDWR, 0777)
		if err != nil {
			fmt.Println(err)
		}
		f.WriteAt([]byte("1"), 100)
		f.Seek(0, 2)
		f.Write([]byte("2"))
		//fmt.Println(f.Seek(0, 2))
		//fmt.Println(f.Seek(3, 2))
		//fmt.Println(f.Seek(3, 0))
		//fmt.Println(f.Seek(3, 1))
		//fmt.Println(f.Seek(3, 0))
		//f.Write([]byte("1"))
	}
	_ = testFile
	//testFile()
	//testLock()
}

type hookDataStore struct {
	tusd.DataStore
	conf *config.Config
}

type httpError struct {
	error
	statusCode int
}

func (err httpError) StatusCode() int {
	return err.statusCode
}

func (err httpError) Body() []byte {
	return []byte(err.Error())
}

func (store hookDataStore) NewUpload(info tusd.FileInfo) (id string, err error) {
	var (
		jsonResult JsonResult
	)
	if store.conf.AuthUrl() != "" {
		if auth_token, ok := info.MetaData["auth_token"]; !ok {
			msg := "token auth fail,auth_token is not in http header Upload-Metadata," +
				"in uppy uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca' })"
			log.Error(msg, fmt.Sprintf("current header:%v", info.MetaData))
			return "", httpError{error: errors.New(msg), statusCode: 401}
		} else {
			req := httplib.Post(store.conf.AuthUrl())
			req.Param("auth_token", auth_token)
			req.SetTimeout(time.Second*5, time.Second*10)
			content, err := req.String()
			content = strings.TrimSpace(content)
			if strings.HasPrefix(content, "{") && strings.HasSuffix(content, "}") {
				if err = json.Unmarshal([]byte(content), &jsonResult); err != nil {
					log.Error(err)
					return "", httpError{error: errors.New(err.Error() + content), statusCode: 401}
				}
				if jsonResult.Data != "ok" {
					return "", httpError{error: errors.New(content), statusCode: 401}
				}
			} else {
				if err != nil {
					log.Error(err)
					return "", err
				}
				if strings.TrimSpace(content) != "ok" {
					return "", httpError{error: errors.New(content), statusCode: 401}
				}
			}
		}
	}
	return store.DataStore.NewUpload(info)
}

func (svr *Server) initTus(conf *config.Config) {
	var (
		err     error
		fileLog *os.File
		bigDir  string
	)
	BIG_DIR := conf.StoreDir() + "/_big/" + conf.PeerId()
	os.MkdirAll(BIG_DIR, 0775)
	os.MkdirAll(conf.LogDir(), 0775)
	store := filestore.FileStore{
		Path: BIG_DIR,
	}
	if fileLog, err = os.OpenFile(conf.LogDir()+"/tusd.log", os.O_CREATE|os.O_RDWR, 0666); err != nil {
		log.Error(err)
		panic("initTus")
	}
	go func() {
		for {
			if fi, err := fileLog.Stat(); err != nil {
				log.Error(err)
			} else {
				if fi.Size() > 1024*1024*500 {
					//500M
					pkg.CopyFile(conf.LogDir()+"/tusd.log", conf.LogDir()+"/tusd.log.2")
					fileLog.Seek(0, 0)
					fileLog.Truncate(0)
					fileLog.Seek(0, 2)
				}
			}
			time.Sleep(time.Second * 30)
		}
	}()
	l := slog.New(fileLog, "[tusd] ", slog.LstdFlags)
	bigDir = conf.BigUploadPathSuffix()

	composer := tusd.NewStoreComposer()
	// support raw tus upload and download
	store.GetReaderExt = func(id string) (io.Reader, error) {
		var (
			offset int64
			err    error
			length int
			buffer []byte
			fi     *FileInfo
			fn     string
		)
		if fi, err = GetFileInfoFromLevelDB(id, conf); err != nil {
			log.Error(err)
			return nil, err
		} else {
			if conf.AuthUrl() != "" {
				fileResult := pkg.JsonEncodePretty(BuildFileResult(fi, "", conf))
				bufferReader := bytes.NewBuffer([]byte(fileResult))
				return bufferReader, nil
			}
			fn = fi.Name
			if fi.ReName != "" {
				fn = fi.ReName
			}
			fp := fi.Path + "/" + fn
			if pkg.FileExists(fp) {
				log.Info(fmt.Sprintf("download:%s", fp))
				return os.Open(fp)
			}
			ps := strings.Split(fp, ",")
			if len(ps) > 2 && pkg.FileExists(ps[0]) {
				if length, err = strconv.Atoi(ps[2]); err != nil {
					return nil, err
				}
				if offset, err = strconv.ParseInt(ps[1], 10, 64); err != nil {
					return nil, err
				}
				if buffer, err = pkg.ReadFileByOffSet(ps[0], offset, length); err != nil {
					return nil, err
				}
				if buffer[0] == '1' {
					bufferReader := bytes.NewBuffer(buffer[1:])
					return bufferReader, nil
				} else {
					msg := "data no sync"
					log.Error(msg)
					return nil, errors.New(msg)
				}
			}
			return nil, errors.New(fmt.Sprintf("%s not found", fp))
		}
	}
	store.UseIn(composer)
	SetupPreHooks := func(composer *tusd.StoreComposer) {
		composer.UseCore(hookDataStore{
			DataStore: composer.Core,
			conf:      conf,
		})
	}
	SetupPreHooks(composer)
	handler, err := tusd.NewHandler(tusd.Config{
		Logger:                  l,
		BasePath:                bigDir,
		StoreComposer:           composer,
		NotifyCompleteUploads:   true,
		RespectForwardedHeaders: true,
	})
	notify := func(handler *tusd.Handler) {
		for {
			select {
			case info := <-handler.CompleteUploads:
				log.Info("CompleteUploads", info)
				name := ""
				pathCustom := ""
				scene := conf.DefaultScene()
				if v, ok := info.MetaData["filename"]; ok {
					name = v
				}
				if v, ok := info.MetaData["scene"]; ok {
					scene = v
				}
				if v, ok := info.MetaData["path"]; ok {
					pathCustom = v
				}
				var err error
				md5sum := ""
				oldFullPath := BIG_DIR + "/" + info.ID + ".bin"
				infoFullPath := BIG_DIR + "/" + info.ID + ".info"
				if md5sum, err = pkg.GetFileSumByName(oldFullPath, conf.FileSumArithmetic()); err != nil {
					log.Error(err)
					continue
				}
				ext := path.Ext(name)
				filename := md5sum + ext
				if name != "" {
					filename = name
				}
				if conf.RenameFile() {
					filename = md5sum + ext
				}
				timeStamp := time.Now().Unix()
				fpath := time.Now().Format("/20060102/15/04/")
				if pathCustom != "" {
					fpath = "/" + strings.Replace(pathCustom, ".", "", -1) + "/"
				}
				newFullPath := conf.StoreDir() + "/" + scene + fpath + conf.PeerId() + "/" + filename
				if pathCustom != "" {
					newFullPath = conf.StoreDir() + "/" + scene + fpath + filename
				}
				if fi, err := GetFileInfoFromLevelDB(md5sum, conf); err != nil {
					log.Error(err)
				} else {
					tpath := GetFilePathByInfo(fi, true)
					if fi.Md5 != "" && pkg.FileExists(tpath) {
						if _, err := svr.SaveFileInfoToLevelDB(info.ID, fi, conf.LevelDB(), conf); err != nil {
							log.Error(err)
						}
						log.Info(fmt.Sprintf("file is found md5:%s", fi.Md5))
						log.Info("remove file:", oldFullPath)
						log.Info("remove file:", infoFullPath)
						os.Remove(oldFullPath)
						os.Remove(infoFullPath)
						continue
					}
				}
				fpath2 := ""
				fpath2 = conf.StoreDir() + "/" + conf.DefaultScene() + fpath + conf.PeerId()
				if pathCustom != "" {
					fpath2 = conf.StoreDir() + "/" + conf.DefaultScene() + fpath
					fpath2 = strings.TrimRight(fpath2, "/")
				}

				os.MkdirAll(fpath2, 0775)
				fileInfo := &FileInfo{
					Name:      name,
					Path:      fpath2,
					ReName:    filename,
					Size:      info.Size,
					TimeStamp: timeStamp,
					Md5:       md5sum,
					Peers:     []string{svr.host},
					OffSet:    -1,
				}
				if err = os.Rename(oldFullPath, newFullPath); err != nil {
					log.Error(err)
					continue
				}
				log.Info(fileInfo)
				os.Remove(infoFullPath)
				if _, err = svr.SaveFileInfoToLevelDB(info.ID, fileInfo, conf.LevelDB(), conf); err != nil {
					//assosiate file id
					log.Error(err)
				}
				svr.SaveFileMd5Log(fileInfo, conf.FileMd5(), conf)
				go svr.postFileToPeer(fileInfo, conf)
				callBack := func(info tusd.FileInfo, fileInfo *FileInfo) {
					if callback_url, ok := info.MetaData["callback_url"]; ok {
						req := httplib.Post(callback_url)
						req.SetTimeout(time.Second*10, time.Second*10)
						req.Param("info", pkg.JsonEncodePretty(fileInfo))
						req.Param("id", info.ID)
						if _, err := req.String(); err != nil {
							log.Error(err)
						}
					}
				}
				go callBack(info, fileInfo)
			}
		}
	}
	go notify(handler)
	if err != nil {
		log.Error(err)
	}
	http.Handle(bigDir, http.StripPrefix(bigDir, handler))
}

// initComponent init current host ip
func (svr *Server) InitComponent(isReload bool, conf *config.Config) {
	var (
		ip string
	)
	if ip = os.Getenv("GO_FASTDFS_IP"); ip == "" {
		ip = pkg.GetPublicIP()
	}
	if conf.Host() == "" {
		if len(strings.Split(conf.Port(), ":")) == 2 {
			Svr.host = fmt.Sprintf("http://%s:%s", ip, strings.Split(conf.Port(), ":")[1])
			conf.SetHost(Svr.host)
			conf.SetDownloadDomain()
		}
	} else {
		if strings.HasPrefix(conf.Host(), "http") {
			Svr.host = conf.Host()
		} else {
			Svr.host = "http://" + conf.Host()
		}
	}
	ex, _ := regexp.Compile("\\d+\\.\\d+\\.\\d+\\.\\d+")
	var peers []string
	for _, peer := range conf.Peers() {
		if pkg.Contains(ip, ex.FindAllString(peer, -1)) ||
			pkg.Contains("127.0.0.1", ex.FindAllString(peer, -1)) {
			continue
		}
		if strings.HasPrefix(peer, "http") {
			peers = append(peers, peer)
		} else {
			peers = append(peers, "http://"+peer)
		}
	}
	conf.SetPeers(peers)
	if !isReload {
		svr.FormatStatInfo(conf)
		if conf.EnableTus() {
			svr.initTus(conf)
		}
	}
	for _, s := range conf.Scenes() {
		kv := strings.Split(s, ":")
		if len(kv) == 2 {
			svr.sceneMap.Put(kv[0], kv[1])
		}
	}
	if conf.ReadTimeout() == 0 {
		conf.SetReadTimeout(60 * 10)
	}
	if conf.WriteTimeout() == 0 {
		conf.SetWriteTimeout(60 * 10)
	}
	if conf.SyncWorker() == 0 {
		conf.SetSyncWorker(200)
	}
	if conf.UploadWorker() == 0 {
		conf.SetUploadWorker(runtime.NumCPU() + 4)
		if runtime.NumCPU() < 4 {
			conf.SetUploadWorker(8)
		}
	}
	if conf.UploadQueueSize() == 0 {
		conf.SetUploadQueueSize(200)
	}
	if conf.RetryCount() == 0 {
		conf.SetRetryCount(3)
	}
	if conf.SyncDelay() == 0 {
		conf.SetSyncDelay(60)
	}
	if conf.WatchChanSize() == 0 {
		conf.SetWatchChanSize(100000)
	}
}

// GetFilePathFromRequest
func GetFilePathFromRequest(ctx *gin.Context, conf *config.Config) (string, string) {
	var (
		err       error
		fullPath  string
		smallPath string
		prefix    string
	)
	r := ctx.Request
	fullPath = r.RequestURI[1:]
	fullPath = strings.Split(fullPath, "?")[0] // just path
	fullPath = conf.StoreDirName() + "/" + fullPath
	prefix = "/" + conf.LargeDir() + "/"

	if strings.HasPrefix(r.RequestURI, prefix) {
		smallPath = fullPath //notice order
		fullPath = strings.Split(fullPath, ",")[0]
	}
	if fullPath, err = url.PathUnescape(fullPath); err != nil {
		log.Println(err)
	}
	return fullPath, smallPath
}

func SaveUploadFile(file multipart.File, header *multipart.FileHeader, fileInfo *FileInfo, r *http.Request, conf *config.Config) (*FileInfo, error) {
	var (
		err     error
		outFile *os.File
		folder  string
		fi      os.FileInfo
	)
	defer file.Close()
	_, fileInfo.Name = filepath.Split(header.Filename)
	// bugfix for ie upload file contain fullpath
	if len(conf.Extensions()) > 0 && !pkg.Contains(path.Ext(fileInfo.Name), conf.Extensions()) {
		return fileInfo, errors.New("(error)file extension mismatch")
	}

	if conf.RenameFile() {
		fileInfo.ReName = pkg.MD5(pkg.GetUUID()) + path.Ext(fileInfo.Name)
	}
	folder = time.Now().Format("20060102/15/04")
	if conf.PeerId() != "" {
		folder = fmt.Sprintf(folder+"/%s", conf.PeerId())
	}
	if fileInfo.Scene != "" {
		folder = fmt.Sprintf(conf.StoreDir()+"/%s/%s", fileInfo.Scene, folder)
	} else {
		folder = fmt.Sprintf(conf.StoreDir()+"/%s", folder)
	}
	if fileInfo.Path != "" {
		if strings.HasPrefix(fileInfo.Path, conf.StoreDir()) {
			folder = fileInfo.Path
		} else {
			folder = conf.StoreDir() + "/" + fileInfo.Path
		}
	}
	if !pkg.FileExists(folder) {
		os.MkdirAll(folder, 0775)
	}
	outPath := fmt.Sprintf(folder+"/%s", fileInfo.Name)
	if fileInfo.ReName != "" {
		outPath = fmt.Sprintf(folder+"/%s", fileInfo.ReName)
	}
	if pkg.FileExists(outPath) && conf.EnableDistinctFile() {
		for i := 0; i < 10000; i++ {
			outPath = fmt.Sprintf(folder+"/%d_%s", i, filepath.Base(header.Filename))
			fileInfo.Name = fmt.Sprintf("%d_%s", i, header.Filename)
			if !pkg.FileExists(outPath) {
				break
			}
		}
	}
	log.Info(fmt.Sprintf("upload: %s", outPath))
	if outFile, err = os.Create(outPath); err != nil {
		return fileInfo, err
	}
	if err != nil {
		log.Error(err)
		return fileInfo, errors.New("(error)fail," + err.Error())
	}
	defer outFile.Close()

	if _, err = io.Copy(outFile, file); err != nil {
		log.Error(err)
		return fileInfo, errors.New("(error)fail," + err.Error())
	}
	if fi, err = outFile.Stat(); err != nil {
		log.Error(err)
	} else {
		fileInfo.Size = fi.Size()
	}

	if fi.Size() != header.Size {
		return fileInfo, errors.New("(error)file uncomplete")
	}
	v := "" // pkg.GetFileSum(outFile, config.Commonconfig.FileSumArithmetic)
	if conf.EnableDistinctFile() {
		v = pkg.GetFileSum(outFile, conf.FileSumArithmetic())
	} else {
		v = pkg.MD5(GetFilePathByInfo(fileInfo, false))
	}
	fileInfo.Md5 = v
	//fileInfo.Path = folder //strings.Replace( folder,DOCKER_DIR,"",1)
	fileInfo.Peers = append(fileInfo.Peers, Svr.host)
	//fmt.Println("upload",fileInfo)

	return fileInfo, nil
}
