package globalConfig

import (
	"fmt"
	"github.com/sjqzhang/goutil"
	"os"
	"strings"
)

/***
通过环境变量来初始化默认配置，如下为Docker启动时的环境变量实例
-e GO_FASTDFS_PEER_ID=1 \
-e GO_FASTDFS_PEERS="\"http://192.168.70.162:8080\"" \
-e GO_FASTDFS_ADMIN_IP="\"0.0.0.0\"" \
-e GO_FASTDFS_PORT=8080 \
-e GO_FASTDFS_GROUP=group1 \
***/
const cutset = " "
var util *goutil.Common = &goutil.Common{}


func getInitPeerId() string{
	var peerId string
	if peerId = strings.Trim(os.Getenv("GO_FASTDFS_PEER_ID"), cutset); peerId == "" {
		peerId = fmt.Sprintf("%d", util.RandInt(0, 9))
	}
	return peerId
}

func getInitPort() string{
	var port string
	if port = strings.Trim(os.Getenv("GO_FASTDFS_PORT"), cutset); port == "" {
		port = "8080"
	}
	return port
}
func GetInitIp() string{
	var ip string
	if ip = strings.Trim(os.Getenv("GO_FASTDFS_IP"), cutset); ip == "" {
		ip = util.GetPulicIP()
	}
	return ip
}
func getInitHost() string{
	peer := "http://" + GetInitIp() + ":" + getInitPort()
	return peer;
}

func getInitPeers() string {
	var peers string
	if peers = strings.Trim(os.Getenv("GO_FASTDFS_PEERS"), cutset); peers == "" {
		peers = "\"" + getInitHost() + "\""
	}
	return peers
}

func getInitGroup() string{
	var group string
	if group = strings.Trim(os.Getenv("GO_FASTDFS_GROUP"), cutset); group == "" {
		group = "group1"
	}
	return group;
}

func getInitAdminIp() string{
	adminIp := strings.Trim(os.Getenv("GO_FASTDFS_ADMIN_IP"), cutset)
	if adminIp == "" {
		adminIp = "\"127.0.0.1\""
	} else{
		if !strings.Contains(adminIp, "127.0.0.1") && !strings.Contains(adminIp, "0.0.0.0") {
			adminIp += ",\"127.0.0.1\""
		}
	}
	return adminIp;
}

func getInitRenameFile() string{
	var renameFile string
	if renameFile = strings.Trim(os.Getenv("GO_FASTDFS_RENAME"), cutset); renameFile == "" {
		renameFile = "false"
	}
	return renameFile;
}

func getInitDistinctFile() string{
	var enableDistinctFile string
	if enableDistinctFile = strings.Trim(os.Getenv("GO_FASTDFS_DISTINCT_FILE"), cutset); enableDistinctFile == "" {
		enableDistinctFile = "true"
	}
	return enableDistinctFile;
}

func (handler *Handler) InitGlobalConfig() bool{
	DOCKER_DIR := os.Getenv("GO_FASTDFS_DIR")
	if DOCKER_DIR != "" {
		if !strings.HasSuffix(DOCKER_DIR, "/") {
			DOCKER_DIR = DOCKER_DIR + "/"
		}
	}

	CONF_DIR = DOCKER_DIR + CONF_DIR_NAME
	CONST_CONF_FILE_NAME = CONF_DIR + "/cfg.json"
	CONST_SERVER_CRT_FILE_NAME = CONF_DIR + "/server.crt"
	CONST_SERVER_KEY_FILE_NAME = CONF_DIR + "/server.key"

	os.MkdirAll(CONF_DIR, 0775)
	peerId := getInitPeerId()
	if !util.FileExists(CONST_CONF_FILE_NAME) {
		host := getInitHost()
		peers := getInitPeers()
		port := getInitPort()
		group := getInitGroup()
		adminIp := getInitAdminIp()
		renameFile := getInitRenameFile()
		distinctFile := getInitDistinctFile()
		cfg := fmt.Sprintf(CfgJson, port, peerId, host, peers, group, renameFile, adminIp, distinctFile)
		result := util.WriteFile(CONST_CONF_FILE_NAME, cfg)
		return result
	}
	return true
}