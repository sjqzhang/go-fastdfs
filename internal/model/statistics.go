package model

import (
	"encoding/json"
	"fmt"
	"runtime/debug"
	"strconv"
	"strings"

	"github.com/luoyunpeng/go-fastdfs/internal/config"
	"github.com/luoyunpeng/go-fastdfs/pkg"
	log "github.com/sirupsen/logrus"
	levelDBUtil "github.com/syndtr/goleveldb/leveldb/util"
)

type StatDateFileInfo struct {
	Date      string `json:"date"`
	TotalSize int64  `json:"totalSize"`
	FileCount int64  `json:"fileCount"`
}

func (svr *Server) RepairStatByDate(date string, conf *config.Config) StatDateFileInfo {
	defer func() {
		if re := recover(); re != nil {
			buffer := debug.Stack()
			log.Error("RepairStatByDate")
			log.Error(re)
			log.Error(string(buffer))
		}
	}()
	var (
		err       error
		keyPrefix string
		fileInfo  FileInfo
		fileCount int64
		fileSize  int64
		stat      StatDateFileInfo
	)
	keyPrefix = "%s_%s_"
	keyPrefix = fmt.Sprintf(keyPrefix, date, conf.FileMd5())
	iter := conf.LevelDB().NewIterator(levelDBUtil.BytesPrefix([]byte(keyPrefix)), nil)
	defer iter.Release()
	for iter.Next() {
		if err = json.Unmarshal(iter.Value(), &fileInfo); err != nil {
			continue
		}
		fileCount = fileCount + 1
		fileSize = fileSize + fileInfo.Size
	}
	svr.statMap.Put(date+"_"+conf.StatisticsFileCountKey(), fileCount)
	svr.statMap.Put(date+"_"+conf.StatFileTotalSizeKey(), fileSize)
	svr.SaveStat(conf)
	stat.Date = date
	stat.FileCount = fileCount
	stat.TotalSize = fileSize
	return stat
}

// Read: SaveStat read data from statMap(which is concurrent safe map), check if the
// "StatisticsFileCountKey" key exists, if exists, then load all statMap data to file "stat.json"
func (svr *Server) SaveStat(conf *config.Config) {
	SaveStatFunc := func() {
		defer func() {
			if re := recover(); re != nil {
				buffer := debug.Stack()
				log.Error("SaveStatFunc")
				log.Error(re)
				log.Error(string(buffer))
			}
		}()
		stat := svr.statMap.Get()
		if v, ok := stat[conf.StatisticsFileCountKey()]; ok {
			switch v.(type) {
			case int64, int32, int, float64, float32:
				if v.(int64) >= 0 {
					if data, err := json.Marshal(stat); err != nil {
						log.Error(err)
					} else {
						pkg.WriteBinFile(conf.StatisticsFile(), data)
					}
				}
			}
		}
	}
	SaveStatFunc()
}

func (svr *Server) GetStat(conf *config.Config) []StatDateFileInfo {
	var (
		min   int64
		max   int64
		err   error
		i     int64
		rows  []StatDateFileInfo
		total StatDateFileInfo
	)
	min = 20190101
	max = 20190101
	for k := range svr.statMap.Get() {
		ks := strings.Split(k, "_")
		if len(ks) == 2 {
			if i, err = strconv.ParseInt(ks[0], 10, 64); err != nil {
				continue
			}
			if i >= max {
				max = i
			}
			if i < min {
				min = i
			}
		}
	}
	for i := min; i <= max; i++ {
		s := fmt.Sprintf("%d", i)
		if v, ok := svr.statMap.GetValue(s + "_" + conf.StatFileTotalSizeKey()); ok {
			var info StatDateFileInfo
			info.Date = s
			switch v.(type) {
			case int64:
				info.TotalSize = v.(int64)
				total.TotalSize = total.TotalSize + v.(int64)
			}
			if v, ok := svr.statMap.GetValue(s + "_" + conf.StatisticsFileCountKey()); ok {
				switch v.(type) {
				case int64:
					info.FileCount = v.(int64)
					total.FileCount = total.FileCount + v.(int64)
				}
			}
			rows = append(rows, info)
		}
	}
	total.Date = "all"
	rows = append(rows, total)
	return rows
}

func (svr *Server) FormatStatInfo(conf *config.Config) {
	var (
		data  []byte
		err   error
		count int64
		stat  map[string]interface{}
	)
	if pkg.FileExists(conf.StatisticsFile()) {
		if data, err = pkg.ReadFile(conf.StatisticsFile()); err != nil {
			log.Error(err)
		} else {
			if err = json.Unmarshal(data, &stat); err != nil {
				log.Error(err)
			} else {
				for k, v := range stat {
					switch v.(type) {
					case float64:
						vv := strings.Split(fmt.Sprintf("%f", v), ".")[0]
						if count, err = strconv.ParseInt(vv, 10, 64); err != nil {
							log.Error(err)
						} else {
							svr.statMap.Put(k, count)
						}
					default:
						svr.statMap.Put(k, v)
					}
				}
			}
		}
	} else {
		svr.RepairStatByDate(pkg.GetToDay(), conf)
	}
}
