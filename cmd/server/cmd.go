package server

import (
	"github.com/sjqzhang/go-fastdfs/server"
	"github.com/spf13/cobra"
)

// Cmd run http server
var Cmd = &cobra.Command{
	Use:   "server",
	Short: "Run fastdfs server",
	Long:  `Run fastdfs server`,
	Run: func(cmd *cobra.Command, args []string) {
		main()
	},
}

func main() {
	server.InitServer()
	server.Start()
}
