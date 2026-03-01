package server

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/astaxie/beego/httplib"
	log "github.com/sjqzhang/seelog"
)

// authCacheItem stores cached authentication result
type authCacheItem struct {
	result   bool
	expireAt time.Time
}

// authCache stores authentication results to reduce auth_url calls
var authCache sync.Map

// init starts a background goroutine to periodically clean expired cache entries
func init() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		for range ticker.C {
			now := time.Now()
			authCache.Range(func(key, value interface{}) bool {
				if item, ok := value.(*authCacheItem); ok && item.expireAt.Before(now) {
					authCache.Delete(key)
				}
				return true
			})
		}
	}()
}

func (c *Server) CheckAuth(w http.ResponseWriter, r *http.Request) bool {
	var (
		err        error
		req        *httplib.BeegoHTTPRequest
		result     string
		jsonResult JsonResult
	)

	// Get authentication token from request header
	authToken := r.Header.Get("Auth-Token")
	if authToken == "" {
		log.Warn("auth_token is missing")
		return false
	}

	// Check cache first to avoid repeated auth_url calls
	if cached, ok := authCache.Load(authToken); ok {
		if item, ok := cached.(*authCacheItem); ok && item.expireAt.After(time.Now()) {
			return item.result // Cache hit, return cached result
		}
		authCache.Delete(authToken) // Clean expired entry
	}

	// Call auth_url for authentication
	req = httplib.Post(Config().AuthUrl)
	req.SetTimeout(time.Second*10, time.Second*10)
	for k := range r.Form {
		req.Param(k, r.FormValue(k))
	}
	for k, v := range r.Header {
		req.Header(k, v[0])
	}
	result, err = req.String()
	result = strings.TrimSpace(result)

	var authResult bool
	if strings.HasPrefix(result, "{") && strings.HasSuffix(result, "}") {
		if err = json.Unmarshal([]byte(result), &jsonResult); err != nil {
			log.Error(err)
			authResult = false
		} else {
			authResult = jsonResult.Data == "ok"
		}
	} else {
		authResult = result == "ok"
	}

	// Cache the result
	ttl := time.Duration(Config().DownloadTokenExpire) * time.Second
	if ttl <= 0 {
		ttl = 5 * time.Minute // Default 5 minutes
	}
	authCache.Store(authToken, &authCacheItem{
		result:   authResult,
		expireAt: time.Now().Add(ttl),
	})

	if !authResult {
		log.Warn(result)
	}
	return authResult
}

func (c *Server) NotPermit(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(401)
}
