package webservice

import "os"

// ServerStatus return actual state and process data
// so you can test with url/state the correct installation of microservice
type ServerStatus struct {
	Running bool `json:"running"`
	// Name    string `json:"name"`
	Process string `json:"process"`
	Pid     int    `json:"pid"`
}

// NewServerStatus create default service status
func NewServerStatus() *ServerStatus {
	return &ServerStatus{
		Running: true,
		// Name:    "TODO",
		Process: os.Args[0],
		Pid:     os.Getpid(),
	}
}
