// Copyright 2026 The Lattice Authors, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package cmd

import (
	"context"
	"fmt"
	"github.com/alatticeio/lattice/internal/agent/client"
	"github.com/alatticeio/lattice/internal/agent/config"
)

func runVersion() error {
	client, err := cmd.NewClient(config.Conf.ServerUrl, config.Conf.AuthToken)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	err = client.Info(context.Background())
	if err != nil {
		return err
	}
	return nil
}
