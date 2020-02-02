package config

import (
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/luoyunpeng/go-fastdfs/pkg"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"
)

type Params struct {
	Port                 string   `yaml:"port"`
	Peers                []string `yaml:"peers"`
	RenameFile           bool     `yaml:"rename_file"`
	ShowDir              bool     `yaml:"show_dir"`
	Extensions           []string `yaml:"extensions"`
	RefreshInterval      int      `yaml:"refresh_interval"`
	EnableWebUpload      bool     `yaml:"enable_web_upload"`
	DownloadDomain       string   `yaml:"download_domain"`
	EnableCustomPath     bool     `yaml:"enable_custom_path"`
	Scenes               []string `yaml:"scenes"`
	AlarmReceivers       []string `yaml:"alarm_receivers"`
	DefaultScene         string   `yaml:"default_scene"`
	MailUser             string   `yaml:"mailUser"`
	MailPassword         string   `yaml:"mailPassword"`
	MailHost             string   `yaml:"mailHost"`
	AlarmUrl             string   `yaml:"alarm_url"`
	DownloadUseToken     bool     `yaml:"download_use_token"`
	DownloadTokenExpire  int      `yaml:"download_token_expire"`
	QueueSize            int      `yaml:"queue_size"`
	AutoRepair           bool     `yaml:"auto_repair"`
	Host                 string   `yaml:"host"`
	FileSumArithmetic    string   `yaml:"file_sum_arithmetic"`
	PeerId               string   `yaml:"peer_id"`
	SupportGroupManage   bool     `yaml:"support_group_manage"`
	AdminIps             []string `yaml:"admin_ips"`
	EnableMergeSmallFile bool     `yaml:"enable_merge_small_file"`
	EnableMigrate        bool     `yaml:"enable_migrate"`
	EnableDistinctFile   bool     `yaml:"enable_distinct_file"`
	ReadOnly             bool     `yaml:"read_only"`
	EnableCrossOrigin    bool     `yaml:"enable_cross_origin"`
	EnableGoogleAuth     bool     `yaml:"enable_google_auth"`
	AuthUrl              string   `yaml:"auth_url"`
	EnableDownloadAuth   bool     `yaml:"enable_download_auth"`
	DefaultDownload      bool     `yaml:"default_download"`
	EnableTus            bool     `yaml:"enable_tus"`
	SyncTimeout          int64    `yaml:"sync_timeout"`
	EnableFsNotify       bool     `yaml:"enable_pgknotify"`
	EnableDiskCache      bool     `yaml:"enable_disk_cache"`
	ConnectTimeout       bool     `yaml:"connect_timeout"`
	ReadTimeout          int      `yaml:"read_timeout"`
	WriteTimeout         int      `yaml:"write_timeout"`
	IdleTimeout          int      `yaml:"idle_timeout"`
	ReadHeaderTimeout    int      `yaml:"read_header_timeout"`
	SyncWorker           int      `yaml:"sync_worker"`
	UploadWorker         int      `yaml:"upload_worker"`
	UploadQueueSize      int      `yaml:"upload_queue_size"`
	RetryCount           int      `yaml:"retry_count"`
	SyncDelay            int64    `yaml:"sync_delay"`
	WatchChanSize        int      `yaml:"watch_chan_size"`

	OriginPath        string `yaml:"originPath"`
	DataDir           string `yaml:"dataDir"`
	StoreDir          string `yaml:"storeDir"`
	ConfigDir         string `yaml:"configDir"`
	LogDir            string `yaml:"logDir"`
	StaticDir         string `yaml:"staticDir"`
	LargeFileStoreDir string `yaml:"largeFileStorePath"`
	// LeveldbFile store levelDB data, eg: fileServer.db
	LeveldbFile string `yaml:"leveldbFile"`
	// LogLeveldbFile store log levelDB data, eg: log.db
	LogLeveldbFile string `yaml:"logLeveldbFile"`
	// LogLeveldbFile store statistics data, eg: stat.json
	StatisticsFile string `yaml:"statisticsFile"`
	// fileServer.yml
	ConfigFile string `yaml:"configFile"`

	// data/search.txt
	SearchFile             string `yaml:"searchFile"`
	BigUploadPathSuffix    string `yaml:"bigUploadPathSuffix"`
	UploadCounterKey       string `yaml:"uploadCounterKey"`
	StatisticsFileCountKey string `yaml:"statisticsFileCountKey"`
	StatFileTotalSizeKey   string `yaml:"statFileTotalSizeKey"`
	// errors.md5
	Md5ErrorFile string `yaml:"md5ErrorFile"`
	// queue.md5
	Md5QueueFile string `yaml:"md5QueueFile"`
	// files.md5
	FilesMd5Name string `yaml:"filesMd5Name"`
	// removes.md5
	RemovesMd5File         string `yaml:"removesMd5File"`
	SmallFileSize          int64  `yaml:"smallFileSize"`
	FileDownloadPathPrefix string `yaml:"fileDownloadPathPrefix"`
	IsInsideContainer      bool
}

func NewParams() *Params {
	p := &Params{}

	if err := p.SetValuesFromFile(DefaultConfigFile); err != nil {
		log.Debug(err)
		err = p.SetValuesFromFile(DefaultConfigFile2)
		if err != nil {
			panic(err)
		}
	}
	var err error
	p.IsInsideContainer, err = pkg.InsideContainer()
	if err != nil {
		panic(err)
	}

	p.expandFilenames()
	return p
}

// SetValuesFromFile uses a yaml config file to initiate the configuration entity.
func (p *Params) SetValuesFromFile(fileName string) error {
	if !pkg.FileExists(fileName) {
		return errors.New(fmt.Sprintf("config file not found: \"%s\"", fileName))
	}

	yamlConfig, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(yamlConfig, p)
}

func (p *Params) expandFilenames() {
	p.OriginPath = pkg.ExpandFilename(p.OriginPath)

	p.DataDir = p.OriginPath + p.DataDir
	p.StoreDir = p.OriginPath + p.StoreDir
	p.ConfigDir = p.OriginPath + p.ConfigDir
	p.LogDir = p.OriginPath + p.LogDir
	p.StaticDir = p.OriginPath + p.StaticDir
	p.LargeFileStoreDir = p.StoreDir + "/" + p.LargeFileStoreDir
	p.LeveldbFile = p.DataDir + "/" + p.LeveldbFile
	p.LogLeveldbFile = p.DataDir + "/" + p.LogLeveldbFile
	p.StatisticsFile = p.DataDir + "/" + p.StatisticsFile
	p.ConfigFile = p.ConfigDir + "/" + p.ConfigFile
	p.SearchFile = p.DataDir + "/" + p.SearchFile
}
