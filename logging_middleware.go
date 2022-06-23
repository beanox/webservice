package webservice

import (
	"context"
	"net/http"

	"github.com/sirupsen/logrus"
)

// Logging object
type Logging struct {
	logger *logrus.Logger
}

// New creates new Logging handler/middleware
func NewLoggingMiddleware(logger *logrus.Logger) *Logging {
	return &Logging{
		logger: logger,
	}
}

// Middleware returns middleware function that can be used in router.Use()
func (l *Logging) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), contextTypeLogger, l.logger)
		if l.logger != nil {
			user := ""
			userInfo, ok := r.Context().Value(contextTypeUserInfo).(*UserInfo)
			if ok && userInfo != nil && userInfo != unauthenticatedUser {

				if userInfo == userWithInvalidToken {
					user = "user_with_invalid_token"
				} else {
					user = userInfo.UserID
				}
			}

			l.logger.WithFields(logrus.Fields{"method": r.Method, "path": r.RequestURI, "user": user}).Debugf("request")
		}
		h.ServeHTTP(w, r.WithContext(ctx))
	})
}
