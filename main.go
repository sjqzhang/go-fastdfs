package main

import (
	"io/fs"

	"github.com/sjqzhang/go-fastdfs/cmd/doc"
	"github.com/sjqzhang/go-fastdfs/cmd/server"
	"github.com/sjqzhang/go-fastdfs/cmd/version"
	dfs "github.com/sjqzhang/go-fastdfs/server"
	"github.com/spf13/cobra"

	//_ "go.uber.org/automaxprocs" // 根据容器配额设置 maxprocs
	"embed"
	"io"
	_ "net/http/pprof" // 注册 pprof 接口
	"os"
)

var (
	VERSION     string
	BUILD_TIME  string
	GO_VERSION  string
	GIT_VERSION string
)

//go:embed static
var staticFs embed.FS

func init() {

	extractFilesIfNotExists(staticFs, "static", "./static")

}

func extractFilesIfNotExists(fs2 embed.FS, sourceDir, targetDir string) error {
	// 检查目标目录是否存在

	return fs.WalkDir(fs2, sourceDir, func(path string, d fs.DirEntry, err error) error {

		// 如果是目录，则创建目录
		if d.IsDir() {
			return os.MkdirAll(path, 0755)
		}

		// 打开嵌入的文件
		ff, err := fs2.Open(path)
		if err != nil {
			return err
		}
		defer ff.Close()

		// 创建目标文件
		tf, err := os.Create(path)
		if err != nil {
			return err
		}
		defer tf.Close()

		// 复制内容
		_, err = io.Copy(tf, ff)
		return err
	})

}

func main() {
	dfs.VERSION = VERSION
	dfs.BUILD_TIME = BUILD_TIME
	dfs.GO_VERSION = GO_VERSION
	dfs.GIT_VERSION = GIT_VERSION
	root := cobra.Command{Use: "fileserver"}
	root.AddCommand(
		version.Cmd,
		doc.Cmd,
		server.Cmd,
	)
	root.Execute()
}
