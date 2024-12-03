package servererror

import (
	"encoding/json"
	"fmt"
	"net/http"
	"runtime/debug"
	"strings"

	"github.com/sirupsen/logrus"
)

// ProcessHTTPError writes formatted error response to w
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
		// Log error with appropriate severity level based on error code
		logger := logrus.WithError(serverError)

		if serverError.Code >= 500 {
			// Log server errors (5xx) as errors
			logger.WithField("code", serverError.Code).Error("server error")
			if serverError.Stack != "" {
				logrus.Error(formatStack(serverError, serverError.Stack))
			}
		} else {
			// Log client errors (4xx) as warnings
			logger.WithField("code", serverError.Code).Warn("server error")
			if serverError.Stack != "" {
				logrus.Warn(formatStack(serverError, serverError.Stack))
			}
		}

		// Include parent error details for debugging if available
		if serverError.Parent != nil {
			logrus.WithError(serverError.Parent).Debug("server error info")
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

var (
	functionsToIgnore = []string{
		"runtime/debug.Stack",
		"github.com/beanox/webservice/servererror.ServerError",
		"github.com/beanox/webservice/servererror.ProcessHTTPError",
	}

	functionsToHide = []string{
		"net/http.HandlerFunc.ServeHTTP",
		"github.com/beanox/webservice/authorization.(*Authorization).Middleware.func1",
		"github.com/beanox/webservice/logging.(*Logging).Middleware.func1",
		"net/http.serverHandler.ServeHTTP",
		"net/http.(*conn).serve",
		"github.com/gorilla/mux.(*Router).ServeHTTP",
		"github.com/rs/cors.(*Cors).Handler.func1",
		"github.com/beanox/webservice/authorization.AppHandler.ServeHTTP",
	}
)

func formatStack(e *ServerErrorData, stack string) string {
	var result strings.Builder
	result.WriteString(fmt.Sprintf("%s\n", e.Error()))
	result.WriteString("  Stack Trace:\n")

	lines := strings.Split(stack, "\n")

	var lastWasHidden bool
	var foundFirstFunction bool
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if strings.HasPrefix(line, "\t") {
			// This is a file line
			line = strings.TrimSpace(line)
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				file := parts[0]
				lineNum := strings.Split(parts[1], " ")[0]

				// Get the previous line which contains the function call
				if i > 0 {
					funcLine := strings.TrimSpace(lines[i-1])
					funcParts := strings.Split(funcLine, "(")
					if len(funcParts) > 0 {
						funcName := funcParts[0]

						funcParams := strings.TrimSpace(funcParts[len(funcParts)-1])
						if len(funcParts) > 2 {
							funcName = strings.Join(funcParts[:len(funcParts)-1], "(")
						}

						// Check if function should be ignored (only at top) or hidden
						shouldSkip := false
						if !foundFirstFunction {
							for _, ignore := range functionsToIgnore {
								if strings.Contains(funcName, ignore) {
									shouldSkip = true
									break
								}
							}
						}

						isHidden := false
						for _, hide := range functionsToHide {
							if strings.Contains(funcName, hide) {
								isHidden = true
								shouldSkip = true
								break
							}
						}

						if shouldSkip {
							if isHidden && !lastWasHidden {
								result.WriteString("    ...\n")
								lastWasHidden = true
							}
							continue
						}

						foundFirstFunction = true
						lastWasHidden = false
						params := "(" + funcParams + ")"
						result.WriteString(fmt.Sprintf("    %s:%s - %s%s\n", file, lineNum, funcName, params))
					}
				}
			}
		}
	}

	return result.String()
}
