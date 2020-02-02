package api

import (
	"net/http"
	"net/http/httptest"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/internal/model"
)

var (
	TestConf   *config.Config
	TestRouter *gin.RouterGroup
	TestApp    *gin.Engine
	once       sync.Once
)

// API test helper
func NewApiTest() {
	once.Do(func() {
		TestConf = config.NewConfig()
		TestApp = gin.New()
		gin.SetMode(gin.TestMode)
		TestRouter = TestApp.Group("/")

		model.Svr = model.NewServer(TestConf)
		model.Svr.InitComponent(false, TestConf)
		svr := model.Svr
		go svr.ConsumerUpload(TestConf)
	})
}

// See https://medium.com/@craigchilds94/testing-gin-json-responses-1f258ce3b0b1
func PerformRequest(r http.Handler, method, path string) *httptest.ResponseRecorder {
	req, _ := http.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
