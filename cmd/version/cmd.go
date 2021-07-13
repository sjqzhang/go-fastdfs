package version

import (
	"fmt"

	"github.com/sjqzhang/go-fastdfs/server"
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "version",
	Short: "version",
	Long:  `version`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("Version   : %s\n", server.VERSION)
		fmt.Printf("GO_VERSION: %s\n", server.GO_VERSION)
		fmt.Printf("GIT_COMMIT: %s\n", server.GIT_VERSION)
		fmt.Printf("BUILD_TIME: %s\n", server.BUILD_TIME)
	},
}
