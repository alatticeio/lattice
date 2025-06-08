package node

// LinkFlags is a struct that contains the flags that are passed to the client
type LinkFlags struct {
	LogLevel      string
	RedisAddr     string
	RedisPassword string
	InterfaceName string
	ForceRelay    bool
	AppKey        string

	//Url
	ManagementUrl string
	SignalingUrl  string
	TurnServerUrl string
}
