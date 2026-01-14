package token

import (
	"fmt"
	"wireflow/internal/config"
	"wireflow/internal/infra"
	"wireflow/pkg/cmd"

	"github.com/spf13/cobra"
)

// start cmd
func NewTokenCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token <sub-command>",
		Short: "",
		Long:  `该命令创建一个token，Peer使用token能一键入网`,
		Args:  cobra.MinimumNArgs(1),
	}

	cmd.AddCommand(tokenCreateCmd())

	return cmd
}

func tokenCreateCmd() *cobra.Command {
	var namespace, expiry string
	var limit int
	cmd := &cobra.Command{
		Use:   "create <token-name>",
		Short: "用户创建Token",
		// Long 字段可以用来详细解释这些参数是什么
		Long: `该命令会创建一个Token。
    
参数说明:
  token-name    创建的token名称`,
		Example: `   wireflow token create dev-team
  
  # 指定限制 5 台设备，有效期 7 天
wireflow token create dev-team --limit 5 --expiry 168h -n wireflow-system`,
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 参数获取
			tokenName := args[0]

			return runCreate(namespace, tokenName, expiry)

		},
	}

	fs := cmd.Flags()
	fs.StringVarP(&namespace, "namespace", "n", "", "namespace of token")
	fs.StringVarP(&expiry, "expiry", "e", "", "token expiry time")
	fs.IntVarP(&limit, "limit", "l", 0, "token limit")

	return cmd
}

func runCreate(namespace, name, expiry string) error {
	if config.GlobalConfig.SignalUrl == "" {
		config.GlobalConfig.SignalUrl = fmt.Sprintf("nats://%s:%d", infra.SignalingDomain, infra.DefaultSignalingPort)
		config.WriteConfig("siganl-url", config.GlobalConfig.SignalUrl)
	}
	client, err := cmd.NewClient(config.GlobalConfig.SignalUrl)
	if err != nil {
		return err
	}

	return client.CreateToken(namespace, name, expiry)
}
