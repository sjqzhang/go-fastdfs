package goutil


import (
	"sync"
	"fmt"
	"log"
	"encoding/json"
	"io"
	"encoding/base64"
	"os"
	"time"
	"net/url"
	"net"
	"crypto/md5"
	"crypto/sha1"
	"reflect"
	"io/ioutil"
	"regexp"
	"runtime/debug"
	"path/filepath"
	"github.com/deckarep/golang-set"
	"strings"
	"errors"
	random "math/rand"
	"crypto/rand"
	"net/http"
)

type CommonMap struct {
	sync.RWMutex
	m map[string]interface{}
}

type Tuple struct {
	Key string
	Val interface{}
}

type Common struct {

}

func NewCommonMap(size int) *CommonMap {
	if size > 0 {
		return &CommonMap{m: make(map[string]interface{}, size)}
	} else {
		return &CommonMap{m: make(map[string]interface{})}
	}
}
func (s *CommonMap) GetValue(k string) (interface{}, bool) {
	s.RLock()
	defer s.RUnlock()
	v, ok := s.m[k]
	return v, ok
}
func (s *CommonMap) Put(k string, v interface{}) {
	s.Lock()
	defer s.Unlock()
	s.m[k] = v
}
func (s *CommonMap) Iter() <-chan Tuple { // reduce memory
	ch := make(chan Tuple)
	go func() {
		s.RLock()
		for k, v := range s.m {
			ch <- Tuple{Key: k, Val: v}
		}
		close(ch)
		s.RUnlock()
	}()
	return ch
}
func (s *CommonMap) LockKey(k string) {
	s.Lock()
	if v, ok := s.m[k]; ok {
		s.m[k+"_lock_"] = true
		s.Unlock()
		switch v.(type) {
		case *sync.Mutex:
			v.(*sync.Mutex).Lock()
		default:
			log.Print(fmt.Sprintf("LockKey %s", k))
		}
	} else {
		s.m[k] = &sync.Mutex{}
		v = s.m[k]
		s.m[k+"_lock_"] = true
		v.(*sync.Mutex).Lock()
		s.Unlock()
	}
}
func (s *CommonMap) UnLockKey(k string) {
	s.Lock()
	if v, ok := s.m[k]; ok {
		switch v.(type) {
		case *sync.Mutex:
			v.(*sync.Mutex).Unlock()
		default:
			log.Print(fmt.Sprintf("UnLockKey %s", k))
		}
		delete(s.m, k+"_lock_") // memory leak
		delete(s.m, k)          // memory leak
	}
	s.Unlock()
}
func (s *CommonMap) IsLock(k string) bool {
	s.Lock()
	if v, ok := s.m[k+"_lock_"]; ok {
		s.Unlock()
		return v.(bool)
	}
	s.Unlock()
	return false
}
func (s *CommonMap) Keys() []string {
	s.Lock()
	keys := make([]string, len(s.m))
	defer s.Unlock()
	for k, _ := range s.m {
		keys = append(keys, k)
	}
	return keys
}
func (s *CommonMap) Clear() {
	s.Lock()
	defer s.Unlock()
	s.m = make(map[string]interface{})
}
func (s *CommonMap) Remove(key string) {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.m[key]; ok {
		delete(s.m, key)
	}
}
func (s *CommonMap) AddUniq(key string) {
	s.Lock()
	defer s.Unlock()
	if _, ok := s.m[key]; !ok {
		s.m[key] = nil
	}
}
func (s *CommonMap) AddCount(key string, count int) {
	s.Lock()
	defer s.Unlock()
	if _v, ok := s.m[key]; ok {
		v := _v.(int)
		v = v + count
		s.m[key] = v
	} else {
		s.m[key] = 1
	}
}
func (s *CommonMap) AddCountInt64(key string, count int64) {
	s.Lock()
	defer s.Unlock()
	if _v, ok := s.m[key]; ok {
		v := _v.(int64)
		v = v + count
		s.m[key] = v
	} else {
		s.m[key] = count
	}
}
func (s *CommonMap) Add(key string) {
	s.Lock()
	defer s.Unlock()
	if _v, ok := s.m[key]; ok {
		v := _v.(int)
		v = v + 1
		s.m[key] = v
	} else {
		s.m[key] = 1
	}
}
func (s *CommonMap) Zero() {
	s.Lock()
	defer s.Unlock()
	for k := range s.m {
		s.m[k] = 0
	}
}
func (s *CommonMap) Contains(i ...interface{}) bool {
	s.Lock()
	defer s.Unlock()
	for _, val := range i {
		if _, ok := s.m[val.(string)]; !ok {
			return false
		}
	}
	return true
}
func (s *CommonMap) Get() map[string]interface{} {
	s.Lock()
	defer s.Unlock()
	m := make(map[string]interface{})
	for k, v := range s.m {
		m[k] = v
	}
	return m
}


func (this *Common) GetUUID() string {
	b := make([]byte, 48)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	id := this.MD5(base64.URLEncoding.EncodeToString(b))
	return fmt.Sprintf("%s-%s-%s-%s-%s", id[0:8], id[8:12], id[12:16], id[16:20], id[20:])
}
func (this *Common) CopyFile(src, dst string) (int64, error) {
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
func (this *Common) RandInt(min, max int) int {
	return func(min, max int) int {
		r := random.New(random.NewSource(time.Now().UnixNano()))
		if min >= max {
			return max
		}
		return r.Intn(max-min) + min
	}(min, max)
}
func (this *Common) GetToDay() string {
	return time.Now().Format("20060102")
}
func (this *Common) UrlEncode(v interface{}) string {
	switch v.(type) {
	case string:
		m := make(map[string]string)
		m["name"] = v.(string)
		return strings.Replace(this.UrlEncodeFromMap(m), "name=", "", 1)
	case map[string]string:
		return this.UrlEncodeFromMap(v.(map[string]string))
	default:
		return fmt.Sprintf("%v", v)
	}
}
func (this *Common) UrlEncodeFromMap(m map[string]string) string {
	vv := url.Values{}
	for k, v := range m {
		vv.Add(k, v)
	}
	return vv.Encode()
}
func (this *Common) UrlDecodeToMap(body string) (map[string]string, error) {
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
func (this *Common) GetDayFromTimeStamp(timeStamp int64) string {
	return time.Unix(timeStamp, 0).Format("20060102")
}
func (this *Common) StrToMapSet(str string, sep string) mapset.Set {
	result := mapset.NewSet()
	for _, v := range strings.Split(str, sep) {
		result.Add(v)
	}
	return result
}
func (this *Common) MapSetToStr(set mapset.Set, sep string) string {
	var (
		ret []string
	)
	for v := range set.Iter() {
		ret = append(ret, v.(string))
	}
	return strings.Join(ret, sep)
}
func (this *Common) GetPulicIP() string {
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
func (this *Common) MD5(str string) string {
	md := md5.New()
	md.Write([]byte(str))
	return fmt.Sprintf("%x", md.Sum(nil))
}
func (this *Common) GetFileMd5(file *os.File) string {
	file.Seek(0, 0)
	md5h := md5.New()
	io.Copy(md5h, file)
	sum := fmt.Sprintf("%x", md5h.Sum(nil))
	return sum
}
func (this *Common) GetFileSum(file *os.File, alg string) string {
	alg = strings.ToLower(alg)
	if alg == "sha1" {
		return this.GetFileSha1Sum(file)
	} else {
		return this.GetFileMd5(file)
	}
}
func (this *Common) GetFileSumByName(filepath string, alg string) (string, error) {
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
		return this.GetFileSha1Sum(file), nil
	} else {
		return this.GetFileMd5(file), nil
	}
}
func (this *Common) GetFileSha1Sum(file *os.File) string {
	file.Seek(0, 0)
	md5h := sha1.New()
	io.Copy(md5h, file)
	sum := fmt.Sprintf("%x", md5h.Sum(nil))
	return sum
}
func (this *Common) WriteFileByOffSet(filepath string, offset int64, data []byte) (error) {
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
func (this *Common) ReadFileByOffSet(filepath string, offset int64, length int) ([]byte, error) {
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
func (this *Common) Contains(obj interface{}, arrayobj interface{}) bool {
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
func (this *Common) FileExists(fileName string) bool {
	_, err := os.Stat(fileName)
	return err == nil
}
func (this *Common) WriteFile(path string, data string) bool {
	if err := ioutil.WriteFile(path, []byte(data), 0775); err == nil {
		return true
	} else {
		return false
	}
}
func (this *Common) WriteBinFile(path string, data []byte) bool {
	if err := ioutil.WriteFile(path, data, 0775); err == nil {
		return true
	} else {
		return false
	}
}
func (this *Common) IsExist(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil || os.IsExist(err)
}
func (this *Common) Match(matcher string, content string) []string {
	var result []string
	if reg, err := regexp.Compile(matcher); err == nil {
		result = reg.FindAllString(content, -1)
	}
	return result
}
func (this *Common) ReadBinFile(path string) ([]byte, error) {
	if this.IsExist(path) {
		fi, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer fi.Close()
		return ioutil.ReadAll(fi)
	} else {
		return nil, errors.New("not found")
	}
}
func (this *Common) RemoveEmptyDir(pathname string) {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Print(string(buffer))
		}
	}()
	handlefunc := func(file_path string, f os.FileInfo, err error) error {
		if f.IsDir() {
			files, _ := ioutil.ReadDir(file_path)
			if len(files) == 0 && file_path != pathname {
				os.Remove(file_path)
			}
		}
		return nil
	}
	fi, _ := os.Stat(pathname)
	if fi.IsDir() {
		filepath.Walk(pathname, handlefunc)
	}
}
func (this *Common) JsonEncodePretty(o interface{}) string {
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
func (this *Common) GetClientIp(r *http.Request) string {
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


