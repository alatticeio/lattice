package node

// LinkFlags is a struct that contains the flags that are passed to the mgtClient
type LinkFlags struct {
	LogLevel      string
	RedisAddr     string
	RedisPassword string
	InterfaceName string
	ForceRelay    bool
	AppKey        string

	// DaemonGround is a flag to indicate whether the node should run in foreground mode
	DaemonGround  bool
	MetricsEnable bool
	DnsEnable     bool

	//Url
	ManagementUrl string
	SignalingUrl  string
	TurnServerUrl string
}
