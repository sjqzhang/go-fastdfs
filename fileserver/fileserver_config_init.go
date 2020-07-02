package fileserver

import (
	"fmt"
	"os"
)

/***
通过环境变量来初始化默认配置，例如
GO_FASTDFS_PEER_ID=1;GO_FASTDFS_IP=tomcat.gavin.com;GO_FASTDFS_PEERS="http://tomcat.gavin.com:8080";GO_FASTDFS_PORT=8080;GO_FASTDFS_GROUP=group1;
***/

func (server *Server) getInitPeerId() string{
	var peerId string
	if peerId = os.Getenv("GO_FASTDFS_PEER_ID"); peerId == "" {
		peerId = fmt.Sprintf("%d", server.util.RandInt(0, 9))
	}
	return peerId
}

func (server *Server) getInitPort() string{
	var port string
	if port = os.Getenv("GO_FASTDFS_PORT"); port == "" {
		port = "8080"
	}
	return port
}

func (server *Server) getInitHost() string{
	var ip string
	if ip = os.Getenv("GO_FASTDFS_IP"); ip == "" {
		ip = server.util.GetPulicIP()
	}
	peer := "http://" + ip + ":" + server.getInitPort()
	return peer;
}

func (server *Server) getInitPeers() string {
	var peers string
	if peers = os.Getenv("GO_FASTDFS_PEERS"); peers == "" {
		peers = "\"" + server.getInitHost() + "\""
	}
	return peers
}

func (server *Server) getInitGroup() string{
	var group string
	if group = os.Getenv("GO_FASTDFS_GROUP"); group == "" {
		group = "group1"
	}
	return group;
}