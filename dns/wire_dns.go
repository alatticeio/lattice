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

package dns

import (
	"fmt"
	"log"

	"github.com/miekg/dns"
)

type LinkDNS struct {
	listenAddr  string
	upstreamDNS string
}

type DNSConfig struct {
	Records       map[string]string `json:"records"`
	UpstreamDNS   string            `json:"upstream_dns"`
	ListenAddress string            `json:"listen_address"`
}

func NewNativeDNS(cfg *DNSConfig) *LinkDNS {
	if cfg.ListenAddress == "" {
		cfg.ListenAddress = ":53"
	}

	if cfg.UpstreamDNS == "" {
		cfg.UpstreamDNS = "81.68.109.143:53"
	}

	return &LinkDNS{
		listenAddr:  cfg.ListenAddress,
		upstreamDNS: cfg.UpstreamDNS,
	}
}

//func loadConfig(filename string) error {
//	configLock.Lock()
//	defer configLock.Unlock()
//
//	data, err := ioutil.ReadFile(filename)
//	if err != nil {
//		return err
//	}
//
//	return json.Unmarshal(data, &config)
//}

func (l *LinkDNS) handleDNSRequest(w dns.ResponseWriter, r *dns.Msg) {
	m := new(dns.Msg)
	m.SetReply(r)
	m.Authoritative = true

	for _, q := range r.Question {
		switch q.Qtype {
		case dns.TypeA:
			log.Printf("查询域名: %s", q.Name)

			// 使用读锁读取配置
			//configLock.RLock()
			//ip, exists := config.Records[q.Name]
			//upstream := config.UpstreamDNS
			//configLock.RUnlock()

			//if exists {
			//	rr, err := dns.NewRR(fmt.Sprintf("%s A %s", q.Name, ip))
			//	if err == nil {
			//		m.Answer = append(m.Answer, rr)
			//		log.Printf("本地解析 %s -> %s", q.Name, ip)
			//	}
			//} else if l.upstreamDNS != "" {
			// 转发到上游服务器
			upstreamMsg := new(dns.Msg)
			upstreamMsg.SetQuestion(q.Name, q.Qtype)
			upstreamMsg.RecursionDesired = true

			c := new(dns.Client)
			response, _, err := c.Exchange(upstreamMsg, l.upstreamDNS)

			if err == nil && response != nil {
				m.Answer = append(m.Answer, response.Answer...)
				log.Printf("从上游服务器解析: %s", q.Name)
			} else {
				log.Printf("上游 DNS 查询失败: %v", err)
			}
			//	}
		}
	}

	err := w.WriteMsg(m)
	if err != nil {
		fmt.Println(err)
	}
}

func startServer(net, addr string) {
	server := &dns.Server{Addr: addr, Net: net}
	log.Printf("启动 DNS 服务器在 %s (%s)", addr, net)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("无法启动 %s 服务器: %s", net, err.Error())
	}
}

// nolint:all
func watchConfigFile(filename string) {
	// 这里可以添加代码来监视配置文件变化
	// 为简单起见，这里省略了该功能
}

func (l *LinkDNS) Start() error {
	//if len(os.Args) < 2 {
	//	log.Fatalf("用法: %s config.json", os.Args[0])
	//}
	//
	//configFile := os.Args[1]
	//if err := loadConfig(configFile); err != nil {
	//	log.Fatalf("加载配置失败: %v", err)
	//}
	//
	//go watchConfigFile(configFile)

	dns.HandleFunc(".", l.handleDNSRequest)

	// 默认监听地址
	//listenAddr := ":53"
	//if config.ListenAddress != "" {
	//	listenAddr = config.ListenAddress
	//}

	// 启动服务器
	go startServer("udp", l.listenAddr)
	go startServer("tcp", l.listenAddr)

	return nil
	//// 等待退出信号
	//sig := make(chan os.Signal, 1)
	//signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	//<-sig
	//log.Println("DNS 服务器关闭")
}
