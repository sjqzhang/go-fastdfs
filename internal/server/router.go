package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/api"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/internal/model"
	"github.com/luoyunpeng/go-fastdfs/pkg"
)

func registerRoutes(app *gin.Engine) {
	if config.CommonConfig.EnableCrossOrigin {
		app.Use(pkg.CrossOrigin)
	}
	app.Static("/"+config.StoreDirName, config.CommonConfig.AbsRunningDir)
	// http.Dir allows to list the files in the given dir, and can not set
	// groupRoute path is not allowed, conflict with normal api
	// app.StaticFS("/file", http.Dir(config.CommonConfig.AbsRunningDir+"/"+config.StoreDirName))
	// gin.Dir can set  if allows to list the files in the given dir
	app.StaticFS("/file", gin.Dir(config.CommonConfig.AbsRunningDir+"/"+config.StoreDirName, false))
	app.LoadHTMLGlob("static/*")
	// JSON-REST API Version 1
	v1 := app.Group("/")

	{
		v1.GET("/index", api.Index)
		v1.GET("/download", api.Download)
		// curl http://ip:9090/test/check_file_exist?md5=b628f8ef4bc0dce120788ab91aaa3ebb
		// curl http://ip:9090/test/check_file_exist?path=files/v1.0.0/bbs.log.txt
		// TODO fix: no error message return, what does offset means? the peer port is wrong
		v1.GET("/check_files_exist", api.CheckFilesExist)
		v1.GET("/check_file_exist", api.CheckFileExist)
		v1.GET("/file-info", model.Svr.GetFileInfo)
		v1.GET("/sync", model.Svr.Sync)
		v1.GET("/stat", model.Svr.Stat)
		v1.GET("/repair", api.Repair)
		v1.GET("/report", api.Report)
		v1.GET("/backup", api.BackUp)
		v1.GET("/search", api.Search)
		v1.GET("/list-dir", api.ListDir)
		v1.GET("/get_md5s_by_date", api.GetMd5sForWeb)
		v1.GET("/receive_md5s", model.Svr.ReceiveMd5s) //?

		v1.POST("/upload", api.Upload)
		v1.POST("/gen_google_secret", api.GenGoogleSecret)
		v1.POST("/gen_google_code", api.GenGoogleCode)

		v1.PUT("/reload", api.Reload)
		v1.PUT("/repair_fileinfo", api.RepairFileInfo)
		v1.PUT("/syncfile_info", model.Svr.SyncFileInfo)

		v1.DELETE("/delete", api.RemoveFile)
		v1.DELETE("/remove_empty_dir", api.RemoveEmptyDir)
	}

	app.NoRoute(func(c *gin.Context) {
		c.HTML(http.StatusOK, "upload.tmpl", gin.H{"title": "Upload website"})
		//c.Redirect(http.StatusOK, "index")
	})
}
