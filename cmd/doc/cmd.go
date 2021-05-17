package doc

import (
	"github.com/sjqzhang/go-fastdfs/doc"
	"github.com/spf13/cobra"
)

// Cmd run http server
var Cmd = &cobra.Command{
	Use:   "doc",
	Short: "Run doc server",
	Long:  `Run doc server`,
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

var (
	url    string
	dir    string
	scene  string
	worker int
)

func init() {
	Cmd.Flags().StringVar(&url, "url", "http://127.0.0.1:8080/group1/upload", "url")
	Cmd.Flags().StringVar(&dir, "dir", "./", "dir to upload")
	Cmd.Flags().StringVar(&scene, "scene", "default", "scene")
	Cmd.Flags().IntVar(&worker, "worker", 100, "num of worker")
}

func main() {
	doc.Url = &url
	doc.Dir = &dir
	doc.Worker = &worker
	doc.Scene = &scene
	doc.StartServer()
}
