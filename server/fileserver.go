package server

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	_ "net/http/pprof"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/astaxie/beego/httplib"
	mapset "github.com/deckarep/golang-set"
	_ "github.com/eventials/go-tus"
	"github.com/radovskyb/watcher"
	"github.com/sjqzhang/goutil"
	log "github.com/sjqzhang/seelog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/util"
)

type FileInfo struct {
	Name      string   `json:"name"`
	ReName    string   `json:"rename"`
	Path      string   `json:"path"`
	Md5       string   `json:"md5"`
	Size      int64    `json:"size"`
	Peers     []string `json:"peers"`
	Scene     string   `json:"scene"`
	TimeStamp int64    `json:"timeStamp"`
	OffSet    int64    `json:"offset"`
	retry     int
	op        string
}

type FileLog struct {
	FileInfo *FileInfo
	FileName string
}

type WrapReqResp struct {
	w    *http.ResponseWriter
	r    *http.Request
	done chan bool
}

type JsonResult struct {
	Message string      `json:"message"`
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
}

type FileResult struct {
	Url     string `json:"url"`
	Md5     string `json:"md5"`
	Path    string `json:"path"`
	Domain  string `json:"domain"`
	Scene   string `json:"scene"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	//Just for Compatibility
	Scenes  string `json:"scenes"`
	RetMsg  string `json:"retmsg"`
	RetCode int    `json:"retcode"`
	Src     string `json:"src"`
}

type StatDateFileInfo struct {
	Date      string `json:"date"`
	TotalSize int64  `json:"totalSize"`
	FileCount int64  `json:"fileCount"`
}

type FileInfoResult struct {
	Name    string `json:"name"`
	Md5     string `json:"md5"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	IsDir   bool   `json:"is_dir"`
}

func (c *Server) WatchFilesChange() {
	var (
		w *watcher.Watcher
		//fileInfo FileInfo
		curDir string
		err    error
		qchan  chan *FileInfo
		isLink bool
	)
	qchan = make(chan *FileInfo, Config().WatchChanSize)
	w = watcher.New()
	w.FilterOps(watcher.Create)
	//w.FilterOps(watcher.Create, watcher.Remove)
	curDir, err = filepath.Abs(filepath.Dir(STORE_DIR_NAME))
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
					fpath = strings.Replace(event.Path, curDir, STORE_DIR_NAME, 1)
				}
				fpath = strings.Replace(fpath, string(os.PathSeparator), "/", -1)
				sum := c.util.MD5(fpath)
				fileInfo := FileInfo{
					Size:      event.Size(),
					Name:      event.Name(),
					Path:      strings.TrimSuffix(fpath, "/"+event.Name()), // files/default/20190927/xxx
					Md5:       sum,
					TimeStamp: event.ModTime().Unix(),
					Peers:     []string{c.host},
					OffSet:    -2,
					op:        event.Op.String(),
				}
				log.Info(fmt.Sprintf("WatchFilesChange op:%s path:%s", event.Op.String(), fpath))
				qchan <- &fileInfo
				//c.AppendToQueue(&fileInfo)
			case err := <-w.Error:
				log.Error(err)
			case <-w.Closed:
				return
			}
		}
	}()
	go func() {
		for {
			ch := <-qchan
			if time.Now().Unix()-ch.TimeStamp < Config().SyncDelay {
				qchan <- ch
				time.Sleep(time.Second * 1)
				continue
			} else {
				//if c.op == watcher.Remove.String() {
				//	req := httplib.Post(fmt.Sprintf("%s%s?md5=%s", c.host, c.getRequestURI("delete"), c.Md5))
				//	req.Param("md5", c.Md5)
				//	req.SetTimeout(time.Second*5, time.Second*10)
				//	log.Infof(req.String())
				//}

				if ch.op == watcher.Create.String() {
					log.Info(fmt.Sprintf("Syncfile Add to Queue path:%s", ch.Path+"/"+ch.Name))
					c.AppendToQueue(ch)
					c.SaveFileInfoToLevelDB(ch.Md5, ch, c.ldb)
				}
			}
		}
	}()
	if dir, err := os.Readlink(STORE_DIR_NAME); err == nil {

		if strings.HasSuffix(dir, string(os.PathSeparator)) {
			dir = strings.TrimSuffix(dir, string(os.PathSeparator))
		}
		curDir = dir
		isLink = true
		if err := w.AddRecursive(dir); err != nil {
			log.Error(err)
		}
		w.Ignore(dir + "/_tmp/")
		w.Ignore(dir + "/" + LARGE_DIR_NAME + "/")
	}
	if err := w.AddRecursive("./" + STORE_DIR_NAME); err != nil {
		log.Error(err)
	}
	w.Ignore("./" + STORE_DIR_NAME + "/_tmp/")
	w.Ignore("./" + STORE_DIR_NAME + "/" + LARGE_DIR_NAME + "/")
	if err := w.Start(time.Millisecond * 100); err != nil {
		log.Error(err)
	}
}

func (c *Server) GetFilePathByInfo(fileInfo *FileInfo, withDocker bool) string {
	var (
		fn string
	)
	fn = fileInfo.Name
	if fileInfo.ReName != "" {
		fn = fileInfo.ReName
	}
	if withDocker {
		return DOCKER_DIR + fileInfo.Path + "/" + fn
	}
	return fileInfo.Path + "/" + fn
}

func (c *Server) CheckFileExistByInfo(md5s string, fileInfo *FileInfo) bool {
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
		if info, err = c.GetFileInfoFromLevelDB(fileInfo.Md5); err == nil && info.Md5 == fileInfo.Md5 {
			return true
		} else {
			return false
		}
	}
	fullpath = c.GetFilePathByInfo(fileInfo, true)
	if fi, err = os.Stat(fullpath); err != nil {
		return false
	}
	if fi.Size() == fileInfo.Size {
		return true
	} else {
		return false
	}
}

func (c *Server) ParseSmallFile(filename string) (string, int64, int, error) {
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
	if length > CONST_SMALL_FILE_SIZE || offset < 0 {
		err = errors.New("invalid filesize or offset")
		return filename, -1, -1, err
	}
	return pos[0], offset, length, nil
}

func (c *Server) GetFilePathFromRequest(w http.ResponseWriter, r *http.Request) (string, string) {
	var (
		err       error
		fullpath  string
		smallPath string
		prefix    string
	)
	fullpath = r.RequestURI[1:]
	if strings.HasPrefix(r.RequestURI, "/"+Config().Group+"/") {
		fullpath = r.RequestURI[len(Config().Group)+2 : len(r.RequestURI)]
	}
	fullpath = strings.Split(fullpath, "?")[0] // just path
	fullpath = DOCKER_DIR + STORE_DIR_NAME + "/" + fullpath
	prefix = "/" + LARGE_DIR_NAME + "/"
	if Config().SupportGroupManage {
		prefix = "/" + Config().Group + "/" + LARGE_DIR_NAME + "/"
	}
	if strings.HasPrefix(r.RequestURI, prefix) {
		smallPath = fullpath //notice order
		fullpath = strings.Split(fullpath, ",")[0]
	}
	if fullpath, err = url.PathUnescape(fullpath); err != nil {
		log.Error(err)
	}
	return fullpath, smallPath
}

func (c *Server) GetSmallFileByURI(w http.ResponseWriter, r *http.Request) ([]byte, bool, error) {
	var (
		err      error
		data     []byte
		offset   int64
		length   int
		fullpath string
		info     os.FileInfo
	)
	fullpath, _ = c.GetFilePathFromRequest(w, r)
	if _, offset, length, err = c.ParseSmallFile(r.RequestURI); err != nil {
		return nil, false, err
	}
	if info, err = os.Stat(fullpath); err != nil {
		return nil, false, err
	}
	if info.Size() < offset+int64(length) {
		return nil, true, errors.New("noFound")
	} else {
		data, err = c.util.ReadFileByOffSet(fullpath, offset, length)
		if err != nil {
			return nil, false, err
		}
		return data, false, err
	}
}

func (c *Server) GetServerURI(r *http.Request) string {
	return fmt.Sprintf("http://%s/", r.Host)
}

func (c *Server) CheckFileAndSendToPeer(date string, filename string, isForceUpload bool) {
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
	if md5set, err = c.GetMd5sByDate(date, filename); err != nil {
		log.Error(err)
		return
	}
	md5s = md5set.ToSlice()
	for _, md := range md5s {
		if md == nil {
			continue
		}
		if fileInfo, _ := c.GetFileInfoFromLevelDB(md.(string)); fileInfo != nil && fileInfo.Md5 != "" {
			if isForceUpload {
				fileInfo.Peers = []string{}
			}
			if len(fileInfo.Peers) > len(Config().Peers) {
				continue
			}
			if !c.util.Contains(c.host, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, c.host) // peer is null
			}
			if filename == CONST_Md5_QUEUE_FILE_NAME {
				c.AppendToDownloadQueue(fileInfo)
			} else {
				c.AppendToQueue(fileInfo)
			}
		}
	}
}

func (c *Server) postFileToPeer(fileInfo *FileInfo) {
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
	for i, peer = range Config().Peers {
		_ = i
		if fileInfo.Peers == nil {
			fileInfo.Peers = []string{}
		}
		if c.util.Contains(peer, fileInfo.Peers) {
			continue
		}
		filename = fileInfo.Name
		if fileInfo.ReName != "" {
			filename = fileInfo.ReName
			if fileInfo.OffSet != -1 {
				filename = strings.Split(fileInfo.ReName, ",")[0]
			}
		}
		fpath = DOCKER_DIR + fileInfo.Path + "/" + filename
		if !c.util.FileExists(fpath) {
			log.Warn(fmt.Sprintf("file '%s' not found", fpath))
			continue
		} else {
			if fileInfo.Size == 0 {
				if fi, err = os.Stat(fpath); err != nil {
					log.Error(err)
				} else {
					fileInfo.Size = fi.Size()
				}
			}
		}
		if fileInfo.OffSet != -2 && Config().EnableDistinctFile {
			//not migrate file should check or update file
			// where not EnableDistinctFile should check
			if info, err = c.checkPeerFileExist(peer, fileInfo.Md5, ""); info.Md5 != "" {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				if _, err = c.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, c.ldb); err != nil {
					log.Error(err)
				}
				continue
			}
		}
		postURL = fmt.Sprintf("%s%s", peer, c.getRequestURI("syncfile_info"))
		b := httplib.Post(postURL)
		b.SetTimeout(time.Second*30, time.Second*30)
		if data, err = json.Marshal(fileInfo); err != nil {
			log.Error(err)
			return
		}
		b.Param("fileInfo", string(data))
		result, err = b.String()
		if err != nil {
			if fileInfo.retry <= Config().RetryCount {
				fileInfo.retry = fileInfo.retry + 1
				c.AppendToQueue(fileInfo)
			}
			log.Error(err, fmt.Sprintf(" path:%s", fileInfo.Path+"/"+fileInfo.Name))
		}
		if !strings.HasPrefix(result, "http://") || err != nil {
			c.SaveFileMd5Log(fileInfo, CONST_Md5_ERROR_FILE_NAME)
		}
		if strings.HasPrefix(result, "http://") {
			log.Info(result)
			if !c.util.Contains(peer, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				if _, err = c.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, c.ldb); err != nil {
					log.Error(err)
				}
			}
		}
		if err != nil {
			log.Error(err)
		}
	}
}

func (c *Server) SaveFileMd5Log(fileInfo *FileInfo, filename string) {
	var (
		info FileInfo
	)
	for len(c.queueFileLog)+len(c.queueFileLog)/10 > CONST_QUEUE_SIZE {
		time.Sleep(time.Second * 1)
	}
	info = *fileInfo
	c.queueFileLog <- &FileLog{FileInfo: &info, FileName: filename}
}

func (c *Server) saveFileMd5Log(fileInfo *FileInfo, filename string) {
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
	logDate = c.util.GetDayFromTimeStamp(fileInfo.TimeStamp)
	outname = fileInfo.Name
	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}
	fullpath = fileInfo.Path + "/" + outname
	logKey = fmt.Sprintf("%s_%s_%s", logDate, filename, fileInfo.Md5)
	if filename == CONST_FILE_Md5_FILE_NAME {
		//c.searchMap.Put(fileInfo.Md5, fileInfo.Name)
		if ok, err = c.IsExistFromLevelDB(fileInfo.Md5, c.ldb); !ok {
			c.statMap.AddCountInt64(logDate+"_"+CONST_STAT_FILE_COUNT_KEY, 1)
			c.statMap.AddCountInt64(logDate+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY, fileInfo.Size)
			c.SaveStat()
		}
		if _, err = c.SaveFileInfoToLevelDB(logKey, fileInfo, c.logDB); err != nil {
			log.Error(err)
		}
		if _, err := c.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, c.ldb); err != nil {
			log.Error("saveToLevelDB", err, fileInfo)
		}
		if _, err = c.SaveFileInfoToLevelDB(c.util.MD5(fullpath), fileInfo, c.ldb); err != nil {
			log.Error("saveToLevelDB", err, fileInfo)
		}
		return
	}
	if filename == CONST_REMOME_Md5_FILE_NAME {
		//c.searchMap.Remove(fileInfo.Md5)
		if ok, err = c.IsExistFromLevelDB(fileInfo.Md5, c.ldb); ok {
			c.statMap.AddCountInt64(logDate+"_"+CONST_STAT_FILE_COUNT_KEY, -1)
			c.statMap.AddCountInt64(logDate+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY, -fileInfo.Size)
			c.SaveStat()
		}
		c.RemoveKeyFromLevelDB(logKey, c.logDB)
		md5Path = c.util.MD5(fullpath)
		if err := c.RemoveKeyFromLevelDB(fileInfo.Md5, c.ldb); err != nil {
			log.Error("RemoveKeyFromLevelDB", err, fileInfo)
		}
		if err = c.RemoveKeyFromLevelDB(md5Path, c.ldb); err != nil {
			log.Error("RemoveKeyFromLevelDB", err, fileInfo)
		}
		// remove files.md5 for stat info(repair from logDB)
		logKey = fmt.Sprintf("%s_%s_%s", logDate, CONST_FILE_Md5_FILE_NAME, fileInfo.Md5)
		c.RemoveKeyFromLevelDB(logKey, c.logDB)
		return
	}
	c.SaveFileInfoToLevelDB(logKey, fileInfo, c.logDB)
}

func (c *Server) checkPeerFileExist(peer string, md5sum string, fpath string) (*FileInfo, error) {
	var (
		err      error
		fileInfo FileInfo
	)
	req := httplib.Post(fmt.Sprintf("%s%s?md5=%s", peer, c.getRequestURI("check_file_exist"), md5sum))
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

func (c *Server) IsExistFromLevelDB(key string, db *leveldb.DB) (bool, error) {
	return db.Has([]byte(key), nil)
}

func (c *Server) GetFileInfoFromLevelDB(key string) (*FileInfo, error) {
	var (
		err      error
		data     []byte
		fileInfo FileInfo
	)
	if data, err = c.ldb.Get([]byte(key), nil); err != nil {
		return nil, err
	}
	if err = json.Unmarshal(data, &fileInfo); err != nil {
		return nil, err
	}
	return &fileInfo, nil
}

func (c *Server) SaveStat() {
	SaveStatFunc := func() {
		defer func() {
			if re := recover(); re != nil {
				buffer := debug.Stack()
				log.Error("SaveStatFunc")
				log.Error(re)
				log.Error(string(buffer))
			}
		}()
		stat := c.statMap.Get()
		if v, ok := stat[CONST_STAT_FILE_COUNT_KEY]; ok {
			switch v.(type) {
			case int64, int32, int, float64, float32:
				if v.(int64) >= 0 {
					if data, err := json.Marshal(stat); err != nil {
						log.Error(err)
					} else {
						c.util.WriteBinFile(CONST_STAT_FILE_NAME, data)
					}
				}
			}
		}
	}
	SaveStatFunc()
}

func (c *Server) RemoveKeyFromLevelDB(key string, db *leveldb.DB) error {
	var (
		err error
	)
	err = db.Delete([]byte(key), nil)
	return err
}

func (c *Server) SaveFileInfoToLevelDB(key string, fileInfo *FileInfo, db *leveldb.DB) (*FileInfo, error) {
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
	if db == c.ldb { //search slow ,write fast, double write logDB
		logDate := c.util.GetDayFromTimeStamp(fileInfo.TimeStamp)
		logKey := fmt.Sprintf("%s_%s_%s", logDate, CONST_FILE_Md5_FILE_NAME, fileInfo.Md5)
		c.logDB.Put([]byte(logKey), data, nil)
	}
	return fileInfo, nil
}

func (c *Server) IsPeer(r *http.Request) bool {
	var (
		ip    string
		peer  string
		bflag bool
		cidr  *net.IPNet
		err   error
	)
	IsPublicIP := func(IP net.IP) bool {
		if IP.IsLoopback() || IP.IsLinkLocalMulticast() || IP.IsLinkLocalUnicast() {
			return false
		}
		if ip4 := IP.To4(); ip4 != nil {
			switch true {
			case ip4[0] == 10:
				return false
			case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
				return false
			case ip4[0] == 192 && ip4[1] == 168:
				return false
			default:
				return true
			}
		}
		return false
	}
	//return true
	ip = c.util.GetClientIp(r)
	if c.util.Contains("0.0.0.0", Config().AdminIps) {
		if IsPublicIP(net.ParseIP(ip)) {
			return false
		}
		return true
	}
	if c.util.Contains(ip, Config().AdminIps) {
		return true
	}
	for _, v := range Config().AdminIps {
		if strings.Contains(v, "/") {
			if _, cidr, err = net.ParseCIDR(v); err != nil {
				log.Error(err)
				return false
			}
			if cidr.Contains(net.ParseIP(ip)) {
				return true
			}
		}
	}
	realIp := os.Getenv("GO_FASTDFS_IP")
	if realIp == "" {
		realIp = c.util.GetPulicIP()
	}
	if ip == "127.0.0.1" || ip == realIp {
		return true
	}
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

func (c *Server) GetMd5sMapByDate(date string, filename string) (*goutil.CommonMap, error) {
	var (
		err     error
		result  *goutil.CommonMap
		fpath   string
		content string
		lines   []string
		line    string
		cols    []string
		data    []byte
	)
	result = goutil.NewCommonMap(0)
	if filename == "" {
		fpath = DATA_DIR + "/" + date + "/" + CONST_FILE_Md5_FILE_NAME
	} else {
		fpath = DATA_DIR + "/" + date + "/" + filename
	}
	if !c.util.FileExists(fpath) {
		return result, errors.New(fmt.Sprintf("fpath %s not found", fpath))
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

func (c *Server) GetMd5sByDate(date string, filename string) (mapset.Set, error) {
	var (
		keyPrefix string
		md5set    mapset.Set
		keys      []string
	)
	md5set = mapset.NewSet()
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, filename)
	iter := server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
	for iter.Next() {
		keys = strings.Split(string(iter.Key()), "_")
		if len(keys) >= 3 {
			md5set.Add(keys[2])
		}
	}
	iter.Release()
	return md5set, nil
}

func (c *Server) getRequestURI(action string) string {
	var (
		uri string
	)
	if Config().SupportGroupManage {
		uri = "/" + Config().Group + "/" + action
	} else {
		uri = "/" + action
	}
	return uri
}

func (c *Server) BuildFileResult(fileInfo *FileInfo, r *http.Request) FileResult {
	var (
		outname     string
		fileResult  FileResult
		p           string
		downloadUrl string
		domain      string
		host        string
		protocol    string
	)
	if Config().EnableHttps {
		protocol = "https"
	} else {
		protocol = "http"
	}
	host = strings.Replace(Config().Host, "http://", "", -1)
	if r != nil {
		host = r.Host
	}
	if !strings.HasPrefix(Config().DownloadDomain, "http") {
		if Config().DownloadDomain == "" {
			Config().DownloadDomain = fmt.Sprintf("%s://%s", protocol, host)
		} else {
			Config().DownloadDomain = fmt.Sprintf("%s://%s", protocol, Config().DownloadDomain)
		}
	}
	if Config().DownloadDomain != "" {
		domain = Config().DownloadDomain
	} else {
		domain = fmt.Sprintf("%s://%s", protocol, host)
	}
	outname = fileInfo.Name
	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}
	p = strings.Replace(fileInfo.Path, STORE_DIR_NAME+"/", "", 1)
	if Config().SupportGroupManage {
		p = Config().Group + "/" + p + "/" + outname
	} else {
		p = p + "/" + outname
	}
	downloadUrl = fmt.Sprintf("%s://%s/%s", protocol, host, p)
	if Config().DownloadDomain != "" {
		downloadUrl = fmt.Sprintf("%s/%s", Config().DownloadDomain, p)
	}
	fileResult.Url = downloadUrl
	if Config().DefaultDownload {
		fileResult.Url = fmt.Sprintf("%s?name=%s&download=1", downloadUrl, url.PathEscape(outname))
	}
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

func (c *Server) GetStat() []StatDateFileInfo {
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
	for k := range c.statMap.Get() {
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
		if v, ok := c.statMap.GetValue(s + "_" + CONST_STAT_FILE_TOTAL_SIZE_KEY); ok {
			var info StatDateFileInfo
			info.Date = s
			switch v.(type) {
			case int64:
				info.TotalSize = v.(int64)
				total.TotalSize = total.TotalSize + v.(int64)
			}
			if v, ok := c.statMap.GetValue(s + "_" + CONST_STAT_FILE_COUNT_KEY); ok {
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

func (c *Server) RegisterExit() {
	ch := make(chan os.Signal)
	signal.Notify(ch, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	go func() {
		for s := range ch {
			switch s {
			case syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT:
				c.ldb.Close()
				log.Info("Exit", s)
				os.Exit(1)
			}
		}
	}()
}

func (c *Server) AppendToQueue(fileInfo *FileInfo) {

	for (len(c.queueToPeers) + CONST_QUEUE_SIZE/10) > CONST_QUEUE_SIZE {
		time.Sleep(time.Millisecond * 50)
	}
	c.queueToPeers <- *fileInfo
}

func (c *Server) AppendToDownloadQueue(fileInfo *FileInfo) {
	for (len(c.queueFromPeers) + CONST_QUEUE_SIZE/10) > CONST_QUEUE_SIZE {
		time.Sleep(time.Millisecond * 50)
	}
	c.queueFromPeers <- *fileInfo
}

func (c *Server) ConsumerLog() {
	go func() {
		var (
			fileLog *FileLog
		)
		for {
			fileLog = <-c.queueFileLog
			c.saveFileMd5Log(fileLog.FileInfo, fileLog.FileName)
		}
	}()
}

func (c *Server) LoadSearchDict() {
	go func() {
		log.Info("Load search dict ....")
		f, err := os.Open(CONST_SEARCH_FILE_NAME)
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
					c.searchMap.Put(kvs[0], kvs[1])
				}
			}
		}
		log.Info("finish load search dict")
	}()
}

func (c *Server) SaveSearchDict() {
	var (
		err        error
		fp         *os.File
		searchDict map[string]interface{}
		k          string
		v          interface{}
	)
	c.lockMap.LockKey(CONST_SEARCH_FILE_NAME)
	defer c.lockMap.UnLockKey(CONST_SEARCH_FILE_NAME)
	searchDict = c.searchMap.Get()
	fp, err = os.OpenFile(CONST_SEARCH_FILE_NAME, os.O_RDWR, 0755)
	if err != nil {
		log.Error(err)
		return
	}
	defer fp.Close()
	for k, v = range searchDict {
		fp.WriteString(fmt.Sprintf("%s\t%s", k, v.(string)))
	}
}

func (c *Server) ConsumerPostToPeer() {
	ConsumerFunc := func() {
		for {
			fileInfo := <-c.queueToPeers
			c.postFileToPeer(&fileInfo)
		}
	}
	for i := 0; i < Config().SyncWorker; i++ {
		go ConsumerFunc()
	}
}

func (c *Server) ConsumerUpload() {
	ConsumerFunc := func() {
		for {
			wr := <-c.queueUpload
			c.upload(*wr.w, wr.r)
			c.rtMap.AddCountInt64(CONST_UPLOAD_COUNTER_KEY, wr.r.ContentLength)
			if v, ok := c.rtMap.GetValue(CONST_UPLOAD_COUNTER_KEY); ok {
				if v.(int64) > 1*1024*1024*1024 {
					var _v int64
					c.rtMap.Put(CONST_UPLOAD_COUNTER_KEY, _v)
					debug.FreeOSMemory()
				}
			}
			wr.done <- true
		}
	}
	for i := 0; i < Config().UploadWorker; i++ {
		go ConsumerFunc()
	}
}

func (c *Server) CleanLogLevelDBByDate(date string, filename string) {
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
	iter := server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
	for iter.Next() {
		keys.Add(string(iter.Value()))
	}
	iter.Release()
	for key := range keys.Iter() {
		err = c.RemoveKeyFromLevelDB(key.(string), c.logDB)
		if err != nil {
			log.Error(err)
		}
	}
}

func (c *Server) CleanAndBackUp() {
	Clean := func() {
		var (
			filenames []string
			yesterday string
		)
		if c.curDate != c.util.GetToDay() {
			filenames = []string{CONST_Md5_QUEUE_FILE_NAME, CONST_Md5_ERROR_FILE_NAME, CONST_REMOME_Md5_FILE_NAME}
			yesterday = c.util.GetDayFromTimeStamp(time.Now().AddDate(0, 0, -1).Unix())
			for _, filename := range filenames {
				c.CleanLogLevelDBByDate(yesterday, filename)
			}
			c.BackUpMetaDataByDate(yesterday)
			c.curDate = c.util.GetToDay()
		}
	}
	go func() {
		for {
			time.Sleep(time.Hour * 6)
			Clean()
		}
	}()
}

func (c *Server) LoadFileInfoByDate(date string, filename string) (mapset.Set, error) {
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
	iter := server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
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

func (c *Server) LoadQueueSendToPeer() {
	if queue, err := c.LoadFileInfoByDate(c.util.GetToDay(), CONST_Md5_QUEUE_FILE_NAME); err != nil {
		log.Error(err)
	} else {
		for fileInfo := range queue.Iter() {
			//c.queueFromPeers <- *fileInfo.(*FileInfo)
			c.AppendToDownloadQueue(fileInfo.(*FileInfo))
		}
	}
}

func (c *Server) CheckClusterStatus() {
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
			data    []byte
		)
		for _, peer := range Config().Peers {
			req = httplib.Get(fmt.Sprintf("%s%s", peer, c.getRequestURI("status")))
			req.SetTimeout(time.Second*5, time.Second*5)
			err = req.ToJSON(&status)
			if err != nil || status.Status != "ok" {
				for _, to := range Config().AlarmReceivers {
					subject = "fastdfs server error"
					if err != nil {
						body = fmt.Sprintf("%s\nserver:%s\nerror:\n%s", subject, peer, err.Error())
					} else {
						body = fmt.Sprintf("%s\nserver:%s\n", subject, peer)
					}
					if err = c.SendToMail(to, subject, body, "text"); err != nil {
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
				log.Error(err)
			} else {
				var statusMap map[string]interface{}
				if data, err = json.Marshal(status.Data); err != nil {
					log.Error(err)
					return
				}
				if err = json.Unmarshal(data, &statusMap); err != nil {
					log.Error(err)
				}
				if v, ok := statusMap["Fs.PeerId"]; ok {
					if v == Config().PeerId {
						log.Error(fmt.Sprintf("PeerId is confict:%s", v))
					}
				}
				if v, ok := statusMap["Fs.Local"]; ok {
					if v == Config().Host {
						log.Error(fmt.Sprintf("Host is confict:%s", v))
					}
				}
			}
		}
	}
	check()
	go func() {
		for {
			time.Sleep(time.Minute * 10)
			check()
		}
	}()
}

func (c *Server) SearchDict(kw string) []FileInfo {
	var (
		fileInfos []FileInfo
		fileInfo  *FileInfo
	)
	for dict := range c.searchMap.Iter() {
		if strings.Contains(dict.Val.(string), kw) {
			if fileInfo, _ = c.GetFileInfoFromLevelDB(dict.Key); fileInfo != nil {
				fileInfos = append(fileInfos, *fileInfo)
			}
		}
	}
	return fileInfos
}

func (c *Server) test() {

	testLock := func() {
		wg := sync.WaitGroup{}
		tt := func(i int, wg *sync.WaitGroup) {
			//if server.lockMap.IsLock("xx") {
			//	return
			//}
			//fmt.Println("timeer len",len(server.lockMap.Get()))
			//time.Sleep(time.Nanosecond*10)
			server.lockMap.LockKey("xx")
			defer server.lockMap.UnLockKey("xx")
			//time.Sleep(time.Nanosecond*1)
			//fmt.Println("xx", i)
			wg.Done()
		}
		go func() {
			for {
				time.Sleep(time.Second * 1)
				fmt.Println("timeer len", len(server.lockMap.Get()), server.lockMap.Get())
			}
		}()
		fmt.Println(len(server.lockMap.Get()))
		for i := 0; i < 10000; i++ {
			wg.Add(1)
			go tt(i, &wg)
		}
		fmt.Println(len(server.lockMap.Get()))
		fmt.Println(len(server.lockMap.Get()))
		server.lockMap.LockKey("abc")
		fmt.Println("lock")
		time.Sleep(time.Second * 5)
		server.lockMap.UnLockKey("abc")
		server.lockMap.LockKey("abc")
		server.lockMap.UnLockKey("abc")
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

func (c *Server) FormatStatInfo() {
	var (
		data  []byte
		err   error
		count int64
		stat  map[string]interface{}
	)
	if c.util.FileExists(CONST_STAT_FILE_NAME) {
		if data, err = c.util.ReadBinFile(CONST_STAT_FILE_NAME); err != nil {
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
							c.statMap.Put(k, count)
						}
					default:
						c.statMap.Put(k, v)
					}
				}
			}
		}
	} else {
		c.RepairStatByDate(c.util.GetToDay())
	}
}
