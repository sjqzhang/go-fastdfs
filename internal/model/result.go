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

func BuildFileResult(fileInfo *FileInfo, r *http.Request, conf *config.Config) FileResult {
	var (
		outName     string
		fileResult  FileResult
		p           string
		downloadUrl string
		host        string
	)
	host = strings.Replace(conf.Host(), "http://", "", -1)
	if r != nil {
		host = r.Host
	}
	if !strings.HasPrefix(conf.DownloadDomain(), "http") {
		if conf.DownloadDomain() == "" {
			conf.SetDownloadDomain(fmt.Sprintf("http://%s", host))
		} else {
			conf.SetDownloadDomain(fmt.Sprintf("http://%s", conf.DownloadDomain()))
		}
	}

	domain := conf.DownloadDomain()
	if domain == "" {
		domain = fmt.Sprintf("http://%s", host)
	}

	outName = fileInfo.Name
	if fileInfo.ReName != "" {
		outName = fileInfo.ReName
	}
	p = strings.Replace(fileInfo.Path, conf.StoreDir()+"/", "", 1)
	p = conf.FileDownloadPathPrefix() + p + "/" + outName

	downloadUrl = fmt.Sprintf("http://%s/%s", host, p)
	if conf.DownloadDomain() != "" {
		downloadUrl = fmt.Sprintf("%s/%s", conf.DownloadDomain(), p)
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
