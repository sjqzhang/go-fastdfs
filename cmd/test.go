package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/luoyunpeng/go-fastdfs-fork/internal/util"
)

func main() {

	fmt.Println("today: ",util.GetToDay())
	name := os.Args[0]
	fmt.Println("name:", name)
	appDir, _ := filepath.Abs(filepath.Dir(name))
	curDir, _ := filepath.Abs(".")

	fmt.Println("app dir: ", appDir)
	fmt.Println("cur dir: ", curDir)
}
