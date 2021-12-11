package webservice

import (
	"encoding/json"
	"net/http"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

// AppHandler is handler that take care of content type and error handling
type AppHandler func(w http.ResponseWriter, r *http.Request) error

// AppHandlerWithUserID is handler that take care of userID, content type and error handling
type AppHandlerWithUserID func(userID string, w http.ResponseWriter, r *http.Request) error

// ProcessHTTPError writes formated error response to w
func ProcessHTTPError(err error, w http.ResponseWriter, _ *http.Request) {
	if err != nil {
		w.Header().Set("X-Content-Type-Options", "nosniff")

		var serverError *ServerErrorData

		switch e := err.(type) {
		case *ServerErrorData:
			serverError = e

		default:
			serverError = ServerError(err, 500, "Internal Server Error")
		}

		if serverError.Code >= 500 {
			logger.WithError(serverError).Error("server error")
			logger.WithError(serverError.Parent).WithFields(logrus.Fields{"callstack": string(debug.Stack())}).Debug("server error info")
		} else {
			logger.WithError(serverError).Warn("server error")
			if serverError.Parent != nil {
				logger.WithError(serverError.Parent).Debug("server error info")
			}
		}

		if serverError.Parent != nil {
			serverError.Description = serverError.Parent.Error()
		}
		serverError.Stack = string(debug.Stack())

		b, _ := json.Marshal(serverError)
		logger.WithField("response", string(b)).Trace("server response")

		w.WriteHeader(serverError.Code)
		w.Write(b)
	}
}

// Satisfies the http.Handler interface
func (ah AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	err := ah(w, r)
	ProcessHTTPError(err, w, r)
}

// Satisfies the http.Handler interface
func (ah AppHandlerWithUserID) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	userID := r.Header.Get("X-User-Id")
	var err error
	if userID == "" {
		err = ServerError(nil, http.StatusUnauthorized, "Unauthorized")
	} else {
		err = ah(userID, w, r)
	}
	ProcessHTTPError(err, w, r)
}
