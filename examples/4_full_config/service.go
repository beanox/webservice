package main

import (
	"fmt"
	"net/http"

	"github.com/beanox/webservice"
	"github.com/gorilla/mux"
)

type service struct {
}

func (s *service) ConfigureRouter(router *mux.Router) (handler http.Handler, err error) {

	router.Handle("/", webservice.AppHandler(s.helloWorldFn).AllowAnonymous()).Methods("GET") // Normal request
	router.Handle("/secret", webservice.AppHandler(s.secretFn)).Methods("GET")
	router.Handle("/secret2", webservice.AppHandler(s.secretFn2).AllowScopes("scope1", "scope2")).Methods("GET")
	handler = router
	return
}

func (s *service) helloWorldFn(w http.ResponseWriter, r *http.Request, userInfo *webservice.UserInfo) error {
	w.Write([]byte("Hello world!"))
	return nil
}

func (s *service) secretFn(w http.ResponseWriter, r *http.Request, userInfo *webservice.UserInfo) error {
	// This function will be called only if user has a valid user token that will be verified with JWKS, otherwise 401 - Unauthorized will be returned

	user := "anonymous"
	if userInfo != nil {
		user = userInfo.UserID
	}
	msg := fmt.Sprintf("Hi %v, you are on the secret place", user)
	w.Write([]byte(msg))
	return nil
}

func (s *service) secretFn2(w http.ResponseWriter, r *http.Request, userInfo *webservice.UserInfo) error {
	user := "anonymous"
	if userInfo != nil {
		user = userInfo.UserID
	}
	msg := fmt.Sprintf("Hi %v, you are on the second secret place", user)
	w.Write([]byte(msg))
	return nil
}
