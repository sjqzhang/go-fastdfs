package server

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	mapset "github.com/deckarep/golang-set"
	log "github.com/sjqzhang/seelog"
	"github.com/syndtr/goleveldb/leveldb/util"
)

func (c *Server) RepairFileInfoFromFile() {
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
	if c.lockMap.IsLock("RepairFileInfoFromFile") {
		log.Warn("Lock RepairFileInfoFromFile")
		return
	}
	c.lockMap.LockKey("RepairFileInfoFromFile")
	defer c.lockMap.UnLockKey("RepairFileInfoFromFile")
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
				if DOCKER_DIR != "" {
					file_path = strings.Replace(file_path, DOCKER_DIR, "", 1)
				}
				if pathPrefix != "" {
					file_path = strings.Replace(file_path, pathPrefix, STORE_DIR_NAME, 1)
				}
				if strings.HasPrefix(file_path, STORE_DIR_NAME+"/"+LARGE_DIR_NAME) {
					log.Info(fmt.Sprintf("ignore small file file %s", file_path+"/"+fi.Name()))
					continue
				}
				pathMd5 = c.util.MD5(file_path + "/" + fi.Name())
				//if finfo, _ := c.GetFileInfoFromLevelDB(pathMd5); finfo != nil && finfo.Md5 != "" {
				//	log.Info(fmt.Sprintf("exist ignore file %s", file_path+"/"+fi.Name()))
				//	continue
				//}
				//sum, err = c.util.GetFileSumByName(file_path+"/"+fi.Name(), Config().FileSumArithmetic)
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
					Peers:     []string{c.host},
					OffSet:    -2,
				}
				//log.Info(fileInfo)
				log.Info(file_path, "/", fi.Name())
				c.AppendToQueue(&fileInfo)
				//c.postFileToPeer(&fileInfo)
				c.SaveFileInfoToLevelDB(fileInfo.Md5, &fileInfo, c.ldb)
				//c.SaveFileMd5Log(&fileInfo, CONST_FILE_Md5_FILE_NAME)
			}
		}
		return nil
	}
	pathname := STORE_DIR
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

func (c *Server) RepairStatByDate(date string) StatDateFileInfo {
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
	keyPrefix = fmt.Sprintf(keyPrefix, date, CONST_FILE_Md5_FILE_NAME)
	iter := server.logDB.NewIterator(util.BytesPrefix([]byte(keyPrefix)), nil)
	defer iter.Release()
	for iter.Next() {
		if err = json.Unmarshal(iter.Value(), &fileInfo); err != nil {
			continue
		}
		fileCount = fileCount + 1
		fileSize = fileSize + fileInfo.Size
	}
	c.statMap.Put(date+"_"+CONST_STAT_FILE_COUNT_KEY, fileCount)
	c.statMap.Put(date+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY, fileSize)
	c.SaveStat()
	stat.Date = date
	stat.FileCount = fileCount
	stat.TotalSize = fileSize
	return stat
}

func (c *Server) AutoRepair(forceRepair bool) {
	if c.lockMap.IsLock("AutoRepair") {
		log.Warn("Lock AutoRepair")
		return
	}
	c.lockMap.LockKey("AutoRepair")
	defer c.lockMap.UnLockKey("AutoRepair")
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
			req := httplib.Get(fmt.Sprintf("%s%s?date=%s&force=%s", peer, c.getRequestURI("sync"), dateStat.Date, "1"))
			req.SetTimeout(time.Second*5, time.Second*5)
			if _, err = req.String(); err != nil {
				log.Error(err)
			}
			log.Info(fmt.Sprintf("syn file from %s date %s", peer, dateStat.Date))
		}
		for _, peer := range Config().Peers {
			req := httplib.Post(fmt.Sprintf("%s%s", peer, c.getRequestURI("stat")))
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
				countKey = dateStat.Date + "_" + CONST_STAT_FILE_COUNT_KEY
				if v, ok := c.statMap.GetValue(countKey); ok {
					switch v.(type) {
					case int64:
						if v.(int64) != dateStat.FileCount || forceRepair {
							//不相等,找差异
							//TODO
							req := httplib.Post(fmt.Sprintf("%s%s", peer, c.getRequestURI("get_md5s_by_date")))
							req.SetTimeout(time.Second*15, time.Second*60)
							req.Param("date", dateStat.Date)
							if md5s, err = req.String(); err != nil {
								continue
							}
							if localSet, err = c.GetMd5sByDate(dateStat.Date, CONST_FILE_Md5_FILE_NAME); err != nil {
								log.Error(err)
								continue
							}
							remoteSet = c.util.StrToMapSet(md5s, ",")
							allSet = localSet.Union(remoteSet)
							md5s = c.util.MapSetToStr(allSet.Difference(localSet), ",")
							req = httplib.Post(fmt.Sprintf("%s%s", peer, c.getRequestURI("receive_md5s")))
							req.SetTimeout(time.Second*15, time.Second*60)
							req.Param("md5s", md5s)
							req.String()
							tmpSet = allSet.Difference(remoteSet)
							for v := range tmpSet.Iter() {
								if v != nil {
									if fileInfo, err = c.GetFileInfoFromLevelDB(v.(string)); err != nil {
										log.Error(err)
										continue
									}
									c.AppendToQueue(fileInfo)
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

func (c *Server) RepairFileInfo(w http.ResponseWriter, r *http.Request) {
	var (
		result JsonResult
	)
	if !c.IsPeer(r) {
		w.Write([]byte(c.GetClusterNotPermitMessage(r)))
		return
	}
	if !Config().EnableMigrate {
		w.Write([]byte("please set enable_migrate=true"))
		return
	}
	result.Status = "ok"
	result.Message = "repair job start,don't try again,very danger "
	go c.RepairFileInfoFromFile()
	w.Write([]byte(c.util.JsonEncodePretty(result)))
}

func (c *Server) RepairStatWeb(w http.ResponseWriter, r *http.Request) {
	var (
		result JsonResult
		date   string
		inner  string
	)
	if !c.IsPeer(r) {
		result.Message = c.GetClusterNotPermitMessage(r)
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	date = r.FormValue("date")
	inner = r.FormValue("inner")
	if ok, err := regexp.MatchString("\\d{8}", date); err != nil || !ok {
		result.Message = "invalid date"
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	if date == "" || len(date) != 8 {
		date = c.util.GetToDay()
	}
	if inner != "1" {
		for _, peer := range Config().Peers {
			req := httplib.Post(peer + c.getRequestURI("repair_stat"))
			req.Param("inner", "1")
			req.Param("date", date)
			if _, err := req.String(); err != nil {
				log.Error(err)
			}
		}
	}
	result.Data = c.RepairStatByDate(date)
	result.Status = "ok"
	w.Write([]byte(c.util.JsonEncodePretty(result)))
}

func (c *Server) Repair(w http.ResponseWriter, r *http.Request) {
	var (
		force       string
		forceRepair bool
		result      JsonResult
	)
	result.Status = "ok"
	r.ParseForm()
	force = r.FormValue("force")
	if force == "1" {
		forceRepair = true
	}
	if c.IsPeer(r) {
		go c.AutoRepair(forceRepair)
		result.Message = "repair job start..."
		w.Write([]byte(c.util.JsonEncodePretty(result)))
	} else {
		result.Message = c.GetClusterNotPermitMessage(r)
		w.Write([]byte(c.util.JsonEncodePretty(result)))
	}

}