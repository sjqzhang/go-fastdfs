package server

import (
	"fmt"
	"net/http"
	"net/url"
)

func (c *Server) CrossOrigin(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Origin") != "" {
		w.Header().Set("Access-Control-Allow-Origin", r.Header.Get("Origin"))
		w.Header().Set("Access-Control-Allow-Credentials", "true")
	} else {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		r.Header.Set("Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Depth, User-Agent, X-File-Size, X-Requested-With, X-Requested-By, If-Modified-Since, X-File-Name, X-File-Type, Cache-Control, Origin")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
	w.Header().Set("Access-Control-Expose-Headers", "Authorization")
	//https://blog.csdn.net/yanzisu_congcong/article/details/80552155
}

func (c *Server) SetDownloadHeader(w http.ResponseWriter, r *http.Request, isSmall bool) {
	w.Header().Set("Content-Type", "application/octet-stream")
	if name, ok := r.URL.Query()["name"]; ok {
		if v, err := url.QueryUnescape(name[0]); err == nil {
			if isSmall {
				name[0] = c.TrimFileNameSpecialChar(v)
			} else {
				name[0] = v
			}
		}
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment;filename=%s", name[0]))
	} else {
		w.Header().Set("Content-Disposition", "attachment")
	}
}
