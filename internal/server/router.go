package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/internal/model"
)

func registerRoutes(app *gin.Engine) {
	// JSON-REST API Version 1
	groupRoute := ""
	if config.CommonConfig.SupportGroupManage {
		groupRoute = "/" + config.CommonConfig.Group
	}
	app.Static("/"+config.StoreDirName, config.CommonConfig.AbsRunningDir)
	// http.Dir allows to list the files in the given dir, and can not set
	// groupRoute path is not allowed, conflict with normal api
	app.StaticFS("/file", http.Dir(config.CommonConfig.AbsRunningDir+"/"+config.StoreDirName))
	// gin.Dir can set  if allows to list the files in the given dir
	app.StaticFS("/file", gin.Dir(config.CommonConfig.AbsRunningDir+"/"+config.StoreDirName, false))

	v1 := app.Group(groupRoute)
	{
		v1.GET("/index", model.Svr.Index)
		v1.GET("/download", model.Svr.Download)
		// curl http://ip:9090/test/check_file_exist?md5=b628f8ef4bc0dce120788ab91aaa3ebb
		// curl http://ip:9090/test/check_file_exist?path=files/v1.0.0/bbs.log.txt
		// TODO fix: no error message return, what does offset means? the peer port is wrong
		v1.GET("/check_files_exist", model.Svr.CheckFilesExist)
		v1.GET("/check_file_exist", model.Svr.CheckFileExist)
		v1.GET("/file-info", model.Svr.GetFileInfo)
		v1.GET("/sync", model.Svr.Sync)
		v1.GET("/stat", model.Svr.Stat)
		v1.GET("/repair", model.Svr.Repair)
		v1.GET("/report", model.Svr.Report)
		v1.GET("/backup", model.Svr.BackUp)
		v1.GET("/search", model.Svr.Search)
		v1.GET("/list-dir", model.Svr.ListDir)
		v1.GET("/get_md5s_by_date", model.Svr.GetMd5sForWeb)
		v1.GET("/receive_md5s", model.Svr.ReceiveMd5s) //?

		v1.POST("/upload", model.Svr.Upload)
		v1.POST("/gen_google_secret", model.Svr.GenGoogleSecret)
		v1.POST("/gen_google_code", model.Svr.GenGoogleCode)

		v1.PUT("/reload", model.Svr.Reload)
		v1.PUT("/repair_fileinfo", model.Svr.RepairFileInfo)
		v1.PUT("/syncfile_info", model.Svr.SyncFileInfo)

		v1.DELETE("/delete", model.Svr.RemoveFile)
		v1.DELETE("/remove_empty_dir", model.Svr.RemoveEmptyDir)
	}

	// Default HTML page (client-side routing implemented via Vue.js)
	app.NoRoute(func(c *gin.Context) {
		//c.HTML(http.StatusOK, "index.tmpl", gin.H{"clientConfig": config.PublicClientConfig()})
		c.Redirect(http.StatusOK, "index")
	})
}
