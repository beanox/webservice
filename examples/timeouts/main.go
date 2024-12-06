package main

import (
	"time"

	"github.com/beanox/webservice"
)

type testService struct {
}

func main() {
	svc := New()
	svc.Start()
}

// New creates App instance
func New() webservice.WebService {
	return webservice.New(
		&testService{},
		webservice.WithTimeouts(time.Second*60, time.Second*60, 0),
	)
}
