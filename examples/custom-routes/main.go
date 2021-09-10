package main

import (
	"encoding/json"
	"net/http"

	"github.com/beanox/webservice"
	"github.com/beanox/webservice/logging"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
)

type customRoutesService struct {
}

func main() {
	svc := New()
	svc.Start()
}

// New creates App instance
func New() webservice.SimpleService {
	return webservice.NewSimpleService(&customRoutesService{})
}

func (s *customRoutesService) GetHTTPHandler() (handler http.Handler, err error) {

	router := mux.NewRouter()

	// Added new route GET /status
	router.Handle("/status", webservice.AppHandler(s.getServerStatus)).Methods("GET")

	handler = router

	// Add logger (will be visible only in trace level)
	handler = logging.New(logrus.WithField("facility", "microservice")).Middleware(handler)

	return
}

func (s *customRoutesService) getServerStatus(w http.ResponseWriter, r *http.Request) error {
	json.NewEncoder(w).Encode(webservice.NewServerStatus())
	return nil
}
