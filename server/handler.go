package server

import (
	"crypto/tls"
	"fmt"
	"github.com/astaxie/beego/httplib"
	"io"
	"net"
	"net/http"
	"os"
	"path"
	"runtime/debug"
	"strings"
	"time"

	log "github.com/sjqzhang/seelog"
)

type HttpHandler struct{}

func (HttpHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	status_code := "200"
	defer func(t time.Time) {
		logStr := fmt.Sprintf("[Access] %s | %s | %s | %s | %s |%s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			//res.Header(),
			time.Since(t).String(),
			server.util.GetClientIp(req),
			req.Method,
			status_code,
			req.RequestURI,
		)
		logacc.Info(logStr)
	}(time.Now())
	defer func() {
		if err := recover(); err != nil {
			status_code = "500"
			res.WriteHeader(500)
			print(err)
			buff := debug.Stack()
			log.Error(err)
			log.Error(string(buff))
		}
	}()
	if Config().EnableCrossOrigin {
		server.CrossOrigin(res, req)
	}
	//http.DefaultServeMux.ServeHTTP(res, req)
	mux.ServeHTTP(res,req)
}

type HttpProxyHandler struct {
	Proxy Proxy
}


func (h *HttpProxyHandler)handleTunneling(w http.ResponseWriter, r *http.Request) {
	dest_conn, err := net.DialTimeout("tcp", r.Host, 10*time.Second)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	w.WriteHeader(http.StatusOK)
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
		return
	}
	client_conn, _, err := hijacker.Hijack()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
	}
	go h.transfer(dest_conn, client_conn)
	go h.transfer(client_conn, dest_conn)
}
func (h *HttpProxyHandler)transfer(destination io.WriteCloser, source io.ReadCloser) {
	defer destination.Close()
	defer source.Close()
	io.Copy(destination, source)
}
func (h *HttpProxyHandler)handleHTTP(w http.ResponseWriter, req *http.Request) {
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	defer resp.Body.Close()
	h.copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}
func (h *HttpProxyHandler)copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func (h *HttpProxyHandler) ServeHTTP(res http.ResponseWriter, req *http.Request) {
	status_code := "200"
	defer func(t time.Time) {
		logStr := fmt.Sprintf("[Access] %s | %s | %s | %s | %s |%s",
			time.Now().Format("2006/01/02 - 15:04:05"),
			//res.Header(),
			time.Since(t).String(),
			server.util.GetClientIp(req),
			req.Method,
			status_code,
			req.RequestURI,
		)
		logacc.Info(logStr)
	}(time.Now())
	defer func() {
		if err := recover(); err != nil {
			status_code = "500"
			res.WriteHeader(500)
			print(err)
			buff := debug.Stack()
			log.Error(err)
			log.Error(string(buff))
		}
	}()
	//if Config().EnableCrossOrigin {
	//	server.CrossOrigin(res, req)
	//}
	if req.Method == http.MethodConnect {
		h.handleTunneling(res, req)
		return
	}
	href := strings.TrimRight(h.Proxy.Origin, "/") + req.RequestURI
	md5 := server.util.MD5(href)
	fpath := STORE_DIR + "/" + h.Proxy.Dir + "/" + md5[0:2] + "/" + md5[2:4] + "/" + md5
	_, err := os.Stat(fpath)
	if err == nil {
		fp, err := os.Open(fpath)
		if err != nil {
			log.Error(err)
			return
		}
		defer fp.Close()
		io.Copy(res, fp)
		return
	}
	go func(href string) {
		os.MkdirAll(path.Dir(fpath), 0755)
		err := httplib.Get(href).ToFile(fpath)
		if err == nil {
			fi, err := os.Stat(fpath)
			if err == nil {
				fileInfo := FileInfo{
					Name: fi.Name(),
					Size: fi.Size(),
					Path: path.Dir(fpath),//  files/default/20211222/15/57/1
					TimeStamp: fi.ModTime().Unix(),
					Scene: Config().DefaultScene,
					Peers: []string{Config().Host},  // ["http://10.12.188.85:8080"]
				}
				server.postFileToPeer(&fileInfo)
			}
		}
	}(href)
	r := httplib.Get(href)
	r.SetTLSClientConfig(&tls.Config{InsecureSkipVerify: true})
	if err != nil {
		log.Error(err)
	}
	response, err := r.DoRequest()
	if err != nil {
		return
	}
	defer response.Body.Close()
	io.Copy(res, response.Body)

}
