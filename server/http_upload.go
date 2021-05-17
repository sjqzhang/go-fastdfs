package server

import (
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sjqzhang/seelog"
)

func (c *Server) SyncFileInfo(w http.ResponseWriter, r *http.Request) {
	var (
		err         error
		fileInfo    FileInfo
		fileInfoStr string
		filename    string
	)
	r.ParseForm()
	fileInfoStr = r.FormValue("fileInfo")
	if !c.IsPeer(r) {
		log.Info("isn't peer fileInfo:", fileInfo)
		return
	}
	if err = json.Unmarshal([]byte(fileInfoStr), &fileInfo); err != nil {
		w.Write([]byte(c.GetClusterNotPermitMessage(r)))
		log.Error(err)
		return
	}
	if fileInfo.OffSet == -2 {
		// optimize migrate
		c.SaveFileInfoToLevelDB(fileInfo.Md5, &fileInfo, c.ldb)
	} else {
		c.SaveFileMd5Log(&fileInfo, CONST_Md5_QUEUE_FILE_NAME)
	}
	c.AppendToDownloadQueue(&fileInfo)
	filename = fileInfo.Name
	if fileInfo.ReName != "" {
		filename = fileInfo.ReName
	}
	p := strings.Replace(fileInfo.Path, STORE_DIR+"/", "", 1)
	downloadUrl := fmt.Sprintf("http://%s/%s", r.Host, Config().Group+"/"+p+"/"+filename)
	log.Info("SyncFileInfo: ", downloadUrl)
	w.Write([]byte(downloadUrl))
}

func (c *Server) CheckScene(scene string) (bool, error) {
	var (
		scenes []string
	)
	if len(Config().Scenes) == 0 {
		return true, nil
	}
	for _, s := range Config().Scenes {
		scenes = append(scenes, strings.Split(s, ":")[0])
	}
	if !c.util.Contains(scene, scenes) {
		return false, errors.New("not valid scene")
	}
	return true, nil
}

func (c *Server) Upload(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		fn     string
		folder string
		fpTmp  *os.File
		fpBody *os.File
	)
	if r.Method == http.MethodGet {
		c.upload(w, r)
		return
	}
	folder = STORE_DIR + "/_tmp/" + time.Now().Format("20060102")
	if !c.util.FileExists(folder) {
		if err = os.MkdirAll(folder, 0777); err != nil {
			log.Error(err)
		}
	}
	fn = folder + "/" + c.util.GetUUID()
	fpTmp, err = os.OpenFile(fn, os.O_RDWR|os.O_CREATE, 0777)
	if err != nil {
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}
	if _, err = io.Copy(fpTmp, r.Body); err != nil {
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}
	fpTmp.Close()
	if fpBody, err = os.Open(fn); err != nil {
		log.Error(err)
		w.Write([]byte(err.Error()))
		return
	}
	r.Body = fpBody
	defer func() {
		err = fpBody.Close()
		if err != nil {
			log.Error(err)
		}
		err = os.Remove(fn)
		if err != nil {
			log.Error(err)
		}
	}()
	done := make(chan bool, 1)
	c.queueUpload <- WrapReqResp{&w, r, done}
	<-done

}

func (c *Server) upload(w http.ResponseWriter, r *http.Request) {
	var (
		err error
		ok  bool
		//		pathname     string
		md5sum       string
		fileName     string
		fileInfo     FileInfo
		uploadFile   multipart.File
		uploadHeader *multipart.FileHeader
		scene        string
		output       string
		fileResult   FileResult
		result       JsonResult
		data         []byte
		code         string
		secret       interface{}
		msg          string
	)
	output = r.FormValue("output")
	if Config().EnableCrossOrigin {
		c.CrossOrigin(w, r)
		if r.Method == http.MethodOptions {
			return
		}
	}
	result.Status = "fail"
	if Config().AuthUrl != "" {
		if !c.CheckAuth(w, r) {
			msg = "auth fail"
			log.Warn(msg, r.Form)
			c.NotPermit(w, r)
			result.Message = msg
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
	}
	if r.Method == http.MethodPost {
		md5sum = r.FormValue("md5")
		fileName = r.FormValue("filename")
		output = r.FormValue("output")
		if Config().ReadOnly {
			msg = "(error) readonly"
			result.Message = msg
			log.Warn(msg)
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		if Config().EnableCustomPath {
			fileInfo.Path = r.FormValue("path")
			fileInfo.Path = strings.Trim(fileInfo.Path, "/")
		}
		scene = r.FormValue("scene")
		code = r.FormValue("code")
		if scene == "" {
			//Just for Compatibility
			scene = r.FormValue("scenes")
		}
		if Config().EnableGoogleAuth && scene != "" {
			if secret, ok = c.sceneMap.GetValue(scene); ok {
				if !c.VerifyGoogleCode(secret.(string), code, int64(Config().DownloadTokenExpire/30)) {
					c.NotPermit(w, r)
					msg = "invalid request,error google code"
					result.Message = msg
					log.Error(msg)
					w.Write([]byte(c.util.JsonEncodePretty(result)))
					return
				}
			}
		}
		fileInfo.Md5 = md5sum
		fileInfo.ReName = fileName
		fileInfo.OffSet = -1
		if uploadFile, uploadHeader, err = r.FormFile("file"); err != nil {
			log.Error(err)
			result.Message = err.Error()
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		fileInfo.Peers = []string{}
		fileInfo.TimeStamp = time.Now().Unix()
		if scene == "" {
			scene = Config().DefaultScene
		}
		if output == "" {
			output = "text"
		}
		if !c.util.Contains(output, []string{"json", "text", "json2"}) {
			msg = "output just support json or text or json2"
			result.Message = msg
			log.Warn(msg)
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		fileInfo.Scene = scene
		if _, err = c.CheckScene(scene); err != nil {
			result.Message = err.Error()
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			log.Error(err)
			return
		}
		if err != nil {
			log.Error(err)
			http.Redirect(w, r, "/", http.StatusMovedPermanently)
			return
		}
		if _, err = c.SaveUploadFile(uploadFile, uploadHeader, &fileInfo, r); err != nil {
			result.Message = err.Error()
			log.Error(err)
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		if Config().EnableDistinctFile {
			if v, _ := c.GetFileInfoFromLevelDB(fileInfo.Md5); v != nil && v.Md5 != "" {
				fileResult = c.BuildFileResult(v, r)
				if c.GetFilePathByInfo(&fileInfo, false) != c.GetFilePathByInfo(v, false) {
					os.Remove(c.GetFilePathByInfo(&fileInfo, false))
				}
				if output == "json" || output == "json2" {
					if output == "json2" {
						result.Data = fileResult
						result.Status = "ok"
						w.Write([]byte(c.util.JsonEncodePretty(result)))
						return
					}
					w.Write([]byte(c.util.JsonEncodePretty(fileResult)))
				} else {
					w.Write([]byte(fileResult.Url))
				}
				return
			}
		}
		if fileInfo.Md5 == "" {
			msg = " fileInfo.Md5 is null"
			log.Warn(msg)
			result.Message = msg
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		if md5sum != "" && fileInfo.Md5 != md5sum {
			msg = " fileInfo.Md5 and md5sum !="
			log.Warn(msg)
			result.Message = msg
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		if !Config().EnableDistinctFile {
			// bugfix filecount stat
			fileInfo.Md5 = c.util.MD5(c.GetFilePathByInfo(&fileInfo, false))
		}
		if Config().EnableMergeSmallFile && fileInfo.Size < CONST_SMALL_FILE_SIZE {
			if err = c.SaveSmallFile(&fileInfo); err != nil {
				log.Error(err)
				result.Message = err.Error()
				w.Write([]byte(c.util.JsonEncodePretty(result)))
				return
			}
		}
		c.saveFileMd5Log(&fileInfo, CONST_FILE_Md5_FILE_NAME) //maybe slow
		go c.postFileToPeer(&fileInfo)
		if fileInfo.Size <= 0 {
			msg = "file size is zero"
			result.Message = msg
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			log.Error(msg)
			return
		}
		fileResult = c.BuildFileResult(&fileInfo, r)

		if output == "json" || output == "json2" {
			if output == "json2" {
				result.Data = fileResult
				result.Status = "ok"
				w.Write([]byte(c.util.JsonEncodePretty(result)))
				return
			}
			w.Write([]byte(c.util.JsonEncodePretty(fileResult)))
		} else {
			w.Write([]byte(fileResult.Url))
		}
		return
	} else {
		md5sum = r.FormValue("md5")
		output = r.FormValue("output")
		if md5sum == "" {
			msg = "(error) if you want to upload fast md5 is require" +
				",and if you want to upload file,you must use post method  "
			result.Message = msg
			log.Error(msg)
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		if v, _ := c.GetFileInfoFromLevelDB(md5sum); v != nil && v.Md5 != "" {
			fileResult = c.BuildFileResult(v, r)
			result.Data = fileResult
			result.Status = "ok"
		}
		if output == "json" || output == "json2" {
			if data, err = json.Marshal(fileResult); err != nil {
				log.Error(err)
				result.Message = err.Error()
				w.Write([]byte(c.util.JsonEncodePretty(result)))
				return
			}
			if output == "json2" {
				w.Write([]byte(c.util.JsonEncodePretty(result)))
				return
			}
			w.Write(data)
		} else {
			w.Write([]byte(fileResult.Url))
		}
	}
}

func (c *Server) SaveUploadFile(file multipart.File, header *multipart.FileHeader, fileInfo *FileInfo, r *http.Request) (*FileInfo, error) {
	var (
		err     error
		outFile *os.File
		folder  string
		fi      os.FileInfo
	)
	defer file.Close()
	_, fileInfo.Name = filepath.Split(header.Filename)
	// bugfix for ie upload file contain fullpath
	if len(Config().Extensions) > 0 && !c.util.Contains(path.Ext(fileInfo.Name), Config().Extensions) {
		return fileInfo, errors.New("(error)file extension mismatch")
	}
	if Config().RenameFile {
		fileInfo.ReName = c.util.MD5(c.util.GetUUID()) + path.Ext(fileInfo.Name)
	}
	folder = time.Now().Format("20060102/15/04")
	if Config().PeerId != "" {
		folder = fmt.Sprintf(folder+"/%s", Config().PeerId)
	}
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
	if !c.util.FileExists(folder) {
		if err = os.MkdirAll(folder, 0775); err != nil {
			log.Error(err)
		}
	}
	outPath := fmt.Sprintf(folder+"/%s", fileInfo.Name)
	if fileInfo.ReName != "" {
		outPath = fmt.Sprintf(folder+"/%s", fileInfo.ReName)
	}
	if c.util.FileExists(outPath) && Config().EnableDistinctFile {
		for i := 0; i < 10000; i++ {
			outPath = fmt.Sprintf(folder+"/%d_%s", i, filepath.Base(header.Filename))
			fileInfo.Name = fmt.Sprintf("%d_%s", i, header.Filename)
			if !c.util.FileExists(outPath) {
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
	if fi, err = outFile.Stat(); err != nil {
		log.Error(err)
		return fileInfo, errors.New("(error)fail," + err.Error())
	} else {
		fileInfo.Size = fi.Size()
	}
	if fi.Size() != header.Size {
		return fileInfo, errors.New("(error)file uncomplete")
	}
	v := "" // c.util.GetFileSum(outFile, Config().FileSumArithmetic)
	if Config().EnableDistinctFile {
		v = c.util.GetFileSum(outFile, Config().FileSumArithmetic)
	} else {
		v = c.util.MD5(c.GetFilePathByInfo(fileInfo, false))
	}
	fileInfo.Md5 = v
	//fileInfo.Path = folder //strings.Replace( folder,DOCKER_DIR,"",1)
	fileInfo.Path = strings.Replace(folder, DOCKER_DIR, "", 1)
	fileInfo.Peers = append(fileInfo.Peers, c.host)
	//fmt.Println("upload", fileInfo)
	return fileInfo, nil
}

func (c *Server) SaveSmallFile(fileInfo *FileInfo) error {
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
	fpath = DOCKER_DIR + fileInfo.Path + "/" + filename
	largeDir = LARGE_DIR + "/" + Config().PeerId
	if !c.util.FileExists(largeDir) {
		os.MkdirAll(largeDir, 0775)
	}
	reName = fmt.Sprintf("%d", c.util.RandInt(100, 300))
	destPath = largeDir + "/" + reName
	c.lockMap.LockKey(destPath)
	defer c.lockMap.UnLockKey(destPath)
	if c.util.FileExists(fpath) {
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
		fileInfo.Path = strings.Replace(largeDir, DOCKER_DIR, "", 1)
	}
	return nil
}

func (c *Server) Report(w http.ResponseWriter, r *http.Request) {
	var (
		reportFileName string
		result         JsonResult
		html           string
	)
	result.Status = "ok"
	r.ParseForm()
	if c.IsPeer(r) {
		reportFileName = STATIC_DIR + "/report.html"
		if c.util.IsExist(reportFileName) {
			if data, err := c.util.ReadBinFile(reportFileName); err != nil {
				log.Error(err)
				result.Message = err.Error()
				w.Write([]byte(c.util.JsonEncodePretty(result)))
				return
			} else {
				html = string(data)
				if Config().SupportGroupManage {
					html = strings.Replace(html, "{group}", "/"+Config().Group, 1)
				} else {
					html = strings.Replace(html, "{group}", "", 1)
				}
				w.Write([]byte(html))
				return
			}
		} else {
			w.Write([]byte(fmt.Sprintf("%s is not found", reportFileName)))
		}
	} else {
		w.Write([]byte(c.GetClusterNotPermitMessage(r)))
	}
}