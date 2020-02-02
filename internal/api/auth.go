package api

import (
	random "math/rand"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/internal/model"
	"github.com/sjqzhang/googleAuthenticator"
)

// GenGoogleCode generate google code
func GenGoogleCode(path string, router *gin.RouterGroup, conf *config.Config) {
	router.POST(path, func(ctx *gin.Context) {
		var (
			err    error
			result model.JsonResult
			secret string
			goAuth *googleAuthenticator.GAuth
		)

		r := ctx.Request
		goAuth = googleAuthenticator.NewGAuth()
		secret = ctx.Query("secret")
		result.Status = "ok"
		result.Message = "ok"
		if !model.IsPeer(r, conf) {
			result.Message = model.GetClusterNotPermitMessage(r)
			ctx.JSON(http.StatusNotAcceptable, result)
			return
		}

		if result.Data, err = goAuth.GetCode(secret); err != nil {
			result.Message = err.Error()
			ctx.JSON(http.StatusNotFound, result)
			return
		}

		ctx.JSON(http.StatusOK, result)
	})
}

// GenGoogleSecret generate google secret
func GenGoogleSecret(path string, router *gin.RouterGroup, conf *config.Config) {
	router.POST(path, func(ctx *gin.Context) {
		var result model.JsonResult

		result.Status = "ok"
		result.Message = "ok"
		r := ctx.Request
		if !model.IsPeer(r, conf) {
			result.Message = model.GetClusterNotPermitMessage(r)
			ctx.JSON(http.StatusNotAcceptable, result)
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
		ctx.JSON(http.StatusOK, result)
	})
}
