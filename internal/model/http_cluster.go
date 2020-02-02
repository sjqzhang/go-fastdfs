package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	mapSet "github.com/deckarep/golang-set"
	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/pkg"
	log "github.com/sirupsen/logrus"
)

// IsPeer check the host that create the request is in the peers
func IsPeer(r *http.Request, conf *config.Config) bool {
	var (
		ip    string
		peer  string
		bflag bool
	)
	//return true
	ip = pkg.GetClientIp(r)
	realIp := os.Getenv("GO_FASTDFS_IP")
	if realIp == "" {
		realIp = pkg.GetPublicIP()
	}
	if ip == "127.0.0.1" || ip == realIp {
		return true
	}
	if pkg.Contains(ip, conf.AdminIps()) {
		return true
	}
	ip = "http://" + ip
	bflag = false
	for _, peer = range conf.Peers() {
		if strings.HasPrefix(peer, ip) {
			bflag = true
			break
		}
	}

	return bflag
}

// CheckClusterStatus
func CheckClusterStatus(conf *config.Config) {
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
		for _, peer := range conf.Peers() {
			req = httplib.Get(peer + GetRequestURI("status"))
			req.SetTimeout(time.Second*5, time.Second*5)
			err = req.ToJSON(&status)
			if err != nil || status.Status != "ok" {
				for _, to := range conf.AlarmReceivers() {
					subject = "fastdfs server error"
					if err != nil {
						body = fmt.Sprintf("%s\nserver:%s\nerror:\n%s", subject, peer, err.Error())
					} else {
						body = fmt.Sprintf("%s\nserver:%s\n", subject, peer)
					}
					if err = SendMail(to, subject, body, "text", conf); err != nil {
						log.Error(err)
					}
				}
				if conf.AlarmUrl() != "" {
					req = httplib.Post(conf.AlarmUrl())
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

// GetClusterNotPermitMessage
func GetClusterNotPermitMessage(r *http.Request) string {
	return fmt.Sprintf(config.MessageClusterIp, pkg.GetClientIp(r))
}

// checkPeerFileExist
func checkPeerFileExist(peer string, md5sum string, fpath string) (*FileInfo, error) {
	var (
		err      error
		fileInfo FileInfo
	)
	req := httplib.Post(fmt.Sprintf("%s%s?md5=%s", peer, GetRequestURI("check_file_exist"), md5sum))
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

func (svr *Server) DownloadFromPeer(peer string, fileInfo *FileInfo, conf *config.Config) {
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
	if conf.ReadOnly() {
		log.Warn("ReadOnly", fileInfo)
		return
	}
	if conf.RetryCount() > 0 && fileInfo.retry >= conf.RetryCount() {
		log.Error("DownloadFromPeer Error ", fileInfo)
		return
	} else {
		fileInfo.retry = fileInfo.retry + 1
	}
	filename = fileInfo.Name
	if fileInfo.ReName != "" {
		filename = fileInfo.ReName
	}
	if fileInfo.OffSet != -2 && conf.EnableDistinctFile() && svr.CheckFileExistByInfo(fileInfo.Md5, fileInfo, conf) {
		// ignore migrate file
		log.Info(fmt.Sprintf("DownloadFromPeer file Exist, path:%s", fileInfo.Path+"/"+fileInfo.Name))
		return
	}
	if (!conf.EnableDistinctFile() || fileInfo.OffSet == -2) && pkg.FileExists(GetFilePathByInfo(fileInfo, true)) {
		// ignore migrate file
		if fi, err = os.Stat(GetFilePathByInfo(fileInfo, true)); err == nil {
			if fi.ModTime().Unix() > fileInfo.TimeStamp {
				log.Info(fmt.Sprintf("ignore file sync path:%s", GetFilePathByInfo(fileInfo, false)))
				fileInfo.TimeStamp = fi.ModTime().Unix()
				svr.postFileToPeer(fileInfo, conf) // keep newer
				return
			}
			os.Remove(GetFilePathByInfo(fileInfo, true))
		}
	}
	if _, err = os.Stat(fileInfo.Path); err != nil {
		os.MkdirAll(fileInfo.Path, 0775)
	}
	//fmt.Println("downloadFromPeer",fileInfo)
	p := strings.Replace(fileInfo.Path, conf.StoreDir()+"/", "", 1)
	//filename=util.UrlEncode(filename)
	downloadUrl = peer + "/" + p + "/" + filename
	log.Info("DownloadFromPeer: ", downloadUrl)
	fpath = fileInfo.Path + "/" + filename
	fpathTmp = fileInfo.Path + "/" + fmt.Sprintf("%s_%s", "tmp_", filename)
	timeout := fileInfo.Size/1024/1024/1 + 30
	if conf.SyncTimeout() > 0 {
		timeout = conf.SyncTimeout()
	}
	svr.lockMap.LockKey(fpath)
	defer svr.lockMap.UnLockKey(fpath)
	downloadKey := fmt.Sprintf("downloading_%d_%s", time.Now().Unix(), fpath)
	conf.LevelDB().Put([]byte(downloadKey), []byte(""), nil)
	defer func() {
		conf.LevelDB().Delete([]byte(downloadKey), nil)
	}()
	if fileInfo.OffSet == -2 {
		//migrate file
		if fi, err = os.Stat(fpath); err == nil && fi.Size() == fileInfo.Size {
			//prevent double download
			svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, conf.LevelDB(), conf)
			//log.Info(fmt.Sprintf("file '%s' has download", fpath))
			return
		}
		req := httplib.Get(downloadUrl)
		req.SetTimeout(time.Second*30, time.Second*time.Duration(timeout))
		if err = req.ToFile(fpathTmp); err != nil {
			svr.AppendToDownloadQueue(fileInfo, conf) //retry
			os.Remove(fpathTmp)
			log.Error(err, fpathTmp)
			return
		}
		if os.Rename(fpathTmp, fpath) == nil {
			//svr.SaveFileMd5Log(fileInfo, FileMd5Name)
			svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, conf.LevelDB(), conf)
		}
		return
	}
	req := httplib.Get(downloadUrl)
	req.SetTimeout(time.Second*30, time.Second*time.Duration(timeout))
	if fileInfo.OffSet >= 0 {
		//small file download
		data, err = req.Bytes()
		if err != nil {
			svr.AppendToDownloadQueue(fileInfo, conf) //retry
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
		err = pkg.WriteFileByOffSet(fpath, fileInfo.OffSet, data)
		if err != nil {
			log.Warn(err)
			return
		}
		svr.SaveFileMd5Log(fileInfo, conf.FileMd5(), conf)
		return
	}
	if err = req.ToFile(fpathTmp); err != nil {
		svr.AppendToDownloadQueue(fileInfo, conf) //retry
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
		svr.SaveFileMd5Log(fileInfo, conf.FileMd5(), conf)
	}
}

func (svr *Server) CheckFileAndSendToPeer(date string, filename string, isForceUpload bool, conf *config.Config) {
	var (
		md5set mapSet.Set
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
	if md5set, err = GetMd5sByDate(date, filename, conf); err != nil {
		log.Error(err)
		return
	}
	md5s = md5set.ToSlice()
	for _, md := range md5s {
		if md == nil {
			continue
		}
		if fileInfo, _ := GetFileInfoFromLevelDB(md.(string), conf); fileInfo != nil && fileInfo.Md5 != "" {
			if isForceUpload {
				fileInfo.Peers = []string{}
			}
			if len(fileInfo.Peers) > len(conf.Peers()) {
				continue
			}
			if !pkg.Contains(svr.host, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, svr.host) // peer is null
			}
			if filename == conf.Md5QueueFile() {
				svr.AppendToDownloadQueue(fileInfo, conf)
				continue
			}
			svr.AppendToQueue(fileInfo, conf)
		}
	}
}

func (svr *Server) postFileToPeer(fileInfo *FileInfo, conf *config.Config) {
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
	for i, peer = range conf.Peers() {
		_ = i
		if fileInfo.Peers == nil {
			fileInfo.Peers = []string{}
		}
		if pkg.Contains(peer, fileInfo.Peers) {
			continue
		}
		filename = fileInfo.Name
		if fileInfo.ReName != "" {
			filename = fileInfo.ReName
			if fileInfo.OffSet != -1 {
				filename = strings.Split(fileInfo.ReName, ",")[0]
			}
		}
		fpath = fileInfo.Path + "/" + filename
		if !pkg.FileExists(fpath) {
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

		if fileInfo.OffSet != -2 && conf.EnableDistinctFile() {
			//not migrate file should check or update file
			// where not EnableDistinctFile should check
			if info, err = checkPeerFileExist(peer, fileInfo.Md5, ""); info.Md5 != "" {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				if _, err = svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, conf.LevelDB(), conf); err != nil {
					log.Error(err)
				}
				continue
			}
		}
		postURL = peer + GetRequestURI("syncfile_info")
		b := httplib.Post(postURL)
		b.SetTimeout(time.Second*30, time.Second*30)
		if data, err = json.Marshal(fileInfo); err != nil {
			log.Error(err)
			return
		}
		b.Param("fileInfo", string(data))
		result, err = b.String()
		if err != nil {
			if fileInfo.retry <= conf.RetryCount() {
				fileInfo.retry = fileInfo.retry + 1
				svr.AppendToQueue(fileInfo, conf)
			}
			log.Error(err, fmt.Sprintf(" path:%s", fileInfo.Path+"/"+fileInfo.Name))
		}
		if !strings.HasPrefix(result, "http://") || err != nil {
			log.Error(err)
			svr.SaveFileMd5Log(fileInfo, conf.Md5ErrorFile(), conf)
		}
		if strings.HasPrefix(result, "http://") {
			log.Info(result)
			if !pkg.Contains(peer, fileInfo.Peers) {
				fileInfo.Peers = append(fileInfo.Peers, peer)
				if _, err = svr.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, conf.LevelDB(), conf); err != nil {
					log.Error(err)
				}
			}
		}
	}
}

func (svr *Server) Sync(path string, router *gin.RouterGroup, conf *config.Config) {
	router.GET(path, func(ctx *gin.Context) {
		var result JsonResult

		r := ctx.Request
		r.ParseForm()
		result.Status = "fail"
		if !IsPeer(r, conf) {
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
			for _, peer := range conf.Peers() {
				req := httplib.Post(peer + GetRequestURI("sync"))
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
			go svr.CheckFileAndSendToPeer(date, conf.FileMd5Name(), isForceUpload, conf)
		} else {
			go svr.CheckFileAndSendToPeer(date, conf.Md5ErrorFile(), isForceUpload, conf)
		}
		result.Status = "ok"
		result.Message = "job is running"
		ctx.JSON(http.StatusOK, result)
	})
}

func (svr *Server) ConsumerPostToPeer(conf *config.Config) {
	ConsumerFunc := func() {
		for fileInfo := range svr.queueToPeers {
			svr.postFileToPeer(&fileInfo, conf)
		}
	}
	for i := 0; i < conf.SyncWorker(); i++ {
		go ConsumerFunc()
	}
}
