package workspace

import (
	"github.com/spf13/cobra"
)

var Cmd = &cobra.Command{
	Use:   "workspace",
	Short: "工作空间管理命令",
	Long:  `管理 Wireflow 工作空间，包括创建、修复等操作`,
}

func init() {
	// 注册子命令
}
