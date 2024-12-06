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
func New() webservice.WebService {
	return webservice.New(&testService{})
}
