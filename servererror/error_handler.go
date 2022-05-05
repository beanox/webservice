package servererror

import (
	"encoding/json"
	"net/http"
	"runtime/debug"

	"github.com/sirupsen/logrus"
)

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
			logrus.WithError(serverError).Error("server error")
			logrus.WithError(serverError.Parent).WithFields(logrus.Fields{"callstack": string(debug.Stack())}).Debug("server error info")
		} else {
			logrus.WithError(serverError).Warn("server error")
			if serverError.Parent != nil {
				logrus.WithError(serverError.Parent).Debug("server error info")
			}
		}

		if serverError.Parent != nil {
			serverError.Description = serverError.Parent.Error()
		}
		serverError.Stack = string(debug.Stack())

		b, _ := json.Marshal(serverError)
		logrus.WithField("response", string(b)).Trace("server response")

		w.WriteHeader(serverError.Code)
		w.Write(b)
	}
}
