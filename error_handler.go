package webservice

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"runtime"

	"github.com/sirupsen/logrus"
)

// processHTTPError writes formated error response to w
func processHTTPError(err error, w http.ResponseWriter, _ *http.Request, logger *logrus.Logger, fn interface{}) {
	if err != nil {
		w.Header().Set("X-Content-Type-Options", "nosniff")

		var serverError *ServerErrorData

		switch e := err.(type) {
		case *ServerErrorData:
			serverError = e

		default:
			serverError = ServerErrorWithoutStack(err, 500, "Internal Server Error")
		}

		if logger != nil {

			logEntry := logger.WithError(serverError)

			funcInfo := serverError.FunctionInfo
			if funcInfo == "" && fn != nil {
				funcInfo = getFunctionInfo(fn)
			}

			if funcInfo != "" {
				logEntry = logEntry.WithField("func", funcInfo)
			}

			if serverError.Code >= 500 {
				logEntry.Error("server error")

			} else {
				logEntry.Warn("server error")
				if serverError.Parent != nil {
					logger.WithError(serverError.Parent).Debug("server error info")
				}
			}
		}

		if serverError.Parent != nil {
			serverError.Description = serverError.Parent.Error()
		}

		b, _ := json.Marshal(serverError)
		if logger != nil {
			logger.WithField("response", string(b)).Trace("server response")
		}

		w.WriteHeader(serverError.Code)
		w.Write(b)
	}
}

func getFunctionInfo(fn interface{}) string {
	frames := runtime.CallersFrames([]uintptr{reflect.ValueOf(fn).Pointer()})
	frame, _ := frames.Next()
	return fmt.Sprintf("%s:%d:%s", frame.File, frame.Line, frame.Function)
}

func getCurrentFunctionInfo(skip int) string {
	pc := make([]uintptr, 15)
	n := runtime.Callers(skip+2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return fmt.Sprintf("%s:%d:%s", frame.File, frame.Line, frame.Function)
}
