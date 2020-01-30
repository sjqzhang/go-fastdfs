package model

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"

	mapSet "github.com/deckarep/golang-set"
	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	levelDBUtil "github.com/syndtr/goleveldb/leveldb/util"
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

func LoadFileInfoByDate(date string, filename string) (mapSet.Set, error) {
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
		fileInfos mapSet.Set
	)
	fileInfos = mapSet.NewSet()
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

	handleFunc := func(filePath string, f os.FileInfo, err error) error {
		var (
			files    []os.FileInfo
			fi       os.FileInfo
			fileInfo FileInfo
			sum      string
			pathMd5  string
		)
		if f.IsDir() {
			files, err = ioutil.ReadDir(filePath)

			if err != nil {
				return err
			}
			for _, fi = range files {
				if fi.IsDir() || fi.Size() == 0 {
					continue
				}
				filePath = strings.Replace(filePath, "\\", "/", -1)
				if config.DockerDir != "" {
					filePath = strings.Replace(filePath, config.DockerDir, "", 1)
				}
				if pathPrefix != "" {
					filePath = strings.Replace(filePath, pathPrefix, config.StoreDirName, 1)
				}
				if strings.HasPrefix(filePath, config.StoreDirName+"/"+config.LargeDirName) {
					log.Info(fmt.Sprintf("ignore small file file %s", filePath+"/"+fi.Name()))
					continue
				}
				pathMd5 = pkg.MD5(filePath + "/" + fi.Name())
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
					Path:      filePath,
					Md5:       sum,
					TimeStamp: fi.ModTime().Unix(),
					Peers:     []string{svr.host},
					OffSet:    -2,
				}
				//log.Info(fileInfo)
				log.Info(filePath, "/", fi.Name())
				svr.AppendToQueue(&fileInfo)
				//svr.postFileToPeer(&fileInfo)
				svr.SaveFileInfoToLevelDB(fileInfo.Md5, &fileInfo, svr.LevelDB)
				//svr.SaveFileMd5Log(&fileInfo, FileMd5Name)
			}
		}

		return nil
	}

	pathname := config.StoreDir
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
		filepath.Walk(pathname, handleFunc)
	}

	log.Info("RepairFileInfoFromFile is finish.")
}

func GetFilePathByInfo(fileInfo *FileInfo, withDocker bool) string {
	fileName := fileInfo.Name
	if fileInfo.ReName != "" {
		fileName = fileInfo.ReName
	}
	if withDocker {
		return config.DockerDir + fileInfo.Path + "/" + fileName
	}
	return fileInfo.Path + "/" + fileName
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
	fullpath = GetFilePathByInfo(fileInfo, true)
	if fi, err = os.Stat(fullpath); err != nil {
		return false
	}
	if fi.Size() == fileInfo.Size {
		return true
	} else {
		return false
	}
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
	if db == svr.LevelDB { //search slow ,write fast, double write logDB
		logDate := pkg.GetDayFromTimeStamp(fileInfo.TimeStamp)
		logKey := fmt.Sprintf("%s_%s_%s", logDate, config.FileMd5Name, fileInfo.Md5)
		svr.logDB.Put([]byte(logKey), data, nil)
	}
	return fileInfo, nil
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
		ctx.JSON(http.StatusNotFound, GetClusterNotPermitMessage(r))
		log.Error(err)
		return
	}
	if fileInfo.OffSet == -2 {
		// optimize migrate
		svr.SaveFileInfoToLevelDB(fileInfo.Md5, &fileInfo, svr.LevelDB)
	} else {
		svr.SaveFileMd5Log(&fileInfo, config.Md5QueueFileName)
	}
	svr.AppendToDownloadQueue(&fileInfo)
	filename = fileInfo.Name
	if fileInfo.ReName != "" {
		filename = fileInfo.ReName
	}
	p := strings.Replace(fileInfo.Path, config.StoreDir+"/", "", 1)
	downloadUrl := fmt.Sprintf("http://%s/%s", r.Host, p+"/"+filename)
	log.Info("SyncFileInfo: ", downloadUrl)

	ctx.JSON(http.StatusOK, downloadUrl)
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
		ctx.JSON(http.StatusNotFound, GetClusterNotPermitMessage(r))
		return
	}
	md5sum = r.FormValue("md5")
	if filePath != "" {
		filePath = strings.Replace(filePath, "/"+config.FileDownloadPathPrefix, config.StoreDirName+"/", 1)
		md5sum = pkg.MD5(filePath)
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
