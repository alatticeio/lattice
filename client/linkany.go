package client

// ClientFlags is a struct that contains the flags that are passed to the client
type ClientFlags struct {
	LogLevel      string
	RedisAddr     string
	RedisPassword string
	InterfaceName string
	ForceRelay    bool

	//Url
	ManagementUrl string
	SignalingUrl  string
	TurnServerUrl string
}
