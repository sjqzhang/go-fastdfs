package doc

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/sjqzhang/goutil"
)

var filesize *int
var filecount *int
var retry *int
var gen *bool

func init() {
	util = goutil.Common{}
}

func genMemFile() []byte {
	bufstr := bytes.Buffer{}
	bufstr.Grow(*filesize)
	for i := 0; i < *filesize; i++ {
		bufstr.Write([]byte("a"))
	}
	return bufstr.Bytes()
}

func genFile() {
	j := 0
	buf := genMemFile()
	for i := 0; i < *filecount; i++ {
		if i%1000 == 0 {
			j = i
			os.Mkdir(fmt.Sprintf("%d", i), 0666)
		}
		uuid := util.GetUUID()
		for k := 0; k < len(uuid); k++ {
			buf[k] = uuid[k]
		}
		ioutil.WriteFile(fmt.Sprintf("%d/%d.txt", j, i), buf, 0666)
	}
}

func BenchmarkServer(b *testing.B) {
	Url = flag.String("url", "http://127.0.0.1:8080/group1/upload", "url")
	Dir = flag.String("dir", "./", "dir to upload")
	Worker = flag.Int("worker", 100, "num of worker")
	retry = flag.Int("retry", -1, "retry times when fail")
	filesize = flag.Int("filesize", 1024*1024, "file of size")
	filecount = flag.Int("filecount", 1000000, "file of count")
	gen = flag.Bool("gen", false, "gen file")
	flag.Parse()
	st := time.Now()
	if *gen {
		genFile()
		fmt.Println(time.Since(st))
		os.Exit(0)
	}
	files := getDir(*Dir)
	wg.Add(len(files))
	queue = make(chan string, len(files))
	for i := 0; i < len(files); i++ {
		queue <- files[i]
	}
	StartWorker()
	wg.Wait()
	fmt.Println(time.Since(st))
}
