package server

import (
	random "math/rand"
	"net/http"
	"time"

	"github.com/sjqzhang/googleAuthenticator"
	log "github.com/sjqzhang/seelog"
)

func (c *Server) VerifyGoogleCode(secret string, code string, discrepancy int64) bool {
	var (
		goauth *googleAuthenticator.GAuth
	)
	goauth = googleAuthenticator.NewGAuth()
	if ok, err := goauth.VerifyCode(secret, code, discrepancy); ok {
		return ok
	} else {
		log.Error(err)
		return ok
	}
}

func (c *Server) GenGoogleCode(w http.ResponseWriter, r *http.Request) {
	var (
		err    error
		result JsonResult
		secret string
		goauth *googleAuthenticator.GAuth
	)
	r.ParseForm()
	goauth = googleAuthenticator.NewGAuth()
	secret = r.FormValue("secret")
	result.Status = "ok"
	result.Message = "ok"
	if !c.IsPeer(r) {
		result.Message = c.GetClusterNotPermitMessage(r)
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	if result.Data, err = goauth.GetCode(secret); err != nil {
		result.Message = err.Error()
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	w.Write([]byte(c.util.JsonEncodePretty(result)))
}

func (c *Server) GenGoogleSecret(w http.ResponseWriter, r *http.Request) {
	var (
		result JsonResult
	)
	result.Status = "ok"
	result.Message = "ok"
	if !c.IsPeer(r) {
		result.Message = c.GetClusterNotPermitMessage(r)
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	GetSeed := func(length int) string {
		seeds := "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
		s := ""
		random.Seed(time.Now().UnixNano())
		for i := 0; i < length; i++ {
			s += string(seeds[random.Intn(32)])
		}
		return s
	}
	result.Data = GetSeed(16)
	w.Write([]byte(c.util.JsonEncodePretty(result)))
}
