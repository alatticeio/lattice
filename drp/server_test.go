// Copyright 2025 Wireflow.io, Inc.
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

package drp

import (
	"fmt"
	"testing"
	client2 "wireflow/management/grpc/client"
	"wireflow/pkg/config"
	"wireflow/pkg/log"
)

func TestVerifyToken(t *testing.T) {

	client, err := client2.NewClient(&client2.GrpcConfig{
		Addr:   "console.linkany.io:32051",
		Logger: log.NewLogger(log.Loglevel, fmt.Sprintf("[%s] ", "mgtclient")),
	})

	if err != nil {
		t.Fatal(err)
	}

	cfg, err := config.GetLocalConfig()
	if err != nil {
		t.Fatal(err)
	}

	resp, err := client.VerifyToken(cfg.Token)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(resp.Token == cfg.Token)
}
