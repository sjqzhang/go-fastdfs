package server

import (
	"fmt"

	"path/filepath"
	"runtime"

	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	mapset "github.com/deckarep/golang-set"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/mem"
	log "github.com/sjqzhang/seelog"
	"github.com/syndtr/goleveldb/leveldb"
)

func (c *Server) CheckFileExist(w http.ResponseWriter, r *http.Request) {
	var (
		data     []byte
		err      error
		fileInfo *FileInfo
		fpath    string
		fi       os.FileInfo
	)
	r.ParseForm()
	md5sum := ""
	md5sum = r.FormValue("md5")
	fpath = r.FormValue("path")
	if fileInfo, err = c.GetFileInfoFromLevelDB(md5sum); fileInfo != nil {
		if fileInfo.OffSet != -1 {
			if data, err = json.Marshal(fileInfo); err != nil {
				log.Error(err)
			}
			w.Write(data)
			return
		}
		fpath = DOCKER_DIR + fileInfo.Path + "/" + fileInfo.Name
		if fileInfo.ReName != "" {
			fpath = DOCKER_DIR + fileInfo.Path + "/" + fileInfo.ReName
		}
		if c.util.IsExist(fpath) {
			if data, err = json.Marshal(fileInfo); err == nil {
				w.Write(data)
				return
			} else {
				log.Error(err)
			}
		} else {
			if fileInfo.OffSet == -1 {
				c.RemoveKeyFromLevelDB(md5sum, c.ldb) // when file delete,delete from leveldb
			}
		}
	} else {
		if fpath != "" {
			fi, err = os.Stat(fpath)
			if err == nil {
				sum := c.util.MD5(fpath)
				//if Config().EnableDistinctFile {
				//	sum, err = c.util.GetFileSumByName(fpath, Config().FileSumArithmetic)
				//	if err != nil {
				//		log.Error(err)
				//	}
				//}
				fileInfo = &FileInfo{
					Path:      path.Dir(fpath),
					Name:      path.Base(fpath),
					Size:      fi.Size(),
					Md5:       sum,
					Peers:     []string{Config().Host},
					OffSet:    -1, //very important
					TimeStamp: fi.ModTime().Unix(),
				}
				data, err = json.Marshal(fileInfo)
				w.Write(data)
				return
			}
		}
	}
	data, _ = json.Marshal(FileInfo{})
	w.Write(data)
	return
}

func (c *Server) CheckFilesExist(w http.ResponseWriter, r *http.Request) {
	var (
		data      []byte
		err       error
		fileInfo  *FileInfo
		fileInfos []*FileInfo
		fpath     string
		result    JsonResult
	)
	r.ParseForm()
	md5sum := ""
	md5sum = r.FormValue("md5s")
	md5s := strings.Split(md5sum, ",")
	for _, m := range md5s {
		if fileInfo, err = c.GetFileInfoFromLevelDB(m); fileInfo != nil {
			if fileInfo.OffSet != -1 {
				if data, err = json.Marshal(fileInfo); err != nil {
					log.Error(err)
				}
				//w.Write(data)
				//return
				fileInfos = append(fileInfos, fileInfo)
				continue
			}
			fpath = DOCKER_DIR + fileInfo.Path + "/" + fileInfo.Name
			if fileInfo.ReName != "" {
				fpath = DOCKER_DIR + fileInfo.Path + "/" + fileInfo.ReName
			}
			if c.util.IsExist(fpath) {
				if data, err = json.Marshal(fileInfo); err == nil {
					fileInfos = append(fileInfos, fileInfo)
					//w.Write(data)
					//return
					continue
				} else {
					log.Error(err)
				}
			} else {
				if fileInfo.OffSet == -1 {
					c.RemoveKeyFromLevelDB(md5sum, c.ldb) // when file delete,delete from leveldb
				}
			}
		}
	}
	result.Data = fileInfos
	data, _ = json.Marshal(result)
	w.Write(data)
	return
}

func (c *Server) Stat(w http.ResponseWriter, r *http.Request) {
	var (
		result   JsonResult
		inner    string
		echart   string
		category []string
		barCount []int64
		barSize  []int64
		dataMap  map[string]interface{}
	)
	if !c.IsPeer(r) {
		result.Message = c.GetClusterNotPermitMessage(r)
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	r.ParseForm()
	inner = r.FormValue("inner")
	echart = r.FormValue("echart")
	data := c.GetStat()
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
		w.Write([]byte(c.util.JsonEncodePretty(data)))
	} else {
		w.Write([]byte(c.util.JsonEncodePretty(result)))
	}
}

func (c *Server) BenchMark(w http.ResponseWriter, r *http.Request) {
	t := time.Now()
	batch := new(leveldb.Batch)
	for i := 0; i < 100000000; i++ {
		f := FileInfo{}
		f.Peers = []string{"http://192.168.0.1", "http://192.168.2.5"}
		f.Path = "20190201/19/02"
		s := strconv.Itoa(i)
		s = c.util.MD5(s)
		f.Name = s
		f.Md5 = s
		if data, err := json.Marshal(&f); err == nil {
			batch.Put([]byte(s), data)
		}
		if i%10000 == 0 {
			if batch.Len() > 0 {
				server.ldb.Write(batch, nil)
				//				batch = new(leveldb.Batch)
				batch.Reset()
			}
			fmt.Println(i, time.Since(t).Seconds())
		}
		//fmt.Println(server.GetFileInfoFromLevelDB(s))
	}
	c.util.WriteFile("time.txt", time.Since(t).String())
	fmt.Println(time.Since(t).String())
}

func (c *Server) GetFileInfo(w http.ResponseWriter, r *http.Request) {
	var (
		fpath    string
		md5sum   string
		fileInfo *FileInfo
		err      error
		result   JsonResult
	)
	md5sum = r.FormValue("md5")
	fpath = r.FormValue("path")
	result.Status = "fail"
	if !c.IsPeer(r) {
		w.Write([]byte(c.GetClusterNotPermitMessage(r)))
		return
	}
	md5sum = r.FormValue("md5")
	if fpath != "" {
		fpath = strings.Replace(fpath, "/"+Config().Group+"/", STORE_DIR_NAME+"/", 1)
		md5sum = c.util.MD5(fpath)
	}
	if fileInfo, err = c.GetFileInfoFromLevelDB(md5sum); err != nil {
		log.Error(err)
		result.Message = err.Error()
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	result.Status = "ok"
	result.Data = fileInfo
	w.Write([]byte(c.util.JsonEncodePretty(result)))
	return
}

func (c *Server) ReceiveMd5s(w http.ResponseWriter, r *http.Request) {
	var (
		err      error
		md5str   string
		fileInfo *FileInfo
		md5s     []string
	)
	if !c.IsPeer(r) {
		log.Warn(fmt.Sprintf("ReceiveMd5s %s", c.util.GetClientIp(r)))
		w.Write([]byte(c.GetClusterNotPermitMessage(r)))
		return
	}
	r.ParseForm()
	md5str = r.FormValue("md5s")
	md5s = strings.Split(md5str, ",")
	AppendFunc := func(md5s []string) {
		for _, m := range md5s {
			if m != "" {
				if fileInfo, err = c.GetFileInfoFromLevelDB(m); err != nil {
					log.Error(err)
					continue
				}
				c.AppendToQueue(fileInfo)
			}
		}
	}
	go AppendFunc(md5s)
}
func (c *Server) GetClusterNotPermitMessage(r *http.Request) string {
	var (
		message string
	)
	message = fmt.Sprintf(CONST_MESSAGE_CLUSTER_IP, c.util.GetClientIp(r))
	return message
}
func (c *Server) GetMd5sForWeb(w http.ResponseWriter, r *http.Request) {
	var (
		date   string
		err    error
		result mapset.Set
		lines  []string
		md5s   []interface{}
	)
	if !c.IsPeer(r) {
		w.Write([]byte(c.GetClusterNotPermitMessage(r)))
		return
	}
	date = r.FormValue("date")
	if result, err = c.GetMd5sByDate(date, CONST_FILE_Md5_FILE_NAME); err != nil {
		log.Error(err)
		return
	}
	md5s = result.ToSlice()
	for _, line := range md5s {
		if line != nil && line != "" {
			lines = append(lines, line.(string))
		}
	}
	w.Write([]byte(strings.Join(lines, ",")))
}
func (c *Server) GetMd5File(w http.ResponseWriter, r *http.Request) {
	var (
		date  string
		fpath string
		data  []byte
		err   error
	)
	if !c.IsPeer(r) {
		return
	}
	fpath = DATA_DIR + "/" + date + "/" + CONST_FILE_Md5_FILE_NAME
	if !c.util.FileExists(fpath) {
		w.WriteHeader(404)
		return
	}
	if data, err = ioutil.ReadFile(fpath); err != nil {
		w.WriteHeader(500)
		return
	}
	w.Write(data)
}

func (c *Server) Status(w http.ResponseWriter, r *http.Request) {
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
	today = c.util.GetToDay()
	sts = make(map[string]interface{})
	sts["Fs.QueueFromPeers"] = len(c.queueFromPeers)
	sts["Fs.QueueToPeers"] = len(c.queueToPeers)
	sts["Fs.QueueFileLog"] = len(c.queueFileLog)
	for _, k := range []string{CONST_FILE_Md5_FILE_NAME, CONST_Md5_ERROR_FILE_NAME, CONST_Md5_QUEUE_FILE_NAME} {
		k2 := fmt.Sprintf("%s_%s", today, k)
		if v, ok = c.sumMap.GetValue(k2); ok {
			sumset = v.(mapset.Set)
			if k == CONST_Md5_QUEUE_FILE_NAME {
				sts["Fs.QueueSetSize"] = sumset.Cardinality()
			}
			if k == CONST_Md5_ERROR_FILE_NAME {
				sts["Fs.ErrorSetSize"] = sumset.Cardinality()
			}
			if k == CONST_FILE_Md5_FILE_NAME {
				sts["Fs.FileSetSize"] = sumset.Cardinality()
			}
		}
	}
	sts["Fs.AutoRepair"] = Config().AutoRepair
	sts["Fs.QueueUpload"] = len(c.queueUpload)
	sts["Fs.RefreshInterval"] = Config().RefreshInterval
	sts["Fs.Peers"] = Config().Peers
	sts["Fs.Local"] = c.host
	sts["Fs.FileStats"] = c.GetStat()
	sts["Fs.ShowDir"] = Config().ShowDir
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
	w.Write([]byte(c.util.JsonEncodePretty(status)))
}

func (c *Server) HeartBeat(w http.ResponseWriter, r *http.Request) {
}

func (c *Server) ListDir(w http.ResponseWriter, r *http.Request) {
	var (
		result      JsonResult
		dir         string
		filesInfo   []os.FileInfo
		err         error
		filesResult []FileInfoResult
		tmpDir      string
	)
	if !c.IsPeer(r) {
		result.Message = c.GetClusterNotPermitMessage(r)
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	dir = r.FormValue("dir")
	//if dir == "" {
	//	result.Message = "dir can't null"
	//	w.Write([]byte(c.util.JsonEncodePretty(result)))
	//	return
	//}
	dir = strings.Replace(dir, ".", "", -1)
	if tmpDir, err = os.Readlink(dir); err == nil {
		dir = tmpDir
	}
	filesInfo, err = ioutil.ReadDir(DOCKER_DIR + STORE_DIR_NAME + "/" + dir)
	if err != nil {
		log.Error(err)
		result.Message = err.Error()
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	for _, f := range filesInfo {
		fi := FileInfoResult{
			Name:    f.Name(),
			Size:    f.Size(),
			IsDir:   f.IsDir(),
			ModTime: f.ModTime().Unix(),
			Path:    dir,
			Md5:     c.util.MD5(strings.Replace(STORE_DIR_NAME+"/"+dir+"/"+f.Name(), "//", "/", -1)),
		}
		filesResult = append(filesResult, fi)
	}
	result.Status = "ok"
	result.Data = filesResult
	w.Write([]byte(c.util.JsonEncodePretty(result)))
	return
}

func (c *Server) Index(w http.ResponseWriter, r *http.Request) {
	var (
		uploadUrl    string
		uploadBigUrl string
		uppy         string
	)
	uploadUrl = "/upload"
	uploadBigUrl = CONST_BIG_UPLOAD_PATH_SUFFIX
	if Config().EnableWebUpload {
		if Config().SupportGroupManage {
			uploadUrl = fmt.Sprintf("/%s/upload", Config().Group)
			uploadBigUrl = fmt.Sprintf("/%s%s", Config().Group, CONST_BIG_UPLOAD_PATH_SUFFIX)
		}
		uppy = `<html>
			  
			  <head>
				<meta charset="utf-8" />
				<title>go-fastdfs</title>
				<style>form { bargin } .form-line { display:block;height: 30px;margin:8px; } #stdUpload {background: #fafafa;border-radius: 10px;width: 745px; }</style>
				<link href="https://transloadit.edgly.net/releases/uppy/v0.30.0/dist/uppy.min.css" rel="stylesheet"></head>
			  
			  <body>
                <div>标准上传(强列建议使用这种方式)</div>
				<div id="stdUpload">
				  
				  <form action="%s" method="post" enctype="multipart/form-data">
					<span class="form-line">文件(file):
					  <input type="file" id="file" name="file" /></span>
					<span class="form-line">场景(scene):
					  <input type="text" id="scene" name="scene" value="%s" /></span>
					<span class="form-line">文件名(filename):
					  <input type="text" id="filename" name="filename" value="" /></span>
					<span class="form-line">输出(output):
					  <input type="text" id="output" name="output" value="json2" title="json|text|json2" /></span>
					<span class="form-line">自定义路径(path):
					  <input type="text" id="path" name="path" value="" /></span>
	              <span class="form-line">google认证码(code):
					  <input type="text" id="code" name="code" value="" /></span>
					 <span class="form-line">自定义认证(auth_token):
					  <input type="text" id="auth_token" name="auth_token" value="" /></span>
					<input type="submit" name="submit" value="upload" />
                </form>
				</div>
                 <div>断点续传（如果文件很大时可以考虑）</div>
				<div>
				 
				  <div id="drag-drop-area"></div>
				  <script src="https://transloadit.edgly.net/releases/uppy/v0.30.0/dist/uppy.min.js"></script>
				  <script>var uppy = Uppy.Core().use(Uppy.Dashboard, {
					  inline: true,
					  target: '#drag-drop-area'
					}).use(Uppy.Tus, {
					  endpoint: '%s'
					})
					uppy.on('complete', (result) => {
					 // console.log(result) console.log('Upload complete! We’ve uploaded these files:', result.successful)
					})
					//uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca',callback_url:'http://127.0.0.1/callback' ,filename:'自定义文件名','path':'自定义path',scene:'自定义场景' })//这里是传递上传的认证参数,callback_url参数中 id为文件的ID,info 文转的基本信息json
					uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca',callback_url:'http://127.0.0.1/callback'})//自定义参数与普通上传类似（虽然支持自定义，建议不要自定义，海量文件情况下，自定义很可能给自已给埋坑）
                </script>
				</div>
			  </body>
			</html>`
		uppyFileName := STATIC_DIR + "/uppy.html"
		if c.util.IsExist(uppyFileName) {
			if data, err := c.util.ReadBinFile(uppyFileName); err != nil {
				log.Error(err)
			} else {
				uppy = string(data)
			}
		} else {
			c.util.WriteFile(uppyFileName, uppy)
		}
		fmt.Fprintf(w,
			fmt.Sprintf(uppy, uploadUrl, Config().DefaultScene, uploadBigUrl))
	} else {
		w.Write([]byte("web upload deny"))
	}
}

// Notice: performance is poor,just for low capacity,but low memory , if you want to high performance,use searchMap for search,but memory ....
func (c *Server) Search(w http.ResponseWriter, r *http.Request) {
	var (
		result    JsonResult
		err       error
		kw        string
		count     int
		fileInfos []FileInfo
		md5s      []string
	)
	kw = r.FormValue("kw")
	if !c.IsPeer(r) {
		result.Message = c.GetClusterNotPermitMessage(r)
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	iter := c.ldb.NewIterator(nil, nil)
	for iter.Next() {
		var fileInfo FileInfo
		value := iter.Value()
		if err = json.Unmarshal(value, &fileInfo); err != nil {
			log.Error(err)
			continue
		}
		if strings.Contains(fileInfo.Name, kw) && !c.util.Contains(fileInfo.Md5, md5s) {
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
	//fileInfos=c.SearchDict(kw) // serch file from map for huge capacity
	result.Status = "ok"
	result.Data = fileInfos
	w.Write([]byte(c.util.JsonEncodePretty(result)))
}

func (c *Server) Sync(w http.ResponseWriter, r *http.Request) {
	var (
		result JsonResult
	)
	r.ParseForm()
	result.Status = "fail"
	if !c.IsPeer(r) {
		result.Message = "client must be in cluster"
		w.Write([]byte(c.util.JsonEncodePretty(result)))
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
		for _, peer := range Config().Peers {
			req := httplib.Post(peer + c.getRequestURI("sync"))
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
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	date = strings.Replace(date, ".", "", -1)
	if isForceUpload {
		go c.CheckFileAndSendToPeer(date, CONST_FILE_Md5_FILE_NAME, isForceUpload)
	} else {
		go c.CheckFileAndSendToPeer(date, CONST_Md5_ERROR_FILE_NAME, isForceUpload)
	}
	result.Status = "ok"
	result.Message = "job is running"
	w.Write([]byte(c.util.JsonEncodePretty(result)))
}
