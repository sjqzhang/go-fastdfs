package model

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"io/ioutil"
	slog "log"
	random "math/rand"
	"mime/multipart"
	"net/http"
	"net/smtp"
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
	mapset "github.com/deckarep/golang-set"
	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/internal/util"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
	levelDBUtil "github.com/syndtr/goleveldb/leveldb/util"

	"github.com/nfnt/resize"
	"github.com/radovskyb/watcher"

	"github.com/sjqzhang/googleAuthenticator"
	"github.com/sjqzhang/tusd"
	"github.com/sjqzhang/tusd/filestore"
)

var (
	Svr           *Server
	StaticHandler http.Handler
)

type Server struct {
	ldb            *leveldb.DB
	logDB          *leveldb.DB
	statMap        *util.CommonMap
	sumMap         *util.CommonMap
	rtMap          *util.CommonMap
	queueToPeers   chan FileInfo
	queueFromPeers chan FileInfo
	queueFileLog   chan *FileLog
	queueUpload    chan WrapReqResp
	lockMap        *util.CommonMap
	sceneMap       *util.CommonMap
	searchMap      *util.CommonMap
	curDate        string
	host           string
}

type WrapReqResp struct {
	ctx  *gin.Context
	done chan bool
}

func NewServer() *Server {
	var (
		server *Server
		err    error
	)
	server = &Server{
		statMap:        util.NewCommonMap(0),
		lockMap:        util.NewCommonMap(0),
		rtMap:          util.NewCommonMap(0),
		sceneMap:       util.NewCommonMap(0),
		searchMap:      util.NewCommonMap(0),
		queueToPeers:   make(chan FileInfo, config.QueueSize),
		queueFromPeers: make(chan FileInfo, config.QueueSize),
		queueFileLog:   make(chan *FileLog, config.QueueSize),
		queueUpload:    make(chan WrapReqResp, 100),
		sumMap:         util.NewCommonMap(365 * 3),
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
	server.statMap.Put(config.StatisticsFileCountKey, int64(0))
	server.statMap.Put(config.StatFileTotalSizeKey, int64(0))
	server.statMap.Put(util.GetToDay()+"_"+config.StatisticsFileCountKey, int64(0))
	server.statMap.Put(util.GetToDay()+"_"+config.StatFileTotalSizeKey, int64(0))
	server.curDate = util.GetToDay()
	opts := &opt.Options{
		CompactionTableSize: 1024 * 1024 * 20,
		WriteBuffer:         1024 * 1024 * 20,
	}
	server.ldb, err = leveldb.OpenFile(config.CONST_LEVELDB_FILE_NAME, opts)
	if err != nil {
		fmt.Println(fmt.Sprintf("open db file %s fail,maybe has opening", config.CONST_LEVELDB_FILE_NAME))
		log.Error(err)
		panic(err)
	}
	server.logDB, err = leveldb.OpenFile(config.CONST_LOG_LEVELDB_FILE_NAME, opts)
	if err != nil {
		fmt.Println(fmt.Sprintf("open db file %s fail,maybe has opening", config.CONST_LOG_LEVELDB_FILE_NAME))
		log.Error(err)
		panic(err)

	}
	return server
}

// Read: BackUpMetaDataByDate back up the file 'files.md5' and 'meta.data' in the directory name with 'date'
func (svr *Server) BackUpMetaDataByDate(date string) {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("BackUpMetaDataByDate")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		err          error
		keyPrefix    string
		msg          string
		name         string
		fileInfo     FileInfo
		logFileName  string
		fileLog      *os.File
		fileMeta     *os.File
		metaFileName string
		fi           os.FileInfo
	)
	logFileName = config.DATA_DIR + "/" + date + "/" + config.FileMd5Name
	svr.lockMap.LockKey(logFileName)
	defer svr.lockMap.UnLockKey(logFileName)
	metaFileName = config.DATA_DIR + "/" + date + "/" + "meta.data"
	os.MkdirAll(config.DATA_DIR+"/"+date, 0775)
	if util.Exist(logFileName) {
		os.Remove(logFileName)
	}
	if util.Exist(metaFileName) {
		os.Remove(metaFileName)
	}
	fileLog, err = os.OpenFile(logFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0664)
	if err != nil {
		log.Error(err)
		return
	}
	defer fileLog.Close()
	fileMeta, err = os.OpenFile(metaFileName, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0664)
	if err != nil {
		log.Error(err)
		return
	}
	defer fileMeta.Close()
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, config.FileMd5Name)
	iter := svr.logDB.NewIterator(levelDBUtil.BytesPrefix([]byte(keyPrefix)), nil)
	defer iter.Release()
	for iter.Next() {
		if err = json.Unmarshal(iter.Value(), &fileInfo); err != nil {
			continue
		}
		name = fileInfo.Name
		if fileInfo.ReName != "" {
			name = fileInfo.ReName
		}
		msg = fmt.Sprintf("%s\t%s\n", fileInfo.Md5, string(iter.Value()))
		if _, err = fileMeta.WriteString(msg); err != nil {
			log.Error(err)
		}
		msg = fmt.Sprintf("%s\t%s\n", util.MD5(fileInfo.Path+"/"+name), string(iter.Value()))
		if _, err = fileMeta.WriteString(msg); err != nil {
			log.Error(err)
		}
		msg = fmt.Sprintf("%s|%d|%d|%s\n", fileInfo.Md5, fileInfo.Size, fileInfo.TimeStamp, fileInfo.Path+"/"+name)
		if _, err = fileLog.WriteString(msg); err != nil {
			log.Error(err)
		}
	}
	if fi, err = fileLog.Stat(); err != nil {
		log.Error(err)
	} else if fi.Size() == 0 {
		fileLog.Close()
		os.Remove(logFileName)
	}
	if fi, err = fileMeta.Stat(); err != nil {
		log.Error(err)
	} else if fi.Size() == 0 {
		fileMeta.Close()
		os.Remove(metaFileName)
	}
}

// Read:
func (svr *Server) RepairFileInfoFromFile() {
	var (
		pathPrefix string
		err        error
		fi         os.FileInfo
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("RepairFileInfoFromFile")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	if svr.lockMap.IsLock("RepairFileInfoFromFile") {
		log.Warn("Lock RepairFileInfoFromFile")
		return
	}
	svr.lockMap.LockKey("RepairFileInfoFromFile")
	defer svr.lockMap.UnLockKey("RepairFileInfoFromFile")
	handlefunc := func(file_path string, f os.FileInfo, err error) error {
		var (
			files    []os.FileInfo
			fi       os.FileInfo
			fileInfo FileInfo
			sum      string
			pathMd5  string
		)
		if f.IsDir() {
			files, err = ioutil.ReadDir(file_path)

			if err != nil {
				return err
			}
			for _, fi = range files {
				if fi.IsDir() || fi.Size() == 0 {
					continue
				}
				file_path = strings.Replace(file_path, "\\", "/", -1)
				if config.DOCKER_DIR != "" {
					file_path = strings.Replace(file_path, config.DOCKER_DIR, "", 1)
				}
				if pathPrefix != "" {
					file_path = strings.Replace(file_path, pathPrefix, config.StoreDirName, 1)
				}
				if strings.HasPrefix(file_path, config.StoreDirName+"/"+config.LARGE_DIR_NAME) {
					log.Info(fmt.Sprintf("ignore small file file %s", file_path+"/"+fi.Name()))
					continue
				}
				pathMd5 = util.MD5(file_path + "/" + fi.Name())
				//if finfo, _ := svr.GetFileInfoFromLevelDB(pathMd5); finfo != nil && finfo.Md5 != "" {
				//	log.Info(fmt.Sprintf("exist ignore file %s", file_path+"/"+fi.Name()))
				//	continue
				//}
				//sum, err = util.GetFileSumByName(file_path+"/"+fi.Name(), config.CommonConfig.FileSumArithmetic)
				sum = pathMd5
				if err != nil {
					log.Error(err)
					continue
				}
				fileInfo = FileInfo{
					Size:      fi.Size(),
					Name:      fi.Name(),
					Path:      file_path,
					Md5:       sum,
					TimeStamp: fi.ModTime().Unix(),
					Peers:     []string{svr.host},
					OffSet:    -2,
				}
				//log.Info(fileInfo)
				log.Info(file_path, "/", fi.Name())
				svr.AppendToQueue(&fileInfo)
				//svr.postFileToPeer(&fileInfo)
				svr.SaveFileInfoToLevelDB(fileInfo.Md5, &fileInfo, svr.ldb)
				//svr.SaveFileMd5Log(&fileInfo, FileMd5Name)
			}
		}
		return nil
	}
	pathname := config.STORE_DIR
	pathPrefix, err = os.Readlink(pathname)
	if err == nil {
		//link
		pathname = pathPrefix
		if strings.HasSuffix(pathPrefix, "/") {
			//bugfix fullpath
			pathPrefix = pathPrefix[0 : len(pathPrefix)-1]
		}
	}
	fi, err = os.Stat(pathname)
	if err != nil {
		log.Error(err)
	}
	if fi.IsDir() {
		filepath.Walk(pathname, handlefunc)
	}
	log.Info("RepairFileInfoFromFile is finish.")
}

//
func (svr *Server) WatchFilesChange() {
	var (
		w *watcher.Watcher
		//fileInfo FileInfo
		curDir     string
		err        error
		qchan      chan *FileInfo
		isLink     bool
		configInfo = config.CommonConfig
	)
	qchan = make(chan *FileInfo, configInfo.WatchChanSize)
	w = watcher.New()
	w.FilterOps(watcher.Create)
	//w.FilterOps(watcher.Create, watcher.Remove)
	curDir, err = filepath.Abs(filepath.Dir(config.StoreDirName))
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
					fpath = strings.Replace(event.Path, curDir, config.StoreDirName, 1)
				}
				fpath = strings.Replace(fpath, string(os.PathSeparator), "/", -1)
				sum := util.MD5(fpath)
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
			if time.Now().Unix()-c.TimeStamp < configInfo.SyncDelay {
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
					svr.AppendToQueue(c)
					svr.SaveFileInfoToLevelDB(c.Md5, c, svr.ldb)
				}
			}
		}
	}()
	if dir, err := os.Readlink(config.StoreDirName); err == nil {

		if strings.HasSuffix(dir, string(os.PathSeparator)) {
			dir = strings.TrimSuffix(dir, string(os.PathSeparator))
		}
		curDir = dir
		isLink = true
		if err := w.AddRecursive(dir); err != nil {
			log.Error(err)
		}
		w.Ignore(dir + "/_tmp/")
		w.Ignore(dir + "/" + config.LARGE_DIR_NAME + "/")
	}
	if err := w.AddRecursive("./" + config.StoreDirName); err != nil {
		log.Error(err)
	}
	w.Ignore("./" + config.StoreDirName + "/_tmp/")
	w.Ignore("./" + config.StoreDirName + "/" + config.LARGE_DIR_NAME + "/")
	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Error(err)
	}
}

func (svr *Server) RepairStatByDate(date string) StatDateFileInfo {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("RepairStatByDate")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		err       error
		keyPrefix string
		fileInfo  FileInfo
		fileCount int64
		fileSize  int64
		stat      StatDateFileInfo
	)
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, config.FileMd5Name)
	iter := Svr.logDB.NewIterator(levelDBUtil.BytesPrefix([]byte(keyPrefix)), nil)
	defer iter.Release()
	for iter.Next() {
		if err = json.Unmarshal(iter.Value(), &fileInfo); err != nil {
			continue
		}
		fileCount = fileCount + 1
		fileSize = fileSize + fileInfo.Size
	}
	svr.statMap.Put(date+"_"+config.StatisticsFileCountKey, fileCount)
	svr.statMap.Put(date+"_"+config.StatFileTotalSizeKey, fileSize)
	svr.SaveStat()
	stat.Date = date
	stat.FileCount = fileCount
	stat.TotalSize = fileSize
	return stat
}

func (svr *Server) GetFilePathByInfo(fileInfo *FileInfo, withDocker bool) string {
	var (
		fn string
	)
	fn = fileInfo.Name
	if fileInfo.ReName != "" {
		fn = fileInfo.ReName
	}
	if withDocker {
		return config.DOCKER_DIR + fileInfo.Path + "/" + fn
	}
	return fileInfo.Path + "/" + fn
}

func (svr *Server) CheckFileExistByInfo(md5s string, fileInfo *FileInfo) bool {
	var (
		err      error
		fullpath string
		fi       os.FileInfo
		info     *FileInfo
	)
	if fileInfo == nil {
		return false
	}
	if fileInfo.OffSet >= 0 {
		//small file
		if info, err = svr.GetFileInfoFromLevelDB(fileInfo.Md5); err == nil && info.Md5 == fileInfo.Md5 {
			return true
		} else {
			return false
		}
	}
	fullpath = svr.GetFilePathByInfo(fileInfo, true)
	if fi, err = os.Stat(fullpath); err != nil {
		return false
	}
	if fi.Size() == fileInfo.Size {
		return true
	} else {
		return false
	}
}

func (svr *Server) ParseSmallFile(filename string) (string, int64, int, error) {
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
	if length > config.SmallFileSize || offset < 0 {
		err = errors.New("invalid filesize or offset")
		return filename, -1, -1, err
	}
	return pos[0], offset, length, nil
}
func (svr *Server) DownloadFromPeer(peer string, fileInfo *FileInfo) {
	var (
		err         error
		filename    string
		fpath       string
		fpathTmp    string
		fi          os.FileInfo
		sum         string
		data        []byte
		downloadUrl string
	)
	if config.CommonConfig.ReadOnly {
		log.Warn("ReadOnly", fileInfo)
		return
	}
	if config.CommonConfig.RetryCount > 0 && fileInfo.retry >= config.CommonConfig.RetryCount {
		log.Error("DownloadFromPeer Error ", fileInfo)
		return
	} else {
		fileInfo.retry = fileInfo.retry + 1
	}
	filename = fileInfo.Name
	if fileInfo.ReName != "" {
		filename = fileInfo.ReName
	}
	if fileInfo.OffSet != -2 && config.CommonConfig.EnableDistinctFile && svr.CheckFileExistByInfo(fileInfo.Md5, fileInfo) {
		// ignore migrate file
		log.Info(fmt.Sprintf("DownloadFromPeer file Exist, path:%s", fileInfo.Path+"/"+fileInfo.Name))
		return
	}
	if (!config.CommonConfig.EnableDistinctFile || fileInfo.OffSet == -2) && util.FileExists(svr.GetFilePathByInfo(fileInfo, true)) {
		// ignore migrate file
		if fi, err = os.Stat(svr.GetFilePathByInfo(fileInfo, true)); err == nil {
			if fi.ModTime().Unix() > fileInfo.TimeStamp {
				log.Info(fmt.Sprintf("ignore file sync path:%s", svr.GetFilePathByInfo(fileInfo, false)))
				fileInfo.TimeStamp = fi.ModTime().Unix()
				svr.postFileToPeer(fileInfo) // keep newer
				return
			}
			os.Remove(svr.GetFilePathByInfo(fileInfo, true))
		}
	}
	if _, err = os.Stat(fileInfo.Path); err != nil {
		os.MkdirAll(config.DOCKER_DIR+fileInfo.Path, 0775)
	}
	//fmt.Println("downloadFromPeer",fileInfo)
	p := strings.Replace(fileInfo.Path, config.StoreDirName+"/", "", 1)
	//filename=util.UrlEncode(filename)
	downloadUrl = peer + "/" + config.CommonConfig.Group + "/" + p + "/" + filename
	log.Info("DownloadFromPeer: ", downloadUrl)
	fpath = config.DOCKER_DIR + fileInfo.Path + "/" + filename
	fpathTmp = config.DOCKER_DIR + fileInfo.Path + "/" + fmt.Sprintf("%s_%s", "tmp_", filename)
	timeout := fileInfo.Size/1024/1024/1 + 30
	if config.CommonConfig.SyncTimeout > 0 {
		timeout = config.CommonConfig.SyncTimeout
	}
	svr.lockMap.LockKey(fpath)
	defer svr.lockMap.UnLockKey(fpath)
	download_key := fmt.Sprintf("downloading_%d_%s", time.Now().Unix(), fpath)
	svr.ldb.Put([]byte(download_key), []byte(""), nil)
	defer func() {
		svr.ldb.Delete([]byte(download_key), nil)
	}()
	if fileInfo.OffSet == -2 {
		//migrate file
		if fi, err = os.Stat(fpath); err == nil && fi.Size() == fileInfo.Size {
			//prevent double download
			svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, svr.ldb)
			//log.Info(fmt.Sprintf("file '%s' has download", fpath))
			return
		}
		req := httplib.Get(downloadUrl)
		req.SetTimeout(time.Second*30, time.Second*time.Duration(timeout))
		if err = req.ToFile(fpathTmp); err != nil {
			svr.AppendToDownloadQueue(fileInfo) //retry
			os.Remove(fpathTmp)
			log.Error(err, fpathTmp)
			return
		}
		if os.Rename(fpathTmp, fpath) == nil {
			//svr.SaveFileMd5Log(fileInfo, FileMd5Name)
			svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, svr.ldb)
		}
		return
	}
	req := httplib.Get(downloadUrl)
	req.SetTimeout(time.Second*30, time.Second*time.Duration(timeout))
	if fileInfo.OffSet >= 0 {
		//small file download
		data, err = req.Bytes()
		if err != nil {
			svr.AppendToDownloadQueue(fileInfo) //retry
			log.Error(err)
			return
		}
		data2 := make([]byte, len(data)+1)
		data2[0] = '1'
		for i, v := range data {
			data2[i+1] = v
		}
		data = data2
		if int64(len(data)) != fileInfo.Size {
			log.Warn("file size is error")
			return
		}
		fpath = strings.Split(fpath, ",")[0]
		err = util.WriteFileByOffSet(fpath, fileInfo.OffSet, data)
		if err != nil {
			log.Warn(err)
			return
		}
		svr.SaveFileMd5Log(fileInfo, config.FileMd5Name)
		return
	}
	if err = req.ToFile(fpathTmp); err != nil {
		svr.AppendToDownloadQueue(fileInfo) //retry
		os.Remove(fpathTmp)
		log.Error(err)
		return
	}
	if fi, err = os.Stat(fpathTmp); err != nil {
		os.Remove(fpathTmp)
		return
	}
	_ = sum
	//if config.CommonConfig.EnableDistinctFile {
	//	//DistinctFile
	//	if sum, err = util.GetFileSumByName(fpathTmp, config.CommonConfig.FileSumArithmetic); err != nil {
	//		log.Error(err)
	//		return
	//	}
	//} else {
	//	//DistinctFile By path
	//	sum = util.MD5(svr.GetFilePathByInfo(fileInfo, false))
	//}
	if fi.Size() != fileInfo.Size { //  maybe has bug remove || sum != fileInfo.Md5
		log.Error("file sum check error")
		os.Remove(fpathTmp)
		return
	}
	if os.Rename(fpathTmp, fpath) == nil {
		svr.SaveFileMd5Log(fileInfo, config.FileMd5Name)
	}
}

func (svr *Server) CheckDownloadAuth(ctx *gin.Context) (bool, error) {
	var (
		err          error
		maxTimestamp int64
		minTimestamp int64
		ts           int64
		token        string
		timestamp    string
		fullpath     string
		smallPath    string
		pathMd5      string
		fileInfo     *FileInfo
		scene        string
		secret       interface{}
		code         string
		ok           bool
	)
	r := ctx.Request
	w := ctx.Writer
	CheckToken := func(token string, md5sum string, timestamp string) bool {
		if util.MD5(md5sum+timestamp) != token {
			return false
		}
		return true
	}
	if config.CommonConfig.EnableDownloadAuth && config.CommonConfig.AuthUrl != "" && !IsPeer(r) && !CheckAuth(w, r) {
		return false, errors.New("auth fail")
	}
	if config.CommonConfig.DownloadUseToken && !IsPeer(r) {
		token = r.FormValue("token")
		timestamp = r.FormValue("timestamp")
		if token == "" || timestamp == "" {
			return false, errors.New("unvalid request")
		}
		maxTimestamp = time.Now().Add(time.Second *
			time.Duration(config.CommonConfig.DownloadTokenExpire)).Unix()
		minTimestamp = time.Now().Add(-time.Second *
			time.Duration(config.CommonConfig.DownloadTokenExpire)).Unix()
		if ts, err = strconv.ParseInt(timestamp, 10, 64); err != nil {
			return false, errors.New("unvalid timestamp")
		}
		if ts > maxTimestamp || ts < minTimestamp {
			return false, errors.New("timestamp expire")
		}
		fullpath, smallPath = GetFilePathFromRequest(ctx)
		if smallPath != "" {
			pathMd5 = util.MD5(smallPath)
		} else {
			pathMd5 = util.MD5(fullpath)
		}
		if fileInfo, err = svr.GetFileInfoFromLevelDB(pathMd5); err != nil {
			// TODO
		} else {
			ok := CheckToken(token, fileInfo.Md5, timestamp)
			if !ok {
				return ok, errors.New("unvalid token")
			}
			return ok, nil
		}
	}
	if config.CommonConfig.EnableGoogleAuth && !IsPeer(r) {
		fullpath = r.RequestURI[len(config.CommonConfig.Group)+2 : len(r.RequestURI)]
		fullpath = strings.Split(fullpath, "?")[0] // just path
		scene = strings.Split(fullpath, "/")[0]
		code = r.FormValue("code")
		if secret, ok = svr.sceneMap.GetValue(scene); ok {
			if !svr.VerifyGoogleCode(secret.(string), code, int64(config.CommonConfig.DownloadTokenExpire/30)) {
				return false, errors.New("invalid google code")
			}
		}
	}
	return true, nil
}

func (svr *Server) GetSmallFileByURI(ctx *gin.Context) ([]byte, bool, error) {
	var (
		err      error
		data     []byte
		offset   int64
		length   int
		fullpath string
		info     os.FileInfo
	)
	r := ctx.Request
	fullpath, _ = GetFilePathFromRequest(ctx)
	if _, offset, length, err = svr.ParseSmallFile(r.RequestURI); err != nil {
		return nil, false, err
	}
	if info, err = os.Stat(fullpath); err != nil {
		return nil, false, err
	}
	if info.Size() < offset+int64(length) {
		return nil, true, errors.New("noFound")
	} else {
		data, err = util.ReadFileByOffSet(fullpath, offset, length)
		if err != nil {
			return nil, false, err
		}
		return data, false, err
	}
}

func (svr *Server) DownloadSmallFileByURI(ctx *gin.Context) (bool, error) {
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
		isDownload = config.CommonConfig.DefaultDownload
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
	data, notFound, err = svr.GetSmallFileByURI(ctx)
	_ = notFound
	if data != nil && string(data[0]) == "1" {
		if isDownload {
			util.SetDownloadHeader(w, r)
		}
		if imgWidth != 0 || imgHeight != 0 {
			svr.ResizeImageByBytes(w, data[1:], uint(imgWidth), uint(imgHeight))
			return true, nil
		}
		w.Write(data[1:])
		return true, nil
	}
	return false, errors.New("not found")
}

func (svr *Server) DownloadNormalFileByURI(ctx *gin.Context) (bool, error) {
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
		isDownload = config.CommonConfig.DefaultDownload
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
		util.SetDownloadHeader(w, r)
	}
	fullpath, _ := GetFilePathFromRequest(ctx)
	if imgWidth != 0 || imgHeight != 0 {
		svr.ResizeImage(w, fullpath, uint(imgWidth), uint(imgHeight))
		return true, nil
	}
	StaticHandler.ServeHTTP(w, r)
	return true, nil
}

func (svr *Server) DownloadNotFound(ctx *gin.Context) {
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
	fullpath, smallPath = GetFilePathFromRequest(ctx)
	isDownload = true
	if r.FormValue("download") == "" {
		isDownload = config.CommonConfig.DefaultDownload
	}
	if r.FormValue("download") == "0" {
		isDownload = false
	}
	if smallPath != "" {
		pathMd5 = util.MD5(smallPath)
	} else {
		pathMd5 = util.MD5(fullpath)
	}
	for _, peer = range config.CommonConfig.Peers {
		if fileInfo, err = svr.checkPeerFileExist(peer, pathMd5, fullpath); err != nil {
			log.Error(err)
			continue
		}
		if fileInfo.Md5 != "" {
			go svr.DownloadFromPeer(peer, fileInfo)
			//http.Redirect(w, r, peer+r.RequestURI, 302)
			if isDownload {
				util.SetDownloadHeader(w, r)
			}
			svr.DownloadFileToResponse(peer+r.RequestURI, ctx)
			return
		}
	}
	w.WriteHeader(404)
	return
}

//
func (svr *Server) Download(ctx *gin.Context) {
	var fileInfo os.FileInfo

	reqURI := ctx.Request.RequestURI
	// if params is not enough then redirect to upload
	if util.CheckUploadURIInvalid(reqURI, config.CommonConfig.Group) {
		ctx.Redirect(http.StatusOK, "/index")
		return
	}
	if ok, err := svr.CheckDownloadAuth(ctx); !ok {
		log.Error(err)
		ctx.JSON(http.StatusUnauthorized, "not Permitted")
		return
	}

	if config.CommonConfig.EnableCrossOrigin {
		util.CrossOrigin(ctx)
	}
	fullPath, smallPath := GetFilePathFromRequest(ctx)
	if smallPath == "" {
		if _, err := os.Stat(fullPath); err != nil {
			svr.DownloadNotFound(ctx)
			return
		}
		if !config.CommonConfig.ShowDir && fileInfo.IsDir() {
			ctx.JSON(http.StatusNotFound, "list dir deny")
			return
		}
		//staticHandler.ServeHTTP(w, r)
		svr.DownloadNormalFileByURI(ctx)
		return
	}

	if ok, _ := svr.DownloadSmallFileByURI(ctx); !ok {
		svr.DownloadNotFound(ctx)
	}
}

func (svr *Server) DownloadFileToResponse(url string, ctx *gin.Context) {
	var (
		err  error
		req  *httplib.BeegoHTTPRequest
		resp *http.Response
	)
	req = httplib.Get(url)
	req.SetTimeout(time.Second*20, time.Second*600)
	resp, err = req.DoRequest()
	if err != nil {
		log.Error(err)
	}
	defer resp.Body.Close()
	_, err = io.Copy(ctx.Writer, resp.Body)
	if err != nil {
		log.Error(err)
	}
}

func (svr *Server) ResizeImageByBytes(w http.ResponseWriter, data []byte, width, height uint) {
	var (
		img     image.Image
		err     error
		imgType string
	)
	reader := bytes.NewReader(data)
	img, imgType, err = image.Decode(reader)
	if err != nil {
		log.Error(err)
		return
	}

	img = resize.Resize(width, height, img, resize.Lanczos3)
	switch imgType {
	case "jpg", "jpeg":
		jpeg.Encode(w, img, nil)
	case "png":
		png.Encode(w, img)
	default:
		w.Write(data)
	}
}

func (svr *Server) ResizeImage(w http.ResponseWriter, fullPath string, width, height uint) {
	file, err := os.Open(fullPath)
	if err != nil {
		log.Error(err)
		return
	}
	img, imgType, err := image.Decode(file)
	if err != nil {
		log.Error(err)
		return
	}
	file.Close()

	img = resize.Resize(width, height, img, resize.Lanczos3)
	switch imgType {
	case "jpg", "jpeg":
		jpeg.Encode(w, img, nil)
	case "png":
		png.Encode(w, img)
	default:
		file.Seek(0, 0)
		io.Copy(w, file)
	}
}

func (svr *Server) GetServerURI(r *http.Request) string {
	return fmt.Sprintf("http://%s/", r.Host)
}

func (svr *Server) CheckFileAndSendToPeer(date string, filename string, isForceUpload bool) {
	var (
		md5set mapset.Set
		err    error
		md5s   []interface{}
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("CheckFileAndSendToPeer")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	if md5set, err = svr.GetMd5sByDate(date, filename); err != nil {
		log.Error(err)
		return
	}
	md5s = md5set.ToSlice()
	for _, md := range md5s {
		if md == nil {
			continue
		}
		if fileInfo, _ := svr.GetFileInfoFromLevelDB(md.(string)); fileInfo != nil && fileInfo.Md5 != "" {
			if isForceUpload {
				fileInfo.Peers = []string{}
			}
			if len(fileInfo.Peers) > len(config.CommonConfig.Peers) {
				continue
			}
			if !util.Contains(svr.host, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, svr.host) // peer is null
			}
			if filename == config.Md5QueueFileName {
				svr.AppendToDownloadQueue(fileInfo)
				continue
			}
			svr.AppendToQueue(fileInfo)
		}
	}
}

func (svr *Server) postFileToPeer(fileInfo *FileInfo) {
	var (
		err      error
		peer     string
		filename string
		info     *FileInfo
		postURL  string
		result   string
		fi       os.FileInfo
		i        int
		data     []byte
		fpath    string
	)
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("postFileToPeer")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	//fmt.Println("postFile",fileInfo)
	for i, peer = range config.CommonConfig.Peers {
		_ = i
		if fileInfo.Peers == nil {
			fileInfo.Peers = []string{}
		}
		if util.Contains(peer, fileInfo.Peers) {
			continue
		}
		filename = fileInfo.Name
		if fileInfo.ReName != "" {
			filename = fileInfo.ReName
			if fileInfo.OffSet != -1 {
				filename = strings.Split(fileInfo.ReName, ",")[0]
			}
		}
		fpath = config.DOCKER_DIR + fileInfo.Path + "/" + filename
		if !util.FileExists(fpath) {
			log.Warn(fmt.Sprintf("file '%s' not found", fpath))
			continue
		}
		if fileInfo.Size == 0 {
			if fi, err = os.Stat(fpath); err != nil {
				log.Error(err)
			} else {
				fileInfo.Size = fi.Size()
			}
		}

		if fileInfo.OffSet != -2 && config.CommonConfig.EnableDistinctFile {
			//not migrate file should check or update file
			// where not EnableDistinctFile should check
			if info, err = svr.checkPeerFileExist(peer, fileInfo.Md5, ""); info.Md5 != "" {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				if _, err = svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, svr.ldb); err != nil {
					log.Error(err)
				}
				continue
			}
		}
		postURL = fmt.Sprintf("%s%s", peer, svr.getRequestURI("syncfile_info"))
		b := httplib.Post(postURL)
		b.SetTimeout(time.Second*30, time.Second*30)
		if data, err = json.Marshal(fileInfo); err != nil {
			log.Error(err)
			return
		}
		b.Param("fileInfo", string(data))
		result, err = b.String()
		if err != nil {
			if fileInfo.retry <= config.CommonConfig.RetryCount {
				fileInfo.retry = fileInfo.retry + 1
				svr.AppendToQueue(fileInfo)
			}
			log.Error(err, fmt.Sprintf(" path:%s", fileInfo.Path+"/"+fileInfo.Name))
		}
		if !strings.HasPrefix(result, "http://") || err != nil {
			log.Error(err)
			svr.SaveFileMd5Log(fileInfo, config.Md5ErrorFileName)
		}
		if strings.HasPrefix(result, "http://") {
			log.Info(result)
			if !util.Contains(peer, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				if _, err = svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, svr.ldb); err != nil {
					log.Error(err)
				}
			}
		}
	}
}

func (svr *Server) SaveFileMd5Log(fileInfo *FileInfo, filename string) {
	var (
		info FileInfo
	)
	for len(svr.queueFileLog)+len(svr.queueFileLog)/10 > config.QueueSize {
		time.Sleep(time.Second * 1)
	}
	info = *fileInfo
	svr.queueFileLog <- &FileLog{FileInfo: &info, FileName: filename}
}

func (svr *Server) saveFileMd5Log(fileInfo *FileInfo, filename string) {
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
	logDate = util.GetDayFromTimeStamp(fileInfo.TimeStamp)
	outname = fileInfo.Name
	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}
	fullpath = fileInfo.Path + "/" + outname
	logKey = fmt.Sprintf("%s_%s_%s", logDate, filename, fileInfo.Md5)
	if filename == config.FileMd5Name {
		//svr.searchMap.Put(fileInfo.Md5, fileInfo.Name)
		if ok, err = svr.IsExistFromLevelDB(fileInfo.Md5, svr.ldb); !ok {
			svr.statMap.AddCountInt64(logDate+"_"+config.StatisticsFileCountKey, 1)
			svr.statMap.AddCountInt64(logDate+"_"+config.StatFileTotalSizeKey, fileInfo.Size)
			svr.SaveStat()
		}
		if _, err = svr.SaveFileInfoToLevelDB(logKey, fileInfo, svr.logDB); err != nil {
			log.Error(err)
		}
		if _, err := svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, svr.ldb); err != nil {
			log.Error("saveToLevelDB", err, fileInfo)
		}
		if _, err = svr.SaveFileInfoToLevelDB(util.MD5(fullpath), fileInfo, svr.ldb); err != nil {
			log.Error("saveToLevelDB", err, fileInfo)
		}
		return
	}
	if filename == config.RemoveMd5FileName {
		//svr.searchMap.Remove(fileInfo.Md5)
		if ok, err = svr.IsExistFromLevelDB(fileInfo.Md5, svr.ldb); ok {
			svr.statMap.AddCountInt64(logDate+"_"+config.StatisticsFileCountKey, -1)
			svr.statMap.AddCountInt64(logDate+"_"+config.StatFileTotalSizeKey, -fileInfo.Size)
			svr.SaveStat()
		}
		svr.RemoveKeyFromLevelDB(logKey, svr.logDB)
		md5Path = util.MD5(fullpath)
		if err := svr.RemoveKeyFromLevelDB(fileInfo.Md5, svr.ldb); err != nil {
			log.Error("RemoveKeyFromLevelDB", err, fileInfo)
		}
		if err = svr.RemoveKeyFromLevelDB(md5Path, svr.ldb); err != nil {
			log.Error("RemoveKeyFromLevelDB", err, fileInfo)
		}
		// remove files.md5 for stat info(repair from logDB)
		logKey = fmt.Sprintf("%s_%s_%s", logDate, config.FileMd5Name, fileInfo.Md5)
		svr.RemoveKeyFromLevelDB(logKey, svr.logDB)
		return
	}
	svr.SaveFileInfoToLevelDB(logKey, fileInfo, svr.logDB)
}

func (svr *Server) checkPeerFileExist(peer string, md5sum string, fpath string) (*FileInfo, error) {
	var (
		err      error
		fileInfo FileInfo
	)
	req := httplib.Post(fmt.Sprintf("%s%s?md5=%s", peer, svr.getRequestURI("check_file_exist"), md5sum))
	req.Param("path", fpath)
	req.Param("md5", md5sum)
	req.SetTimeout(time.Second*5, time.Second*10)
	if err = req.ToJSON(&fileInfo); err != nil {
		return &FileInfo{}, err
	}
	if fileInfo.Md5 == "" {
		return &fileInfo, errors.New("not found")
	}
	return &fileInfo, nil
}

func (svr *Server) CheckFileExist(ctx *gin.Context) {
	var (
		err      error
		fileInfo *FileInfo
		fpath    string
		fi       os.FileInfo
	)
	r := ctx.Request
	r.ParseForm()
	md5sum := ""
	md5sum = r.FormValue("md5")
	fpath = r.FormValue("path")
	if fileInfo, err = svr.GetFileInfoFromLevelDB(md5sum); fileInfo != nil {
		if fileInfo.OffSet != -1 {
			ctx.JSON(http.StatusOK, fileInfo)
			return
		}
		fpath = config.DOCKER_DIR + fileInfo.Path + "/" + fileInfo.Name
		if fileInfo.ReName != "" {
			fpath = config.DOCKER_DIR + fileInfo.Path + "/" + fileInfo.ReName
		}
		if util.Exist(fpath) {
			ctx.JSON(http.StatusOK, fileInfo)
			return
		}
		if fileInfo.OffSet == -1 {
			svr.RemoveKeyFromLevelDB(md5sum, svr.ldb) // when file delete,delete from leveldb
		}

		ctx.JSON(http.StatusNotFound, FileInfo{})
	}

	if fpath != "" {
		fi, err = os.Stat(fpath)
		if err == nil {
			sum := util.MD5(fpath)
			//if config.CommonConfig.EnableDistinctFile {
			//	sum, err = util.GetFileSumByName(fpath, config.CommonConfig.FileSumArithmetic)
			//	if err != nil {
			//		log.Error(err)
			//	}
			//}
			fileInfo = &FileInfo{
				Path:      path.Dir(fpath),
				Name:      path.Base(fpath),
				Size:      fi.Size(),
				Md5:       sum,
				Peers:     []string{config.CommonConfig.Host},
				OffSet:    -1, //very important
				TimeStamp: fi.ModTime().Unix(),
			}
			ctx.JSON(http.StatusOK, fileInfo)
			return
		}
	}

	ctx.JSON(http.StatusNotFound, FileInfo{})
}

func (svr *Server) CheckFilesExist(ctx *gin.Context) {
	var (
		fileInfos []*FileInfo
		filePath  string
		result    JsonResult
	)
	r := ctx.Request
	r.ParseForm()
	md5sum := r.FormValue("md5s")
	md5s := strings.Split(md5sum, ",")
	for _, m := range md5s {
		if fileInfo, _ := svr.GetFileInfoFromLevelDB(m); fileInfo != nil {
			if fileInfo.OffSet != -1 {
				fileInfos = append(fileInfos, fileInfo)
				continue
			}
			filePath = config.DOCKER_DIR + fileInfo.Path + "/" + fileInfo.Name
			if fileInfo.ReName != "" {
				filePath = config.DOCKER_DIR + fileInfo.Path + "/" + fileInfo.ReName
			}
			if util.Exist(filePath) {
				fileInfos = append(fileInfos, fileInfo)
				continue
			} else {
				if fileInfo.OffSet == -1 {
					svr.RemoveKeyFromLevelDB(md5sum, svr.ldb) // when file delete,delete from leveldb
				}
			}
		}
	}

	result.Data = fileInfos
	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) Sync(ctx *gin.Context) {
	var (
		result JsonResult
	)
	r := ctx.Request
	r.ParseForm()
	result.Status = "fail"
	if !IsPeer(r) {
		result.Message = "client must be in cluster"
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	date := ""
	force := ""
	inner := ""
	isForceUpload := false
	force = r.FormValue("force")
	date = r.FormValue("date")
	inner = r.FormValue("inner")
	if force == "1" {
		isForceUpload = true
	}
	if inner != "1" {
		for _, peer := range config.CommonConfig.Peers {
			req := httplib.Post(peer + svr.getRequestURI("sync"))
			req.Param("force", force)
			req.Param("inner", "1")
			req.Param("date", date)
			if _, err := req.String(); err != nil {
				log.Error(err)
			}
		}
	}
	if date == "" {
		result.Message = "require paramete date &force , ?date=20181230"
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	date = strings.Replace(date, ".", "", -1)
	if isForceUpload {
		go svr.CheckFileAndSendToPeer(date, config.FileMd5Name, isForceUpload)
	} else {
		go svr.CheckFileAndSendToPeer(date, config.Md5ErrorFileName, isForceUpload)
	}
	result.Status = "ok"
	result.Message = "job is running"
	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) IsExistFromLevelDB(key string, db *leveldb.DB) (bool, error) {
	return db.Has([]byte(key), nil)
}

func (svr *Server) GetFileInfoFromLevelDB(key string) (*FileInfo, error) {
	var (
		err      error
		data     []byte
		fileInfo FileInfo
	)
	if data, err = svr.ldb.Get([]byte(key), nil); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &fileInfo); err != nil {
		return nil, err
	}
	return &fileInfo, nil
}

// Read: SaveStat read data from statMap(which is concurrent safe map), check if the
// "StatisticsFileCountKey" key exists, if exists, then load all statMap data to file "stat.json"
func (svr *Server) SaveStat() {
	SaveStatFunc := func() {
		defer func() {
			if re := recover(); re != nil {
				buffer := debug.Stack()
				log.Error("SaveStatFunc")
				log.Error(re)
				log.Error(string(buffer))
			}
		}()
		stat := svr.statMap.Get()
		if v, ok := stat[config.StatisticsFileCountKey]; ok {
			switch v.(type) {
			case int64, int32, int, float64, float32:
				if v.(int64) >= 0 {
					if data, err := json.Marshal(stat); err != nil {
						log.Error(err)
					} else {
						util.WriteBinFile(config.CONST_STAT_FILE_NAME, data)
					}
				}
			}
		}
	}
	SaveStatFunc()
}

//
func (svr *Server) RemoveKeyFromLevelDB(key string, db *leveldb.DB) error {
	var (
		err error
	)
	err = db.Delete([]byte(key), nil)
	return err
}

func (svr *Server) SaveFileInfoToLevelDB(key string, fileInfo *FileInfo, db *leveldb.DB) (*FileInfo, error) {
	var (
		err  error
		data []byte
	)
	if fileInfo == nil || db == nil {
		return nil, errors.New("fileInfo is null or db is null")
	}
	if data, err = json.Marshal(fileInfo); err != nil {
		return fileInfo, err
	}
	if err = db.Put([]byte(key), data, nil); err != nil {
		return fileInfo, err
	}
	if db == svr.ldb { //search slow ,write fast, double write logDB
		logDate := util.GetDayFromTimeStamp(fileInfo.TimeStamp)
		logKey := fmt.Sprintf("%s_%s_%s", logDate, config.FileMd5Name, fileInfo.Md5)
		svr.logDB.Put([]byte(logKey), data, nil)
	}
	return fileInfo, nil
}

// Read: ReceiveMd5s get md5s from request, and append every one that exist in levelDB to queue channel
func (svr *Server) ReceiveMd5s(ctx *gin.Context) {
	var (
		err      error
		md5str   string
		fileInfo *FileInfo
		md5s     []string
	)
	r := ctx.Request
	if !IsPeer(r) {
		log.Warn(fmt.Sprintf("ReceiveMd5s %s", util.GetClientIp(r)))
		ctx.JSON(http.StatusNotFound, svr.GetClusterNotPermitMessage(r))
		return
	}
	r.ParseForm()
	md5str = r.FormValue("md5s")
	md5s = strings.Split(md5str, ",")
	AppendFunc := func(md5s []string) {
		for _, m := range md5s {
			if m != "" {
				if fileInfo, err = svr.GetFileInfoFromLevelDB(m); err != nil {
					log.Error(err)
					continue
				}
				svr.AppendToQueue(fileInfo)
			}
		}
	}
	go AppendFunc(md5s)
}

//
func (svr *Server) GetClusterNotPermitMessage(r *http.Request) string {
	var (
		message string
	)
	message = fmt.Sprintf(config.MessageClusterIp, util.GetClientIp(r))
	return message
}

func (svr *Server) GetMd5sForWeb(ctx *gin.Context) {
	var (
		date   string
		err    error
		result mapset.Set
		lines  []string
		md5s   []interface{}
	)

	r := ctx.Request
	if !IsPeer(r) {
		ctx.JSON(http.StatusNotFound, svr.GetClusterNotPermitMessage(r))
		return
	}
	date = r.FormValue("date")
	if result, err = svr.GetMd5sByDate(date, config.FileMd5Name); err != nil {
		log.Error(err)
		ctx.JSON(http.StatusNotFound, err.Error())
		return
	}
	md5s = result.ToSlice()
	for _, line := range md5s {
		if line != nil && line != "" {
			lines = append(lines, line.(string))
		}
	}

	ctx.JSON(http.StatusOK, strings.Join(lines, ","))
}

//Read: GetMd5File download file 'data/files.md5'?
func (svr *Server) GetMd5File(ctx *gin.Context) {
	var date string
	r := ctx.Request

	if !IsPeer(r) {
		return
	}
	filePath := config.DATA_DIR + "/" + date + "/" + config.FileMd5Name
	if !util.FileExists(filePath) {
		ctx.JSON(http.StatusNotFound, filePath+"does not exist")
		return
	}

	ctx.File(filePath)
}

// Read: GetMd5sMapByDate use given date and file name to get md5 which will uer to create a commonMap
func (svr *Server) GetMd5sMapByDate(date string, filename string) (*util.CommonMap, error) {
	var (
		err     error
		result  *util.CommonMap
		fpath   string
		content string
		lines   []string
		line    string
		cols    []string
		data    []byte
	)
	result = util.NewCommonMap(0)
	if filename == "" {
		fpath = config.DATA_DIR + "/" + date + "/" + config.FileMd5Name
	} else {
		fpath = config.DATA_DIR + "/" + date + "/" + filename
	}
	if !util.FileExists(fpath) {
		return result, fmt.Errorf("fpath %s not found", fpath)
	}
	if data, err = ioutil.ReadFile(fpath); err != nil {
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
func (svr *Server) GetMd5sByDate(date string, filename string) (mapset.Set, error) {
	var (
		keyPrefix string
		md5set    mapset.Set
		keys      []string
	)
	md5set = mapset.NewSet()
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, filename)
	iter := Svr.logDB.NewIterator(levelDBUtil.BytesPrefix([]byte(keyPrefix)), nil)
	for iter.Next() {
		keys = strings.Split(string(iter.Key()), "_")
		if len(keys) >= 3 {
			md5set.Add(keys[2])
		}
	}
	iter.Release()
	return md5set, nil
}

func (svr *Server) SyncFileInfo(ctx *gin.Context) {
	var (
		err         error
		fileInfo    FileInfo
		fileInfoStr string
		filename    string
	)
	r := ctx.Request
	r.ParseForm()
	if !IsPeer(r) {
		return
	}
	fileInfoStr = r.FormValue("fileInfo")
	if err = json.Unmarshal([]byte(fileInfoStr), &fileInfo); err != nil {
		ctx.JSON(http.StatusNotFound, svr.GetClusterNotPermitMessage(r))
		log.Error(err)
		return
	}
	if fileInfo.OffSet == -2 {
		// optimize migrate
		svr.SaveFileInfoToLevelDB(fileInfo.Md5, &fileInfo, svr.ldb)
	} else {
		svr.SaveFileMd5Log(&fileInfo, config.Md5QueueFileName)
	}
	svr.AppendToDownloadQueue(&fileInfo)
	filename = fileInfo.Name
	if fileInfo.ReName != "" {
		filename = fileInfo.ReName
	}
	p := strings.Replace(fileInfo.Path, config.STORE_DIR+"/", "", 1)
	downloadUrl := fmt.Sprintf("http://%s/%s", r.Host, config.CommonConfig.Group+"/"+p+"/"+filename)
	log.Info("SyncFileInfo: ", downloadUrl)

	ctx.JSON(http.StatusOK, downloadUrl)
}

func (svr *Server) CheckScene(scene string) (bool, error) {
	var scenes []string

	if len(config.CommonConfig.Scenes) == 0 {
		return true, nil
	}
	for _, s := range config.CommonConfig.Scenes {
		scenes = append(scenes, strings.Split(s, ":")[0])
	}
	if !util.Contains(scene, scenes) {
		return false, errors.New("not valid scene")
	}
	return true, nil
}

func (svr *Server) GetFileInfo(ctx *gin.Context) {
	var (
		filePath string
		md5sum   string
		fileInfo *FileInfo
		err      error
		result   JsonResult
	)
	r := ctx.Request
	md5sum = r.FormValue("md5")
	filePath = r.FormValue("path")
	result.Status = "fail"
	if !IsPeer(r) {
		ctx.JSON(http.StatusNotFound, svr.GetClusterNotPermitMessage(r))
		return
	}
	md5sum = r.FormValue("md5")
	if filePath != "" {
		filePath = strings.Replace(filePath, "/"+config.CommonConfig.Group+"/", config.StoreDirName+"/", 1)
		md5sum = util.MD5(filePath)
	}
	if fileInfo, err = svr.GetFileInfoFromLevelDB(md5sum); err != nil {
		log.Error(err)
		result.Message = err.Error()
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	result.Status = "ok"
	result.Data = fileInfo

	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) RemoveFile(ctx *gin.Context) {
	var (
		err      error
		md5sum   string
		fileInfo *FileInfo
		fpath    string
		delUrl   string
		result   JsonResult
		inner    string
		name     string
	)
	r := ctx.Request
	w := ctx.Writer
	r.ParseForm()
	md5sum = r.FormValue("md5")
	fpath = r.FormValue("path")
	inner = r.FormValue("inner")
	result.Status = "fail"
	if !IsPeer(r) {
		ctx.JSON(http.StatusUnauthorized, svr.GetClusterNotPermitMessage(r))
		return
	}
	if config.CommonConfig.AuthUrl != "" && !CheckAuth(w, r) {
		ctx.JSON(http.StatusUnauthorized, "Unauthorized")
		return
	}
	if fpath != "" && md5sum == "" {
		fpath = strings.Replace(fpath, "/"+config.CommonConfig.Group+"/", config.StoreDirName+"/", 1)
		md5sum = util.MD5(fpath)
	}
	if inner != "1" {
		for _, peer := range config.CommonConfig.Peers {
			delFile := func(peer string, md5sum string, fileInfo *FileInfo) {
				delUrl = fmt.Sprintf("%s%s", peer, svr.getRequestURI("delete"))
				req := httplib.Post(delUrl)
				req.Param("md5", md5sum)
				req.Param("inner", "1")
				req.SetTimeout(time.Second*5, time.Second*10)
				if _, err = req.String(); err != nil {
					log.Error(err)
				}
			}
			go delFile(peer, md5sum, fileInfo)
		}
	}
	if len(md5sum) < 32 {
		result.Message = "md5 unvalid"
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	if fileInfo, err = svr.GetFileInfoFromLevelDB(md5sum); err != nil {
		result.Message = err.Error()
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	if fileInfo.OffSet >= 0 {
		result.Message = "small file delete not support"
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	name = fileInfo.Name
	if fileInfo.ReName != "" {
		name = fileInfo.ReName
	}
	fpath = fileInfo.Path + "/" + name
	if fileInfo.Path != "" && util.FileExists(config.DOCKER_DIR+fpath) {
		svr.SaveFileMd5Log(fileInfo, config.RemoveMd5FileName)
		if err = os.Remove(config.DOCKER_DIR + fpath); err != nil {
			result.Message = err.Error()
			ctx.JSON(http.StatusNotFound, result)
			return
		} else {
			result.Message = "remove success"
			result.Status = "ok"
			ctx.JSON(http.StatusOK, result)
			return
		}
	}
	result.Message = "fail remove"
	ctx.JSON(http.StatusNotFound, result)
}

func (svr *Server) getRequestURI(action string) string {
	if config.CommonConfig.SupportGroupManage {
		return "/" + config.CommonConfig.Group + "/" + action
	}

	return "/" + action
}

func BuildFileResult(fileInfo *FileInfo, r *http.Request) FileResult {
	var (
		outname     string
		fileResult  FileResult
		p           string
		downloadUrl string
		host        string
	)
	host = strings.Replace(config.CommonConfig.Host, "http://", "", -1)
	if r != nil {
		host = r.Host
	}
	if !strings.HasPrefix(config.CommonConfig.DownloadDomain, "http") {
		if config.CommonConfig.DownloadDomain == "" {
			config.CommonConfig.DownloadDomain = fmt.Sprintf("http://%s", host)
		} else {
			config.CommonConfig.DownloadDomain = fmt.Sprintf("http://%s", config.CommonConfig.DownloadDomain)
		}
	}

	domain := config.CommonConfig.DownloadDomain
	if domain == "" {
		domain = fmt.Sprintf("http://%s", host)
	}

	outname = fileInfo.Name
	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}
	p = strings.Replace(fileInfo.Path, config.StoreDirName+"/", "", 1)
	p = config.FileDownloadPathPrefix + p + "/" + outname

	downloadUrl = fmt.Sprintf("http://%s/%s", host, p)
	if config.CommonConfig.DownloadDomain != "" {
		downloadUrl = fmt.Sprintf("%s/%s", config.CommonConfig.DownloadDomain, p)
	}
	fileResult.Url = downloadUrl
	fileResult.Md5 = fileInfo.Md5
	fileResult.Path = "/" + p
	fileResult.Domain = domain
	fileResult.Scene = fileInfo.Scene
	fileResult.Size = fileInfo.Size
	fileResult.ModTime = fileInfo.TimeStamp
	// Just for Compatibility
	fileResult.Src = fileResult.Path
	fileResult.Scenes = fileInfo.Scene
	return fileResult
}

func (svr *Server) SaveUploadFile(file multipart.File, header *multipart.FileHeader, fileInfo *FileInfo, r *http.Request) (*FileInfo, error) {
	var (
		err     error
		outFile *os.File
		folder  string
		fi      os.FileInfo
	)
	defer file.Close()
	_, fileInfo.Name = filepath.Split(header.Filename)
	// bugfix for ie upload file contain fullpath
	if len(config.CommonConfig.Extensions) > 0 && !util.Contains(path.Ext(fileInfo.Name), config.CommonConfig.Extensions) {
		return fileInfo, errors.New("(error)file extension mismatch")
	}

	if config.CommonConfig.RenameFile {
		fileInfo.ReName = util.MD5(util.GetUUID()) + path.Ext(fileInfo.Name)
	}
	folder = time.Now().Format("20060102/15/04")
	if config.CommonConfig.PeerId != "" {
		folder = fmt.Sprintf(folder+"/%s", config.CommonConfig.PeerId)
	}
	if fileInfo.Scene != "" {
		folder = fmt.Sprintf(config.STORE_DIR+"/%s/%s", fileInfo.Scene, folder)
	} else {
		folder = fmt.Sprintf(config.STORE_DIR+"/%s", folder)
	}
	if fileInfo.Path != "" {
		if strings.HasPrefix(fileInfo.Path, config.STORE_DIR) {
			folder = fileInfo.Path
		} else {
			folder = config.STORE_DIR + "/" + fileInfo.Path
		}
	}
	if !util.FileExists(folder) {
		os.MkdirAll(folder, 0775)
	}
	outPath := fmt.Sprintf(folder+"/%s", fileInfo.Name)
	if fileInfo.ReName != "" {
		outPath = fmt.Sprintf(folder+"/%s", fileInfo.ReName)
	}
	if util.FileExists(outPath) && config.CommonConfig.EnableDistinctFile {
		for i := 0; i < 10000; i++ {
			outPath = fmt.Sprintf(folder+"/%d_%s", i, filepath.Base(header.Filename))
			fileInfo.Name = fmt.Sprintf("%d_%s", i, header.Filename)
			if !util.FileExists(outPath) {
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
	v := "" // util.GetFileSum(outFile, config.CommonConfig.FileSumArithmetic)
	if config.CommonConfig.EnableDistinctFile {
		v = util.GetFileSum(outFile, config.CommonConfig.FileSumArithmetic)
	} else {
		v = util.MD5(svr.GetFilePathByInfo(fileInfo, false))
	}
	fileInfo.Md5 = v
	//fileInfo.Path = folder //strings.Replace( folder,DOCKER_DIR,"",1)
	fileInfo.Path = strings.Replace(folder, config.DOCKER_DIR, "", 1)
	fileInfo.Peers = append(fileInfo.Peers, svr.host)
	//fmt.Println("upload",fileInfo)
	return fileInfo, nil
}

func (svr *Server) Upload(ctx *gin.Context) {
	var (
		fn     string
		folder string
		fpBody *os.File
	)
	folder = config.STORE_DIR + "/_tmp/" + util.GetToDay()
	if !util.FileExists(folder) {
		os.MkdirAll(folder, 0777)

	}
	fn = folder + "/" + util.GetUUID()
	fpTmp, err := os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		log.Error(err)
		ctx.JSON(http.StatusNotFound, err.Error())
		return
	}
	defer os.Remove(fn)
	defer fpTmp.Close()

	if _, err = io.Copy(fpTmp, ctx.Request.Body); err != nil {
		log.Error(err)
		ctx.JSON(http.StatusNotFound, err.Error())
		return
	}
	fpBody, err = os.Open(fn)
	ctx.Request.Body = fpBody
	done := make(chan bool, 1)
	svr.queueUpload <- WrapReqResp{ctx, done}
	<-done
}

func (svr *Server) upload(ctx *gin.Context) {
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
	if config.CommonConfig.EnableCrossOrigin {
		util.CrossOrigin(ctx)
	}

	if config.CommonConfig.AuthUrl != "" {
		if !CheckAuth(w, r) {
			log.Warn("auth fail", r.Form)
			util.NotPermit(w, r)
			ctx.JSON(http.StatusNotFound, "auth fail")
			return
		}
	}

	md5sum = r.FormValue("md5")
	fileName = r.FormValue("filename")
	output = r.FormValue("output")
	if config.CommonConfig.ReadOnly {
		ctx.JSON(http.StatusNotFound, "(error) readonly")
		return
	}
	if config.CommonConfig.EnableCustomPath {
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
	if config.CommonConfig.EnableGoogleAuth && scene != "" {
		if secret, ok = svr.sceneMap.GetValue(scene); ok {
			if !svr.VerifyGoogleCode(secret.(string), code, int64(config.CommonConfig.DownloadTokenExpire/30)) {
				util.NotPermit(w, r)
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
		scene = config.CommonConfig.DefaultScene // Read: scene="default"
	}
	if output == "" {
		output = "text" //Read: default output = "json"
	}
	if !util.Contains(output, []string{"json", "text"}) {
		ctx.JSON(http.StatusNotFound, "output just support json or text")
		return
	}
	fileInfo.Scene = scene
	if _, err = svr.CheckScene(scene); err != nil {
		ctx.JSON(http.StatusNotFound, err.Error())
		return
	}
	if _, err = svr.SaveUploadFile(uploadFile, uploadHeader, &fileInfo, r); err != nil {
		ctx.JSON(http.StatusNotFound, err.Error())
		return
	}
	if config.CommonConfig.EnableDistinctFile {
		if v, _ := svr.GetFileInfoFromLevelDB(fileInfo.Md5); v != nil && v.Md5 != "" {
			fileResult = BuildFileResult(v, r)
			if config.CommonConfig.RenameFile {
				os.Remove(config.DOCKER_DIR + fileInfo.Path + "/" + fileInfo.ReName)
			} else {
				os.Remove(config.DOCKER_DIR + fileInfo.Path + "/" + fileInfo.Name)
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
	if !config.CommonConfig.EnableDistinctFile {
		// bugfix filecount stat
		fileInfo.Md5 = util.MD5(svr.GetFilePathByInfo(&fileInfo, false))
	}
	if config.CommonConfig.EnableMergeSmallFile && fileInfo.Size < config.SmallFileSize {
		if err = svr.SaveSmallFile(&fileInfo); err != nil {
			log.Error(err)
			ctx.JSON(http.StatusNotFound, err.Error())
			return
		}
	}
	svr.saveFileMd5Log(&fileInfo, config.FileMd5Name) //maybe slow
	go svr.postFileToPeer(&fileInfo)
	if fileInfo.Size <= 0 {
		log.Error("file size is zero")
		ctx.JSON(http.StatusNotFound, "file size is zero")
		return
	}
	fileResult = BuildFileResult(&fileInfo, r)
	ctx.JSON(http.StatusOK, fileResult)
}

func (svr *Server) SaveSmallFile(fileInfo *FileInfo) error {
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
	fpath = config.DOCKER_DIR + fileInfo.Path + "/" + filename
	largeDir = config.LARGE_DIR + "/" + config.CommonConfig.PeerId
	if !util.FileExists(largeDir) {
		os.MkdirAll(largeDir, 0775)
	}
	reName = fmt.Sprintf("%d", util.RandInt(100, 300))
	destPath = largeDir + "/" + reName
	svr.lockMap.LockKey(destPath)
	defer svr.lockMap.UnLockKey(destPath)
	if util.FileExists(fpath) {
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
		fileInfo.Path = strings.Replace(largeDir, config.DOCKER_DIR, "", 1)
	}
	return nil
}

func (svr *Server) SendToMail(to, subject, body, mailtype string) error {
	host := config.CommonConfig.Mail.Host
	user := config.CommonConfig.Mail.User
	password := config.CommonConfig.Mail.Password
	hp := strings.Split(host, ":")
	auth := smtp.PlainAuth("", user, password, hp[0])
	var contentType string
	if mailtype == "html" {
		contentType = "Content-Type: text/" + mailtype + "; charset=UTF-8"
	} else {
		contentType = "Content-Type: text/plain" + "; charset=UTF-8"
	}
	msg := []byte("To: " + to + "\r\nFrom: " + user + ">\r\nSubject: " + "\r\n" + contentType + "\r\n\r\n" + body)
	sendTo := strings.Split(to, ";")
	err := smtp.SendMail(host, auth, user, sendTo, msg)
	return err
}

func BenchMark(ctx *gin.Context) {
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
		if data, err := json.Marshal(&f); err == nil {
			batch.Put([]byte(s), data)
		}
		if i%10000 == 0 {
			if batch.Len() > 0 {
				Svr.ldb.Write(batch, nil)
				//				batch = new(leveldb.Batch)
				batch.Reset()
			}
			fmt.Println(i, time.Since(t).Seconds())
		}
		//fmt.Println(server.GetFileInfoFromLevelDB(s))
	}
	util.WriteFile("time.txt", time.Since(t).String())
	fmt.Println(time.Since(t).String())
}

func (svr *Server) RepairStatWeb(ctx *gin.Context) {
	var (
		result JsonResult
		date   string
		inner  string
	)
	r := ctx.Request
	if !IsPeer(r) {
		result.Message = svr.GetClusterNotPermitMessage(r)
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
		date = util.GetToDay()
	}
	if inner != "1" {
		for _, peer := range config.CommonConfig.Peers {
			req := httplib.Post(peer + svr.getRequestURI("repair_stat"))
			req.Param("inner", "1")
			req.Param("date", date)
			if _, err := req.String(); err != nil {
				log.Error(err)
			}
		}
	}
	result.Data = svr.RepairStatByDate(date)
	result.Status = "ok"

	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) Stat(ctx *gin.Context) {
	var (
		result   JsonResult
		inner    string
		echart   string
		category []string
		barCount []int64
		barSize  []int64
		dataMap  map[string]interface{}
	)
	r := ctx.Request
	if !IsPeer(r) {
		result.Message = svr.GetClusterNotPermitMessage(r)
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	r.ParseForm()
	inner = r.FormValue("inner")
	echart = r.FormValue("echart")
	data := svr.GetStat()
	result.Status = "ok"
	result.Data = data
	if echart == "1" {
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

}

func (svr *Server) GetStat() []StatDateFileInfo {
	var (
		min   int64
		max   int64
		err   error
		i     int64
		rows  []StatDateFileInfo
		total StatDateFileInfo
	)
	min = 20190101
	max = 20190101
	for k := range svr.statMap.Get() {
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
		if v, ok := svr.statMap.GetValue(s + "_" + config.StatFileTotalSizeKey); ok {
			var info StatDateFileInfo
			info.Date = s
			switch v.(type) {
			case int64:
				info.TotalSize = v.(int64)
				total.TotalSize = total.TotalSize + v.(int64)
			}
			if v, ok := svr.statMap.GetValue(s + "_" + config.StatisticsFileCountKey); ok {
				switch v.(type) {
				case int64:
					info.FileCount = v.(int64)
					total.FileCount = total.FileCount + v.(int64)
				}
			}
			rows = append(rows, info)
		}
	}
	total.Date = "all"
	rows = append(rows, total)
	return rows
}

func (svr *Server) RegisterExit() {
	c := make(chan os.Signal)
	signal.Notify(c, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range c {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				svr.ldb.Close()
				log.Info("Exit", s)
				os.Exit(1)
			}
		}
	}()
}

// Read: append the file info to queen channel, the file info will send to all peers
func (svr *Server) AppendToQueue(fileInfo *FileInfo) {

	for (len(svr.queueToPeers) + config.QueueSize/10) > config.QueueSize {
		time.Sleep(time.Millisecond * 50)
	}
	svr.queueToPeers <- *fileInfo
}

func (svr *Server) AppendToDownloadQueue(fileInfo *FileInfo) {
	for (len(svr.queueFromPeers) + config.QueueSize/10) > config.QueueSize {
		time.Sleep(time.Millisecond * 50)
	}
	svr.queueFromPeers <- *fileInfo
}

func (svr *Server) ConsumerDownLoad() {
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
					svr.DownloadFromPeer(peer, &fileInfo)
					break
				}
			}
		}
	}
	for i := 0; i < config.CommonConfig.SyncWorker; i++ {
		go ConsumerFunc()
	}
}

func (svr *Server) RemoveDownloading() {
	go func() {
		for {
			iter := svr.ldb.NewIterator(levelDBUtil.BytesPrefix([]byte("downloading_")), nil)
			for iter.Next() {
				key := iter.Key()
				keys := strings.Split(string(key), "_")
				if len(keys) == 3 {
					if t, err := strconv.ParseInt(keys[1], 10, 64); err == nil && time.Now().Unix()-t > 60*10 {
						os.Remove(config.DOCKER_DIR + keys[2])
					}
				}
			}
			iter.Release()
			time.Sleep(time.Minute * 3)
		}
	}()
}

func (svr *Server) ConsumerLog() {
	go func() {
		for fileLog := range svr.queueFileLog {
			svr.saveFileMd5Log(fileLog.FileInfo, fileLog.FileName)
		}
	}()
}

func (svr *Server) LoadSearchDict() {
	go func() {
		log.Info("Load search dict ....")
		f, err := os.Open(config.CONST_SEARCH_FILE_NAME)
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

func (svr *Server) SaveSearchDict() {
	svr.lockMap.LockKey(config.CONST_SEARCH_FILE_NAME)
	defer svr.lockMap.UnLockKey(config.CONST_SEARCH_FILE_NAME)

	searchDict := svr.searchMap.Get()
	searchFile, err := os.OpenFile(config.CONST_SEARCH_FILE_NAME, os.O_RDWR, 0755)
	if err != nil {
		log.Error(err)
		return
	}
	defer searchFile.Close()

	for k, v := range searchDict {
		searchFile.WriteString(fmt.Sprintf("%s\t%s", k, v.(string)))
	}
}

func (svr *Server) ConsumerPostToPeer() {
	ConsumerFunc := func() {
		for fileInfo := range svr.queueToPeers {
			svr.postFileToPeer(&fileInfo)
		}
	}
	for i := 0; i < config.CommonConfig.SyncWorker; i++ {
		go ConsumerFunc()
	}
}

func (svr *Server) ConsumerUpload() {
	ConsumerFunc := func() {
		for wr := range svr.queueUpload {
			svr.upload(wr.ctx)
			svr.rtMap.AddCountInt64(config.CONST_UPLOAD_COUNTER_KEY, wr.ctx.Request.ContentLength)
			if v, ok := svr.rtMap.GetValue(config.CONST_UPLOAD_COUNTER_KEY); ok {
				if v.(int64) > 1*1024*1024*1024 {
					var _v int64
					svr.rtMap.Put(config.CONST_UPLOAD_COUNTER_KEY, _v)
					debug.FreeOSMemory()
				}
			}
			wr.done <- true
		}
	}
	for i := 0; i < config.CommonConfig.UploadWorker; i++ {
		go ConsumerFunc()
	}
}

// Read :  AutoRepair what?
func (svr *Server) AutoRepair(forceRepair bool) {
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
			localSet  mapset.Set
			remoteSet mapset.Set
			allSet    mapset.Set
			tmpSet    mapset.Set
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
			req := httplib.Get(fmt.Sprintf("%s%s?date=%s&force=%s", peer, svr.getRequestURI("sync"), dateStat.Date, "1"))
			req.SetTimeout(time.Second*5, time.Second*5)
			if _, err = req.String(); err != nil {
				log.Error(err)
			}
			log.Info(fmt.Sprintf("syn file from %s date %s", peer, dateStat.Date))
		}
		for _, peer := range config.CommonConfig.Peers {
			req := httplib.Post(fmt.Sprintf("%s%s", peer, svr.getRequestURI("stat")))
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
				countKey = dateStat.Date + "_" + config.StatisticsFileCountKey
				if v, ok := svr.statMap.GetValue(countKey); ok {
					switch v.(type) {
					case int64:
						if v.(int64) != dateStat.FileCount || forceRepair {
							//不相等,找差异
							//TODO
							req := httplib.Post(fmt.Sprintf("%s%s", peer, svr.getRequestURI("get_md5s_by_date")))
							req.SetTimeout(time.Second*15, time.Second*60)
							req.Param("date", dateStat.Date)
							if md5s, err = req.String(); err != nil {
								continue
							}
							if localSet, err = svr.GetMd5sByDate(dateStat.Date, config.FileMd5Name); err != nil {
								log.Error(err)
								continue
							}
							remoteSet = util.StrToMapSet(md5s, ",")
							allSet = localSet.Union(remoteSet)
							md5s = util.MapSetToStr(allSet.Difference(localSet), ",")
							req = httplib.Post(fmt.Sprintf("%s%s", peer, svr.getRequestURI("receive_md5s")))
							req.SetTimeout(time.Second*15, time.Second*60)
							req.Param("md5s", md5s)
							req.String()
							tmpSet = allSet.Difference(remoteSet)
							for v := range tmpSet.Iter() {
								if v != nil {
									if fileInfo, err = svr.GetFileInfoFromLevelDB(v.(string)); err != nil {
										log.Error(err)
										continue
									}
									svr.AppendToQueue(fileInfo)
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

func (svr *Server) CleanLogLevelDBByDate(date string, filename string) {
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
		keys      mapset.Set
	)
	keys = mapset.NewSet()
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, filename)
	iter := Svr.logDB.NewIterator(levelDBUtil.BytesPrefix([]byte(keyPrefix)), nil)
	for iter.Next() {
		keys.Add(string(iter.Value()))
	}
	iter.Release()
	for key := range keys.Iter() {
		err = svr.RemoveKeyFromLevelDB(key.(string), svr.logDB)
		if err != nil {
			log.Error(err)
		}
	}
}

func (svr *Server) CleanAndBackUp() {
	Clean := func() {
		if svr.curDate != util.GetToDay() {
			filenames := []string{config.Md5QueueFileName, config.Md5ErrorFileName, config.RemoveMd5FileName}
			yesterday := util.GetDayFromTimeStamp(time.Now().AddDate(0, 0, -1).Unix())
			for _, filename := range filenames {
				svr.CleanLogLevelDBByDate(yesterday, filename)
			}
			svr.BackUpMetaDataByDate(yesterday)
			svr.curDate = util.GetToDay()
		}
	}
	go func() {
		for {
			time.Sleep(time.Hour * 6)
			Clean()
		}
	}()
}

func (svr *Server) LoadFileInfoByDate(date string, filename string) (mapset.Set, error) {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("LoadFileInfoByDate")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		err       error
		keyPrefix string
		fileInfos mapset.Set
	)
	fileInfos = mapset.NewSet()
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, filename)
	iter := Svr.logDB.NewIterator(levelDBUtil.BytesPrefix([]byte(keyPrefix)), nil)
	for iter.Next() {
		var fileInfo FileInfo
		if err = json.Unmarshal(iter.Value(), &fileInfo); err != nil {
			continue
		}
		fileInfos.Add(&fileInfo)
	}
	iter.Release()
	return fileInfos, nil
}

func (svr *Server) LoadQueueSendToPeer() {
	if queue, err := svr.LoadFileInfoByDate(util.GetToDay(), config.Md5QueueFileName); err != nil {
		log.Error(err)
	} else {
		for fileInfo := range queue.Iter() {
			//svr.queueFromPeers <- *fileInfo.(*FileInfo)
			svr.AppendToDownloadQueue(fileInfo.(*FileInfo))
		}
	}
}

func (svr *Server) CheckClusterStatus() {
	check := func() {
		defer func() {
			if re := recover(); re != nil {
				buffer := debug.Stack()
				log.Error("CheckClusterStatus")
				log.Error(re)
				log.Error(string(buffer))
			}
		}()
		var (
			status  JsonResult
			err     error
			subject string
			body    string
			req     *httplib.BeegoHTTPRequest
		)
		for _, peer := range config.CommonConfig.Peers {
			req = httplib.Get(fmt.Sprintf("%s%s", peer, svr.getRequestURI("status")))
			req.SetTimeout(time.Second*5, time.Second*5)
			err = req.ToJSON(&status)
			if err != nil || status.Status != "ok" {
				for _, to := range config.CommonConfig.AlarmReceivers {
					subject = "fastdfs server error"
					if err != nil {
						body = fmt.Sprintf("%s\nserver:%s\nerror:\n%s", subject, peer, err.Error())
					} else {
						body = fmt.Sprintf("%s\nserver:%s\n", subject, peer)
					}
					if err = svr.SendToMail(to, subject, body, "text"); err != nil {
						log.Error(err)
					}
				}
				if config.CommonConfig.AlarmUrl != "" {
					req = httplib.Post(config.CommonConfig.AlarmUrl)
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

func (svr *Server) RepairFileInfo(ctx *gin.Context) {
	var (
		result JsonResult
	)
	if !IsPeer(ctx.Request) {
		ctx.JSON(http.StatusNotFound, svr.GetClusterNotPermitMessage(ctx.Request))
		return
	}

	if !config.CommonConfig.EnableMigrate {
		ctx.JSON(http.StatusNotFound, "please set enable_migrate=true")
		return
	}

	result.Status = "ok"
	result.Message = "repair job start,don't try again,very danger "
	go svr.RepairFileInfoFromFile()

	ctx.JSON(http.StatusNotFound, result)
}

func (svr *Server) Reload(ctx *gin.Context) {
	var (
		err     error
		data    []byte
		cfg     config.Config
		action  string
		cfgJson string
		result  JsonResult
	)
	r := ctx.Request
	result.Status = "fail"
	r.ParseForm()
	if !IsPeer(r) {
		ctx.JSON(http.StatusNotFound, svr.GetClusterNotPermitMessage(r))
		return
	}
	cfgJson = r.FormValue("cfg")
	action = r.FormValue("action")
	_ = cfgJson
	if action == "get" {
		result.Data = config.CommonConfig
		result.Status = "ok"
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	if action == "set" {
		if cfgJson == "" {
			result.Message = "(error)parameter cfg(json) require"
			ctx.JSON(http.StatusNotFound, result)
			return
		}
		if err = json.Unmarshal([]byte(cfgJson), &cfg); err != nil {
			log.Error(err)
			result.Message = err.Error()
			ctx.JSON(http.StatusNotFound, result)
			return
		}
		result.Status = "ok"
		cfgJson = util.JsonEncodePretty(cfg)
		util.WriteFile(config.CONST_CONF_FILE_NAME, cfgJson)
		ctx.JSON(http.StatusOK, result)
		return
	}
	if action == "reload" {
		if data, err = ioutil.ReadFile(config.CONST_CONF_FILE_NAME); err != nil {
			result.Message = err.Error()
			ctx.JSON(http.StatusNotFound, result)
			return
		}
		if err = json.Unmarshal(data, &cfg); err != nil {
			result.Message = err.Error()
			ctx.JSON(http.StatusNotFound, result)
			return
		}
		config.ParseConfig(config.CONST_CONF_FILE_NAME)
		svr.InitComponent(true)
		result.Status = "ok"
		ctx.JSON(http.StatusOK, result)
		return
	}

	ctx.JSON(http.StatusNotFound, "(error)action support set(json) get reload")

}

func (svr *Server) RemoveEmptyDir(ctx *gin.Context) {
	var (
		result JsonResult
	)
	r := ctx.Request
	result.Status = "ok"
	if IsPeer(r) {
		go util.RemoveEmptyDir(config.DATA_DIR)
		go util.RemoveEmptyDir(config.STORE_DIR)
		result.Message = "clean job start ..,don't try again!!!"
		ctx.JSON(http.StatusOK, result)
		return
	}
	result.Message = svr.GetClusterNotPermitMessage(r)
	ctx.JSON(http.StatusOK, result)

}

func (svr *Server) BackUp(ctx *gin.Context) {
	var (
		err    error
		date   string
		result JsonResult
		inner  string
		url    string
	)
	r := ctx.Request
	result.Status = "ok"
	r.ParseForm()
	date = r.FormValue("date")
	inner = r.FormValue("inner")
	if date == "" {
		date = util.GetToDay()
	}
	if IsPeer(r) {
		if inner != "1" {
			for _, peer := range config.CommonConfig.Peers {
				backUp := func(peer string, date string) {
					url = fmt.Sprintf("%s%s", peer, svr.getRequestURI("backup"))
					req := httplib.Post(url)
					req.Param("date", date)
					req.Param("inner", "1")
					req.SetTimeout(time.Second*5, time.Second*600)
					if _, err = req.String(); err != nil {
						log.Error(err)
					}
				}
				go backUp(peer, date)
			}
		}
		go svr.BackUpMetaDataByDate(date)
		result.Message = "back job start..."
		ctx.JSON(http.StatusOK, result)
		return
	}

	result.Message = svr.GetClusterNotPermitMessage(r)
	ctx.JSON(http.StatusNotAcceptable, result)
}

// Notice: performance is poor,just for low capacity,but low memory ,
//if you want to high performance,use searchMap for search,but memory ....
func (svr *Server) Search(ctx *gin.Context) {
	var (
		result    JsonResult
		err       error
		kw        string
		count     int
		fileInfos []FileInfo
		md5s      []string
	)
	r := ctx.Request
	kw = r.FormValue("kw")
	if !IsPeer(r) {
		result.Message = svr.GetClusterNotPermitMessage(r)
		ctx.JSON(http.StatusNotAcceptable, result)
		return
	}
	iter := svr.ldb.NewIterator(nil, nil)
	for iter.Next() {
		var fileInfo FileInfo
		value := iter.Value()
		if err = json.Unmarshal(value, &fileInfo); err != nil {
			log.Error(err)
			continue
		}
		if strings.Contains(fileInfo.Name, kw) && !util.Contains(fileInfo.Md5, md5s) {
			count = count + 1
			fileInfos = append(fileInfos, fileInfo)
			md5s = append(md5s, fileInfo.Md5)
		}
		if count >= 100 {
			break
		}
	}
	iter.Release()
	err = iter.Error()
	if err != nil {
		log.Error()
	}
	//fileInfos=svr.SearchDict(kw) // serch file from map for huge capacity
	result.Status = "ok"
	result.Data = fileInfos
	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) SearchDict(kw string) []FileInfo {
	var (
		fileInfos []FileInfo
		fileInfo  *FileInfo
	)
	for dict := range svr.searchMap.Iter() {
		if strings.Contains(dict.Val.(string), kw) {
			if fileInfo, _ = svr.GetFileInfoFromLevelDB(dict.Key); fileInfo != nil {
				fileInfos = append(fileInfos, *fileInfo)
			}
		}
	}
	return fileInfos
}

func (svr *Server) ListDir(ctx *gin.Context) {
	var (
		result      JsonResult
		dir         string
		filesInfo   []os.FileInfo
		err         error
		filesResult []FileInfoResult
		tmpDir      string
	)
	r := ctx.Request
	if !IsPeer(r) {
		result.Message = svr.GetClusterNotPermitMessage(r)
		ctx.JSON(http.StatusNotAcceptable, result)
		return
	}
	dir = r.FormValue("dir")
	//if dir == "" {
	//	result.Message = "dir can't null"
	//	w.Write([]byte(util.JsonEncodePretty(result)))
	//	return
	//}
	dir = strings.Replace(dir, ".", "", -1)
	if tmpDir, err = os.Readlink(dir); err == nil {
		dir = tmpDir
	}
	filesInfo, err = ioutil.ReadDir(config.DOCKER_DIR + config.StoreDirName + "/" + dir)
	if err != nil {
		log.Error(err)
		result.Message = err.Error()
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	for _, f := range filesInfo {
		fi := FileInfoResult{
			Name:    f.Name(),
			Size:    f.Size(),
			IsDir:   f.IsDir(),
			ModTime: f.ModTime().Unix(),
			Path:    dir,
			Md5:     util.MD5(strings.Replace(config.StoreDirName+"/"+dir+"/"+f.Name(), "//", "/", -1)),
		}
		filesResult = append(filesResult, fi)
	}
	result.Status = "ok"
	result.Data = filesResult
	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) VerifyGoogleCode(secret string, code string, discrepancy int64) bool {
	var (
		goauth *googleAuthenticator.GAuth
	)
	goauth = googleAuthenticator.NewGAuth()
	if ok, err := goauth.VerifyCode(secret, code, discrepancy); ok {
		return ok
	} else {
		log.Error(err)
		return ok
	}
}

func (svr *Server) GenGoogleCode(ctx *gin.Context) {
	var (
		err    error
		result JsonResult
		secret string
		goauth *googleAuthenticator.GAuth
	)
	r := ctx.Request
	r.ParseForm()
	goauth = googleAuthenticator.NewGAuth()
	secret = r.FormValue("secret")
	result.Status = "ok"
	result.Message = "ok"
	if !IsPeer(r) {
		result.Message = svr.GetClusterNotPermitMessage(r)
		ctx.JSON(http.StatusNotAcceptable, result)
		return
	}
	if result.Data, err = goauth.GetCode(secret); err != nil {
		result.Message = err.Error()
		ctx.JSON(http.StatusNotFound, result)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) GenGoogleSecret(ctx *gin.Context) {
	var (
		result JsonResult
	)
	result.Status = "ok"
	result.Message = "ok"
	r := ctx.Request
	if !IsPeer(r) {
		result.Message = svr.GetClusterNotPermitMessage(r)
		ctx.JSON(http.StatusNotAcceptable, result)
	}
	GetSeed := func(length int) string {
		seeds := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
		s := ""
		random.Seed(time.Now().UnixNano())
		for i := 0; i < length; i++ {
			s += string(seeds[random.Intn(32)])
		}
		return s
	}

	result.Data = GetSeed(16)
	ctx.JSON(http.StatusOK, result)
}

func (svr *Server) Report(ctx *gin.Context) {
	var (
		data           []byte
		reportFileName string
		result         JsonResult
		html           string
		err            error
	)
	r := ctx.Request
	result.Status = "ok"
	r.ParseForm()
	if IsPeer(r) {
		reportFileName = config.STATIC_DIR + "/report.html"
		if util.Exist(reportFileName) {
			if data, err = util.ReadFile(reportFileName); err != nil {
				log.Error(err)
				result.Message = err.Error()
				ctx.JSON(http.StatusNotFound, result)
				return
			}
			html = string(data)
			if config.CommonConfig.SupportGroupManage {
				html = strings.Replace(html, "{group}", "/"+config.CommonConfig.Group, 1)
			} else {
				html = strings.Replace(html, "{group}", "", 1)
			}
			ctx.HTML(http.StatusOK, "report.html", html)
			return
		}
		ctx.JSON(http.StatusNotFound, fmt.Sprintf("%s is not found", reportFileName))
		return
	}
	ctx.JSON(http.StatusNotAcceptable, svr.GetClusterNotPermitMessage(r))
}

func (svr *Server) Repair(ctx *gin.Context) {
	var (
		force       string
		forceRepair bool
		result      JsonResult
	)
	r := ctx.Request
	result.Status = "ok"
	r.ParseForm()
	force = r.FormValue("force")
	if force == "1" {
		forceRepair = true
	}
	if IsPeer(r) {
		go svr.AutoRepair(forceRepair)
		result.Message = "repair job start..."
		ctx.JSON(http.StatusOK, result)
		return
	}

	result.Message = svr.GetClusterNotPermitMessage(r)
	ctx.JSON(http.StatusNotAcceptable, result)
}

func (svr *Server) Status(ctx *gin.Context) {
	var (
		status   JsonResult
		sts      map[string]interface{}
		today    string
		sumset   mapset.Set
		ok       bool
		v        interface{}
		err      error
		appDir   string
		diskInfo *disk.UsageStat
		memInfo  *mem.VirtualMemoryStat
	)
	memStat := new(runtime.MemStats)
	runtime.ReadMemStats(memStat)
	today = util.GetToDay()
	sts = make(map[string]interface{})
	sts["Fs.QueueFromPeers"] = len(svr.queueFromPeers)
	sts["Fs.QueueToPeers"] = len(svr.queueToPeers)
	sts["Fs.QueueFileLog"] = len(svr.queueFileLog)
	for _, k := range []string{config.FileMd5Name, config.Md5ErrorFileName, config.Md5QueueFileName} {
		k2 := fmt.Sprintf("%s_%s", today, k)
		if v, ok = svr.sumMap.GetValue(k2); ok {
			sumset = v.(mapset.Set)
			if k == config.Md5QueueFileName {
				sts["Fs.QueueSetSize"] = sumset.Cardinality()
			}
			if k == config.Md5ErrorFileName {
				sts["Fs.ErrorSetSize"] = sumset.Cardinality()
			}
			if k == config.FileMd5Name {
				sts["Fs.FileSetSize"] = sumset.Cardinality()
			}
		}
	}
	sts["Fs.AutoRepair"] = config.CommonConfig.AutoRepair
	sts["Fs.QueueUpload"] = len(svr.queueUpload)
	sts["Fs.RefreshInterval"] = config.CommonConfig.RefreshInterval
	sts["Fs.Peers"] = config.CommonConfig.Peers
	sts["Fs.Local"] = svr.host
	sts["Fs.FileStats"] = svr.GetStat()
	sts["Fs.ShowDir"] = config.CommonConfig.ShowDir
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

func (svr *Server) Index(ctx *gin.Context) {
	var (
		uploadUrl    string
		uploadBigUrl string
	)
	uploadPage := config.DefaultUploadPage
	uploadUrl = "/upload"
	uploadBigUrl = config.BigUploadPathSuffix
	if config.CommonConfig.EnableWebUpload {
		if config.CommonConfig.SupportGroupManage {
			uploadUrl = "/" + config.CommonConfig.Group + uploadUrl
			uploadBigUrl = "/" + config.CommonConfig.Group + config.BigUploadPathSuffix
		}
		uploadPageName := config.STATIC_DIR + "/upload.html"
		if util.Exist(uploadPageName) {
			if data, err := util.ReadFile(uploadPageName); err != nil {
				log.Error(err)
			} else {
				uploadPage = string(data)
			}
		} else {
			util.WriteFile(uploadPageName, uploadPage)
		}
		uploadPage = fmt.Sprintf(uploadPage, uploadUrl, config.CommonConfig.DefaultScene, uploadBigUrl)
		//ctx.HTML(http.StatusOK, "upload.html", gin.H{"title": "Main website"})
		ctx.Data(http.StatusOK, "text/html", []byte(uploadPage))
	}

	ctx.JSON(http.StatusNotFound, "web upload deny")
}

func (svr *Server) test() {

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
	if config.CommonConfig.AuthUrl != "" {
		if auth_token, ok := info.MetaData["auth_token"]; !ok {
			msg := "token auth fail,auth_token is not in http header Upload-Metadata," +
				"in uppy uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca' })"
			log.Error(msg, fmt.Sprintf("current header:%v", info.MetaData))
			return "", httpError{error: errors.New(msg), statusCode: 401}
		} else {
			req := httplib.Post(config.CommonConfig.AuthUrl)
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

func (svr *Server) initTus() {
	var (
		err     error
		fileLog *os.File
		bigDir  string
	)
	BIG_DIR := config.STORE_DIR + "/_big/" + config.CommonConfig.PeerId
	os.MkdirAll(BIG_DIR, 0775)
	os.MkdirAll(config.LOG_DIR, 0775)
	store := filestore.FileStore{
		Path: BIG_DIR,
	}
	if fileLog, err = os.OpenFile(config.LOG_DIR+"/tusd.log", os.O_CREATE|os.O_RDWR, 0666); err != nil {
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
					util.CopyFile(config.LOG_DIR+"/tusd.log", config.LOG_DIR+"/tusd.log.2")
					fileLog.Seek(0, 0)
					fileLog.Truncate(0)
					fileLog.Seek(0, 2)
				}
			}
			time.Sleep(time.Second * 30)
		}
	}()
	l := slog.New(fileLog, "[tusd] ", slog.LstdFlags)
	bigDir = config.BigUploadPathSuffix
	if config.CommonConfig.SupportGroupManage {
		bigDir = fmt.Sprintf("/%s%s", config.CommonConfig.Group, config.BigUploadPathSuffix)
	}
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
		if fi, err = svr.GetFileInfoFromLevelDB(id); err != nil {
			log.Error(err)
			return nil, err
		} else {
			if config.CommonConfig.AuthUrl != "" {
				fileResult := util.JsonEncodePretty(BuildFileResult(fi, nil))
				bufferReader := bytes.NewBuffer([]byte(fileResult))
				return bufferReader, nil
			}
			fn = fi.Name
			if fi.ReName != "" {
				fn = fi.ReName
			}
			fp := config.DOCKER_DIR + fi.Path + "/" + fn
			if util.FileExists(fp) {
				log.Info(fmt.Sprintf("download:%s", fp))
				return os.Open(fp)
			}
			ps := strings.Split(fp, ",")
			if len(ps) > 2 && util.FileExists(ps[0]) {
				if length, err = strconv.Atoi(ps[2]); err != nil {
					return nil, err
				}
				if offset, err = strconv.ParseInt(ps[1], 10, 64); err != nil {
					return nil, err
				}
				if buffer, err = util.ReadFileByOffSet(ps[0], offset, length); err != nil {
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
				scene := config.CommonConfig.DefaultScene
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
				if md5sum, err = util.GetFileSumByName(oldFullPath, config.CommonConfig.FileSumArithmetic); err != nil {
					log.Error(err)
					continue
				}
				ext := path.Ext(name)
				filename := md5sum + ext
				if name != "" {
					filename = name
				}
				if config.CommonConfig.RenameFile {
					filename = md5sum + ext
				}
				timeStamp := time.Now().Unix()
				fpath := time.Now().Format("/20060102/15/04/")
				if pathCustom != "" {
					fpath = "/" + strings.Replace(pathCustom, ".", "", -1) + "/"
				}
				newFullPath := config.STORE_DIR + "/" + scene + fpath + config.CommonConfig.PeerId + "/" + filename
				if pathCustom != "" {
					newFullPath = config.STORE_DIR + "/" + scene + fpath + filename
				}
				if fi, err := svr.GetFileInfoFromLevelDB(md5sum); err != nil {
					log.Error(err)
				} else {
					tpath := svr.GetFilePathByInfo(fi, true)
					if fi.Md5 != "" && util.FileExists(tpath) {
						if _, err := svr.SaveFileInfoToLevelDB(info.ID, fi, svr.ldb); err != nil {
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
				fpath2 = config.StoreDirName + "/" + config.CommonConfig.DefaultScene + fpath + config.CommonConfig.PeerId
				if pathCustom != "" {
					fpath2 = config.StoreDirName + "/" + config.CommonConfig.DefaultScene + fpath
					fpath2 = strings.TrimRight(fpath2, "/")
				}

				os.MkdirAll(config.DOCKER_DIR+fpath2, 0775)
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
				if _, err = svr.SaveFileInfoToLevelDB(info.ID, fileInfo, svr.ldb); err != nil {
					//assosiate file id
					log.Error(err)
				}
				svr.SaveFileMd5Log(fileInfo, config.FileMd5Name)
				go svr.postFileToPeer(fileInfo)
				callBack := func(info tusd.FileInfo, fileInfo *FileInfo) {
					if callback_url, ok := info.MetaData["callback_url"]; ok {
						req := httplib.Post(callback_url)
						req.SetTimeout(time.Second*10, time.Second*10)
						req.Param("info", util.JsonEncodePretty(fileInfo))
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

func (svr *Server) FormatStatInfo() {
	var (
		data  []byte
		err   error
		count int64
		stat  map[string]interface{}
	)
	if util.FileExists(config.CONST_STAT_FILE_NAME) {
		if data, err = util.ReadFile(config.CONST_STAT_FILE_NAME); err != nil {
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
							svr.statMap.Put(k, count)
						}
					default:
						svr.statMap.Put(k, v)
					}
				}
			}
		}
	} else {
		svr.RepairStatByDate(util.GetToDay())
	}
}

// initComponent init current host ip
func (svr *Server) InitComponent(isReload bool) {
	var (
		ip string
	)
	if ip = os.Getenv("GO_FASTDFS_IP"); ip == "" {
		ip = util.GetPublicIP()
	}
	if config.CommonConfig.Host == "" {
		if len(strings.Split(config.CommonConfig.Addr, ":")) == 2 {
			Svr.host = fmt.Sprintf("http://%s:%s", ip, strings.Split(config.CommonConfig.Addr, ":")[1])
			config.CommonConfig.Host = Svr.host
		}
	} else {
		if strings.HasPrefix(config.CommonConfig.Host, "http") {
			Svr.host = config.CommonConfig.Host
		} else {
			Svr.host = "http://" + config.CommonConfig.Host
		}
	}
	ex, _ := regexp.Compile("\\d+\\.\\d+\\.\\d+\\.\\d+")
	var peers []string
	for _, peer := range config.CommonConfig.Peers {
		if util.Contains(ip, ex.FindAllString(peer, -1)) ||
			util.Contains("127.0.0.1", ex.FindAllString(peer, -1)) {
			continue
		}
		if strings.HasPrefix(peer, "http") {
			peers = append(peers, peer)
		} else {
			peers = append(peers, "http://"+peer)
		}
	}
	config.CommonConfig.Peers = peers
	if !isReload {
		svr.FormatStatInfo()
		if config.CommonConfig.EnableTus {
			svr.initTus()
		}
	}
	for _, s := range config.CommonConfig.Scenes {
		kv := strings.Split(s, ":")
		if len(kv) == 2 {
			svr.sceneMap.Put(kv[0], kv[1])
		}
	}
	if config.CommonConfig.ReadTimeout == 0 {
		config.CommonConfig.ReadTimeout = 60 * 10
	}
	if config.CommonConfig.WriteTimeout == 0 {
		config.CommonConfig.WriteTimeout = 60 * 10
	}
	if config.CommonConfig.SyncWorker == 0 {
		config.CommonConfig.SyncWorker = 200
	}
	if config.CommonConfig.UploadWorker == 0 {
		config.CommonConfig.UploadWorker = runtime.NumCPU() + 4
		if runtime.NumCPU() < 4 {
			config.CommonConfig.UploadWorker = 8
		}
	}
	if config.CommonConfig.UploadQueueSize == 0 {
		config.CommonConfig.UploadQueueSize = 200
	}
	if config.CommonConfig.RetryCount == 0 {
		config.CommonConfig.RetryCount = 3
	}
	if config.CommonConfig.SyncDelay == 0 {
		config.CommonConfig.SyncDelay = 60
	}
	if config.CommonConfig.WatchChanSize == 0 {
		config.CommonConfig.WatchChanSize = 100000
	}
}

type HttpHandler struct {
}

func (HttpHandler) ServeHTTP(ctx *gin.Context) {
	status_code := "200"
	req := ctx.Request
	res := ctx.Writer
	defer func(t time.Time) {
		logStr := fmt.Sprintf("[Access] %s | %s | %s | %s | %s |%s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			//res.Header(),
			time.Since(t).String(),
			util.GetClientIp(req),
			req.Method,
			status_code,
			req.RequestURI,
		)
		log.Println(logStr)
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
	if config.CommonConfig.EnableCrossOrigin {
		util.CrossOrigin(ctx)
	}
	http.DefaultServeMux.ServeHTTP(res, req)
}

// GetFilePathFromRequest
func GetFilePathFromRequest(ctx *gin.Context) (string, string) {
	var (
		err       error
		fullPath  string
		smallPath string
		prefix    string
	)
	r := ctx.Request
	fullPath = r.RequestURI[1:]
	if strings.HasPrefix(r.RequestURI, "/"+config.CommonConfig.Group+"/") {
		fullPath = r.RequestURI[len(config.CommonConfig.Group)+2 : len(r.RequestURI)]
	}
	fullPath = strings.Split(fullPath, "?")[0] // just path
	fullPath = config.DOCKER_DIR + config.StoreDirName + "/" + fullPath
	prefix = "/" + config.LARGE_DIR_NAME + "/"
	if config.CommonConfig.SupportGroupManage {
		prefix = "/" + config.CommonConfig.Group + "/" + config.LARGE_DIR_NAME + "/"
	}
	if strings.HasPrefix(r.RequestURI, prefix) {
		smallPath = fullPath //notice order
		fullPath = strings.Split(fullPath, ",")[0]
	}
	if fullPath, err = url.PathUnescape(fullPath); err != nil {
		log.Println(err)
	}
	return fullPath, smallPath
}

// IsPeer check the host that create the request is in the peers
func IsPeer(r *http.Request) bool {
	var (
		ip    string
		peer  string
		bflag bool
	)
	//return true
	ip = util.GetClientIp(r)
	realIp := os.Getenv("GO_FASTDFS_IP")
	if realIp == "" {
		realIp = util.GetPublicIP()
	}
	if ip == "127.0.0.1" || ip == realIp {
		return true
	}
	if util.Contains(ip, config.CommonConfig.AdminIps) {
		return true
	}
	ip = "http://" + ip
	bflag = false
	for _, peer = range config.CommonConfig.Peers {
		if strings.HasPrefix(peer, ip) {
			bflag = true
			break
		}
	}
	return bflag
}
