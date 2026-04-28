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

package server

import (
	"fmt"

	"wireflow/api/v1alpha1"
	"wireflow/management/dto"
	"wireflow/management/server/middleware"
	"wireflow/pkg/utils/resp"

	"github.com/gin-gonic/gin"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (s *Server) peeringRouter() {
	g := s.Group("/api/v1/peering")
	g.Use(middleware.AuthMiddleware())
	{
		g.GET("/list", s.tenantMiddleware.Handle(), s.listPeerings)
		g.POST("", s.tenantMiddleware.Handle(), s.createPeering)
		g.DELETE("/:name", s.tenantMiddleware.Handle(), s.deletePeering)
		g.GET("/gateway-info", s.gatewayInfo())
	}
}

func (s *Server) listPeerings(c *gin.Context) {
	vos, err := s.peeringService.List(c.Request.Context())
	if err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, vos)
}

func (s *Server) createPeering(c *gin.Context) {
	var d dto.PeeringDto
	if err := c.ShouldBindJSON(&d); err != nil {
		resp.BadRequest(c, "invalid params")
		return
	}
	if d.NamespaceB == "" {
		resp.BadRequest(c, "namespaceB is required")
		return
	}
	result, err := s.peeringService.Create(c.Request.Context(), &d)
	if err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, result)
}

func (s *Server) deletePeering(c *gin.Context) {
	name := c.Param("name")
	if err := s.peeringService.Delete(c.Request.Context(), name); err != nil {
		resp.Error(c, err.Error())
		return
	}
	resp.OK(c, nil)
}

// gatewayInfo returns the gateway peer's public key, IP, and network CIDR for a
// given namespace/network. Remote clusters call this to set up cross-cluster tunnels.
//
// Query params:
//   - namespace (required): the K8s namespace of the local network
//   - network   (required): the WireflowNetwork name
func (s *Server) gatewayInfo() gin.HandlerFunc {
	return func(c *gin.Context) {
		ns := c.Query("namespace")
		networkName := c.Query("network")
		if ns == "" || networkName == "" {
			resp.BadRequest(c, "namespace and network are required")
			return
		}

		ctx := c.Request.Context()

		var network v1alpha1.WireflowNetwork
		if err := s.client.Get(ctx, types.NamespacedName{Namespace: ns, Name: networkName}, &network); err != nil {
			if k8serrors.IsNotFound(err) {
				resp.Error(c, fmt.Sprintf("network %s/%s not found", ns, networkName))
			} else {
				resp.Error(c, err.Error())
			}
			return
		}
		if network.Status.Phase != v1alpha1.NetworkPhaseReady || network.Status.ActiveCIDR == "" {
			resp.Error(c, fmt.Sprintf("network %s/%s is not ready (phase=%s)", ns, networkName, network.Status.Phase))
			return
		}

		var peerList v1alpha1.WireflowPeerList
		if err := s.client.List(ctx, &peerList, client.InNamespace(ns), client.MatchingLabels{
			"wireflow.run/gateway":                              "true",
			fmt.Sprintf("wireflow.run/network-%s", networkName): "true",
		}); err != nil {
			resp.Error(c, err.Error())
			return
		}
		if len(peerList.Items) == 0 {
			resp.Error(c, fmt.Sprintf("no gateway peer found in %s/%s", ns, networkName))
			return
		}
		gw := &peerList.Items[0]
		if gw.Status.AllocatedAddress == nil {
			resp.Error(c, "gateway peer has no allocated address yet")
			return
		}

		resp.OK(c, gin.H{
			"publicKey": gw.Spec.PublicKey,
			"gatewayIP": *gw.Status.AllocatedAddress,
			"cidr":      network.Status.ActiveCIDR,
			"appId":     gw.Spec.AppId,
			"peerId":    gw.Spec.PeerId,
		})
	}
}
