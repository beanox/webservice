package webservice

import (
	"os"

	"github.com/spf13/viper"
)

// ServerStatus return actual state and process data
// so you can test with url/state the correct installation of microservice
type ServerStatus struct {
	Running   bool   `json:"running"`
	Process   string `json:"process"`
	Pid       int    `json:"pid"`
	StripPath string `json:"strip_path"`
}

// NewServerStatus create default service status
func NewServerStatus() *ServerStatus {
	return &ServerStatus{
		Running:   true,
		Process:   os.Args[0],
		Pid:       os.Getpid(),
		StripPath: viper.GetString("strip_path"),
	}
}
