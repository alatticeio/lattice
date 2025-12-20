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
	"wireflow/internal/grpc"
)

// 指定网络里添加节点
func (n *networkManager) AddOrRmNode(ctx context.Context, networkId, action string, nodeIds []string) error {
	params := &NetworkParams{
		Name: networkId,
	}

	params.AppIds = append(params.AppIds, nodeIds...)

	bs, err := json.Marshal(params)
	if err != nil {
		return err
	}

	message := &grpc.ManagementMessage{
		Body: bs,
	}

	switch action {
	case "add":
		message.Type = grpc.Type_MessageTypeNetworkAddNode
	case "rm":
		message.Type = grpc.Type_MessageTypeNetworkRemoveNode
	}
	_, err = n.client.Request(ctx, message)
	if err != nil {
		return err
	}

	//fmt.Fprintln(os.Stdout, string(resp.Body))
	return nil
}
