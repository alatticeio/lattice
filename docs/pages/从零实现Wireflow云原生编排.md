一、为什么需要 WireGuard 编排

WireGuard 本身是极简主义的——内核模块、一个虚拟网卡、一份配置文件。这种简洁是它的优点，也是它的边界。当你需
要在几十台、几百台机器之间动态建立 Overlay 网络，让节点自由加入退出、分配地址不冲突、跨 NAT                
打洞、按策略控制流量，WireGuard 内核给你的那把锤子就不够用了。

你需要一套编排层。

传统方案往往是在 WireGuard 之上包一层 daemon，手工维护 /etc/wireguard/*.conf，用 cron 定期                 
sync。这条路走到多云、边缘设备、k8s 原生场景时就会崩塌：配置漂移、地址冲突、NAT
打洞失败没有重试、节点下线后对端残留无效 peer。

Wireflow 的出发点是：把 WireGuard 节点当作 Kubernetes 资源来声明，用 Operator 来编排，用 NATS 来信令，用   
ICE 来穿透。
                                                                                                             
---                                                       
二、整体架构

┌─────────────────────────────────────────────────────────┐
│                   Control Plane (wireflowd)              │
│                                                          │                                               
│  ┌───────────────┐  ┌──────────────┐  ┌──────────────┐  │
│  │  K8s Operator │  │  NATS Server │  │  Management  │  │                                                
│  │  (Reconciler) │  │  (embedded)  │  │  API / UI    │  │
│  └───────┬───────┘  └──────┬───────┘  └──────────────┘  │                                                
│          │ CRD CRUD        │ Pub/Sub                     │                                               
└──────────┼─────────────────┼───────────────────────────-─┘                                               
│                 │  wireflow.signals.peers.*                                                   
┌──────┴──────┐   ┌──────┴──────┐   ┌─────────────┐                                                    
│  Agent A    │   │  Agent B    │   │  Agent C    │                                                    
│  (wireflow) │   │  (wireflow) │   │  (wireflow) │                                                    
│             │   │             │   │             │                                                    
│  WireGuard  │◄──►  WireGuard  │◄──►  WireGuard  │   
│  TUN + UDP  │   │  TUN + UDP  │   │  TUN + UDP  │                                                    
└─────────────┘   └─────────────┘   └─────────────┘   
ICE / WRRP (P2P or relay)

三个层次职责分明：

┌────────┬───────────────────┬──────────────┐                                                              
│  层次  │       组件        │     职责     │
├────────┼───────────────────┼──────────────┤                                                              
│ 声明层 │ Kubernetes CRD    │ 描述期望状态 │             
├────────┼───────────────────┼──────────────┤
│ 控制层 │ Operator + NATS   │ 协调实际状态 │                                                              
├────────┼───────────────────┼──────────────┤                                                              
│ 数据层 │ Agent + WireGuard │ 执行配置变更 │                                                              
└────────┴───────────────────┴──────────────┘
                                                            
---
三、数据模型：用 CRD 描述网络拓扑

3.1 核心资源类型

WireflowNetwork — 一张 Overlay 网络：

type WireflowNetworkSpec struct {                                                                          
NetworkId    string            `json:"networkId"`                                                      
CIDR         string            `json:"cidr"`           // e.g. "10.10.0.0/16"
MTU          int               `json:"mtu,omitempty"`                                                  
PeerSelector *metav1.LabelSelector `json:"peerSelector,omitempty"`
Policies     []string          `json:"policies,omitempty"`                                             
}

WireflowPeer — 一个节点的期望状态：

type WireflowPeerSpec struct {
AppID         string   `json:"appId"`
InterfaceName string   `json:"interfaceName"`    // e.g. "wg0"                                         
PublicKey     string   `json:"publicKey,omitempty"`
Network       string   `json:"network,omitempty"`                                                      
AllowedIPs    []string `json:"allowedIPs,omitempty"`  
MTU           int      `json:"mtu,omitempty"`                                                          
}

type WireflowPeerStatus struct {                                                                           
Phase            PeerPhase   // Pending → Provisioning → Ready
AllocatedAddress string      // e.g. "10.10.1.5/24"                                                    
Conditions       []metav1.Condition                                                                    
}

节点的生命周期通过 Status.Phase 和 Status.Conditions                                                       
追踪，条件包括：Initialized、IPAllocated、NetworkConfigured、PolicyApplied。
                                                                                                             
---                                                       
四、IPAM：两级地址分配

IP 分配是 Overlay 网络编排最容易出错的环节。Wireflow 使用两级分配模型，完全依托 Kubernetes
资源的原子性来避免竞态。

4.1 第一级：子网分配

控制平面从 WireflowGlobalIPPool（如 10.10.0.0/16）中为每个网络分配一个 /24 子网：

func AllocateSubnet(networkName, globalPool string) (string, error) {                                      
// 枚举所有 WireflowSubnetAllocation，收集已用子网    
used := collectUsedSubnets()

      for _, subnet := range enumerate(globalPool, 24) {                                                     
          if used.Contains(subnet) {                        
              continue                                                                                       
          }                                                 
          // 用资源名唯一性做原子锁：如果创建成功就拿到了这个子网
          err := k8sClient.Create(ctx, &WireflowSubnetAllocation{                                            
              ObjectMeta: metav1.ObjectMeta{                                                                 
                  Name: networkName, // 同名资源只能存在一个                                                 
              },                                                                                             
              Spec: SubnetAllocationSpec{CIDR: subnet},     
          })                                                                                                 
          if err == nil {                                   
              return subnet, nil
          }
          // 409 Conflict → 被抢占，继续下一个
      }                                                                                                      
      return "", ErrNoAvailableSubnet
}

关键设计：用 Kubernetes 资源名唯一性作为分布式锁，无需额外的锁服务。

4.2 第二级：IP 分配

在子网范围内为每个 Peer 分配单个 IP：

func AllocateIP(networkCIDR string, peer *WireflowPeer) (string, error) {                                  
used := collectUsedEndpoints(namespace)

      for _, ip := range hosts(networkCIDR) {                                                                
          if ip.IsNetworkAddr() || ip.IsGateway() {         
              continue // 跳过 .0 和 .1                                                                      
          }                                                                                                  
          if used.Contains(ip) {                                                                             
              continue                                                                                       
          }                                                 
          err := k8sClient.Create(ctx, &WireflowEndpoint{
              ObjectMeta: metav1.ObjectMeta{
                  Name: fmt.Sprintf("%s-%s", networkName, peer.Name),                                        
              },
              Spec: EndpointSpec{IP: ip.String(), PeerRef: peer.Name},                                       
          })                                                                                                 
          if err == nil {
              return ip.String(), nil                                                                        
          }                                                                                                  
      }
      return "", ErrNoAvailableIP                                                                            
}

  ---
五、Operator：声明式协调

5.1 PeerReconciler 状态机

Pending                                                   
│  Generate WireGuard key pair
│  Add network label                                                                                     
▼
Provisioning                                                                                               
│  AllocateIP()                                         
│  Update status.allocatedAddress                                                                        
│  Set condition: IPAllocated=True
▼                                                                                                        
Ready                                                     
│  Publish config to NATS                                                                                
│  Set condition: NetworkConfigured=True

核心代码骨架：

func (r *PeerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
peer := &WireflowPeer{}                                                                                
r.Get(ctx, req.NamespacedName, peer)

      switch peer.Spec.Action {                                                                              
      case NodeJoinNetwork:
          return r.reconcileJoinNetwork(ctx, peer)                                                           
      case NodeLeaveNetwork:                                
          return r.reconcileLeaveNetwork(ctx, peer)
      }                                                                                                      
      return ctrl.Result{}, nil
}

func (r *PeerReconciler) reconcileJoinNetwork(ctx context.Context, peer *WireflowPeer) (ctrl.Result, error)
{
// 1. 生成密钥对（幂等：已有则跳过）
if peer.Spec.PublicKey == "" {                                                                         
privateKey, _ := wgtypes.GeneratePrivateKey()                                                      
peer.Spec.PublicKey = privateKey.PublicKey().String()                                              
peer.Spec.PrivateKey = privateKey.String()                                                         
r.Update(ctx, peer)                                                                                
return ctrl.Result{Requeue: true}, nil                                                             
}

      // 2. 分配 IP                                         
      if peer.Status.AllocatedAddress == "" {
          ip, err := r.ipam.AllocateIP(peer.Spec.Network, peer)                                              
          if err != nil {
              return ctrl.Result{RequeueAfter: 5 * time.Second}, err                                         
          }                                                                                                  
          peer.Status.AllocatedAddress = ip
          apimeta.SetStatusCondition(&peer.Status.Conditions, metav1.Condition{                              
              Type:   "IPAllocated",                        
              Status: metav1.ConditionTrue,                                                                  
          })
          r.Status().Update(ctx, peer)                                                                       
          return ctrl.Result{Requeue: true}, nil            
      }
                                                                                                             
      // 3. 下发配置
      r.publishNetworkConfig(ctx, peer)                                                                      
      peer.Status.Phase = PeerPhaseReady                    
      r.Status().Update(ctx, peer)                                                                           
      return ctrl.Result{}, nil
}

Reconciler 天然具备幂等性：每一步都先检查当前状态，已完成的步骤直接跳过。这是 controller-runtime           
的核心范式——不管触发多少次 Reconcile，最终效果相同。
                                                                                                             
---                                                       
六、信令层：嵌入式 NATS

Wireflow 没有引入独立的消息队列服务，而是在 wireflowd 进程内直接嵌入 NATS Server：

func RunEmbedded(conf *Config) (*natsserver.Server, error) {
opts := &natsserver.Options{                                                                           
Host:          "0.0.0.0",                         
Port:          4222,                                                                               
JetStream:     true,                              
StoreDir:      "data/nats-jetstream/",                                                             
MaxPayload:    1024 * 1024, // 1MB                                                                 
PingInterval:  20 * time.Second,                                                                   
}                                                                                                      
server, err := natsserver.NewServer(opts)                                                              
server.Start()                                        
server.ReadyForConnections(10 * time.Second)                                                           
return server, nil
}

优势：all-in-one 模式下只有一个进程、一个二进制，用户 docker run 即可启动完整控制平面。

NATS Subject 设计

wireflow.signals.peer              ← Agent 注册/上报                                                       
wireflow.signals.peers.{peerId}    ← 点对点配置下发                                                        
wireflow.signals.peers.{peerId}.ice ← ICE candidate 交换

peerId 是公钥的前 8 字节，兼顾唯一性和紧凑性：

type PeerID [8]byte

func PeerIDFromKey(pubKey wgtypes.Key) PeerID {                                                            
var id PeerID
copy(id[:], pubKey[:8])                                                                                
return id                                             
}

  ---
七、Agent：数据平面的全貌

Agent 是跑在每台机器上的轻量进程，负责实际的 WireGuard 操作。

7.1 启动流程

func NewAgent(ctx context.Context, conf *Config) (*Agent, error) {                                         
// 1. 创建 TUN 设备                                   
tun, err := infra.CreateTUN(conf.InterfaceName, conf.MTU)

      // 2. 打开 UDP 套接字（51820）                                                                         
      v4, _ := net.ListenUDP("udp4", &net.UDPAddr{Port: 51820})                                              
      v6, _ := net.ListenUDP("udp6", &net.UDPAddr{Port: 51820})                                              
   
      // 3. 创建 ICE UniversalUDPMux（复用同一端口做 NAT 穿透）                                              
      udpMux := ice.NewUniversalUDPMuxDefault(ice.UniversalUDPMuxParams{
          UDPConn: v4,                                                                                       
      })                                                    
                                                                                                             
      // 4. 连接 NATS                                                                                        
      natsConn, _ := nats.Connect(conf.SignalingURL)
                                                                                                             
      // 5. 向控制平面注册，获取密钥对和地址                                                                 
      identity, _ := ctrClient.Register(ctx, conf.Token)
                                                                                                             
      // 6. 创建 ProbeFactory（管理所有 P2P 连接）                                                           
      probeFactory := transport.NewProbeFactory(identity, natsConn, udpMux)                                  
                                                                                                             
      return &Agent{                                        
          iface:        wg.NewDevice(tun),                                                                   
          bind:         infra.NewDefaultBind(v4, v6, udpMux),                                                
          natsService:  nats.NewService(natsConn),
          probeFactory: probeFactory,                                                                        
          provisioner:  infra.NewProvisioner(conf.InterfaceName),
      }, nil                                                                                                 
}

7.2 配置变更处理

控制平面通过 NATS 推送 Message，Agent 收到后应用：

type Message struct {                                                                                      
EventType     EventType                                                                                
ConfigVersion string
Current       *Peer          // 自身当前状态                                                           
Network       *Network       // 所在网络配置                                                           
ComputedPeers []*Peer        // 应该建立连接的对端列表                                                 
ComputedRules *FirewallRule  // 防火墙规则                                                             
Changes       *DetailsInfo  // 增量变更（可选）                                                        
}

增量处理（Changes != nil 时）：

func (h *MessageHandler) HandleEvent(msg *Message) error {                                                 
changes := msg.Changes

      if changes.AddressChanged {                                                                            
          h.provisioner.ApplyIP("add", msg.Current.Address, ifName)
      }                                                                                                      
                                                            
      for _, peer := range changes.PeersAdded {                                                              
          h.deviceManager.AddPeer(&infra.SetPeer{           
              PublicKey:           peer.PublicKey,                                                           
              Endpoint:            peer.Endpoint,           
              AllowedIPs:          peer.AllowedIPs,                                                          
              PersistentKeepalive: 25,                                                                       
          })
      }                                                                                                      
                                                            
      for _, peer := range changes.PeersRemoved {                                                            
          h.deviceManager.RemovePeer(&infra.SetPeer{PublicKey: peer.PublicKey})
      }                                                                                                      
                                                            
      for _, policy := range changes.PoliciesAdded {                                                         
          h.provisioner.ApplyFirewallRule("add", policy)
      }                                                                                                      
      return nil                                                                                             
}

增量模式避免了全量 diff，在节点数量大时显著降低 Agent CPU 开销。
   
---                                                                                                        
八、NAT 穿透：ICE + WRRP 双保险

这是 Overlay 网络最棘手的问题：两台机器都在 NAT 后面，如何建立直连？

8.1 ICE 候选收集

func (d *ICEDialer) GatherCandidates() error {                                                             
agent, _ := ice.NewAgent(&ice.AgentConfig{                                                             
Urls: []*stun.URI{{                                                                                
Scheme: stun.SchemeTypeSTUN,                                                                   
Host:   "stun.wireflow.run",                                                                   
Port:   3478,                                 
}},                                                                                                
NetworkTypes: []ice.NetworkType{ice.NetworkTypeUDP4},
UDPMux:       d.udpMux, // 复用 Agent 的 51820 端口                                                
})

      agent.OnCandidate(func(c ice.Candidate) {                                                              
          if c != nil {                                                                                      
              // 通过 NATS 把候选发给对端                   
              d.signal.Publish(remoteSubject, &ICEMessage{                                                   
                  Type:      ICECandidate,                                                                   
                  Candidate: c.Marshal(),                                                                    
              })                                                                                             
          }                                                 
      })                                                                                                     
                                                            
      agent.GatherCandidates()
      return nil
}

8.2 探测协调（ProbeFactory）

每个远端 Peer 对应一个 Probe，负责协调连接建立：

type Probe struct {                                                                                        
localId  PeerIdentity                                                                                  
remoteId PeerIdentity
signal   SignalService                                                                                 
dialers  []Dialer  // [ICEDialer, WRRPDialer]         
}

func (p *Probe) Connect() error {                                                                          
for _, dialer := range p.dialers {                    
endpoint, err := dialer.Dial(ctx)
if err == nil {                                                                                    
// 拿到真实 UDP endpoint 后更新 WireGuard peer
p.agent.UpdatePeerEndpoint(p.remoteId, endpoint)                                               
return nil                                                                                     
}                                                                                                  
}                                                                                                      
return ErrCannotConnect                               
}

ICE 成功则直连（延迟低），失败则 fallback 到 WRRP relay（延迟高但保证可达）。

8.3 WRRP Endpoint 格式

// Endpoint 格式：wrrp://{remoteId}
// DefaultBind.ParseEndpoint() 识别 scheme 并路由给 WRRPDialer                                             
func (b *DefaultBind) ParseEndpoint(s string) (conn.Endpoint, error) {                                     
u, _ := url.Parse(s)                                                                                   
switch u.Scheme {                                                                                      
case "wrrp":                                                                                           
return &WRRPEndpoint{RemoteID: u.Host}, nil                                                        
default:
return parseUDPEndpoint(s)                                                                         
}                                                                                                      
}
                                                                                                             
---                                                       
九、平台适配：OS 差异的隔离

WireGuard 的 IP/路由操作在各平台命令不同，Wireflow 用接口隔离：

type Provisioner interface {                              
SetupInterface(conf *DeviceConfig) error                                                               
AddPeer(peer *SetPeer) error                          
RemovePeer(peer *SetPeer) error                                                                        
ApplyRoute(action, address, ifName string) error
ApplyIP(action, address, ifName string) error                                                          
}

- provision_linux.go → ip addr add, ip route add, iptables
- provision_darwin.go → ifconfig, route add, pf
- provision_windows.go → PowerShell, netsh

Agent 代码对平台无感知，测试时只需 mock Provisioner。
                                                                                                             
---                                                                                                        
十、All-in-One 与生产分离部署

开发测试用 all-in-one：单进程内嵌 NATS + SQLite，docker run 一键启动。

wireflowd                                                 
├── Embedded NATS  :4222                                                                                   
├── SQLite         ./wireflow.db                                                                           
└── Management API :8080

生产用分离式，通过 kustomize overlay 组合：

# config/wireflow/overlays/production/kustomization.yaml
resources:                                                                                                 
- ../../base
- ../../components/nats      # 独立 NATS StatefulSet                                                     
- ../../components/database  # MariaDB                                                                   
- ../../components/dex       # OIDC

两套部署模式复用同一套控制器代码，只是外部依赖的注入方式不同。
                                                                                                             
---                                                                                                        
十一、关键设计决策回顾

┌──────────────────────┬──────────────────────────────────────┬───────────────────────────────────┐
│         问题         │                 选择                 │               原因                │        
├──────────────────────┼──────────────────────────────────────┼───────────────────────────────────┤
│ 如何避免 IP 分配竞态 │ Kubernetes 资源名唯一性              │ 不依赖外部锁服务，CRD 即状态      │        
├──────────────────────┼──────────────────────────────────────┼───────────────────────────────────┤
│ 如何分发配置变更     │ NATS Pub/Sub                         │ 解耦控制平面与 Agent，天然 fanout │        
├──────────────────────┼──────────────────────────────────────┼───────────────────────────────────┤        
│ 如何处理 NAT         │ ICE + WRRP fallback                  │ 覆盖直连、STUN、TURN 所有场景     │        
├──────────────────────┼──────────────────────────────────────┼───────────────────────────────────┤        
│ 如何保证幂等         │ ConfigVersion + 全量 reconcile       │ 增量失败时可安全重放全量          │
├──────────────────────┼──────────────────────────────────────┼───────────────────────────────────┤        
│ 如何标识节点         │ AppID（逻辑）+ PeerID（密钥前8字节） │ 允许密钥轮转而不改变逻辑身份      │
├──────────────────────┼──────────────────────────────────────┼───────────────────────────────────┤        
│ 平台差异             │ Provisioner 接口隔离                 │ 平台代码不污染业务逻辑            │
└──────────────────────┴──────────────────────────────────────┴───────────────────────────────────┘
                                                            
---                                                                                                        
结语

WireGuard 的简洁是其内核层面的美德，但编排层的复杂性不会消失，只是被转移到了别处。Wireflow
的做法是把这份复杂性收敛到 Kubernetes Operator 模型里——声明式、幂等、事件驱动，用你已经熟悉的 CRD +        
Reconciler 范式来表达网络拓扑的期望状态，让 WireGuard     
的每一次配置变更都有来源可溯、有状态可查、有错误可重试。

这不是"WireGuard 加了一个管理界面"，而是从控制平面到数据平面的完整重新设计。