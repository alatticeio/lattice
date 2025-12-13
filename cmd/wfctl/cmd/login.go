// Copyright 2025 The Wireflow Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"wireflow/internal"
	"wireflow/management/client"
	mgtclient "wireflow/management/grpc/client"
	"wireflow/pkg/config"
	"wireflow/pkg/log"
	"wireflow/pkg/redis"
	"wireflow/pkg/wferrors"

	"github.com/moby/term"
	"github.com/pion/turn/v4"
	"github.com/spf13/cobra"
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
		Short:        "logon to wireflow",
		Long:         `when you are using wireflow, you should logon first,use username and password that registered on our site, once you logon, you can join your own created networks.`,

		PreRunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},

		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(opts)
		},
	}
	fs := cmd.Flags()
	fs.StringVarP(&opts.Username, "username", "u", "", "username for login")
	fs.StringVarP(&opts.Password, "password", "p", "", "username for login")
	fs.StringVarP(&opts.RedisAddr, "redis-addr", "", "", "redis-addr for your custom turn server")
	fs.StringVarP(&opts.RedisPass, "redis-password", "", "", "redis password for your custom turn server")

	return cmd
}

// runJoin join a network cmd
func runLogin(opts loginOptions) error {
	logger := log.NewLogger(log.Loglevel, "wireflow")
	var err error
	defer func() {
		if err == nil {
			logger.Infof("login success")
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
				return wferrors.ErrPasswordRequired
			} else {
				opts.Password = password
			}
		}
	}

	grpcClient, err := mgtclient.NewClient(&mgtclient.GrpcConfig{Addr: fmt.Sprintf("%s:%d", internal.ManagementDomain, internal.DefaultManagementPort), Logger: log.NewLogger(log.Loglevel, "mgtclient")})
	if err != nil {
		return err
	}

	mgtClient := client.NewClient(&client.ClientConfig{
		GrpcClient: grpcClient,
		Conf:       conf,
	})
	user := &config.User{
		Username: opts.Username,
		Password: opts.Password,
	}
	err = mgtClient.Login(user)

	if err != nil {
		return err
	}

	//set turn key to redis
	if opts.RedisAddr != "" && opts.RedisPass != "" {
		// set user to redis
		rdbClient, err := redis.NewClient(&redis.ClientConfig{
			Addr:     opts.RedisAddr,
			Password: opts.RedisPass,
		})

		if err != nil {
			return fmt.Errorf("failed to connect redis: %v", err)
		}

		key := turn.GenerateAuthKey(opts.Username, "wireflowio.com", opts.Password)
		if err = rdbClient.Set(context.Background(), opts.Username, string(key)); err != nil {
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
