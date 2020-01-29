package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	jsoniter "github.com/json-iterator/go"
	"github.com/luoyunpeng/go-fastdfs/internal/util"
	log "github.com/sirupsen/logrus"
)

var (
	CommonConfig Config

	Json               = jsoniter.ConfigCompatibleWithStandardLibrary
	FileName           string
	DockerDir          = ""
	StoreDir           = StoreDirName
	ConfDir            = ConfDirName
	LogDir             = LogDirName
	DataDir            = DataDirName
	StaticDir          = StaticDirName
	LargeDirName       = "haystack"
	LargeDir           = StoreDir + "/haystack"
	LeveldbFileName    = DataDir + "/fileserver.db"
	LogLeveldbFileName = DataDir + "/log.db"
	StatFileName       = DataDir + "/stat.json"
	ConfFileName       = ConfDir + "/cfg.json"
	SearchFileName     = DataDir + "/search.txt"
	UploadCounterKey   = "__CONST_UPLOAD_COUNTER_KEY__"
	LogConfigStr       = `
<seelog type="asynctimer" asyncinterval="1000" minlevel="trace" maxlevel="error">  
	<outputs formatid="common">  
		<buffered formatid="common" size="1048576" flushperiod="1000">  
			<rollingfile type="size" filename="{DOCKER_DIR}log/fileserver.log" maxsize="104857600" maxrolls="10"/>  
		</buffered>
	</outputs>  	  
	 <formats>
		 <format id="common" format="%Date %Time [%LEV] [%File:%Line] [%Func] %Msg%n" />  
	 </formats>  
</seelog>
`
	LogAccessConfigStr = `
<seelog type="asynctimer" asyncinterval="1000" minlevel="trace" maxlevel="error">  
	<outputs formatid="common">  
		<buffered formatid="common" size="1048576" flushperiod="1000">  
			<rollingfile type="size" filename="{DOCKER_DIR}log/access.log" maxsize="104857600" maxrolls="10"/>  
		</buffered>
	</outputs>  	  
	 <formats>
		 <format id="common" format="%Date %Time [%LEV] [%File:%Line] [%Func] %Msg%n" />  
	 </formats>  
</seelog>
`
	DefaultUploadPage = `<html>
			  
			  <head>
				<meta charset="utf-8" />
				<title>go-fastdfs</title>
				<style>form { bargin } .form-line { display:block;height: 30px;margin:8px; } #stdUpload {background: #fafafa;border-radius: 10px;width: 745px; }</style>
				<link href="https://transloadit.edgly.net/releases/uppy/v0.30.0/dist/uppy.min.css" rel="stylesheet"></head>
			  
			  <body>
                <div>标准上传(强列建议使用这种方式)</div>
				<div id="stdUpload">
				  
				  <form action="%s" method="post" enctype="multipart/form-data">
					<span class="form-line">文件(file):
					  <input type="file" id="file" name="file" /></span>
					<span class="form-line">场景(scene):
					  <input type="text" id="scene" name="scene" value="%s" /></span>
					<span class="form-line">文件名(filename):
					  <input type="text" id="filename" name="filename" value="" /></span>
					<span class="form-line">输出(output):
					  <input type="text" id="output" name="output" value="json" /></span>
					<span class="form-line">自定义路径(path):
					  <input type="text" id="path" name="path" value="" /></span>
	              <span class="form-line">google认证码(code):
					  <input type="text" id="code" name="code" value="" /></span>
					 <span class="form-line">自定义认证(auth_token):
					  <input type="text" id="auth_token" name="auth_token" value="" /></span>
					<input type="submit" name="submit" value="upload" />
                </form>
				</div>
                 <div>断点续传（如果文件很大时可以考虑）</div>
				<div>
				 
				  <div id="drag-drop-area"></div>
				  <script src="https://transloadit.edgly.net/releases/uppy/v0.30.0/dist/uppy.min.js"></script>
				  <script>var uppy = Uppy.Core().use(Uppy.Dashboard, {
					  inline: true,
					  target: '#drag-drop-area'
					}).use(Uppy.Tus, {
					  endpoint: '%s'
					})
					uppy.on('complete', (result) => {
					 // console.log(result) console.log('Upload complete! We’ve uploaded these files:', result.successful)
					})
					//uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca',callback_url:'http://127.0.0.1/callback' ,filename:'自定义文件名','path':'自定义path',scene:'自定义场景' })//这里是传递上传的认证参数,callback_url参数中 id为文件的ID,info 文转的基本信息json
					uppy.setMeta({ auth_token: '9ee60e59-cb0f-4578-aaba-29b9fc2919ca',callback_url:'http://127.0.0.1/callback'})//自定义参数与普通上传类似（虽然支持自定义，建议不要自定义，海量文件情况下，自定义很可能给自已给埋坑）
                </script>
				</div>
			  </body>
			</html>`
)

const (
	defaultPort            = ":8080"
	QueueSize              = 10000
	StoreDirName           = "files"
	FileDownloadPathPrefix = "file/"
	LogDirName             = "log"
	DataDirName            = "data"
	ConfDirName            = "conf"
	StaticDirName          = "static"
	StatisticsFileCountKey = "fileCount"
	UploadPrefix           = "/upload"
	BigUploadPathSuffix    = "/big/upload/"
	StatFileTotalSizeKey   = "totalSize"
	Md5ErrorFileName       = "errors.md5"
	Md5QueueFileName       = "queue.md5"
	FileMd5Name            = "files.md5"
	RemoveMd5FileName      = "removes.md5"
	SmallFileSize          = 1024 * 1024
	MessageClusterIp       = "Can only be called by the cluster ip or 127.0.0.1 or admin_ips(cfg.json),current ip:%s"
	CfgJson                = `{
	"绑定端号": "端口",
	"addr": "%s",
	"PeerID": "集群内唯一,请使用0-9的单字符，默认自动生成",
	"peer_id": "%s",
	"本主机地址": "本机http地址,默认自动生成(注意端口必须与addr中的端口一致），必段为内网，自动生成不为内网请自行修改，下同",
	"host": "%s",
	"集群": "集群列表,注意为了高可用，IP必须不能是同一个,同一不会自动备份，且不能为127.0.0.1,且必须为内网IP，默认自动生成",
	"peers": ["%s"],
	"组号": "用于区别不同的集群(上传或下载)与support_group_manage配合使用,带在下载路径中",
	"group": "group1",
	"是否支持按组（集群）管理,主要用途是Nginx支持多集群": "默认支持,不支持时路径为http://10.1.5.4:8080/action,支持时为http://10.1.5.4:8080/group(配置中的group参数)/action,action为动作名，如status,delete,sync等",
	"support_group_manage": true,
	"是否合并小文件": "默认不合并,合并可以解决inode不够用的情况（当前对于小于1M文件）进行合并",
	"enable_merge_small_file": false,
    "允许后缀名": "允许可以上传的文件后缀名，如jpg,jpeg,png等。留空允许所有。",
	"extensions": [],
	"重试同步失败文件的时间": "单位秒",
	"refresh_interval": 1800,
	"是否自动重命名": "默认不自动重命名,使用原文件名",
	"rename_file": false,
	"是否支持web上传,方便调试": "默认支持web上传",
	"enable_web_upload": true,
	"是否支持非日期路径": "默认支持非日期路径,也即支持自定义路径,需要上传文件时指定path",
	"enable_custom_path": true,
	"下载域名": "用于外网下载文件的域名,不包含http://",
	"download_domain": "",
	"场景列表": "当设定后，用户指的场景必项在列表中，默认不做限制(注意：如果想开启场景认功能，格式如下：'场景名:googleauth_secret' 如 default:N7IET373HB2C5M6D ",
	"scenes": [],
	"默认场景": "默认default",
	"default_scene": "default",
	"是否显示目录": "默认显示,方便调试用,上线时请关闭",
	"show_dir": true,
	"邮件配置": "",
	"mail": {
		"user": "abc@163.com",
		"password": "abc",
		"host": "smtp.163.com:25"
	},
	"告警接收邮件列表": "接收人数组",
	"alarm_receivers": [],
	"告警接收URL": "方法post,参数:subject,message",
	"alarm_url": "",
	"下载是否需带token": "真假",
	"download_use_token": false,
	"下载token过期时间": "单位秒",
	"download_token_expire": 600,
	"是否自动修复": "在超过1亿文件时出现性能问题，取消此选项，请手动按天同步，请查看FAQ",
	"auto_repair": true,
	"文件去重算法md5可能存在冲突，默认md5": "sha1|md5",
	"file_sum_arithmetic": "md5",
	"管理ip列表": "用于管理集的ip白名单,",
	"admin_ips": ["127.0.0.1"],
	"是否启用迁移": "默认不启用",
	"enable_migrate": false,
	"文件是否去重": "默认去重",
	"enable_distinct_file": true,
	"是否开启跨站访问": "默认开启",
	"enable_cross_origin": true,
	"是否开启Google认证，实现安全的上传、下载": "默认不开启",
	"enable_google_auth": false,
	"认证url": "当url不为空时生效,注意:普通上传中使用http参数 auth_token 作为认证参数, 在断点续传中通过HTTP头Upload-Metadata中的auth_token作为认证参数,认证流程参考认证架构图",
	"auth_url": "",
	"下载是否认证": "默认不认证(注意此选项是在auth_url不为空的情况下生效)",
	"enable_download_auth": false,
	"默认是否下载": "默认下载",
	"default_download": true,
	"本机是否只读": "默认可读可写",
	"read_only": false,
	"是否开启断点续传": "默认开启",
	"enable_tus": true,
	"同步单一文件超时时间（单位秒）": "默认为0,程序自动计算，在特殊情况下，自已设定",
	"sync_timeout": 0
}`
)

type Config struct {
	Addr                 string    `json:"addr"`
	Peers                []string  `json:"peers"`
	Group                string    `json:"group"`
	RenameFile           bool      `json:"rename_file"`
	ShowDir              bool      `json:"show_dir"`
	Extensions           []string  `json:"extensions"`
	RefreshInterval      int       `json:"refresh_interval"`
	EnableWebUpload      bool      `json:"enable_web_upload"`
	DownloadDomain       string    `json:"download_domain"`
	EnableCustomPath     bool      `json:"enable_custom_path"`
	Scenes               []string  `json:"scenes"`
	AlarmReceivers       []string  `json:"alarm_receivers"`
	DefaultScene         string    `json:"default_scene"`
	Mail                 util.Mail `json:"mail"`
	AlarmUrl             string    `json:"alarm_url"`
	DownloadUseToken     bool      `json:"download_use_token"`
	DownloadTokenExpire  int       `json:"download_token_expire"`
	QueueSize            int       `json:"queue_size"`
	AutoRepair           bool      `json:"auto_repair"`
	Host                 string    `json:"host"`
	FileSumArithmetic    string    `json:"file_sum_arithmetic"`
	PeerId               string    `json:"peer_id"`
	SupportGroupManage   bool      `json:"support_group_manage"`
	AdminIps             []string  `json:"admin_ips"`
	EnableMergeSmallFile bool      `json:"enable_merge_small_file"`
	EnableMigrate        bool      `json:"enable_migrate"`
	EnableDistinctFile   bool      `json:"enable_distinct_file"`
	ReadOnly             bool      `json:"read_only"`
	EnableCrossOrigin    bool      `json:"enable_cross_origin"`
	EnableGoogleAuth     bool      `json:"enable_google_auth"`
	AuthUrl              string    `json:"auth_url"`
	EnableDownloadAuth   bool      `json:"enable_download_auth"`
	DefaultDownload      bool      `json:"default_download"`
	EnableTus            bool      `json:"enable_tus"`
	SyncTimeout          int64     `json:"sync_timeout"`
	EnableFsnotify       bool      `json:"enable_fsnotify"`
	EnableDiskCache      bool      `json:"enable_disk_cache"`
	ConnectTimeout       bool      `json:"connect_timeout"`
	ReadTimeout          int       `json:"read_timeout"`
	WriteTimeout         int       `json:"write_timeout"`
	IdleTimeout          int       `json:"idle_timeout"`
	ReadHeaderTimeout    int       `json:"read_header_timeout"`
	SyncWorker           int       `json:"sync_worker"`
	UploadWorker         int       `json:"upload_worker"`
	UploadQueueSize      int       `json:"upload_queue_size"`
	RetryCount           int       `json:"retry_count"`
	SyncDelay            int64     `json:"sync_delay"`
	WatchChanSize        int       `json:"watch_chan_size"`
	AbsRunningDir        string
}

// Read: ParseConfig parse the configure info, and load into 'GlobalConfig', if filePath is given,
// if not given, default configure 'cfgJson' will be load inf
func ParseConfig(filePath string) {
	var (
		data []byte
	)
	if filePath == "" {
		data = []byte(strings.TrimSpace(CfgJson))
	} else {
		file, err := os.Open(filePath)
		if err != nil {
			panic(fmt.Sprintln("open file path:", filePath, "error:", err))
		}
		defer file.Close()
		FileName = filePath
		data, err = ioutil.ReadAll(file)
		if err != nil {
			panic(fmt.Sprintln("file path:", filePath, " read all error:", err))
		}
	}
	if err := Json.Unmarshal(data, &CommonConfig); err != nil {
		panic(fmt.Sprintln("file path:", filePath, "json unmarshal error:", err))
	}
	log.Info(CommonConfig)
	log.Info("config parse success")
}

func LoadDefaultConfig() {
	DockerDir = os.Getenv("GO_FASTDFS_DIR")
	if DockerDir != "" {
		if !strings.HasSuffix(DockerDir, "/") {
			DockerDir = DockerDir + "/"
		}
	}
	StoreDir = DockerDir + StoreDirName
	ConfDir = DockerDir + ConfDirName
	DataDir = DockerDir + DataDirName
	LogDir = DockerDir + LogDirName
	StaticDir = DockerDir + StaticDirName
	LargeDirName = "haystack"
	LargeDir = StoreDir + "/haystack"
	LeveldbFileName = DataDir + "/fileserver.db"
	LogLeveldbFileName = DataDir + "/log.db"
	StatFileName = DataDir + "/stat.json"
	ConfFileName = ConfDir + "/cfg.json"
	SearchFileName = DataDir + "/search.txt"
	LogAccessConfigStr = strings.Replace(LogAccessConfigStr, "{DOCKER_DIR}", DockerDir, -1)
	LogConfigStr = strings.Replace(LogConfigStr, "{DOCKER_DIR}", DockerDir, -1)

	//Read: if configure file does not exist, create one and write the default to it
	peerId := fmt.Sprintf("%d", util.RandInt(0, 9))
	if !util.FileExists(ConfFileName) {
		var ip string
		if ip = os.Getenv("GO_FASTDFS_IP"); ip == "" {
			ip = util.GetPublicIP()
		}
		peer := "http://" + ip + defaultPort
		cfg := fmt.Sprintf(CfgJson, defaultPort, peerId, peer, peer)
		util.WriteFile(ConfFileName, cfg)
	}

	ParseConfig(ConfFileName)
	if CommonConfig.QueueSize == 0 {
		CommonConfig.QueueSize = QueueSize
	}
	if CommonConfig.PeerId == "" {
		CommonConfig.PeerId = peerId
	}

	LoadUploadPage()
}

func LoadUploadPage() {
	uploadUrl := UploadPrefix
	uploadBigUrl := BigUploadPathSuffix
	if CommonConfig.SupportGroupManage {
		uploadUrl = "/" + CommonConfig.Group + uploadUrl
		uploadBigUrl = "/" + CommonConfig.Group + BigUploadPathSuffix
	}
	uploadPageName := StaticDir + "/upload.tmpl"
	DefaultUploadPage = fmt.Sprintf(DefaultUploadPage, uploadUrl, CommonConfig.DefaultScene, uploadBigUrl)
	if !util.Exist(uploadPageName) {
		util.WriteFile(uploadPageName, DefaultUploadPage)
	}
}
