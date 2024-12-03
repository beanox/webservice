package webservice

import (
	"os"

	"github.com/spf13/viper"
)

// ServerStatus return actual state and process data
// so you can test with url/state the correct installation of microservice
type ServerStatus struct {
	Running   bool   `json:"running,omitempty"`
	Process   string `json:"process,omitempty"`
	Pid       int    `json:"pid,omitempty"`
	StripPath string `json:"strip_path,omitempty"`
	JwksURL   string `json:"jwks_url,omitempty"`
}

// NewServerStatus create default service status
func NewServerStatus() *ServerStatus {
	return &ServerStatus{
		Running:   true,
		Process:   os.Args[0],
		Pid:       os.Getpid(),
		StripPath: viper.GetString("strip_path"),
		JwksURL:   viper.GetString("authorization.jwks"),
	}
}
