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

package network

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"time"
	"wireflow/internal/config"
	"wireflow/internal/core/domain"
	"wireflow/management/dto"
	"wireflow/management/nats"

	"github.com/spf13/viper"
)

// NetworkManager operations for network
type NetworkManager interface {
	CreateNetwork(ctx context.Context, opts *config.NetworkOptions) error
	JoinNetwork(ctx context.Context, opts *config.NetworkOptions) error
	LeaveNetwork(ctx context.Context, opts *config.NetworkOptions) error
	AddOrRmNode(ctx context.Context, networkId, action string, nodeIds []string) error
}

var (
	_ NetworkManager = (*networkManager)(nil)
)

type networkManager struct {
	client domain.SignalService
}

func NewNetworkManager(signalUrl string) (NetworkManager, error) {
	natsClient, err := nats.NewNatsService(signalUrl)
	if err != nil {
		return nil, err
	}

	return &networkManager{client: natsClient}, nil
}

func (n *networkManager) CreateNetwork(ctx context.Context, opts *config.NetworkOptions) error {

	params := &dto.NetworkDto{
		Name: opts.Name,
		CIDR: opts.CIDR,
	}

	bs, err := json.Marshal(params)
	if err != nil {
		return err
	}

	_, err = n.client.Request(ctx, "wireflow.signals.register", "create_network", bs)
	if err != nil {
		return err
	}

	return nil
}

func (n *networkManager) JoinNetwork(ctx context.Context, opts *config.NetworkOptions) error {
	params := &dto.NetworkDto{
		Name: opts.Name,
		CIDR: opts.CIDR,
	}

	params.AppIds = append(params.AppIds, viper.GetString(config.APP_ID))

	bs, err := json.Marshal(params)
	if err != nil {
		return err
	}

	resp, err := n.client.Request(ctx, "wireflow.signals.register", "join_network", bs)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, string(resp))
	return nil
}

func (n *networkManager) LeaveNetwork(ctx context.Context, opts *config.NetworkOptions) error {
	params := &dto.NetworkDto{
		Name: opts.Name,
		CIDR: opts.CIDR,
	}
	params.AppIds = append(params.AppIds, viper.GetString(config.APP_ID))

	bs, err := json.Marshal(params)
	if err != nil {
		return err
	}

	resp, err := n.client.Request(ctx, "wireflow.signals.register", "leave_network", bs)
	if err != nil {
		return err
	}

	fmt.Fprintln(os.Stdout, string(resp))
	return nil
}

// 定义 ID 的字符集：大写字母 (A-Z) 和数字 (0-9)
const baseCharset = "abcdefghijklmnopqrstuvwxyz0123456789"

// 定义生成的 ID 长度
const idLength = 10

// GenerateNetworkID 生成一个指定长度 (10位) 的随机网络 ID。
// ID 仅包含大写字母和数字。
func GenerateNetworkID() string {
	rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, idLength)

	// 1. 生成所有 10 位的基础字符 (a-z0-9)
	for i := range b {
		b[i] = baseCharset[rand.Intn(len(baseCharset))]
	}

	return string(b)
}
