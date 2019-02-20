package main

import (
	"fmt"
	"github.com/astaxie/beego/httplib"
	"github.com/eventials/go-tus"
	"io/ioutil"
	_ "net/http/pprof"
	"os"
	"testing"
	"time"
)

const (
	CONST_SMALL_FILE_NAME="small.txt"
	CONST_BIG_FILE_NAME="big.txt"
	CONST_DOWNLOAD_BIG_FILE_NAME="big_dowload.txt"
	CONST_DOWNLOAD_SMALL_FILE_NAME="small_dowload.txt"
)

var testUtil=Common{}


var testSmallFileMd5=""
var testBigFileMd5=""

func init()  {

	var (
		err error
	)

	smallBytes:=make([]byte,1025*512)
	for i:=0;i<len(smallBytes);i++ {
		smallBytes[i]='a'
	}
	bigBytes:=make([]byte,1025*1024*2)
	for i:=0;i<len(smallBytes);i++ {
		bigBytes[i]='a'
	}

	ioutil.WriteFile(CONST_SMALL_FILE_NAME,smallBytes,0664)
	ioutil.WriteFile(CONST_BIG_FILE_NAME,bigBytes,0664)
	testSmallFileMd5,err=testUtil.GetFileSumByName(CONST_SMALL_FILE_NAME,"")
	if err!=nil {
	//	testing.T.Error(err)
	fmt.Println(err)
	}
	testBigFileMd5,err=testUtil.GetFileSumByName(CONST_BIG_FILE_NAME,"")
	if err!=nil {
		//testing.T.Error(err)
		fmt.Println(err)
	}
	fmt.Println(CONST_SMALL_FILE_NAME,testSmallFileMd5)
	fmt.Println(CONST_BIG_FILE_NAME,testBigFileMd5)

}

func uploadContinueBig(t *testing.T)  {
	f, err := os.Open(CONST_BIG_FILE_NAME)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	client, err := tus.NewClient("http://127.0.0.1:8080/big/upload/", nil)
	if err!=nil {
		t.Error(err)
	}
	upload, err := tus.NewUploadFromFile(f)
	if err!=nil {
		t.Error(err)
	}
	uploader, err := client.CreateUpload(upload)
	if err!=nil {
		t.Error(err)
	}
	url:=uploader.Url()
    err=uploader.Upload()
	time.Sleep(time.Second*1)
	if err!=nil {
		t.Error(err)
	}
    if err:=httplib.Get(url).ToFile(CONST_DOWNLOAD_BIG_FILE_NAME);err!=nil {
    	t.Error(err)
	}
    fmt.Println(url)

    if md5sum,err:= testUtil.GetFileSumByName(CONST_DOWNLOAD_BIG_FILE_NAME,"");md5sum!=testBigFileMd5 {
    	t.Error("uploadContinue bigfile  download fail")
    	t.Error(err)
	}




}

func uploadContinueSmall(t *testing.T)  {
	f, err := os.Open(CONST_SMALL_FILE_NAME)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	client, err := tus.NewClient("http://127.0.0.1:8080/big/upload/", nil)
	if err!=nil {
		t.Error(err)
	}
	upload, err := tus.NewUploadFromFile(f)
	if err!=nil {
		t.Error(err)
	}
	uploader, err := client.CreateUpload(upload)
	if err!=nil {
		t.Error(err)
	}
	url:=uploader.Url()
	err=uploader.Upload()
	time.Sleep(time.Second*1)
	if err!=nil {
		t.Error(err)
	}
	if err:=httplib.Get(url).ToFile(CONST_DOWNLOAD_SMALL_FILE_NAME);err!=nil {
		t.Error(err)
	}
	fmt.Println(url)

	if md5sum,err:= testUtil.GetFileSumByName(CONST_DOWNLOAD_SMALL_FILE_NAME,"");md5sum!=testSmallFileMd5 {
		t.Error("uploadContinue smallfile  download fail")
		t.Error(err)
	}




}


func uploadSmall(t *testing.T)  {
	var obj FileResult
	req:=httplib.Post("http://127.0.0.1:8080/upload")
	req.PostFile("file",CONST_SMALL_FILE_NAME)
	req.Param("output","json")
	req.Param("scene","")
	req.Param("path","")
	req.ToJSON(&obj)
	fmt.Println(obj.Url)
	if obj.Md5!=testSmallFileMd5 {
		t.Error("file not equal")
	}else {
		req=httplib.Get(obj.Url)
		req.ToFile(CONST_DOWNLOAD_SMALL_FILE_NAME)
		if md5sum,err:= testUtil.GetFileSumByName(CONST_DOWNLOAD_SMALL_FILE_NAME,"");md5sum!=testSmallFileMd5 {
			t.Error("small file not equal",err)
		}
	}
}

func uploadLarge(t *testing.T)  {
	var obj FileResult
	req:=httplib.Post("http://127.0.0.1:8080/upload")
	req.PostFile("file",CONST_BIG_FILE_NAME)
	req.Param("output","json")
	req.Param("scene","")
	req.Param("path","")
	req.ToJSON(&obj)
	fmt.Println(obj.Url)
	if obj.Md5!=testBigFileMd5 {
		t.Error("file not equal")
	} else {
		req=httplib.Get(obj.Url)
		req.ToFile(CONST_DOWNLOAD_BIG_FILE_NAME)
		if md5sum,err:= testUtil.GetFileSumByName(CONST_DOWNLOAD_BIG_FILE_NAME,"");md5sum!=testBigFileMd5 {

			t.Error("big file not equal",err)
		}
	}
}

func checkFileExist(t *testing.T)  {
	var obj FileInfo
	req:=httplib.Post("http://127.0.0.1:8080/check_file_exist")
	req.Param("md5",testBigFileMd5)
	req.ToJSON(&obj)
	if obj.Md5!=testBigFileMd5 {
		t.Error("file not equal testBigFileMd5")
	}
	req=httplib.Get("http://127.0.0.1:8080/check_file_exist?md5="+testSmallFileMd5)
	req.ToJSON(&obj)
	if obj.Md5!=testSmallFileMd5 {
		t.Error("file not equal testSmallFileMd5")
	}
}



func Test_main(t *testing.T) {

	tests := []struct {
		name string
	}{
     {"main"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
		 go	main()
		 time.Sleep(time.Second*1)
		 uploadContinueBig(t)
		 uploadContinueSmall(t)
		 uploadSmall(t)
		 uploadLarge(t)
         checkFileExist(t)
         time.Sleep(time.Second*2)
		})
	}
}
