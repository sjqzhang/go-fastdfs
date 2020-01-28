package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"

	"github.com/luoyunpeng/go-fastdfs-fork/internal/config"
	"github.com/luoyunpeng/go-fastdfs-fork/internal/model"
	"github.com/luoyunpeng/go-fastdfs-fork/internal/server"
	"github.com/luoyunpeng/go-fastdfs-fork/internal/util"
	log "github.com/sirupsen/logrus"
)

var (
	FOLDERS     = []string{config.DATA_DIR, config.STORE_DIR, config.CONF_DIR, config.STATIC_DIR}
	VERSION     string
	BuildTime   string
	GoVersion   string
	GitVersion  string
	versionInfo = flag.Bool("v", false, "display version")
)

func init() {
	flag.Parse()
	if *versionInfo {
		fmt.Printf("%s\n%s\n%s\n%s\n", VERSION, BuildTime, GoVersion, GitVersion)
		os.Exit(0)
	}
	appDir, e1 := util.GetFileServerRunningAbsDir(os.Args[0])
	curDir, e2 := filepath.Abs(".")
	if e1 == nil && e2 == nil && appDir != curDir {
		msg := fmt.Sprintf("please change directory to '%s' start fileserver\n", appDir)
		msg = msg + fmt.Sprintf("请切换到 '%s' 目录启动 fileserver ", appDir)
		log.Warn(msg)
		fmt.Println(msg)
		os.Exit(1)
	}

	peerId := fmt.Sprintf("%d", util.RandInt(0, 9))
	config.LoadDefaultConfig(peerId)
	config.CommonConfig.AbsRunningDir = appDir
	FOLDERS = []string{config.DATA_DIR, config.STORE_DIR, config.CONF_DIR, config.STATIC_DIR}

	for _, folder := range FOLDERS {
		os.MkdirAll(folder, 0775)
	}
	model.Svr = model.NewServer()

	//Read: if configure file does not exist, create one and write the default to it
	if !util.FileExists(config.CONST_CONF_FILE_NAME) {
		var ip string
		if ip = os.Getenv("GO_FASTDFS_IP"); ip == "" {
			ip = util.GetPublicIP()
		}
		peer := "http://" + ip + config.CommonConfig.Addr
		cfg := fmt.Sprintf(config.CfgJson, peerId, peer, peer)
		util.WriteFile(config.CONST_CONF_FILE_NAME, cfg)
	}

	prefix := "/"
	if config.CommonConfig.SupportGroupManage {
		prefix =  prefix + config.CommonConfig.Group+"/"
	}
	model.StaticHandler = http.StripPrefix(prefix, http.FileServer(http.Dir(config.STORE_DIR)))
	model.Svr.InitComponent(false)
}
func main() {
	svr := model.Svr
	go func() {
		for {
			svr.CheckFileAndSendToPeer(util.GetToDay(), config.CONST_Md5_ERROR_FILE_NAME, false)
			//fmt.Println("CheckFileAndSendToPeer")
			time.Sleep(time.Second * time.Duration(config.CommonConfig.RefreshInterval))
			//svr.util.RemoveEmptyDir(STORE_DIR)
		}
	}()
	go svr.CleanAndBackUp()
	go svr.CheckClusterStatus()
	go svr.LoadQueueSendToPeer()
	go svr.ConsumerPostToPeer()
	go svr.ConsumerLog()
	go svr.ConsumerDownLoad()
	go svr.ConsumerUpload()
	go svr.RemoveDownloading()
	if config.CommonConfig.EnableFsnotify {
		go svr.WatchFilesChange()
	}
	//go svr.LoadSearchDict()
	if config.CommonConfig.EnableMigrate {
		go svr.RepairFileInfoFromFile()
	}
	if config.CommonConfig.AutoRepair {
		go func() {
			for {
				time.Sleep(time.Minute * 3)
				svr.AutoRepair(false)
				time.Sleep(time.Minute * 60)
			}
		}()
	}
	go func() { // force free memory
		for {
			time.Sleep(time.Minute * 1)
			debug.FreeOSMemory()
		}
	}()

	server.Start("")
}
