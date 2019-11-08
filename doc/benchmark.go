package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/astaxie/beego/httplib"
	"github.com/sjqzhang/goutil"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var url *string
var dir *string
var worker *int
var queue chan string
var filesize *int
var filecount *int
var gen *bool
var done chan bool=make(chan bool,1)
var wg sync.WaitGroup=sync.WaitGroup{}
var util goutil.Common

func init()  {
	util=goutil.Common{}
}

func getDir(dir string) []string  {
    cmd:=exec.Command("find",dir,"-type","f")
	var out bytes.Buffer
	cmd.Stdout = &out
	if err:=cmd.Run();err!=nil {
		fmt.Println(err)
		os.Exit(1)
	}
	return strings.Split( out.String(),"\n")
}

func sendFile()  {
        for {
        	filePath:= <-queue
			req := httplib.Post(*url)
			req.PostFile("file", filePath) //注意不是全路径
			req.Param("output", "text")
			req.Param("scene", "")
			req.Param("path", "")
			if s,err:=req.String();err!=nil {
				fmt.Println(err,filePath)
			} else {
				fmt.Println(s,filePath)
			}
			wg.Done()
		}

}

func genMemFile() []byte {
	bufstr:=bytes.Buffer{}
	bufstr.Grow(*filesize)
	for i:=0;i<*filesize;i++ {
		bufstr.Write([]byte("a"))
	}
	return bufstr.Bytes()
}

func genFile()  {
	j:=0

    buf:=genMemFile()
	for i:=0;i<*filecount;i++ {
		if i%1000==0 {
			j=i
			os.Mkdir(fmt.Sprintf("%d",i),0666)
		}
		uuid:=util.GetUUID()
		for k:=0;k<len(uuid);k++{
			buf[k]=uuid[k]
		}
		ioutil.WriteFile(fmt.Sprintf("%d/%d.txt",j,i),buf,0666)
	}
}

func startWorker()  {
	for i:=0;i<*worker;i++ {
    	go sendFile()
	}
}

func main()  {
	url=flag.String("url", "http://127.0.0.1:8080/group1/upload", "url")
	dir=flag.String("dir", "./", "dir to upload")
	worker=flag.Int("worker", 100, "num of worker")
	filesize=flag.Int("filesize", 1024*1024, "file of size")
	filecount=flag.Int("filecount", 1000000, "file of count")
	gen=flag.Bool("gen", false, "is gen file")
	flag.Parse()
	st:=time.Now()
	if *gen {
		genFile()
		fmt.Println(time.Since(st))
		os.Exit(0)
	}
	files:=getDir(*dir)
	wg.Add(len(files))
	queue=make(chan string,len(files))
	for i:=0;i<len(files);i++{
		queue<-files[i]
	}
	startWorker()
	wg.Wait()
	fmt.Println(time.Since(st))
}