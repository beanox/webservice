package webservice

import (
	"net/http"

	"github.com/beanox/webservice/servererror"
)

// AppHandler is handler that take care of content type and error handling
type AppHandler func(w http.ResponseWriter, r *http.Request) error

// AppHandlerWithUserID is handler that take care of userID, content type and error handling
type AppHandlerWithUserID func(userID string, w http.ResponseWriter, r *http.Request) error

// Satisfies the http.Handler interface
func (ah AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	err := ah(w, r)
	servererror.ProcessHTTPError(err, w, r)
}

// Satisfies the http.Handler interface
func (ah AppHandlerWithUserID) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	userID := r.Header.Get("X-User-Id")
	var err error
	if userID == "" {
		err = servererror.ServerError(nil, http.StatusUnauthorized, "Unauthorized")
	} else {
		err = ah(userID, w, r)
	}
	servererror.ProcessHTTPError(err, w, r)
}
