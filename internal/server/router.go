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
	// Static assets like js and css files
	app.Static("/"+config.STORE_DIR_NAME, config.CommonConfig.AbsRunningDir)
	v1 := app.Group(groupRoute)
	{
		v1.GET("/index", model.Svr.Index)
		v1.GET("/download", model.Svr.Download)
		// curl http://47.97.117.196:9090/test/check_file_exist?md5=b628f8ef4bc0dce120788ab91aaa3ebb
		// curl http://47.97.117.196:9090/test/check_file_exist?path=files/v1.0.0/bbs.log.txt
		// TODO fix: no error message return, what does offset means? the peer port is wrong
		v1.GET("/check_files_exist", model.Svr.CheckFileExist)
		v1.GET("/check_file_exist")
		v1.GET("/file-info")
		v1.GET("/sync")
		v1.GET("/stat")
		v1.GET("/repair")
		v1.GET("/report")
		v1.GET("/backup")
		v1.GET("/search")
		v1.GET("/list-dir")
		v1.GET("/get_md5s_by_date")
		v1.GET("/receive_md5s") //?
		v1.GET("/stat")

		v1.POST("/upload")
		v1.POST("/gen_google_secret")
		v1.POST("/gen_google_code")

		v1.PUT("/reload")
		v1.PUT("/repair_fileinfo")
		v1.PUT("/syncfile_info")

		// TODO fix: the URLGet if not good
		v1.DELETE("/delete", model.Svr.RemoveFile)
		v1.DELETE("/remove_empty_dir", model.Svr.RemoveEmptyDir)
	}

	// Default HTML page (client-side routing implemented via Vue.js)
	app.NoRoute(func(c *gin.Context) {
		//c.HTML(http.StatusOK, "index.tmpl", gin.H{"clientConfig": config.PublicClientConfig()})
		c.Redirect(http.StatusOK, "index")
	})
}
