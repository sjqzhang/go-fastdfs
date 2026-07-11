package server

// =============================================================================
// Bug 验证单元测试
// -----------------------------------------------------------------------------
// 本文件针对此前分析中"可运行时复现"的 bug 编写单元测试。
// 每个测试都断言"正确行为"，因此当 bug 存在时测试会 FAIL，修复后应通过。
//
// 覆盖的 bug：
//   - Bug 2  HttpProxyHandler 上游失败时静默返回（不写任何 HTTP 响应），
//            且每次缓存未命中都会把 os.Stat 的错误当成上游错误打印。
//   - Bug 3  关闭去重(enable_distinct_file=false)后，SaveUploadFile 把 fileInfo.Md5
//            设为"路径 md5"而非"内容 md5"，导致携带正确 md5 的上传被拒绝/秒传失效。
//   - Bug 5  下载重试守卫 `RetryCount > 0` 在 RetryCount=0 时彻底失效，
//            失败文件会被无限重新入队。
//   - Bug 6  RepairFileInfoFromFile 把本应计算"内容 md5"的 GetFileSumByName 注释掉，
//            直接用"路径 md5"写入 leveldb，导致按内容去重失效。
//   - Bug 8  DownloadFileToResponse 透传上游响应时不校验/不转发状态码，
//            上游返回 404 时客户端仍收到 200。
//
// 未编写运行时失败测试（结构性问题，安全复现会致进程崩溃或不可观测）：
//   - Bug 1  POST 上传在独立 worker goroutine 执行，panic 无法被 ServeHTTP 的
//            recover 捕获，会直接 crash 整个进程。无法在不崩溃测试进程的前提下验证。
//   - Bug 4  upload 中 CheckScene 之后的 `if err != nil { http.Redirect }` 是死代码，
//            运行时无任何可观测差异。
//   - Bug 7  ParseConfig 把栈上局部变量 &c 存入全局指针，依赖编译器逃逸分析；
//            当前能"碰巧"工作，属于脆弱写法，无法稳定复现为失败。
// =============================================================================

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sjqzhang/goutil"
	log "github.com/sjqzhang/seelog"
)

var bugLoggerOnce sync.Once

// ensureLogger 保证 seelog 已初始化，避免在未调用 InitServer 的测试中调用 log.* 出错。
// 同时初始化访问日志 logger `logacc`（HttpProxyHandler.ServeHTTP 的 defer 会用到）。
func ensureLogger() {
	bugLoggerOnce.Do(func() {
		cfg := `<seelog><outputs><console/></outputs></seelog>`
		if logger, err := log.LoggerFromConfigAsBytes([]byte(cfg)); err == nil {
			log.ReplaceLogger(logger)
			logacc = logger
		}
	})
}

// newLdbServer 构造一个带 leveldb 的 Server（不绑定端口、不启动后台协程），
// 并让 STORE_DIR / DOCKER_DIR / leveldb 路径都落在临时目录，避免污染与冲突。
func newLdbServer(t *testing.T) *Server {
	t.Helper()
	ensureLogger()
	tmp := t.TempDir()
	DOCKER_DIR = tmp + "/"
	STORE_DIR = DOCKER_DIR + "files"
	if err := os.MkdirAll(STORE_DIR, 0775); err != nil {
		t.Fatal(err)
	}
	CONST_LEVELDB_FILE_NAME = filepath.Join(tmp, "fileserver.db")
	CONST_LOG_LEVELDB_FILE_NAME = filepath.Join(tmp, "log.db")

	ParseConfig("")
	// 消除未格式化 cfgJson 中的 "%s" 占位符带来的副作用（PeerId 等）
	Config().PeerId = "0"
	Config().Host = "http://127.0.0.1:8080"
	Config().Peers = []string{}
	Config().AdminKey = "test"
	server = nil
	s := NewServer()
	s.host = "http://127.0.0.1:8080"
	return s
}

// contentMD5 计算给定内容的 md5（与开启去重时 SaveUploadFile 计算的一致）。
func contentMD5(t *testing.T, content []byte) string {
	t.Helper()
	f, err := os.CreateTemp("", "bug-content-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(content); err != nil {
		t.Fatal(err)
	}
	f.Close()
	m, err := (&goutil.Common{}).GetFileSumByName(f.Name(), "md5")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// buildUploadRequest 构造一个 multipart POST /upload 请求。
func buildUploadRequest(t *testing.T, content []byte, fields map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range fields {
		fw, err := mw.CreateFormField(k)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = fw.Write([]byte(v)); err != nil {
			t.Fatal(err)
		}
	}
	fw, err := mw.CreateFormFile("file", "hello.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = fw.Write(content); err != nil {
		t.Fatal(err)
	}
	if err = mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/upload", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

// writeCapture 记录 WriteHeader 是否被调用，用于检测"静默返回"。
type writeCapture struct {
	http.ResponseWriter
	wrote bool
	code  int
}

func (w *writeCapture) WriteHeader(c int) {
	w.wrote = true
	w.code = c
	w.ResponseWriter.WriteHeader(c)
}

// -----------------------------------------------------------------------------
// Bug 2：HttpProxyHandler 上游失败时静默返回，且不写任何 HTTP 响应
// -----------------------------------------------------------------------------
func TestBug2ProxySilentFailureOnUpstreamError(t *testing.T) {
	ensureLogger()
	tmp := t.TempDir()
	oldStore := STORE_DIR
	STORE_DIR = tmp // 避免代理在 CWD 下创建 files/ 目录
	defer func() { STORE_DIR = oldStore }()

	// ServeHTTP 的 defer 中使用了全局 server.util.GetClientIp
	server = &Server{util: &goutil.Common{}}

	// 指向一个必然连接失败的源站（connection refused）
	h := &HttpProxyHandler{Proxy: Proxy{Dir: "pypi", Origin: "http://127.0.0.1:1"}}

	req := httptest.NewRequest("GET", "/simple/foo/bar", nil)
	rec := httptest.NewRecorder()
	wc := &writeCapture{ResponseWriter: rec}

	h.ServeHTTP(wc, req)

	// 期望：上游失败时，代理应当返回一个错误响应（如 502/504/500），
	// 而不是什么都不写，让客户端拿到空响应/连接挂起。
	if !wc.wrote {
		t.Errorf("Bug2: 上游请求失败时 HttpProxyHandler 未写入任何 HTTP 状态码（静默返回），"+
			"客户端会收到空响应。body 长度=%d", rec.Body.Len())
	}
	if rec.Body.Len() == 0 && wc.code == 0 {
		t.Errorf("Bug2: 上游失败却未向客户端返回任何内容（状态码与 body 均为空）")
	}
}

// -----------------------------------------------------------------------------
// Bug 3a：去重关闭后 SaveUploadFile 的 md5 是"路径 md5"而非"内容 md5"
// -----------------------------------------------------------------------------
func TestBug3aSaveUploadFileMd5IsPathMd5(t *testing.T) {
	ensureLogger()
	tmp := t.TempDir()
	oldStore := STORE_DIR
	STORE_DIR = tmp
	defer func() { STORE_DIR = oldStore }()

	ParseConfig("")
	Config().EnableDistinctFile = false
	Config().EnableTrimFileNameSpecialChar = false
	Config().EnableCrossOrigin = false
	Config().PeerId = "0"
	Config().Host = "http://127.0.0.1:8080"

	c := &Server{util: &goutil.Common{}, host: "http://127.0.0.1:8080"}

	content := []byte("go-fastdfs bug3 test content, must be unique: 0xCAFEBABE")
	contentMd5 := contentMD5(t, content)

	req := buildUploadRequest(t, content, map[string]string{
		"scene":  "default",
		"output": "text",
	})
	if err := req.ParseMultipartForm(32 << 20); err != nil {
		t.Fatal(err)
	}
	uploadFile, uploadHeader, err := req.FormFile("file")
	if err != nil {
		t.Fatal(err)
	}

	fileInfo := FileInfo{Scene: "default"}
	if _, err = c.SaveUploadFile(uploadFile, uploadHeader, &fileInfo, req); err != nil {
		t.Fatal(err)
	}

	// 正确行为：fileInfo.Md5 应为内容 md5（去重语义一致）。
	// 实际行为（bug）：fileInfo.Md5 为路径 md5（甚至是 Path 尚未赋值时的 "/<name>" 哈希）。
	if fileInfo.Md5 != contentMd5 {
		t.Errorf("Bug3: SaveUploadFile(enable_distinct_file=false) 计算的 md5=%q 不是内容 md5=%q，"+
			"而是基于路径的哈希，导致后续 md5 校验/秒传失效", fileInfo.Md5, contentMd5)
	}
}

// -----------------------------------------------------------------------------
// Bug 3b：去重关闭后，携带正确内容 md5 的 POST 上传被错误拒绝（秒传/校验失效）
// -----------------------------------------------------------------------------
func TestBug3bUploadRejectsValidMd5WhenDistinctDisabled(t *testing.T) {
	s := newLdbServer(t)
	Config().EnableDistinctFile = false
	Config().EnableTrimFileNameSpecialChar = false
	Config().EnableCrossOrigin = false
	Config().EnableCustomPath = false
	Config().EnableMergeSmallFile = false
	Config().Peers = []string{}

	content := []byte("go-fastdfs bug3b upload content, unique: 0xDEADBEEF")
	contentMd5 := contentMD5(t, content)

	// 客户端上传文件，并附带正确的内容 md5（秒传/一致性校验场景）
	req := buildUploadRequest(t, content, map[string]string{
		"md5":    contentMd5,
		"scene":  "default",
		"output": "json",
	})
	rec := httptest.NewRecorder()

	s.upload(rec, req)

	body := rec.Body.String()
	// 正确行为：上传成功，返回 FileResult（含 url 与正确的内容 md5）。
	// bug 表现：body 含 "fileInfo.Md5 and md5sum" 拒绝信息。
	if strings.Contains(body, "md5sum") {
		t.Errorf("Bug3b: 携带正确内容 md5 的上传被拒绝（body=%q）。"+
			"根因：enable_distinct_file=false 时 fileInfo.Md5 被写成路径 md5。", body)
		return
	}
	var fr FileResult
	if err := json.Unmarshal(rec.Body.Bytes(), &fr); err != nil {
		t.Fatalf("Bug3b: 无法解析成功响应 %q: %v", body, err)
	}
	if fr.Url == "" {
		t.Errorf("Bug3b: 上传成功响应缺少 url（body=%q）", body)
	}
	if fr.Md5 != contentMd5 {
		t.Errorf("Bug3b: 返回的 md5=%q 不是内容 md5=%q，去重/秒传校验未能对齐", fr.Md5, contentMd5)
	}
}

// -----------------------------------------------------------------------------
// Bug 5：RetryCount=0 时重试守卫失效，失败文件被无限重新入队
// -----------------------------------------------------------------------------
func TestBug5RetryCountZeroInfiniteRetry(t *testing.T) {
	s := newLdbServer(t)
	Config().EnableDistinctFile = false
	Config().ReadOnly = false

	peer := "http://127.0.0.1:1" // 必然连接失败
	mkInfo := func() *FileInfo {
		return &FileInfo{
			Name:      "bug5.txt",
			Path:      "files/default/20260101/00/00",
			Size:      123,
			OffSet:    -1,
			Peers:     []string{peer},
			TimeStamp: time.Now().Unix(),
		}
	}
	waitOrRequeue := func() bool {
		select {
		case <-s.queueFromPeers:
			return true // 被重新入队
		case <-time.After(200 * time.Millisecond):
			return false // 已放弃
		}
	}

	// 对照：RetryCount=1 且已重试 1 次，应当放弃（不再入队）。
	Config().RetryCount = 1
	fi := mkInfo()
	fi.retry = 1
	s.queueFromPeers = make(chan FileInfo, 10000)
	s.DownloadFromPeer(peer, fi)
	if waitOrRequeue() {
		t.Errorf("Bug5 对照项异常：RetryCount=1 且 retry=1 时仍被重新入队（应已放弃）")
	}

	// Bug：RetryCount=0 时守卫 `RetryCount > 0` 恒假，重试上限被关闭，
	// 失败文件会被无限重新入队（此处只调用一次即能看到重新入队）。
	Config().RetryCount = 0
	fi = mkInfo()
	fi.retry = 1
	s.queueFromPeers = make(chan FileInfo, 10000)
	s.DownloadFromPeer(peer, fi)
	if waitOrRequeue() {
		t.Errorf("Bug5: RetryCount=0 导致重试上限失效，失败文件被重新入队（无限重试）。" +
			"守卫条件 `Config().RetryCount > 0` 在 RetryCount=0 时错误地禁用了重试次数限制。")
	}
}

// -----------------------------------------------------------------------------
// Bug 6：RepairFileInfoFromFile 用"路径 md5"而非"内容 md5"索引文件
// -----------------------------------------------------------------------------
func TestBug6RepairStoresPathMd5InsteadOfContentMd5(t *testing.T) {
	s := newLdbServer(t)

	content := []byte("go-fastdfs bug6 repair content, unique: 0x13579BDF")
	name := "bug6_test.txt"
	fpath := filepath.Join(STORE_DIR, name)
	if err := os.WriteFile(fpath, content, 0644); err != nil {
		t.Fatal(err)
	}
	contentMd5, err := s.util.GetFileSumByName(fpath, "md5")
	if err != nil {
		t.Fatal(err)
	}

	// 触发修复：扫描 STORE_DIR 并把文件信息写入 leveldb
	s.RepairFileInfoFromFile()

	// 修复后修复逻辑按"内容 md5"索引，因此应当能直接用内容 md5 检索到。
	stored, err := s.GetFileInfoFromLevelDB(contentMd5)
	if err != nil {
		t.Fatalf("Bug6: 修复后未能以内容 md5 找到记录（err=%v），"+
			"因为 repair 错误地用路径 md5 做了索引", err)
	}
	// 正确行为：stored.Md5 应为内容 md5。
	if stored.Md5 != contentMd5 {
		t.Errorf("Bug6: RepairFileInfoFromFile 写入的 md5=%q，应为内容 md5=%q，"+
			"导致按内容去重/秒传失效", stored.Md5, contentMd5)
	}
}

// -----------------------------------------------------------------------------
// Bug 8：DownloadFileToResponse 透传上游响应时不转发状态码（404 被吞成 200）
// -----------------------------------------------------------------------------
func TestBug8DownloadFileToResponseIgnoresUpstreamStatus(t *testing.T) {
	// 上游固定返回 404
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("upstream 404 not found"))
	}))
	defer ts.Close()

	req := httptest.NewRequest("GET", "/group1/files/bug8.txt", nil)
	rec := httptest.NewRecorder()

	// DownloadFileToResponse 不依赖接收者 c 的字段，用空结构体即可
	(&Server{}).DownloadFileToResponse(ts.URL, rec, req)

	// 正确行为：应把上游的 404 状态码转发给客户端。
	// 实际行为（bug）：只 io.Copy 了 body，状态码保持默认 200。
	if rec.Code != http.StatusNotFound {
		t.Errorf("Bug8: 上游返回 404，但 DownloadFileToResponse 未转发状态码，"+
			"客户端收到 %d（body=%q）。应转发上游状态码。", rec.Code, rec.Body.String())
	}
}
