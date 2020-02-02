package api

import (
	"net/http"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/stretchr/testify/assert"
)

func TestRemoveEmptyDir(t *testing.T) {
	t.Run("successful request", func(t *testing.T) {
	})

}

func TestBackUp(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestCheckFileExist(t *testing.T) {
	type args struct {
		reqPath string
		router  *gin.RouterGroup
		conf    *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestCheckFilesExist(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestDownload(t *testing.T) {
	type args struct {
		uri    string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestGetMd5sForWeb(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestIndex(t *testing.T) {
	type args struct {
		uri    string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestListDir(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestReload(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestRemoveEmptyDir1(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestRemoveFile(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestRepair(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestRepairFileInfo(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestReport(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestSearch(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestUpload(t *testing.T) {
	type args struct {
		path   string
		router *gin.RouterGroup
		conf   *config.Config
	}
	tests := []struct {
		name string
		args args
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		})
	}
}

func TestCheckFileExist1(t *testing.T) {
	NewApiTest()
	tests := []struct {
		name string
		path string
	}{
		{name: "successful request", path: "/check_file_exist?path=files/svg/architecture.svg"},
		{name: "inValid request", path: "/check_file_exist?path=files/svg/architecture.svg1"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			CheckFileExist(test.path, TestRouter, TestConf)

			result := PerformRequest(TestApp, "GET", test.path)
			assert.Equal(t, http.StatusOK, result.Code)
		})
	}
}
