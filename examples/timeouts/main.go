package main

import (
	"time"

	"github.com/beanox/webservice"
)

type testService struct {
}

func main() {
	svc := New()
	svc.SetTimeouts(time.Second*60, time.Second*60, 0)
	svc.Start()
}

// New creates App instance
func New() webservice.SimpleService {
	return webservice.NewSimpleService(&testService{})
}
