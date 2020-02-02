package model

import (
	"fmt"
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
	RetMsg  string `json:"retmsg"`
	RetCode int    `json:"retcode"`
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

func BuildFileResult(fileInfo *FileInfo, reqHost string, conf *config.Config) FileResult {
	host := strings.Replace(conf.Host(), "http://", "", -1)
	if reqHost != "" {
		// if is not null use the requestHost(ip:port)
		host = reqHost
	}
	domain := conf.DownloadDomain()
	if domain == "" {
		domain = fmt.Sprintf("http://%s", host)
	}

	fileName := fileInfo.Name
	if fileInfo.ReName != "" {
		fileName = fileInfo.ReName
	}
	path := strings.Replace(fileInfo.Path, conf.StoreDir()+"/", "", 1)
	//eg: /file/svg/1.svg,
	downloadPathSubfix := conf.FileDownloadPathPrefix() + "/" + path + "/" + fileName

	downloadUrl := fmt.Sprintf("http://%s%s", host, downloadPathSubfix)
	if conf.DownloadDomain() != "" {
		downloadUrl = fmt.Sprintf("%s%s", conf.DownloadDomain(), downloadPathSubfix)
	}
	result := FileResult{
		Url:     downloadUrl,
		Md5:     fileInfo.Md5,
		Path:    downloadPathSubfix,
		Domain:  domain,
		Scene:   fileInfo.Scene,
		Size:    fileInfo.Size,
		ModTime: fileInfo.TimeStamp,
		Src:     downloadPathSubfix,
		Scenes:  fileInfo.Scene,
	}

	return result
}
