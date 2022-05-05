package main

import (
	"encoding/json"
	"net/http"

	"github.com/beanox/webservice"

	"github.com/gorilla/mux"
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

func (s *customRoutesService) ConfigureRouter(router *mux.Router) (handler http.Handler, err error) {

	// Added new route GET /test
	router.Handle("/test", webservice.AppHandler(s.doSomeTest)).Methods("GET")

	handler = router
	return
}

func (s *customRoutesService) doSomeTest(w http.ResponseWriter, r *http.Request) error {
	json.NewEncoder(w).Encode("test")
	return nil
}
