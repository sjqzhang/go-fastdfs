package server

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"github.com/astaxie/beego/httplib"
	"github.com/sjqzhang/goutil"
	log "github.com/sjqzhang/seelog"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

type Server struct {
	ldb            *leveldb.DB
	logDB          *leveldb.DB
	util           *goutil.Common
	statMap        *goutil.CommonMap
	sumMap         *goutil.CommonMap
	rtMap          *goutil.CommonMap
	queueToPeers   chan FileInfo
	queueFromPeers chan FileInfo
	queueFileLog   chan *FileLog
	queueUpload    chan WrapReqResp
	lockMap        *goutil.CommonMap
	sceneMap       *goutil.CommonMap
	searchMap      *goutil.CommonMap
	curDate        string
	host           string
}

func InitServer() {
	appDir, e1 := filepath.Abs(filepath.Dir(os.Args[0]))
	curDir, e2 := filepath.Abs(".")
	if e1 == nil && e2 == nil && appDir != curDir && !strings.Contains(appDir, "go-build") &&
		!strings.Contains(appDir, "GoLand") {
		msg := fmt.Sprintf("please change directory to '%s' start fileserver\n", appDir)
		msg = msg + fmt.Sprintf("请切换到 '%s' 目录启动 fileserver ", appDir)
		log.Warn(msg)
		fmt.Println(msg)
		os.Exit(1)
	}
	DOCKER_DIR = os.Getenv("GO_FASTDFS_DIR")
	if DOCKER_DIR != "" {
		if !strings.HasSuffix(DOCKER_DIR, "/") {
			DOCKER_DIR = DOCKER_DIR + "/"
		}
	}
	STORE_DIR = DOCKER_DIR + STORE_DIR_NAME
	CONF_DIR = DOCKER_DIR + CONF_DIR_NAME
	DATA_DIR = DOCKER_DIR + DATA_DIR_NAME
	LOG_DIR = DOCKER_DIR + LOG_DIR_NAME
	STATIC_DIR = DOCKER_DIR + STATIC_DIR_NAME
	LARGE_DIR_NAME = "haystack"
	LARGE_DIR = STORE_DIR + "/haystack"
	CONST_LEVELDB_FILE_NAME = DATA_DIR + "/fileserver.db"
	CONST_LOG_LEVELDB_FILE_NAME = DATA_DIR + "/log.db"
	CONST_STAT_FILE_NAME = DATA_DIR + "/stat.json"
	CONST_CONF_FILE_NAME = CONF_DIR + "/cfg.json"
	CONST_SERVER_CRT_FILE_NAME = CONF_DIR + "/server.crt"
	CONST_SERVER_KEY_FILE_NAME = CONF_DIR + "/server.key"
	CONST_SEARCH_FILE_NAME = DATA_DIR + "/search.txt"
	FOLDERS = []string{DATA_DIR, STORE_DIR, CONF_DIR, STATIC_DIR}
	logAccessConfigStr = strings.Replace(logAccessConfigStr, "{DOCKER_DIR}", DOCKER_DIR, -1)
	logConfigStr = strings.Replace(logConfigStr, "{DOCKER_DIR}", DOCKER_DIR, -1)
	for _, folder := range FOLDERS {
		os.MkdirAll(folder, 0775)
	}
	server = NewServer()

	var peerId string
	if peerId = os.Getenv("GO_FASTDFS_PEER_ID"); peerId == "" {
		peerId = fmt.Sprintf("%d", server.util.RandInt(0, 9))
	}
	if !server.util.FileExists(CONST_CONF_FILE_NAME) {
		var ip string
		if ip = os.Getenv("GO_FASTDFS_IP"); ip == "" {
			ip = server.util.GetPulicIP()
		}
		peer := "http://" + ip + ":8080"
		var peers string
		if peers = os.Getenv("GO_FASTDFS_PEERS"); peers == "" {
			peers = peer
		}
		cfg := fmt.Sprintf(cfgJson, peerId, peer, peers)
		server.util.WriteFile(CONST_CONF_FILE_NAME, cfg)
	}
	if logger, err := log.LoggerFromConfigAsBytes([]byte(logConfigStr)); err != nil {
		panic(err)
	} else {
		log.ReplaceLogger(logger)
	}
	if _logacc, err := log.LoggerFromConfigAsBytes([]byte(logAccessConfigStr)); err == nil {
		logacc = _logacc
		log.Info("succes init log access")
	} else {
		log.Error(err.Error())
	}
	ParseConfig(CONST_CONF_FILE_NAME)
	if ips, _ := server.util.GetAllIpsV4(); len(ips) > 0 {
		_ip := server.util.Match("\\d+\\.\\d+\\.\\d+\\.\\d+", Config().Host)
		if len(_ip) > 0 && !server.util.Contains(_ip[0], ips) {
			msg := fmt.Sprintf("host config is error,must in local ips:%s", strings.Join(ips, ","))
			log.Warn(msg)
			fmt.Println(msg)
		}
	}
	if Config().QueueSize == 0 {
		Config().QueueSize = CONST_QUEUE_SIZE
	}
	if Config().PeerId == "" {
		Config().PeerId = peerId
	}
	if Config().SupportGroupManage {
		staticHandler = http.StripPrefix("/"+Config().Group+"/", http.FileServer(http.Dir(STORE_DIR)))
	} else {
		staticHandler = http.StripPrefix("/", http.FileServer(http.Dir(STORE_DIR)))
	}
	server.initComponent(false)
}

func NewServer() *Server {
	var (
		//server *Server
		err error
	)
	if server != nil {
		return server
	}
	server = &Server{
		util:           &goutil.Common{},
		statMap:        goutil.NewCommonMap(0),
		lockMap:        goutil.NewCommonMap(0),
		rtMap:          goutil.NewCommonMap(0),
		sceneMap:       goutil.NewCommonMap(0),
		searchMap:      goutil.NewCommonMap(0),
		queueToPeers:   make(chan FileInfo, CONST_QUEUE_SIZE),
		queueFromPeers: make(chan FileInfo, CONST_QUEUE_SIZE),
		queueFileLog:   make(chan *FileLog, CONST_QUEUE_SIZE),
		queueUpload:    make(chan WrapReqResp, 100),
		sumMap:         goutil.NewCommonMap(365 * 3),
	}

	defaultTransport := &http.Transport{
		DisableKeepAlives:   true,
		Dial:                httplib.TimeoutDialer(time.Second*15, time.Second*300),
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 100,
	}
	settins := httplib.BeegoHTTPSettings{
		UserAgent:        "Go-FastDFS",
		ConnectTimeout:   15 * time.Second,
		ReadWriteTimeout: 15 * time.Second,
		Gzip:             true,
		DumpBody:         true,
		Transport:        defaultTransport,
	}
	httplib.SetDefaultSetting(settins)
	server.statMap.Put(CONST_STAT_FILE_COUNT_KEY, int64(0))
	server.statMap.Put(CONST_STAT_FILE_TOTAL_SIZE_KEY, int64(0))
	server.statMap.Put(server.util.GetToDay()+"_"+CONST_STAT_FILE_COUNT_KEY, int64(0))
	server.statMap.Put(server.util.GetToDay()+"_"+CONST_STAT_FILE_TOTAL_SIZE_KEY, int64(0))
	server.curDate = server.util.GetToDay()
	opts := &opt.Options{
		CompactionTableSize: 1024 * 1024 * 20,
		WriteBuffer:         1024 * 1024 * 20,
	}
	server.ldb, err = leveldb.OpenFile(CONST_LEVELDB_FILE_NAME, opts)
	if err != nil {
		fmt.Println(fmt.Sprintf("open db file %s fail,maybe has opening", CONST_LEVELDB_FILE_NAME))
		log.Error(err)
		panic(err)
	}
	server.logDB, err = leveldb.OpenFile(CONST_LOG_LEVELDB_FILE_NAME, opts)
	if err != nil {
		fmt.Println(fmt.Sprintf("open db file %s fail,maybe has opening", CONST_LOG_LEVELDB_FILE_NAME))
		log.Error(err)
		panic(err)

	}
	return server
}

func (c *Server) Start() {
	go func() {
		for {
			c.CheckFileAndSendToPeer(c.util.GetToDay(), CONST_Md5_ERROR_FILE_NAME, false)
			//fmt.Println("CheckFileAndSendToPeer")
			time.Sleep(time.Second * time.Duration(Config().RefreshInterval))
			//c.util.RemoveEmptyDir(STORE_DIR)
		}
	}()
	go c.CleanAndBackUp()
	go c.CheckClusterStatus()
	go c.LoadQueueSendToPeer()
	go c.ConsumerPostToPeer()
	go c.ConsumerLog()
	go c.ConsumerDownLoad()
	go c.ConsumerUpload()
	go c.RemoveDownloading()

	if Config().EnableFsNotify {
		go c.WatchFilesChange()
	}

	//go c.LoadSearchDict()
	if Config().EnableMigrate {
		go c.RepairFileInfoFromFile()
	}

	if Config().AutoRepair {
		go func() {
			for {
				time.Sleep(time.Minute * 3)
				c.AutoRepair(false)
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

	c.initRouter()

	//if Config().Proxies != nil && len(Config().Proxies) > 0 {
	//	for _, proxy := range Config().Proxies {
	//		go func(proxy Proxy) {
	//			handler := HttpProxyHandler{
	//				Proxy: proxy,
	//			}
	//			fmt.Println("Proxy on " + proxy.Addr)
	//			server := &http.Server{
	//				Addr:         proxy.Addr,
	//				Handler:      &handler,
	//				TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
	//			}
	//			server.ListenAndServe()
	//
	//		}(proxy)
	//	}
	//}

	fmt.Println("Listen on " + Config().Addr)
	if Config().EnableHttps {
		err := http.ListenAndServeTLS(Config().Addr, CONST_SERVER_CRT_FILE_NAME, CONST_SERVER_KEY_FILE_NAME, new(HttpHandler))
		log.Error(err)
		fmt.Println(err)
	} else {
		srv := &http.Server{
			Addr:              Config().Addr,
			Handler:           new(HttpHandler),
			ReadTimeout:       time.Duration(Config().ReadTimeout) * time.Second,
			ReadHeaderTimeout: time.Duration(Config().ReadHeaderTimeout) * time.Second,
			WriteTimeout:      time.Duration(Config().WriteTimeout) * time.Second,
			IdleTimeout:       time.Duration(Config().IdleTimeout) * time.Second,
		}
		err := srv.ListenAndServe()
		log.Error(err)
		fmt.Println(err)
	}
}

func Start() {
	server.Start()
}
