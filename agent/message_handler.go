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

package agent

import (
	"context"
	"fmt"
	"wireflow/internal/infra"
	"wireflow/internal/log"
)

type Handler interface {
	HandleEvent(ctx context.Context, msg *infra.Message) error
	ApplyFullConfig(ctx context.Context, msg *infra.Message) error
}

// event handler for wireflow to handle event from management
type MessageHandler struct {
	deviceManager infra.AgentInterface
	logger        *log.Logger
	provisioner   infra.Provisioner
}

func NewMessageHandler(e infra.AgentInterface, logger *log.Logger, provisioner infra.Provisioner) *MessageHandler {
	return &MessageHandler{
		deviceManager: e,
		logger:        logger,
		provisioner:   provisioner,
	}
}

type HandlerFunc func(ctx context.Context, msg *infra.Message) error

func (h *MessageHandler) HandleEvent(ctx context.Context, msg *infra.Message) error {
	// 1. 基础合法性检查
	if msg == nil || msg.Current == nil {
		h.logger.Warn("dropping config update: nil or missing current peer")
		return nil
	}

	h.logger.Debug("config update received",
		"version", msg.ConfigVersion,
		"incremental", msg.Changes != nil)

	// 2. 增量处理逻辑 (Fast Path)
	// 只有当 Changes 不为 nil 且确实有变化时，才执行精细化的设备操作
	if msg.Changes != nil && msg.Changes.HasChanges() {
		h.logger.Debug("applying incremental changes", "summary", msg.Changes.Summary())

		// --- 地址与网络变更 ---
		if msg.Changes.AddressChanged {
			if msg.Current.Address == nil {
				// 情况 A: 节点失去了分配的 IP，执行清理
				if len(msg.Changes.NetworkLeft) > 0 {
					h.logger.Warn("node left network, clearing IP and peer table")
					if err := h.provisioner.ApplyIP("remove", "", h.deviceManager.GetDeviceName()); err != nil {
						return fmt.Errorf("failed to remove IP: %w", err)
					}
					h.deviceManager.RemoveAllPeers()
				}
			} else {
				// 情况 B: 分配了新地址，强制更新掩码为 /32 (WireGuard 标准做法)
				msg.Current.AllowedIPs = fmt.Sprintf("%s/32", *msg.Current.Address)
			}
		}

		// --- 密钥变更（预留逻辑） ---
		if msg.Changes.KeyChanged {
			h.logger.Info("WireGuard key rotation detected", "pub_key", msg.Current.PublicKey)
			// 这里可以触发本地密钥重生成或更新逻辑
		}

		// --- Peer 新增 ---
		if len(msg.Changes.PeersAdded) > 0 {
			for _, peer := range msg.Changes.PeersAdded {
				// 严格过滤掉自身，防止回环或配置冲突
				if peer.PublicKey == msg.Current.PublicKey {
					continue
				}
				h.logger.Debug("adding peer", "peer_id", peer.PeerID, "endpoint", peer.Endpoint)
				if err := h.deviceManager.AddPeer(peer); err != nil {
					// 记录错误但不中断，尝试处理后续 Peer
					h.logger.Error("failed to add peer", err, "peer_id", peer.PeerID)
				}
			}
		}

		// --- Peer 移除 ---
		if len(msg.Changes.PeersRemoved) > 0 {
			for _, peer := range msg.Changes.PeersRemoved {
				h.logger.Debug("removing peer", "peer_id", peer.PeerID)
				if err := h.deviceManager.RemovePeer(peer); err != nil {
					h.logger.Error("failed to remove peer", err, "peer_id", peer.PeerID)
				}
			}
		}
	} else {
		// 如果 Changes == nil，说明这是一次全量快照分发（Snapshot）
		h.logger.Debug("no incremental changes, falling back to full reconciliation")
	}

	// 3. 核心出口：最终一致性对齐 (Safe Path)
	// 无论有没有增量，最后都调用 ApplyFullConfig。
	// 该函数内部应实现“幂等性”：即如果内核状态已与 msg.Current 一致，则不执行任何写操作。
	if err := h.ApplyFullConfig(ctx, msg); err != nil {
		return fmt.Errorf("failed to apply full configuration: %w", err)
	}

	h.logger.Debug("config applied", "version", msg.ConfigVersion)
	return nil
}

// ApplyFullConfig when wireflow start, apply full config
func (h *MessageHandler) ApplyFullConfig(ctx context.Context, msg *infra.Message) error {
	h.logger.Debug("reconciling full config", "version", msg.ConfigVersion)
	var err error

	// 设置本机IP（注册时 ConfigMap 可能尚未就绪，依赖后续推送补齐地址）
	if msg.Current != nil && msg.Current.Address != nil {
		if err = h.provisioner.ApplyIP("add", *msg.Current.Address, h.deviceManager.GetDeviceName()); err != nil {
			h.logger.Error("failed to apply local IP", err, "addr", *msg.Current.Address)
			return err
		}
		// 将 msg.Current（含服务端分配的 AllowedIPs）回写到 peerManager，
		// 确保后续 ICE offer 的 Current 字段携带正确的 AllowedIPs。
		if msg.Current.AllowedIPs == "" {
			msg.Current.AllowedIPs = fmt.Sprintf("%s/32", *msg.Current.Address)
		}
		if err = h.deviceManager.AddPeer(msg.Current); err != nil {
			h.logger.Error("failed to register local peer", err)
			return err
		}
	}

	//设置Peers
	if err = h.applyRemotePeers(ctx, msg); err != nil {
		h.logger.Error("failed to sync remote peers", err)
		return err
	}

	if err = h.applyFirewallRules(ctx, msg); err != nil {
		h.logger.Error("failed to apply firewall rules", err)
		return err
	}

	h.logger.Debug("full config reconciled", "version", msg.ConfigVersion)
	return nil
}

func (h *MessageHandler) applyRemotePeers(ctx context.Context, msg *infra.Message) error {
	for _, peer := range msg.ComputedPeers {
		// add peer to peers cached and probe start
		if err := h.deviceManager.AddPeer(peer); err != nil {
			return err
		}
	}
	return nil
}

func (h *MessageHandler) applyFirewallRules(ctx context.Context, msg *infra.Message) error {
	if msg.ComputedRules == nil {
		return nil
	}
	return h.provisioner.Provision(msg.ComputedRules)
}
