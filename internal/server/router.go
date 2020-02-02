package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/api"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/internal/model"
	"github.com/luoyunpeng/go-fastdfs/pkg"
)

func registerRoutes(app *gin.Engine, conf *config.Config) {
	if conf.EnableCrossOrigin() {
		app.Use(pkg.CrossOrigin)
	}
	// http.Dir allows to list the files in the given dir, and can not set
	// groupRoute path is not allowed, conflict with normal api
	// app.StaticFS("/file", http.Dir(config.CommonConfig.AbsRunningDir+"/"+config.StoreDirName))
	// gin.Dir can set  if allows to list the files in the given dir
	app.StaticFS(conf.FileDownloadPathPrefix(), gin.Dir(conf.StoreDirName(), false))
	app.LoadHTMLGlob("static/*")
	// JSON-REST API Version 1
	v1 := app.Group("/")
	{
		api.Index("/index", v1, conf)
		api.Download("", v1, conf)
		// curl http://ip:9090/test/check_file_exist?md5=b628f8ef4bc0dce120788ab91aaa3ebb
		// curl http://ip:9090/test/check_file_exist?path=files/v1.0.0/bbs.log.txt
		// TODO fix: no error message return, what does offset means? the peer port is wrong
		api.CheckFilesExist("/check_files_exist", v1, conf)
		api.CheckFileExist("/check_file_exist", v1, conf)
		model.Svr.GetFileInfo("/info", v1, conf)
		model.Svr.Sync("/sync", v1, conf)
		model.Svr.Stat("/stat", v1, conf)
		api.Report("/report", v1, conf)
		api.Search("/search", v1, conf)
		api.ListDir("/list-dir", v1, conf)

		api.GetMd5sForWeb("/get_md5s_by_date", v1, conf)
		model.Svr.ReceiveMd5s("/receive_md5s", v1, conf) // ?

		api.Upload("/upload", v1, conf)
		api.Repair("/repair", v1, conf)
		api.BackUp("/backup", v1, conf)
		api.GenGoogleSecret("/gen_google_secret", v1, conf)
		api.GenGoogleCode("/gen_google_code", v1, conf)

		api.Reload("/reload", v1, conf)
		api.RepairFileInfo("/repair_fileinfo", v1, conf)
		model.Svr.SyncFileInfo("/syncfile_info", v1, conf)

		api.RemoveFile("/delete", v1, conf)
		api.RemoveEmptyDir("/remove_empty_dir", v1, conf)
	}

	app.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusMisdirectedRequest, "upload.tmpl", gin.H{"title": "Upload website"})
		//c.Redirect(http.StatusOK, "index")
	})
}
