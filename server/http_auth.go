package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	log "github.com/sjqzhang/seelog"
)

func (c *Server) CheckAuth(w http.ResponseWriter, r *http.Request) bool {
	var (
		err        error
		req        *httplib.BeegoHTTPRequest
		result     string
		jsonResult JsonResult
	)
	if err = r.ParseForm(); err != nil {
		log.Error(err)
		return false
	}
	req = httplib.Post(Config().AuthUrl)
	req.SetTimeout(time.Second*10, time.Second*10)
	req.Param("__path__", r.URL.Path)
	req.Param("__query__", r.URL.RawQuery)
	for k, _ := range r.Form {
		req.Param(k, r.FormValue(k))
	}
	for k, v := range r.Header {
		req.Header(k, v[0])
	}
	result, err = req.String()
	result = strings.TrimSpace(result)
	if strings.HasPrefix(result, "{") && strings.HasSuffix(result, "}") {
		if err = json.Unmarshal([]byte(result), &jsonResult); err != nil {
			log.Error(err)
			return false
		}
		if jsonResult.Data != "ok" {
			log.Warn(result)
			return false
		}
	} else {
		if result != "ok" {
			log.Warn(result)
			return false
		}
	}
	return true
}

func (c *Server) NotPermit(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(401)
}
