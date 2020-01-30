package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	mapSet "github.com/deckarep/golang-set"
	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/internal/model"
	"github.com/luoyunpeng/go-fastdfs/pkg"
	log "github.com/sirupsen/logrus"
)

//Read: GetMd5File download file 'data/files.md5'?
func GetMd5File(ctx *gin.Context) {
	var date string
	r := ctx.Request

	if !model.IsPeer(r) {
		return
	}
	filePath := config.DataDir + "/" + date + "/" + config.FileMd5Name
	if !pkg.FileExists(filePath) {
		ctx.JSON(http.StatusNotFound, filePath+"does not exist")
		return
	}

	ctx.File(filePath)
}

// RemoveEmptyDir remove empty dir
func RemoveEmptyDir(ctx *gin.Context) {
	var (
		result model.JsonResult
	)
	r := ctx.Request
	result.Status = "ok"
	if model.IsPeer(r) {
		go pkg.RemoveEmptyDir(config.DataDir)
		go pkg.RemoveEmptyDir(config.StoreDir)
		result.Message = "clean job start ..,don't try again!!!"
		ctx.JSON(http.StatusOK, result)
		return
	}

	result.Message = model.GetClusterNotPermitMessage(r)
	ctx.JSON(http.StatusOK, result)
}

//ListDir list all file in given dir
func ListDir(ctx *gin.Context) {
	var (
		result      model.JsonResult
		dir         string
		filesInfo   []os.FileInfo
		err         error
		filesResult []model.FileInfoResult
		tmpDir      string
	)
	r := ctx.Request
	if !model.IsPeer(r) {
		result.Message = model.GetClusterNotPermitMessage(r)
		ctx.JSON(http.StatusNotAcceptable, result)
		return
	}
	dir = r.FormValue("dir")
	//if dir == "" {
	//	result.Message = "dir can't null"
	//	w.Write([]byte(pkg.JsonEncodePretty(result)))
	//	return
	//}
	dir = strings.Replace(dir, ".", "", -1)
	if tmpDir, err = os.Readlink(dir); err == nil {
		dir = tmpDir
	}
	filesInfo, err = ioutil.ReadDir(config.DockerDir + config.StoreDirName + "/" + dir)
	if err != nil {
		log.Error(err)
		result.Message = err.Error()
		ctx.JSON(http.StatusNotFound, result)
		return
	}
	for _, f := range filesInfo {
		fi := model.FileInfoResult{
			Name:    f.Name(),
			Size:    f.Size(),
			IsDir:   f.IsDir(),
			ModTime: f.ModTime().Unix(),
			Path:    dir,
			Md5:     pkg.MD5(strings.Replace(config.StoreDirName+"/"+dir+"/"+f.Name(), "//", "/", -1)),
		}
		filesResult = append(filesResult, fi)
	}
	result.Status = "ok"
	result.Data = filesResult
	ctx.JSON(http.StatusOK, result)
}

// Report
func Report(ctx *gin.Context) {
	var (
		data           []byte
		reportFileName string
		result         model.JsonResult
		html           string
		err            error
	)
	r := ctx.Request
	result.Status = "ok"
	r.ParseForm()
	if model.IsPeer(r) {
		reportFileName = config.StaticDir + "/report.html"
		if pkg.Exist(reportFileName) {
			if data, err = pkg.ReadFile(reportFileName); err != nil {
				log.Error(err)
				result.Message = err.Error()
				ctx.JSON(http.StatusNotFound, result)
				return
			}
			html = string(data)
			html = strings.Replace(html, "{group}", "", 1)

			ctx.HTML(http.StatusOK, "report.html", html)
			return
		}
		ctx.JSON(http.StatusNotFound, fmt.Sprintf("%s is not found", reportFileName))
		return
	}
	ctx.JSON(http.StatusNotAcceptable, model.GetClusterNotPermitMessage(r))
}

// Index point to upload page
func Index(ctx *gin.Context) {
	if config.CommonConfig.EnableWebUpload {

		ctx.HTML(http.StatusOK, "upload.tmpl", gin.H{"title": "Main website"})
		//ctx.Data(http.StatusOK, "text/html", []byte(config.DefaultUploadPage))
	}

	ctx.JSON(http.StatusNotFound, "web upload deny")
}

// GetMd5sForWeb
func GetMd5sForWeb(ctx *gin.Context) {
	var (
		date   string
		err    error
		result mapSet.Set
		lines  []string
		md5s   []interface{}
	)

	r := ctx.Request
	if !model.IsPeer(r) {
		ctx.JSON(http.StatusNotFound, model.GetClusterNotPermitMessage(r))
		return
	}
	date = r.FormValue("date")
	if result, err = model.GetMd5sByDate(date, config.FileMd5Name); err != nil {
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

//
func Download(ctx *gin.Context) {
	var fileInfo os.FileInfo

	reqURI := ctx.Request.RequestURI
	// if params is not enough then redirect to upload
	if pkg.CheckUploadURIInvalid(reqURI) {
		log.Warnf("RequestURI-%s is invalid, redirect to index", reqURI)
		ctx.JSON(http.StatusNotFound, "RequestURI is invalid, redirect to index")
		return
	}
	if ok, err := model.Svr.CheckDownloadAuth(ctx); !ok {
		log.Error(err)
		ctx.JSON(http.StatusUnauthorized, "not Permitted")
		return
	}

	fullPath, smallPath := model.GetFilePathFromRequest(ctx)
	if smallPath == "" {
		if _, err := os.Stat(fullPath); err != nil {
			model.Svr.DownloadNotFound(ctx)
			return
		}
		if !config.CommonConfig.ShowDir && fileInfo.IsDir() {
			ctx.JSON(http.StatusNotAcceptable, "list dir deny")
			return
		}
		model.DownloadNormalFileByURI(ctx)
		return
	}

	if ok, _ := model.DownloadSmallFileByURI(ctx); !ok {
		model.Svr.DownloadNotFound(ctx)
	}
}

func CheckFileExist(ctx *gin.Context) {
	var (
		err      error
		fileInfo *model.FileInfo
		fpath    string
		fi       os.FileInfo
	)
	r := ctx.Request
	r.ParseForm()
	md5sum := ""
	md5sum = r.FormValue("md5")
	fpath = r.FormValue("path")
	if fileInfo, err = model.Svr.GetFileInfoFromLevelDB(md5sum); fileInfo != nil {
		if fileInfo.OffSet != -1 {
			ctx.JSON(http.StatusOK, fileInfo)
			return
		}
		fpath = config.DockerDir + fileInfo.Path + "/" + fileInfo.Name
		if fileInfo.ReName != "" {
			fpath = config.DockerDir + fileInfo.Path + "/" + fileInfo.ReName
		}
		if pkg.Exist(fpath) {
			ctx.JSON(http.StatusOK, fileInfo)
			return
		}
		if fileInfo.OffSet == -1 {
			model.RemoveKeyFromLevelDB(md5sum, model.Svr.LevelDB) // when file delete,delete from leveldb
		}

		ctx.JSON(http.StatusNotFound, model.FileInfo{})
	}

	if fpath != "" {
		fi, err = os.Stat(fpath)
		if err == nil {
			sum := pkg.MD5(fpath)
			//if config.CommonConfig.EnableDistinctFile {
			//	sum, err = pkg.GetFileSumByName(fpath, config.CommonConfig.FileSumArithmetic)
			//	if err != nil {
			//		log.Error(err)
			//	}
			//}
			fileInfo = &model.FileInfo{
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

	ctx.JSON(http.StatusNotFound, model.FileInfo{})
}

func CheckFilesExist(ctx *gin.Context) {
	var (
		fileInfos []*model.FileInfo
		filePath  string
		result    model.JsonResult
	)
	r := ctx.Request
	r.ParseForm()
	md5sum := r.FormValue("md5s")
	md5s := strings.Split(md5sum, ",")
	for _, m := range md5s {
		if fileInfo, _ := model.Svr.GetFileInfoFromLevelDB(m); fileInfo != nil {
			if fileInfo.OffSet != -1 {
				fileInfos = append(fileInfos, fileInfo)
				continue
			}
			filePath = config.DockerDir + fileInfo.Path + "/" + fileInfo.Name
			if fileInfo.ReName != "" {
				filePath = config.DockerDir + fileInfo.Path + "/" + fileInfo.ReName
			}
			if pkg.Exist(filePath) {
				fileInfos = append(fileInfos, fileInfo)
				continue
			} else {
				if fileInfo.OffSet == -1 {
					model.RemoveKeyFromLevelDB(md5sum, model.Svr.LevelDB) // when file delete,delete from leveldb
				}
			}
		}
	}

	result.Data = fileInfos
	ctx.JSON(http.StatusOK, result)
}

func Upload(ctx *gin.Context) {
	var (
		fn     string
		folder string
		fpBody *os.File
	)
	folder = config.StoreDir + "/_tmp/" + pkg.GetToDay()
	if !pkg.FileExists(folder) {
		os.MkdirAll(folder, 0777)

	}
	fn = folder + "/" + pkg.GetUUID()
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
	model.Svr.QueueUpload <- model.WrapReqResp{Ctx: ctx, Done: done}
	<-done
}

func RemoveFile(ctx *gin.Context) {
	var (
		err      error
		md5sum   string
		fileInfo *model.FileInfo
		fpath    string
		delUrl   string
		result   model.JsonResult
		inner    string
		name     string
	)
	r := ctx.Request
	r.ParseForm()
	md5sum = r.FormValue("md5")
	fpath = r.FormValue("path")
	inner = r.FormValue("inner")
	result.Status = "fail"
	if !model.IsPeer(r) {
		ctx.JSON(http.StatusUnauthorized, model.GetClusterNotPermitMessage(r))
		return
	}
	if config.CommonConfig.AuthUrl != "" && !model.CheckAuth(r) {
		ctx.JSON(http.StatusUnauthorized, "Unauthorized")
		return
	}
	if fpath != "" && md5sum == "" {
		fpath = strings.Replace(fpath, "/"+config.FileDownloadPathPrefix, config.StoreDirName+"/", 1)
		md5sum = pkg.MD5(fpath)
	}
	if inner != "1" {
		for _, peer := range config.CommonConfig.Peers {
			delFile := func(peer string, md5sum string, fileInfo *model.FileInfo) {
				delUrl = fmt.Sprintf("%s%s", peer, model.GetRequestURI("delete"))
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
	if fileInfo, err = model.Svr.GetFileInfoFromLevelDB(md5sum); err != nil {
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
	if fileInfo.Path != "" && pkg.FileExists(config.DockerDir+fpath) {
		model.Svr.SaveFileMd5Log(fileInfo, config.RemoveMd5FileName)
		if err = os.Remove(config.DockerDir + fpath); err != nil {
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

func RepairFileInfo(ctx *gin.Context) {
	var (
		result model.JsonResult
	)
	if !model.IsPeer(ctx.Request) {
		ctx.JSON(http.StatusNotFound, model.GetClusterNotPermitMessage(ctx.Request))
		return
	}

	if !config.CommonConfig.EnableMigrate {
		ctx.JSON(http.StatusNotFound, "please set enable_migrate=true")
		return
	}

	result.Status = "ok"
	result.Message = "repair job start,don't try again,very danger "
	go model.Svr.RepairFileInfoFromFile()

	ctx.JSON(http.StatusNotFound, result)
}

func Reload(ctx *gin.Context) {
	var (
		err     error
		data    []byte
		cfg     config.Config
		action  string
		cfgJson string
		result  model.JsonResult
	)
	r := ctx.Request
	result.Status = "fail"
	r.ParseForm()
	if !model.IsPeer(r) {
		ctx.JSON(http.StatusNotFound, model.GetClusterNotPermitMessage(r))
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
		cfgJson = pkg.JsonEncodePretty(cfg)
		pkg.WriteFile(config.ConfFileName, cfgJson)
		ctx.JSON(http.StatusOK, result)
		return
	}
	if action == "reload" {
		if data, err = ioutil.ReadFile(config.ConfFileName); err != nil {
			result.Message = err.Error()
			ctx.JSON(http.StatusNotFound, result)
			return
		}
		if err = json.Unmarshal(data, &cfg); err != nil {
			result.Message = err.Error()
			ctx.JSON(http.StatusNotFound, result)
			return
		}
		config.ParseConfig(config.ConfFileName)
		model.Svr.InitComponent(true)
		result.Status = "ok"
		ctx.JSON(http.StatusOK, result)
		return
	}

	ctx.JSON(http.StatusNotFound, "(error)action support set(json) get reload")
}

func BackUp(ctx *gin.Context) {
	var (
		err    error
		date   string
		result model.JsonResult
		inner  string
		url    string
	)
	r := ctx.Request
	result.Status = "ok"
	r.ParseForm()
	date = r.FormValue("date")
	inner = r.FormValue("inner")
	if date == "" {
		date = pkg.GetToDay()
	}
	if model.IsPeer(r) {
		if inner != "1" {
			for _, peer := range config.CommonConfig.Peers {
				backUp := func(peer string, date string) {
					url = peer + model.GetRequestURI("backup")
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
		go model.Svr.BackUpMetaDataByDate(date)
		result.Message = "back job start..."
		ctx.JSON(http.StatusOK, result)
		return
	}

	result.Message = model.GetClusterNotPermitMessage(r)
	ctx.JSON(http.StatusNotAcceptable, result)
}

// Notice: performance is poor,just for low capacity,but low memory ,
//if you want to high performance,use searchMap for search,but memory ....
func Search(ctx *gin.Context) {
	var (
		result    model.JsonResult
		err       error
		kw        string
		count     int
		fileInfos []model.FileInfo
		md5s      []string
	)
	r := ctx.Request
	kw = r.FormValue("kw")
	if !model.IsPeer(r) {
		result.Message = model.GetClusterNotPermitMessage(r)
		ctx.JSON(http.StatusNotAcceptable, result)
		return
	}
	iter := model.Svr.LevelDB.NewIterator(nil, nil)
	for iter.Next() {
		var fileInfo model.FileInfo
		value := iter.Value()
		if err = json.Unmarshal(value, &fileInfo); err != nil {
			log.Error(err)
			continue
		}
		if strings.Contains(fileInfo.Name, kw) && !pkg.Contains(fileInfo.Md5, md5s) {
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

func Repair(ctx *gin.Context) {
	var (
		force       string
		forceRepair bool
		result      model.JsonResult
	)
	r := ctx.Request
	result.Status = "ok"
	r.ParseForm()
	force = r.FormValue("force")
	if force == "1" {
		forceRepair = true
	}
	if model.IsPeer(r) {
		go model.Svr.AutoRepair(forceRepair)
		result.Message = "repair job start..."
		ctx.JSON(http.StatusOK, result)
		return
	}

	result.Message = model.GetClusterNotPermitMessage(r)
	ctx.JSON(http.StatusNotAcceptable, result)
}
