package main

import (
	"net/http"

	"github.com/beanox/webservice"
	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type helloWorldService struct {
}

func (s *helloWorldService) ConfigureRouter(router *mux.Router) (handler http.Handler, err error) {

	router.Handle("/", webservice.AppHandler(s.helloWorldFn)).Methods("GET")
	handler = router
	return
}

func (s *helloWorldService) helloWorldFn(w http.ResponseWriter, r *http.Request, userInfo *webservice.UserInfo) error {
	w.Write([]byte("Hello world!"))
	return nil
}

func main() {

	// Create web service
	svc := webservice.New(&helloWorldService{})

	// Set logger
	svc.SetLogger(logrus.StandardLogger())
	logrus.SetLevel(logrus.TraceLevel)

	// Start web service
	svc.Start()
}
