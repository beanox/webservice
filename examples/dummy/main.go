package main

import (
	"github.com/beanox/webservice"
)

type testService struct {
}

func main() {
	svc := New()
	svc.Start()
}

// New creates App instance
func New() webservice.SimpleService {
	return webservice.NewSimpleService(&testService{})
}
