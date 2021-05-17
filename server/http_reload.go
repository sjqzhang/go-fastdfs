package server

import (
	log "github.com/sjqzhang/seelog"
	"io/ioutil"
	"net/http"
)

func (c *Server) Reload(w http.ResponseWriter, r *http.Request) {
	var (
		err     error
		data    []byte
		cfg     GlobalConfig
		action  string
		cfgjson string
		result  JsonResult
	)
	result.Status = "fail"
	r.ParseForm()
	if !c.IsPeer(r) {
		w.Write([]byte(c.GetClusterNotPermitMessage(r)))
		return
	}
	cfgjson = r.FormValue("cfg")
	action = r.FormValue("action")
	_ = cfgjson
	if action == "get" {
		result.Data = Config()
		result.Status = "ok"
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	if action == "set" {
		if cfgjson == "" {
			result.Message = "(error)parameter cfg(json) require"
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		if err = json.Unmarshal([]byte(cfgjson), &cfg); err != nil {
			log.Error(err)
			result.Message = err.Error()
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		result.Status = "ok"
		cfgjson = c.util.JsonEncodePretty(cfg)
		c.util.WriteFile(CONST_CONF_FILE_NAME, cfgjson)
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	if action == "reload" {
		if data, err = ioutil.ReadFile(CONST_CONF_FILE_NAME); err != nil {
			result.Message = err.Error()
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		if err = json.Unmarshal(data, &cfg); err != nil {
			result.Message = err.Error()
			w.Write([]byte(c.util.JsonEncodePretty(result)))
			return
		}
		ParseConfig(CONST_CONF_FILE_NAME)
		c.initComponent(true)
		result.Status = "ok"
		w.Write([]byte(c.util.JsonEncodePretty(result)))
		return
	}
	if action == "" {
		w.Write([]byte("(error)action support set(json) get reload"))
	}
}