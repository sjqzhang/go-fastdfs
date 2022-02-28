package server

import (
	"fmt"
	"net/http"
)

var mux *http.ServeMux

func (c *Server) initRouter() {
	groupRoute := ""
	if Config().SupportGroupManage && Config().Group != "" {
		groupRoute = "/" + Config().Group
	}
	uploadPage := "upload.html"
	if groupRoute == "" {
		mux.HandleFunc(fmt.Sprintf("%s", "/"), c.Download)
		mux.HandleFunc(fmt.Sprintf("/%s", uploadPage), c.Index)
	} else {
		mux.HandleFunc(fmt.Sprintf("%s", "/"), c.Download)
		mux.HandleFunc(fmt.Sprintf("%s", groupRoute), c.Download)
		mux.HandleFunc(fmt.Sprintf("%s/%s", groupRoute, uploadPage), c.Index)
	}
	mux.HandleFunc(fmt.Sprintf("%s/check_files_exist", groupRoute), c.CheckFilesExist)
	mux.HandleFunc(fmt.Sprintf("%s/check_file_exist", groupRoute), c.CheckFileExist)
	mux.HandleFunc(fmt.Sprintf("%s/upload", groupRoute), c.Upload)
	mux.HandleFunc(fmt.Sprintf("%s/delete", groupRoute), c.RemoveFile)
	mux.HandleFunc(fmt.Sprintf("%s/get_file_info", groupRoute), c.GetFileInfo)
	mux.HandleFunc(fmt.Sprintf("%s/sync", groupRoute), c.Sync)
	mux.HandleFunc(fmt.Sprintf("%s/stat", groupRoute), c.Stat)
	mux.HandleFunc(fmt.Sprintf("%s/repair_stat", groupRoute), c.RepairStatWeb)
	mux.HandleFunc(fmt.Sprintf("%s/status", groupRoute), c.Status)
	mux.HandleFunc(fmt.Sprintf("%s/repair", groupRoute), c.Repair)
	mux.HandleFunc(fmt.Sprintf("%s/report", groupRoute), c.Report)
	mux.HandleFunc(fmt.Sprintf("%s/backup", groupRoute), c.BackUp)
	mux.HandleFunc(fmt.Sprintf("%s/search", groupRoute), c.Search)
	mux.HandleFunc(fmt.Sprintf("%s/list_dir", groupRoute), c.ListDir)
	mux.HandleFunc(fmt.Sprintf("%s/remove_empty_dir", groupRoute), c.RemoveEmptyDir)
	mux.HandleFunc(fmt.Sprintf("%s/repair_fileinfo", groupRoute), c.RepairFileInfo)
	mux.HandleFunc(fmt.Sprintf("%s/reload", groupRoute), c.Reload)
	mux.HandleFunc(fmt.Sprintf("%s/syncfile_info", groupRoute), c.SyncFileInfo)
	mux.HandleFunc(fmt.Sprintf("%s/get_md5s_by_date", groupRoute), c.GetMd5sForWeb)
	mux.HandleFunc(fmt.Sprintf("%s/receive_md5s", groupRoute), c.ReceiveMd5s)
	mux.HandleFunc(fmt.Sprintf("%s/gen_google_secret", groupRoute), c.GenGoogleSecret)
	mux.HandleFunc(fmt.Sprintf("%s/gen_google_code", groupRoute), c.GenGoogleCode)
	mux.Handle(fmt.Sprintf("%s/static/", groupRoute), http.StripPrefix(fmt.Sprintf("%s/static/", groupRoute), http.FileServer(http.Dir("./static"))))
	mux.HandleFunc("/"+Config().Group+"/", c.Download)
}
