package main

import (
	"encoding/json"
	"fmt"
	"github.com/sjqzhang/goutil"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/filter"
	"github.com/syndtr/goleveldb/leveldb/opt"
	"time"
)

type FileInfo struct {
	Name      string   `json:"name"`
	ReName    string   `json:"rename"`
	Path      string   `json:"path"`
	Md5       string   `json:"md5"`
	Size      int64    `json:"size"`
	Peers     []string `json:"peers"`
	Scene     string   `json:"scene"`
	TimeStamp int64    `json:"timeStamp"`
	OffSet    int64    `json:"offset"`
	retry     int
	op        string
}

func main()  {

	util:=goutil.Common{}
	o := &opt.Options{
		Filter: filter.NewBloomFilter(256),
		CompactionTableSize: 1024 * 1024 * 20,
		WriteBuffer:         1024 * 1024 * 20,
	}
	db, err := leveldb.OpenFile("path/to/db", o)

	count:=100000000
    t:=time.Now()
	for i:=0;i<count;i++ {
		name:=fmt.Sprintf("%d",i)
		md5:=util.MD5(name)
		info:=FileInfo{
			Name:      name,
			ReName:    name,
			Path:      "files/default/20191015/26/45/23",
			Md5:       md5,
			Size:      0,
			Peers:     []string{"http://10.1.50.90:8080","http://10.1.50.91:8080","http://10.1.50.92:8080"},
			Scene:     "default",
			TimeStamp: time.Now().Unix(),
			OffSet:    0,
			retry:     0,
			op:        "",
		}
		data,_:=json.Marshal(info)
		err=db.Put([]byte(md5),data,nil)
		if err!=nil {
			fmt.Println(err)
		}
		if i%100000==0 {
			fmt.Println(time.Since(t), " ",i)
		}
	}


	for i:=0;i<1000;i++ {
		t:=time.Now()
		k:=fmt.Sprintf("%d",util.RandInt(0,count))
		md5:=util.MD5(k)
		db.Get([]byte(md5),nil)
		fmt.Println(time.Since(t))
	}





	if err!=nil {
		fmt.Println(err)
	}

	defer db.Close()

}