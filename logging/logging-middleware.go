package logging

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

// Logging object
type Logging struct {
	logger *logrus.Entry
}

// New creates new Logging handler/middeware
func New(logger *logrus.Entry) *Logging {
	return &Logging{
		logger: logger,
	}
}

// Middleware returns middleware function that can be used in router.Use()
func (l *Logging) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l.logger.WithFields(logrus.Fields{"method": r.Method, "path": r.RequestURI}).Tracef("http request")
		h.ServeHTTP(w, r)
	})
}
