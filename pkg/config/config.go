package config

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"

	"github.com/spf13/viper"
)

type Protocol string

type LoginInfo struct {
	Token string `json:"token,omitempty"`
}

// HttpRequest will take any real request to linkany server
type HttpRequest[T any] struct {
	T T
}

// PeerRegisterInfo current peer info, will register to linkany server,then on web
// you can configure the peer online.
type PeerRegisterInfo struct {
	ID                  string `json:"id,omitempty"`
	AppId               string `json:"appId,omitempty"`
	Hostname            string `json:"hostname,omitempty"`
	PrivateKey          string `json:"privateKey,omitempty"`
	PublicKey           string `json:"publicKey,omitempty"`
	PersistentKeepalive int    `json:"persistentKeepalive,omitempty"`

	Ufrag      string `json:"ufrag,omitempty"`
	Pwd        string `json:"pwd,omitempty"`
	TieBreaker uint32 `json:"tieBreaker,omitempty"`
	HostIP     string `json:"hostIP,omitempty"`  // inner ip port
	SrflxIP    string `json:"srflxIP,omitempty"` // nat ip port
	RelayIP    string `json:"relayIP,omitempty"` // relay ip port
	Status     int    `json:"status,omitempty"`
}

type PeerInfo struct {
	PrivateKey          string   `json:"privateKey,omitempty"`
	PublicKey           string   `json:"publicKey,omitempty"`
	PersistentKeepalive int      `json:"persistentKeepalive,omitempty"`
	AllowedIPS          []string `json:"allowedIPS,omitempty"`
}

func InitConfig() (config *LocalConfig, err error) {
	viper.SetConfigName("app")             // name of config file (without extension)
	viper.SetConfigType("yaml")            // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("/etc/wireflow/")  // path to look for the config file in
	viper.AddConfigPath("$HOME/.wireflow") // call multiple times to add many search paths
	viper.AddConfigPath(".")               // optionally look for config in the working directory
	viper.AddConfigPath("./deploy")

	// read default
	//viper.ReadConfig(bytes.NewBuffer(defaultYaml))
	//viper.SetDefault("node.ConsoleUrl", "https://www.tiptopsoft.cn")

	if err = viper.ReadInConfig(); err != nil { // Handle errors reading the config file
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if errors.As(err, &configFileNotFoundError) {
			// using default configuration
		}
	}

	if err = viper.UnmarshalExact(&config); err != nil {
		return nil, err
	}

	return

}

// GetAppId is a unique identify for a node
func GetAppId() (string, error) {
	l, err := GetLocalConfig()
	if err != nil {
		return "", err
	}

	if l.AppId == "" {
		var appId [5]byte
		if _, err := io.ReadFull(rand.Reader, appId[:]); err != nil {
			return "", errors.New("generate GetAppId failed")
		}

		l.AppId = hex.EncodeToString(appId[:])
		err = UpdateLocalConfig(l)
		if err != nil {
			return "", err
		}
	}

	return l.AppId, nil
}
