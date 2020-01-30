package model

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/sjqzhang/googleAuthenticator"
)

// CheckAuth
func CheckAuth(r *http.Request) bool {
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
	req = httplib.Post(config.CommonConfig.AuthUrl)
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

func VerifyGoogleCode(secret string, code string, discrepancy int64) bool {
	var (
		gAuth *googleAuthenticator.GAuth
	)
	gAuth = googleAuthenticator.NewGAuth()
	if ok, err := gAuth.VerifyCode(secret, code, discrepancy); ok {
		return ok
	} else {
		log.Error(err)
		return ok
	}
}

func (svr *Server) CheckDownloadAuth(ctx *gin.Context) (bool, error) {
	var (
		err          error
		maxTimestamp int64
		minTimestamp int64
		ts           int64
		token        string
		timestamp    string
		fullPath     string
		smallPath    string
		pathMd5      string
		fileInfo     *FileInfo
		scene        string
		secret       interface{}
		code         string
		ok           bool
	)
	r := ctx.Request
	if config.CommonConfig.EnableDownloadAuth && config.CommonConfig.AuthUrl != "" && !IsPeer(r) && !CheckAuth(r) {
		return false, errors.New("auth fail")
	}
	if config.CommonConfig.DownloadUseToken && !IsPeer(r) {
		token = r.FormValue("token")
		timestamp = r.FormValue("timestamp")
		if token == "" || timestamp == "" {
			return false, errors.New("invalid request")
		}
		maxTimestamp = time.Now().Add(time.Second *
			time.Duration(config.CommonConfig.DownloadTokenExpire)).Unix()
		minTimestamp = time.Now().Add(-time.Second *
			time.Duration(config.CommonConfig.DownloadTokenExpire)).Unix()
		if ts, err = strconv.ParseInt(timestamp, 10, 64); err != nil {
			return false, errors.New("invalid timestamp")
		}
		if ts > maxTimestamp || ts < minTimestamp {
			return false, errors.New("timestamp expire")
		}
		fullPath, smallPath = GetFilePathFromRequest(ctx)
		if smallPath != "" {
			pathMd5 = pkg.MD5(smallPath)
		} else {
			pathMd5 = pkg.MD5(fullPath)
		}
		if fileInfo, err = svr.GetFileInfoFromLevelDB(pathMd5); err != nil {
			// TODO
		} else {
			if !(pkg.MD5(fileInfo.Md5+timestamp) == token) {
				return ok, errors.New("invalid token")
			}
			return ok, nil
		}
	}
	if config.CommonConfig.EnableGoogleAuth && !IsPeer(r) {
		fullPath = r.RequestURI[2:len(r.RequestURI)]
		fullPath = strings.Split(fullPath, "?")[0] // just path
		scene = strings.Split(fullPath, "/")[0]
		code = r.FormValue("code")
		if secret, ok = svr.sceneMap.GetValue(scene); ok {
			if !VerifyGoogleCode(secret.(string), code, int64(config.CommonConfig.DownloadTokenExpire/30)) {
				return false, errors.New("invalid google code")
			}
		}
	}
	return true, nil
}
