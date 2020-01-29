package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/luoyunpeng/go-fastdfs/internal/util"
)

func main() {

	fmt.Println("today: ", util.GetToDay())
	name := os.Args[0]
	fmt.Println("name:", name)
	appDir, _ := filepath.Abs(filepath.Dir(name))
	curDir, _ := filepath.Abs(".")

	fmt.Println("app dir: ", appDir)
	fmt.Println("cur dir: ", curDir)
	app()
}

func app() {
	nameCH := make(chan string)
	go func() {
		for i := 0; i < 5; i++ {
			nameCH <- "name"
		}
		close(nameCH)
	}()
	time.Sleep(time.Millisecond * 88)
	for v := range nameCH {
		fmt.Println("v: ", v)
	}
}
