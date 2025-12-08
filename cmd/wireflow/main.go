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

package main

import (
	"flag"
	"fmt"
	"os"
	"wireflow/client"
	"wireflow/pkg/config"
	"wireflow/pkg/log"
)

func main() {
	flags := &config.Flags{}
	//configFile := flag.String("config", "/etc/wireflow/client.yaml", "config file")
	flag.StringVar(&flags.InterfaceName, "interface-name", "", "name which create interface use")
	flag.BoolVar(&flags.ForceRelay, "force-relay", false, "force relay mode")
	flag.StringVar(&flags.LogLevel, "log-level", "silent", "log level (silent, info, error, warn, verbose)")
	flag.StringVar(&flags.ManagementUrl, "control-url", "", "management server url, need not give when you are using our service")
	flag.StringVar(&flags.TurnServerUrl, "turn-url", "", "just need modify when you custom your own relay server")
	flag.StringVar(&flags.SignalingUrl, "", "", "signaling service, not need to modify")
	flag.BoolVar(&flags.DaemonGround, "daemon", false, "run in daemon mode, default is forground mode")
	flag.BoolVar(&flags.MetricsEnable, "metrics", false, "enable metrics")
	flag.BoolVar(&flags.DnsEnable, "dns", false, "enable dns")
	flag.Parse()

	log.SetLogLevel(flags.LogLevel)
	if err := client.Start(flags); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
