package cmd

import (
	"bufio"
	"context"
	"fmt"
	"github.com/moby/term"
	"github.com/pion/turn/v4"
	"github.com/spf13/cobra"
	"k8s.io/klog/v2"
	"linkany/internal"
	"linkany/management/client"
	grpcclient "linkany/management/grpc/client"
	"linkany/pkg/config"
	"linkany/pkg/linkerrors"
	"linkany/pkg/redis"
	"os"
)

type loginOptions struct {
	Username  string
	Password  string
	RedisAddr string
	RedisPass string
}

func loginCmd() *cobra.Command {
	var opts loginOptions
	var cmd = &cobra.Command{
		Use:          "login",
		SilenceUsage: true,
		Short:        "login up",
		Long:         `when you are using up, you should logon first, login up use username and password which registered on our site, if you did not logon, you can not join any networks.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.Username, "username", "u", "", "username for up")
	fs.StringVarP(&opts.Password, "password", "p", "", "username for up")
	fs.StringVarP(&opts.RedisAddr, "redis-addr", "", "", "username for up")
	fs.StringVarP(&opts.RedisPass, "redis-password", "", "", "username for up")

	return cmd
}

// runJoin join a network cmd
func runLogin(opts loginOptions) error {
	var err error
	defer func() {
		if err == nil {
			klog.Infof("login success")
		}
	}()
	conf, err := config.InitConfig()
	if err != nil {
		return err
	}
	if opts.Password == "" {
		if opts.Username == "" {
			opts.Username, _ = readLine("username: ", false)
		}

		if opts.Password == "" {
			//	if token, err := readLine("Token", true); err != nil {
			//		return errors.New("token required")
			//	} else {
			//		opts.Password = token
			//	}
			//} else {
			if password, err := readLine("password: ", false); err != nil {
				return linkerrors.ErrPasswordRequired
			} else {
				opts.Password = password
			}
		}
	}

	grpcClient, err := grpcclient.NewClient(&grpcclient.GrpcConfig{Addr: internal.ManagementDomain + ":32051"})
	if err != nil {
		return err
	}

	client := client.NewClient(&client.ClientConfig{
		GrpcClient: grpcClient,
		Conf:       conf,
	})
	user := &config.User{
		Username: opts.Username,
		Password: opts.Password,
	}
	err = client.Login(user)

	if err != nil {
		return err
	}

	//set turn key to redis
	if opts.RedisAddr != "" && opts.RedisPass != "" {
		// set user to redis
		client, err := redis.NewClient(&redis.ClientConfig{
			Addr:     opts.RedisAddr,
			Password: opts.RedisPass,
		})

		if err != nil {
			return fmt.Errorf("failed to connect redis: %v", err)
		}

		key := turn.GenerateAuthKey(opts.Username, "linkany.io", opts.Password)
		if err = client.Set(context.Background(), opts.Username, string(key)); err != nil {
			return fmt.Errorf("failed to set user turnKey to redis: %v", err)
		}

	}

	return err
}

func readLine(prompt string, slient bool) (string, error) {
	fmt.Print(prompt)
	if slient {
		fd := os.Stdin.Fd()
		state, err := term.SaveState(fd)
		if err != nil {
			fmt.Println(err)
			return "", err
		}
		term.DisableEcho(fd, state)
		defer term.RestoreTerminal(fd, state)
	}

	reader := bufio.NewReader(os.Stdin)
	line, _, err := reader.ReadLine()
	if err != nil {
		return "", err
	}
	if slient {
		fmt.Println()
	}

	return string(line), nil
}
