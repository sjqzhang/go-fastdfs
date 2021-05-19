package server

import (
	"fmt"
	"net/http"
)

func (c *Server) initRouter() {
	groupRoute := ""
	if Config().SupportGroupManage {
		groupRoute = "/" + Config().Group
	}
	uploadPage := "upload.html"
	if groupRoute == "" {
		http.HandleFunc(fmt.Sprintf("%s", "/"), c.Download)
		http.HandleFunc(fmt.Sprintf("/%s", uploadPage), c.Index)
	} else {
		http.HandleFunc(fmt.Sprintf("%s", "/"), c.Download)
		http.HandleFunc(fmt.Sprintf("%s", groupRoute), c.Download)
		http.HandleFunc(fmt.Sprintf("%s/%s", groupRoute, uploadPage), c.Index)
	}
	http.HandleFunc(fmt.Sprintf("%s/check_files_exist", groupRoute), c.CheckFilesExist)
	http.HandleFunc(fmt.Sprintf("%s/check_file_exist", groupRoute), c.CheckFileExist)
	http.HandleFunc(fmt.Sprintf("%s/upload", groupRoute), c.Upload)
	http.HandleFunc(fmt.Sprintf("%s/delete", groupRoute), c.RemoveFile)
	http.HandleFunc(fmt.Sprintf("%s/get_file_info", groupRoute), c.GetFileInfo)
	http.HandleFunc(fmt.Sprintf("%s/sync", groupRoute), c.Sync)
	http.HandleFunc(fmt.Sprintf("%s/stat", groupRoute), c.Stat)
	http.HandleFunc(fmt.Sprintf("%s/repair_stat", groupRoute), c.RepairStatWeb)
	http.HandleFunc(fmt.Sprintf("%s/status", groupRoute), c.Status)
	http.HandleFunc(fmt.Sprintf("%s/repair", groupRoute), c.Repair)
	http.HandleFunc(fmt.Sprintf("%s/report", groupRoute), c.Report)
	http.HandleFunc(fmt.Sprintf("%s/backup", groupRoute), c.BackUp)
	http.HandleFunc(fmt.Sprintf("%s/search", groupRoute), c.Search)
	http.HandleFunc(fmt.Sprintf("%s/list_dir", groupRoute), c.ListDir)
	http.HandleFunc(fmt.Sprintf("%s/remove_empty_dir", groupRoute), c.RemoveEmptyDir)
	http.HandleFunc(fmt.Sprintf("%s/repair_fileinfo", groupRoute), c.RepairFileInfo)
	http.HandleFunc(fmt.Sprintf("%s/reload", groupRoute), c.Reload)
	http.HandleFunc(fmt.Sprintf("%s/syncfile_info", groupRoute), c.SyncFileInfo)
	http.HandleFunc(fmt.Sprintf("%s/get_md5s_by_date", groupRoute), c.GetMd5sForWeb)
	http.HandleFunc(fmt.Sprintf("%s/receive_md5s", groupRoute), c.ReceiveMd5s)
	http.HandleFunc(fmt.Sprintf("%s/gen_google_secret", groupRoute), c.GenGoogleSecret)
	http.HandleFunc(fmt.Sprintf("%s/gen_google_code", groupRoute), c.GenGoogleCode)
	http.Handle(fmt.Sprintf("%s/static/", groupRoute), http.StripPrefix(fmt.Sprintf("%s/static/", groupRoute), http.FileServer(http.Dir("./static"))))
	http.HandleFunc("/"+Config().Group+"/", c.Download)
}
