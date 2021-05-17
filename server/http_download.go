package server

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	log "github.com/sjqzhang/seelog"
)

func (c *Server) SetDownloadHeader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment")
	if name, ok := r.URL.Query()["name"]; ok {
		if v, err := url.QueryUnescape(name[0]); err == nil {
			name[0] = v
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s", name[0]))
	}
}

func (c *Server) ConsumerDownLoad() {
	ConsumerFunc := func() {
		for {
			fileInfo := <-c.queueFromPeers
			if len(fileInfo.Peers) <= 0 {
				log.Warn("Peer is null", fileInfo)
				continue
			}
			for _, peer := range fileInfo.Peers {
				if strings.Contains(peer, "127.0.0.1") {
					log.Warn("sync error with 127.0.0.1", fileInfo)
					continue
				}
				if peer != c.host {
					c.DownloadFromPeer(peer, &fileInfo)
					break
				}
			}
		}
	}
	for i := 0; i < Config().SyncWorker; i++ {
		go ConsumerFunc()
	}
}

func (c *Server) DownloadFromPeer(peer string, fileInfo *FileInfo) {
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
	if Config().ReadOnly {
		log.Warn("ReadOnly", fileInfo)
		return
	}
	if Config().RetryCount > 0 && fileInfo.retry >= Config().RetryCount {
		log.Error("DownloadFromPeer Error ", fileInfo)
		return
	} else {
		fileInfo.retry = fileInfo.retry + 1
	}
	filename = fileInfo.Name
	if fileInfo.ReName != "" {
		filename = fileInfo.ReName
	}
	if fileInfo.OffSet != -2 && Config().EnableDistinctFile && c.CheckFileExistByInfo(fileInfo.Md5, fileInfo) {
		// ignore migrate file
		log.Info(fmt.Sprintf("DownloadFromPeer file Exist, path:%s", fileInfo.Path+"/"+fileInfo.Name))
		return
	}
	if (!Config().EnableDistinctFile || fileInfo.OffSet == -2) && c.util.FileExists(c.GetFilePathByInfo(fileInfo, true)) {
		// ignore migrate file
		if fi, err = os.Stat(c.GetFilePathByInfo(fileInfo, true)); err == nil {
			if fi.ModTime().Unix() > fileInfo.TimeStamp {
				log.Info(fmt.Sprintf("ignore file sync path:%s", c.GetFilePathByInfo(fileInfo, false)))
				fileInfo.TimeStamp = fi.ModTime().Unix()
				c.postFileToPeer(fileInfo) // keep newer
				return
			}
			os.Remove(c.GetFilePathByInfo(fileInfo, true))
		}
	}
	if _, err = os.Stat(fileInfo.Path); err != nil {
		os.MkdirAll(DOCKER_DIR+fileInfo.Path, 0775)
	}
	//fmt.Println("downloadFromPeer",fileInfo)
	p := strings.Replace(fileInfo.Path, STORE_DIR_NAME+"/", "", 1)
	//filename=c.util.UrlEncode(filename)
	if Config().SupportGroupManage {
		downloadUrl = peer + "/" + Config().Group + "/" + p + "/" + filename
	} else {
		downloadUrl = peer + "/" + p + "/" + filename
	}
	log.Info("DownloadFromPeer: ", downloadUrl)
	fpath = DOCKER_DIR + fileInfo.Path + "/" + filename
	fpathTmp = DOCKER_DIR + fileInfo.Path + "/" + fmt.Sprintf("%s_%s", "tmp_", filename)
	timeout := fileInfo.Size/1024/1024/1 + 30
	if Config().SyncTimeout > 0 {
		timeout = Config().SyncTimeout
	}
	c.lockMap.LockKey(fpath)
	defer c.lockMap.UnLockKey(fpath)
	download_key := fmt.Sprintf("downloading_%d_%s", time.Now().Unix(), fpath)
	c.ldb.Put([]byte(download_key), []byte(""), nil)
	defer func() {
		c.ldb.Delete([]byte(download_key), nil)
	}()
	if fileInfo.OffSet == -2 {
		//migrate file
		if fi, err = os.Stat(fpath); err == nil && fi.Size() == fileInfo.Size {
			//prevent double download
			c.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, c.ldb)
			//log.Info(fmt.Sprintf("file '%s' has download", fpath))
			return
		}
		req := httplib.Get(downloadUrl)
		req.SetTimeout(time.Second*30, time.Second*time.Duration(timeout))
		if err = req.ToFile(fpathTmp); err != nil {
			c.AppendToDownloadQueue(fileInfo) //retry
			os.Remove(fpathTmp)
			log.Error(err, fpathTmp)
			return
		}
		if fi, err = os.Stat(fpathTmp); err != nil {
			os.Remove(fpathTmp)
			return
		}
		if fi.Size() != fileInfo.Size {
			log.Error("file size check error")
			os.Remove(fpathTmp)
		}
		if os.Rename(fpathTmp, fpath) == nil {
			//c.SaveFileMd5Log(fileInfo, CONST_FILE_Md5_FILE_NAME)
			c.SaveFileInfoToLevelDB(fileInfo.Md5, fileInfo, c.ldb)
		}
		return
	}
	req := httplib.Get(downloadUrl)
	req.SetTimeout(time.Second*30, time.Second*time.Duration(timeout))
	if fileInfo.OffSet >= 0 {
		//small file download
		data, err = req.Bytes()
		if err != nil {
			c.AppendToDownloadQueue(fileInfo) //retry
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
		err = c.util.WriteFileByOffSet(fpath, fileInfo.OffSet, data)
		if err != nil {
			log.Warn(err)
			return
		}
		c.SaveFileMd5Log(fileInfo, CONST_FILE_Md5_FILE_NAME)
		return
	}
	if err = req.ToFile(fpathTmp); err != nil {
		c.AppendToDownloadQueue(fileInfo) //retry
		os.Remove(fpathTmp)
		log.Error(err)
		return
	}
	if fi, err = os.Stat(fpathTmp); err != nil {
		os.Remove(fpathTmp)
		return
	}
	_ = sum
	//if Config().EnableDistinctFile {
	//	//DistinctFile
	//	if sum, err = c.util.GetFileSumByName(fpathTmp, Config().FileSumArithmetic); err != nil {
	//		log.Error(err)
	//		return
	//	}
	//} else {
	//	//DistinctFile By path
	//	sum = c.util.MD5(c.GetFilePathByInfo(fileInfo, false))
	//}
	if fi.Size() != fileInfo.Size { //  maybe has bug remove || sum != fileInfo.Md5
		log.Error("file sum check error")
		os.Remove(fpathTmp)
		return
	}
	if os.Rename(fpathTmp, fpath) == nil {
		c.SaveFileMd5Log(fileInfo, CONST_FILE_Md5_FILE_NAME)
	}
}

func (c *Server) CheckDownloadAuth(w http.ResponseWriter, r *http.Request) (bool, error) {
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
	CheckToken := func(token string, md5sum string, timestamp string) bool {
		if c.util.MD5(md5sum+timestamp) != token {
			return false
		}
		return true
	}
	if Config().EnableDownloadAuth && Config().AuthUrl != "" && !c.IsPeer(r) && !c.CheckAuth(w, r) {
		return false, errors.New("auth fail")
	}
	if Config().DownloadUseToken && !c.IsPeer(r) {
		token = r.FormValue("token")
		timestamp = r.FormValue("timestamp")
		if token == "" || timestamp == "" {
			return false, errors.New("unvalid request")
		}
		maxTimestamp = time.Now().Add(time.Second *
			time.Duration(Config().DownloadTokenExpire)).Unix()
		minTimestamp = time.Now().Add(-time.Second *
			time.Duration(Config().DownloadTokenExpire)).Unix()
		if ts, err = strconv.ParseInt(timestamp, 10, 64); err != nil {
			return false, errors.New("unvalid timestamp")
		}
		if ts > maxTimestamp || ts < minTimestamp {
			return false, errors.New("timestamp expire")
		}
		fullpath, smallPath = c.GetFilePathFromRequest(w, r)
		if smallPath != "" {
			pathMd5 = c.util.MD5(smallPath)
		} else {
			pathMd5 = c.util.MD5(fullpath)
		}
		if fileInfo, err = c.GetFileInfoFromLevelDB(pathMd5); err != nil {
			// TODO
		} else {
			ok := CheckToken(token, fileInfo.Md5, timestamp)
			if !ok {
				return ok, errors.New("unvalid token")
			}
			return ok, nil
		}
	}
	if Config().EnableGoogleAuth && !c.IsPeer(r) {
		fullpath = r.RequestURI[len(Config().Group)+2 : len(r.RequestURI)]
		fullpath = strings.Split(fullpath, "?")[0] // just path
		scene = strings.Split(fullpath, "/")[0]
		code = r.FormValue("code")
		if secret, ok = c.sceneMap.GetValue(scene); ok {
			if !c.VerifyGoogleCode(secret.(string), code, int64(Config().DownloadTokenExpire/30)) {
				return false, errors.New("invalid google code")
			}
		}
	}
	return true, nil
}

func (c *Server) DownloadSmallFileByURI(w http.ResponseWriter, r *http.Request) (bool, error) {
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
	r.ParseForm()
	isDownload = true
	if r.FormValue("download") == "" {
		isDownload = Config().DefaultDownload
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
		if imgWidth > Config().ImageMaxWidth {
			imgWidth = Config().ImageMaxWidth
		}
	}
	if height != "" {
		imgHeight, err = strconv.Atoi(height)
		if err != nil {
			log.Error(err)
		}
		if imgHeight > Config().ImageMaxHeight {
			imgHeight = Config().ImageMaxHeight
		}
	}
	data, notFound, err = c.GetSmallFileByURI(w, r)
	_ = notFound
	if data != nil && string(data[0]) == "1" {
		if isDownload {
			c.SetDownloadHeader(w, r)
		}
		if imgWidth != 0 || imgHeight != 0 {
			c.ResizeImageByBytes(w, data[1:], uint(imgWidth), uint(imgHeight))
			return true, nil
		}
		w.Write(data[1:])
		return true, nil
	}
	return false, errors.New("not found")
}

func (c *Server) DownloadNormalFileByURI(w http.ResponseWriter, r *http.Request) (bool, error) {
	var (
		err        error
		isDownload bool
		imgWidth   int
		imgHeight  int
		width      string
		height     string
	)
	r.ParseForm()
	isDownload = true
	if r.FormValue("download") == "" {
		isDownload = Config().DefaultDownload
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
		if imgWidth > Config().ImageMaxWidth {
			imgWidth = Config().ImageMaxWidth
		}
	}
	if height != "" {
		imgHeight, err = strconv.Atoi(height)
		if err != nil {
			log.Error(err)
		}
		if imgHeight > Config().ImageMaxHeight {
			imgHeight = Config().ImageMaxHeight
		}
	}
	if isDownload {
		c.SetDownloadHeader(w, r)
	}
	fullpath, _ := c.GetFilePathFromRequest(w, r)
	if imgWidth != 0 || imgHeight != 0 {
		c.ResizeImage(w, fullpath, uint(imgWidth), uint(imgHeight))
		return true, nil
	}
	staticHandler.ServeHTTP(w, r)
	return true, nil
}

func (c *Server) DownloadNotFound(w http.ResponseWriter, r *http.Request) {
	var (
		err        error
		fullpath   string
		smallPath  string
		isDownload bool
		pathMd5    string
		peer       string
		fileInfo   *FileInfo
	)
	fullpath, smallPath = c.GetFilePathFromRequest(w, r)
	isDownload = true
	if r.FormValue("download") == "" {
		isDownload = Config().DefaultDownload
	}
	if r.FormValue("download") == "0" {
		isDownload = false
	}
	if smallPath != "" {
		pathMd5 = c.util.MD5(smallPath)
	} else {
		pathMd5 = c.util.MD5(fullpath)
	}
	for _, peer = range Config().Peers {
		if fileInfo, err = c.checkPeerFileExist(peer, pathMd5, fullpath); err != nil {
			log.Error(err)
			continue
		}
		if fileInfo.Md5 != "" {
			go c.DownloadFromPeer(peer, fileInfo)
			//http.Redirect(w, r, peer+r.RequestURI, 302)
			if isDownload {
				c.SetDownloadHeader(w, r)
			}
			c.DownloadFileToResponse(peer+r.RequestURI, w, r)
			return
		}
	}
	w.WriteHeader(404)
	return
}

func (c *Server) Download(w http.ResponseWriter, r *http.Request) {
	var (
		err       error
		ok        bool
		fullpath  string
		smallPath string
		fi        os.FileInfo
	)
	// redirect to upload
	if r.RequestURI == "/" || r.RequestURI == "" ||
		r.RequestURI == "/"+Config().Group ||
		r.RequestURI == "/"+Config().Group+"/" {
		c.Index(w, r)
		return
	}
	if ok, err = c.CheckDownloadAuth(w, r); !ok {
		log.Error(err)
		c.NotPermit(w, r)
		return
	}

	if Config().EnableCrossOrigin {
		c.CrossOrigin(w, r)
	}
	fullpath, smallPath = c.GetFilePathFromRequest(w, r)
	if smallPath == "" {
		if fi, err = os.Stat(fullpath); err != nil {
			c.DownloadNotFound(w, r)
			return
		}
		if !Config().ShowDir && fi.IsDir() {
			w.Write([]byte("list dir deny"))
			return
		}
		//staticHandler.ServeHTTP(w, r)
		c.DownloadNormalFileByURI(w, r)
		return
	}
	if smallPath != "" {
		if ok, err = c.DownloadSmallFileByURI(w, r); !ok {
			c.DownloadNotFound(w, r)
			return
		}
		return
	}

}

func (c *Server) DownloadFileToResponse(url string, w http.ResponseWriter, r *http.Request) {
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
		return
	}
	defer resp.Body.Close()
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Error(err)
	}
}
