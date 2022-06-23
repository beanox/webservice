package main

import (
	"fmt"
	"net/http"

	"github.com/beanox/webservice"
	"github.com/gorilla/mux"
)

type service struct {
}

// Router configuration
func (s *service) ConfigureRouter(router *mux.Router) (handler http.Handler, err error) {

	// Normal request - AllowAnonymous() will allow not authorized user to access it
	router.Handle("/", webservice.AppHandler(s.helloWorldFn).AllowAnonymous()).Methods("GET")
	// Authorized only with default scope from configuration (default = *)
	router.Handle("/secret", webservice.AppHandler(s.secretFn)).Methods("GET")
	// More specific Autorization configuration
	router.Handle("/secret2", webservice.AppHandler(s.secretFn2).AllowScopes("scope1", "scope2")).Methods("GET")
	// custom error
	router.Handle("/no-space", webservice.AppHandler(s.testFn).AllowAnonymous()).Methods("GET")

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

func (s *service) testFn(w http.ResponseWriter, r *http.Request, userInfo *webservice.UserInfo) error {

	// custom error can be simple error interface, but it's recomended to use webservice.ServerError to set valid status code
	return webservice.ServerError(nil, http.StatusInsufficientStorage, "not space on device")
}
