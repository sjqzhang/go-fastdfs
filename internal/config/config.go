package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/luoyunpeng/go-fastdfs/pkg"
	log "github.com/sirupsen/logrus"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/opt"
)

const (
	MessageClusterIp   = "Can only be called by the cluster ip or 127.0.0.1 or admin_ips(cfg.json),current ip:%s"
	DefaultConfigFile  = "/opt/fileServer/conf/fileServer.yml"
	DefaultConfigFile2 = "./fileServer.yml"
)

type Config struct {
	levelDB       *leveldb.DB
	logLevelDB    *leveldb.DB
	params        *Params
	absRunningDir string
}

func NewConfig() *Config {
	conf := &Config{}
	conf.CheckRunningDir()

	params := NewParams()
	conf.params = params

	opts := &opt.Options{
		CompactionTableSize: 1024 * 1024 * 20,
		WriteBuffer:         1024 * 1024 * 20,
	}
	levelDB, err := leveldb.OpenFile(params.LeveldbFile, opts)
	if err != nil {
		fmt.Println(fmt.Sprintf("open db file %s fail,maybe has opening", conf.LeveldbFile()))
		panic(err)
	}
	conf.levelDB = levelDB

	logLevelDB, err := leveldb.OpenFile(conf.LogLeveldbFile(), opts)
	if err != nil {
		fmt.Println(fmt.Sprintf("open db file %s fail,maybe has opening", conf.LogLeveldbFile()))
		panic(err)

	}
	conf.logLevelDB = logLevelDB

	conf.createFileServerDirectory()
	conf.initPeer()
	conf.initUploadPage()

	return conf
}

func (c *Config) RegisterExit() {
	err := c.LevelDB().Close()
	if err != nil {
		log.Info("close levelDB error: ", err)
	}
	err = c.LogLevelDB().Close()
	if err != nil {
		log.Info("close LogLevelDB error: ", err)
	}
}

func (c *Config) EnableFsNotify() bool {
	return c.params.EnableFsNotify
}

func (c *Config) EnableMigrate() bool {
	return c.params.EnableMigrate
}

func (c *Config) RefreshInterval() int {
	return c.params.RefreshInterval
}

func (c *Config) AutoRepair() bool {
	return c.params.AutoRepair
}

func (c *Config) Md5ErrorFile() string {
	return c.params.Md5ErrorFile
}

func (c *Config) initPeer() {
	var ip string
	if ip = os.Getenv("FileServer_IP"); ip == "" {
		ip = pkg.GetPublicIP()
	}
	peer := "http://" + ip + c.Port()

	c.params.Peers = append(c.params.Peers, peer)
	if c.params.QueueSize == 0 {
		c.params.QueueSize = 10000
	}
	if c.params.PeerId == "" {
		c.params.PeerId = fmt.Sprintf("%d", pkg.RandInt(0, 9))
	}
}

func (c *Config) initUploadPage() {
	uploadPageName := c.params.StaticDir + "/upload.tmpl"
	DefaultUploadPage = fmt.Sprintf(DefaultUploadPage, "/upload", "", "")

	if !pkg.Exist(uploadPageName) {
		pkg.WriteFile(uploadPageName, DefaultUploadPage)
	}
}

func (c *Config) CheckRunningDir() {
	appDir, e1 := pkg.GetFileServerRunningAbsDir(os.Args[0])

	if e1 == nil {
		panic(fmt.Sprintf("please switch directory to '%s' and start fileserver\n", appDir))
	}
	c.absRunningDir = appDir
}

func (c *Config) createFileServerDirectory() {
	dirs := []string{c.params.OriginPath, c.DataDir(), c.StoreDir(), c.ConfigDir(), c.StaticDir()}

	for _, dir := range dirs {
		if err := pkg.CreateDirectories(dir, 0775); err != nil {
			panic(err)
		}
	}
}

func (c *Config) Port() string {
	return c.params.Port
}

func (c *Config) ReadTimeout() int {
	return c.params.ReadTimeout
}

func (c *Config) ReadHeaderTimeout() int {
	return c.params.ReadHeaderTimeout
}

func (c *Config) WriteTimeout() int {
	return c.params.WriteTimeout
}

func (c *Config) IdleTimeout() int {
	return c.params.IdleTimeout
}

func (c *Config) EnableCrossOrigin() bool {
	return c.params.EnableCrossOrigin
}

func (c *Config) EnableWebUpload() bool {
	return c.params.EnableWebUpload
}

func (c *Config) ShowDir() bool {
	return c.params.ShowDir
}

func (c *Config) Addr() string {
	return c.params.Addr
}

func (c *Config) FileDownloadPathPrefix() string {
	return c.params.FileDownloadPathPrefix
}

func (c *Config) StoreDirName() string {
	return c.params.StoreDir
}

func (c *Config) Peers() []string {
	var peers []string
	copy(c.params.Peers, peers)

	return peers
}

func (c *Config) FileMd5Name() string {
	return c.params.FilesMd5Name
}

func (c *Config) StaticDir() string {
	return c.params.StaticDir
}

func (c *Config) StoreDir() string {
	return c.params.StoreDir
}

func (c *Config) Md5QueueFile() string {
	return c.params.Md5QueueFile
}

func (c *Config) AuthUrl() string {
	return c.params.AuthUrl
}

func (c *Config) RemoveMd5File() string {
	return c.params.RemovesMd5File
}

func (c *Config) DataDir() string {
	return c.params.DataDir
}

func (c *Config) AbsRunningDir() string {
	return c.absRunningDir
}

func (c *Config) EnableDownloadAuth() bool {
	return c.params.EnableDownloadAuth
}

func (c *Config) DownloadUseToken() bool {
	return c.params.DownloadUseToken
}

func (c *Config) DownloadTokenExpire() int {
	return c.params.DownloadTokenExpire
}

func (c *Config) EnableGoogleAuth() bool {
	return c.params.EnableGoogleAuth
}

func (c *Config) LargeDir() string {
	return c.params.LargeFileStoreDir
}

func (c *Config) QueueSize() int {
	return c.params.QueueSize
}

func (c *Config) StatisticsFileCountKey() string {
	return c.params.StatisticsFileCountKey
}

func (c *Config) LeveldbFile() string {
	return c.params.LeveldbFile
}

func (c *Config) StatFileTotalSizeKey() string {
	return c.params.StatFileTotalSizeKey
}

func (c *Config) LogLeveldbFile() string {
	return c.params.LogLeveldbFile
}

func (c *Config) WatchChanSize() int {
	return c.params.WatchChanSize
}

func (c *Config) SyncDelay() int64 {
	return c.params.SyncDelay
}

func (c *Config) SmallFileSize() int {
	return int(c.params.SmallFileSize)
}

func (c *Config) DefaultDownload() bool {
	return c.params.DefaultDownload
}

func (c *Config) FileMd5() string {
	return c.params.FilesMd5Name
}

func (c *Config) ReadOnly() bool {
	return c.params.ReadOnly
}

func (c *Config) EnableCustomPath() bool {
	return c.params.EnableCustomPath
}

func (c *Config) DefaultScene() string {
	return c.params.DefaultScene
}

func (c *Config) EnableDistinctFile() bool {
	return c.params.EnableDistinctFile
}

func (c *Config) RenameFile() bool {
	return c.params.RenameFile
}

func (c *Config) EnableMergeSmallFile() bool {
	return c.params.EnableMergeSmallFile
}

func (c *Config) PeerId() string {
	return c.params.PeerId
}

func (c *Config) SyncWorker() int {
	return c.params.SyncWorker
}

func (c *Config) SearchFile() string {
	return c.params.SearchFile
}

func (c *Config) UploadCounterKey() string {
	return c.params.UploadCounterKey
}

func (c *Config) UploadWorker() int {
	return c.params.UploadWorker
}

func (c *Config) LogDir() string {
	return c.params.LogDir
}

func (c *Config) BigUploadPathSuffix() string {
	return c.params.BigUploadPathSuffix
}

func (c *Config) FileSumArithmetic() string {
	return c.params.FileSumArithmetic
}

func (c *Config) Extensions() []string {
	return c.params.Extensions
}

func (c *Config) SetAddr(host string) {
	c.params.Addr = host
}

func (c *Config) SetPeers(peers []string) {
	c.params.Peers = peers
}

func (c *Config) EnableTus() bool {
	return c.params.EnableTus
}

func (c *Config) Scenes() []string {
	return c.params.Scenes
}

func (c *Config) SetReadTimeout(seconds int) {
	c.params.ReadTimeout = seconds
}

func (c *Config) SetWriteTimeout(seconds int) {
	c.params.WriteTimeout = seconds
}

func (c *Config) SetSyncWorker(num int) {
	c.params.SyncWorker = num
}

func (c *Config) SetUploadWorker(num int) {
	c.params.UploadWorker = num
}

func (c *Config) UploadQueueSize() int {
	return c.params.UploadQueueSize
}

func (c *Config) SetUploadQueueSize(size int) {
	c.params.UploadQueueSize = size
}

func (c *Config) RetryCount() int {
	return c.params.RetryCount
}

func (c *Config) SetRetryCount(count int) {
	c.params.RetryCount = count
}

func (c *Config) SetSyncDelay(syncDelay int) {
	c.params.SyncDelay = int64(syncDelay)
}

func (c *Config) SetWatchChanSize(size int) {
	c.params.WatchChanSize = size
}

func (c *Config) AdminIps() []string {
	var ips []string
	copy(ips, c.params.AdminIps)
	return ips
}

func (c *Config) AlarmReceivers() []string {
	var receivers []string
	copy(receivers, c.params.AlarmReceivers)
	return receivers
}

func (c *Config) AlarmUrl() string {
	return c.params.AlarmUrl
}

func (c *Config) SyncTimeout() int64 {
	return c.params.SyncTimeout
}

func (c *Config) MailHost() string {
	return c.params.MailHost
}

func (c *Config) MailUser() string {
	return c.params.MailUser
}

func (c *Config) MailPassword() string {
	return c.params.MailPassword
}

func (c *Config) DownloadDomain() string {
	return c.params.DownloadDomain
}

func (c *Config) SetDownloadDomain() {
	if !strings.HasPrefix(c.DownloadDomain(), "http") {
		if c.DownloadDomain() == "" {
			c.params.DownloadDomain = c.Addr()
			return
		}
		c.params.DownloadDomain = fmt.Sprintf("http://%s", c.DownloadDomain())
	}
}

func (c *Config) StatisticsFile() string {
	return c.params.StatisticsFile
}

func (c *Config) ConfigDir() string {
	return c.params.ConfigDir
}

func (c *Config) LogLevelDB() *leveldb.DB {
	return c.logLevelDB
}

func (c *Config) LevelDB() *leveldb.DB {
	return c.levelDB
}
