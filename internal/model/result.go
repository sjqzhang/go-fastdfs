package model

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/luoyunpeng/go-fastdfs/internal/config"
)

type JsonResult struct {
	Message string      `json:"message"`
	Status  string      `json:"status"`
	Data    interface{} `json:"data"`
}

type FileResult struct {
	Url     string `json:"url"`
	Md5     string `json:"md5"`
	Path    string `json:"path"`
	Domain  string `json:"domain"`
	Scene   string `json:"scene"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	//Just for Compatibility
	Scenes  string `json:"scenes"`
	Retmsg  string `json:"retmsg"`
	Retcode int    `json:"retcode"`
	Src     string `json:"src"`
}

type FileInfoResult struct {
	Name    string `json:"name"`
	Md5     string `json:"md5"`
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	ModTime int64  `json:"mtime"`
	IsDir   bool   `json:"is_dir"`
}

func BuildFileResult(fileInfo *FileInfo, r *http.Request) FileResult {
	var (
		outname     string
		fileResult  FileResult
		p           string
		downloadUrl string
		host        string
	)
	host = strings.Replace(config.CommonConfig.Host, "http://", "", -1)
	if r != nil {
		host = r.Host
	}
	if !strings.HasPrefix(config.CommonConfig.DownloadDomain, "http") {
		if config.CommonConfig.DownloadDomain == "" {
			config.CommonConfig.DownloadDomain = fmt.Sprintf("http://%s", host)
		} else {
			config.CommonConfig.DownloadDomain = fmt.Sprintf("http://%s", config.CommonConfig.DownloadDomain)
		}
	}

	domain := config.CommonConfig.DownloadDomain
	if domain == "" {
		domain = fmt.Sprintf("http://%s", host)
	}

	outname = fileInfo.Name
	if fileInfo.ReName != "" {
		outname = fileInfo.ReName
	}
	p = strings.Replace(fileInfo.Path, config.StoreDirName+"/", "", 1)
	p = config.FileDownloadPathPrefix + p + "/" + outname

	downloadUrl = fmt.Sprintf("http://%s/%s", host, p)
	if config.CommonConfig.DownloadDomain != "" {
		downloadUrl = fmt.Sprintf("%s/%s", config.CommonConfig.DownloadDomain, p)
	}
	fileResult.Url = downloadUrl
	fileResult.Md5 = fileInfo.Md5
	fileResult.Path = "/" + p
	fileResult.Domain = domain
	fileResult.Scene = fileInfo.Scene
	fileResult.Size = fileInfo.Size
	fileResult.ModTime = fileInfo.TimeStamp
	// Just for Compatibility
	fileResult.Src = fileResult.Path
	fileResult.Scenes = fileInfo.Scene
	return fileResult
}
