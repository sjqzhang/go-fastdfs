package pkg

import (
	"bufio"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	random "math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/astaxie/beego/httplib"
	mapSet "github.com/deckarep/golang-set"
	"github.com/gin-gonic/gin"
)

type CommonMap struct {
	m sync.Map
}

type Tuple struct {
	Key string
	Val interface{}
}

func NewCommonMap(size int) *CommonMap {
	return &CommonMap{}

}

func (cMap *CommonMap) GetValue(k string) (interface{}, bool) {
	return cMap.m.Load(k)
}

func (cMap *CommonMap) Put(k string, v interface{}) {
	cMap.m.Store(k, v)
}

func (cMap *CommonMap) Iter() <-chan Tuple { // reduce memory
	ch := make(chan Tuple)
	go func() {

		cMap.m.Range(func(key, value interface{}) bool {
			ch <- Tuple{Key: key.(string), Val: value}
			return true
		})
		close(ch)
	}()
	return ch
}

func (cMap *CommonMap) LockKey(k string) {
	if v, ok := cMap.m.Load(k); ok {
		cMap.m.Store(k+"_locak_", true)
		switch v.(type) {
		case *sync.Mutex:
			v.(*sync.Mutex).Lock()
		default:
			log.Print(fmt.Sprintf("LockKey %s", k))
		}
		return
	}
	lock := &sync.Mutex{}
	cMap.m.Store(k, lock)
	lock.Lock()
}

func (cMap *CommonMap) UnLockKey(k string) {
	if v, ok := cMap.m.Load(k); ok {
		cMap.m.Store(k+"_locak_", true)
		switch v.(type) {
		case *sync.Mutex:
			v.(*sync.Mutex).Unlock()
		default:
			log.Print(fmt.Sprintf("UnLockKey %s", k))
		}
	}
}

func (cMap *CommonMap) IsLock(k string) bool {
	if v, ok := cMap.m.Load(k + "_lock_"); ok {
		return v.(bool)
	}

	return false
}

func (cMap *CommonMap) Keys() []string {
	keys := make([]string, 0)
	cMap.m.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})
	return keys
}

func (cMap *CommonMap) Clear() {
	cMap.m = sync.Map{}
}

func (cMap *CommonMap) Remove(key string) {
	if _, ok := cMap.m.Load(key); ok {
		cMap.m.Delete(key)
	}
}

func (cMap *CommonMap) AddUniq(key string) {
	if _, ok := cMap.m.Load(key); !ok {
		cMap.m.Store(key, nil)
	}
}

func (cMap *CommonMap) AddCount(key string, count int) {
	if v, ok := cMap.m.Load(key); ok {
		tmp := v.(int)
		tmp = tmp + count
		cMap.m.Store(key, tmp)
		return
	}
	cMap.m.Store(key, 1)
}

func (cMap *CommonMap) AddCountInt64(key string, count int64) {
	if v, ok := cMap.m.Load(key); ok {
		tmp := v.(int64)
		tmp = tmp + count
		cMap.m.Store(key, tmp)
		return
	}
	cMap.m.Store(key, count)

}
func (cMap *CommonMap) Add(key string) {
	if v, ok := cMap.m.Load(key); ok {
		tmp := v.(int)
		tmp = tmp + 1
		cMap.m.Store(key, tmp)
		return
	}
	cMap.m.Store(key, 1)
}

func (cMap *CommonMap) Zero() {
	var keys []string
	cMap.m.Range(func(key, value interface{}) bool {
		keys = append(keys, key.(string))
		return true
	})

	for _, k := range keys {
		cMap.m.Store(k, 0)
	}
}

func (cMap *CommonMap) Contains(i ...interface{}) bool {
	for _, val := range i {
		if _, ok := cMap.m.Load(val.(string)); !ok {
			return false
		}
	}
	return true
}

func (cMap *CommonMap) Get() map[string]interface{} {
	m := make(map[string]interface{})
	cMap.m.Range(func(key, value interface{}) bool {
		m[key.(string)] = value
		return true
	})
	return m
}

// TODO: the uuid method is not good enough, to change
func GetUUID() string {
	b := make([]byte, 48)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	id := MD5(base64.URLEncoding.EncodeToString(b))
	return fmt.Sprintf("%s-%s-%s-%s-%s", id[0:8], id[8:12], id[12:16], id[16:20], id[20:])
}

func GetUUID_op() string {
	var b [32]byte
	bSlice := b[:]
	if _, err := io.ReadFull(rand.Reader, bSlice); err != nil {
		return ""
	}
	id := MD5(base64.URLEncoding.EncodeToString(bSlice))
	return fmt.Sprintf("%s-%s-%s-%s-%s", id[0:8], id[8:12], id[12:16], id[16:20], id[20:])
}

func CopyFile(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, err
	}
	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}
	source, err := os.Open(src)
	if err != nil {
		return 0, err
	}
	defer source.Close()
	destination, err := os.Create(dst)
	if err != nil {
		return 0, err
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
}

func RandInt(min, max int) int {
	return func(min, max int) int {
		r := random.New(random.NewSource(time.Now().UnixNano()))
		if min >= max {
			return max
		}
		return r.Intn(max-min) + min
	}(min, max)
}

// GetToDay get today's string info
func GetToDay() string {
	var tmp [8]byte
	tmpSlice := tmp[:0]
	time.Now().AppendFormat(tmpSlice, "20060102")
	return string(tmp[:])
}

func UrlEncode(v interface{}) string {
	switch v.(type) {
	case string:
		m := make(map[string]string)
		m["name"] = v.(string)
		return strings.Replace(UrlEncodeFromMap(m), "name=", "", 1)
	case map[string]string:
		return UrlEncodeFromMap(v.(map[string]string))
	default:
		return fmt.Sprintf("%v", v)
	}
}

func UrlEncodeFromMap(m map[string]string) string {
	vv := url.Values{}
	for k, v := range m {
		vv.Add(k, v)
	}
	return vv.Encode()
}

func UrlDecodeToMap(body string) (map[string]string, error) {
	var (
		err error
		m   map[string]string
		v   url.Values
	)
	m = make(map[string]string)
	if v, err = url.ParseQuery(body); err != nil {
		return m, err
	}
	for _k, _v := range v {
		if len(_v) > 0 {
			m[_k] = _v[0]
		}
	}
	return m, nil
}

func GetDayFromTimeStamp(timeStamp int64) string {
	return time.Unix(timeStamp, 0).Format("20060102")
}

func StrToMapSet(str string, sep string) mapSet.Set {
	result := mapSet.NewSet()
	for _, v := range strings.Split(str, sep) {
		result.Add(v)
	}
	return result
}

func MapSetToStr(set mapSet.Set, sep string) string {
	var (
		ret []string
	)
	for v := range set.Iter() {
		ret = append(ret, v.(string))
	}
	return strings.Join(ret, sep)
}

func GetPublicIP() string {
	var (
		err  error
		conn net.Conn
	)
	if conn, err = net.Dial("udp", "8.8.8.8:80"); err != nil {
		return "127.0.0.1"
	}
	defer conn.Close()
	localAddr := conn.LocalAddr().String()
	idx := strings.LastIndex(localAddr, ":")
	return localAddr[0:idx]
}

func MD5(str string) string {
	md := md5.New()
	md.Write([]byte(str))
	return fmt.Sprintf("%x", md.Sum(nil))
}

func GetFileMd5(file *os.File) string {
	file.Seek(0, 0)
	md5h := md5.New()
	io.Copy(md5h, file)
	sum := fmt.Sprintf("%x", md5h.Sum(nil))
	return sum
}

func GetFileSum(file *os.File, alg string) string {
	alg = strings.ToLower(alg)
	if alg == "sha1" {
		return GetFileSha1Sum(file)
	} else {
		return GetFileMd5(file)
	}
}

func GetFileSumByName(filepath string, alg string) (string, error) {
	var (
		err  error
		file *os.File
	)
	file, err = os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()
	alg = strings.ToLower(alg)
	if alg == "sha1" {
		return GetFileSha1Sum(file), nil
	} else {
		return GetFileMd5(file), nil
	}
}

func GetFileSha1Sum(file *os.File) string {
	file.Seek(0, 0)
	md5h := sha1.New()
	io.Copy(md5h, file)
	sum := fmt.Sprintf("%x", md5h.Sum(nil))
	return sum
}

func WriteFileByOffSet(filepath string, offset int64, data []byte) error {
	var (
		err   error
		file  *os.File
		count int
	)
	file, err = os.OpenFile(filepath, os.O_CREATE|os.O_RDWR, 0666)
	if err != nil {
		return err
	}
	defer file.Close()
	count, err = file.WriteAt(data, offset)
	if err != nil {
		return err
	}
	if count != len(data) {
		return errors.New(fmt.Sprintf("write %s error", filepath))
	}
	return nil
}

func ReadFileByOffSet(filepath string, offset int64, length int) ([]byte, error) {
	var (
		err    error
		file   *os.File
		result []byte
		count  int
	)
	file, err = os.Open(filepath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result = make([]byte, length)
	count, err = file.ReadAt(result, offset)
	if err != nil {
		return nil, err
	}
	if count != length {
		return nil, errors.New("read error")
	}
	return result, nil
}

func Contains(obj interface{}, arrayobj interface{}) bool {
	targetValue := reflect.ValueOf(arrayobj)
	switch reflect.TypeOf(arrayobj).Kind() {
	case reflect.Slice, reflect.Array:
		for i := 0; i < targetValue.Len(); i++ {
			if targetValue.Index(i).Interface() == obj {
				return true
			}
		}
	case reflect.Map:
		if targetValue.MapIndex(reflect.ValueOf(obj)).IsValid() {
			return true
		}
	}
	return false
}

func FileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}

// FileAndExists check is the file exists, not dir
func FileAndExists(fileName string) bool {
	fileInfo, err := os.Stat(fileName)
	return err == nil && !fileInfo.IsDir()
}

func WriteFile(path string, data string) bool {
	if err := ioutil.WriteFile(path, []byte(data), 0775); err == nil {
		return true
	} else {
		return false
	}
}

func WriteBinFile(path string, data []byte) bool {
	if err := ioutil.WriteFile(path, data, 0775); err == nil {
		return true
	} else {
		return false
	}
}

func Exist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}

func Match(matcher string, content string) []string {
	var result []string
	if reg, err := regexp.Compile(matcher); err == nil {
		result = reg.FindAllString(content, -1)
	}
	return result
}

func ReadFile(path string) ([]byte, error) {
	if Exist(path) {
		fi, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer fi.Close()
		return ioutil.ReadAll(fi)
	}

	return nil, errors.New("not found")
}

func RemoveEmptyDir(pathName string) {
	handleFunc := func(filePath string, f os.FileInfo, err error) error {
		if f.IsDir() {
			files, _ := ioutil.ReadDir(filePath)
			if len(files) == 0 && filePath != pathName {
				os.Remove(filePath)
			}
		}
		return nil
	}

	fileInfo, err := os.Stat(pathName)
	if err != nil {
		log.Warnf("delete dir-%s  error: %v", pathName, err)
	}
	if fileInfo.IsDir() {
		filepath.Walk(pathName, handleFunc)
	}
}

func JsonEncodePretty(o interface{}) string {
	resp := ""
	switch o.(type) {
	case map[string]interface{}:
		if data, err := json.Marshal(o); err == nil {
			resp = string(data)
		}
	case map[string]string:
		if data, err := json.Marshal(o); err == nil {
			resp = string(data)
		}
	case []interface{}:
		if data, err := json.Marshal(o); err == nil {
			resp = string(data)
		}
	case []string:
		if data, err := json.Marshal(o); err == nil {
			resp = string(data)
		}
	case string:
		resp = o.(string)
	default:
		if data, err := json.Marshal(o); err == nil {
			resp = string(data)
		}
	}
	var v interface{}
	if ok := json.Unmarshal([]byte(resp), &v); ok == nil {
		if buf, ok := json.MarshalIndent(v, "", "  "); ok == nil {
			resp = string(buf)
		}
	}
	return resp
}

func GetClientIp(r *http.Request) string {
	client_ip := ""
	headers := []string{"X_Forwarded_For", "X-Forwarded-For", "X-Real-Ip",
		"X_Real_Ip", "Remote_Addr", "Remote-Addr"}
	for _, v := range headers {
		if _v, ok := r.Header[v]; ok {
			if len(_v) > 0 {
				client_ip = _v[0]
				break
			}
		}
	}
	if client_ip == "" {
		clients := strings.Split(r.RemoteAddr, ":")
		client_ip = clients[0]
	}
	return client_ip
}

// GetRequestURI returns the request url, if group-manage is enable, add the group info to req url
func GetRequestURI(action string) string {
	if strings.HasPrefix(action, "/") {
		return action
	}
	return "/" + action
}

//
func GetFileServerRunningAbsDir(appName string) (path string, err error) {
	return filepath.Abs(filepath.Dir(appName))
}

//
func CheckUploadURIInvalid(uri string) bool {
	return uri == "/" || uri == ""
}

// SetDownloadHeader add dowoload info to header
func SetDownloadHeader(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment")
}

// CrossOrigin add cross info to header
func CrossOrigin(ctx *gin.Context) {
	ctx.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	ctx.Writer.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Depth, User-Agent, X-File-Size, X-Requested-With, X-Requested-By, If-Modified-Since, X-File-Name, X-File-Type, Cache-Control, Origin")
	ctx.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	ctx.Writer.Header().Set("Access-Control-Expose-Headers", "Authorization")
	//https://blog.csdn.net/yanzisu_congcong/article/details/80552155
}

//NotPermit adds 401 code to header
func NotPermit(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(401)
}

// InsideContainer check if running inside container
func InsideContainer() (bool, error) {
	lines, err := ReadLinesOffsetN("/proc/1/cgroup", 0, -1)
	if err != nil {
		return false, err
	}

	for _, line := range lines {
		if !strings.HasSuffix(line, "/") {
			return true, nil
		}
	}

	return false, nil
}

// ReadLinesOffsetN reads contents from file and splits them by new line.
// The offset tells at which line number to start.
// The count determines the number of lines to read (starting from offset):
//   n >= 0: at most n lines
//   n < 0: whole file
func ReadLinesOffsetN(filename string, offset uint, n int) ([]string, error) {
	f, err := os.Open(filename)
	if err != nil {
		return []string{""}, err
	}
	defer f.Close()

	var ret []string

	r := bufio.NewReader(f)
	for i := 0; i < n+int(offset) || n < 0; i++ {
		line, err := r.ReadString('\n')
		if err != nil {
			break
		}
		if i < int(offset) {
			continue
		}
		ret = append(ret, strings.Trim(line, "\n"))
	}

	return ret, nil
}

// DownloadFileToResponse
func DownloadFileToResponse(url string, ctx *gin.Context) {
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

// GetServerURI
func GetServerURI(r *http.Request) string {
	return fmt.Sprintf("http://%s/", r.Host)
}

// CreateDirectories creates directories for storing photos, metadata and cache files.
func CreateDirectories(dir string, perm os.FileMode) error {
	if FileAndExists(dir) {
		return fmt.Errorf("%s is file and already exists, please checj", dir)
	}

	if err := os.MkdirAll(dir, perm); err != nil {
		return err
	}
	return nil
}
