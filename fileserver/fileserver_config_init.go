package fileserver

import (
	"fmt"
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
func (server *Server) getInitPeerId() string{
	var peerId string
	if peerId = strings.Trim(os.Getenv("GO_FASTDFS_PEER_ID"), cutset); peerId == "" {
		peerId = fmt.Sprintf("%d", server.util.RandInt(0, 9))
	}
	return peerId
}

func (server *Server) getInitPort() string{
	var port string
	if port = strings.Trim(os.Getenv("GO_FASTDFS_PORT"), cutset); port == "" {
		port = "8080"
	}
	return port
}
func (server *Server) getInitIp() string{
	var ip string
	if ip = strings.Trim(os.Getenv("GO_FASTDFS_IP"), cutset); ip == "" {
		ip = server.util.GetPulicIP()
	}
	return ip
}
func (server *Server) getInitHost() string{
	peer := "http://" + server.getInitIp() + ":" + server.getInitPort()
	return peer;
}

func (server *Server) getInitPeers() string {
	var peers string
	if peers = strings.Trim(os.Getenv("GO_FASTDFS_PEERS"), cutset); peers == "" {
		peers = "\"" + server.getInitHost() + "\""
	}
	return peers
}

func (server *Server) getInitGroup() string{
	var group string
	if group = strings.Trim(os.Getenv("GO_FASTDFS_GROUP"), cutset); group == "" {
		group = "group1"
	}
	return group;
}

func (server *Server) getInitAdminIp() string{
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

func (server *Server) getInitRenameFile() string{
	var renameFile string
	if renameFile = strings.Trim(os.Getenv("GO_FASTDFS_RENAME"), cutset); renameFile == "" {
		renameFile = "false"
	}
	return renameFile;
}