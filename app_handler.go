package webservice

import (
	"net/http"

	"github.com/sirupsen/logrus"
)

type apphandler struct {
	fn HandlerFn
	// Specific settings for single route only
	allowedScopes           *[]string
	allowAnonymous          *bool
	invalidTokenIsAnonymous *bool
	invalidScopeIsAnonymous *bool
}

// WithRequiredScope implements AppHandlerBuilder
func (ah *apphandler) AllowScopes(allowedScopes ...string) Handler {
	ah.allowedScopes = &allowedScopes
	return ah
}

func (ah *apphandler) AllowAnonymous() Handler {
	v := true
	ah.allowAnonymous = &v
	ah.invalidTokenIsAnonymous = &v
	ah.invalidScopeIsAnonymous = &v
	return ah
}

func (ah *apphandler) InvalidTokenIsAnonymous() Handler {
	v := true
	ah.invalidTokenIsAnonymous = &v
	return ah
}

func (ah *apphandler) InvalidScopeIsAnonymous() Handler {
	v := true
	ah.invalidScopeIsAnonymous = &v
	return ah
}

type Handler interface {
	http.Handler
	AllowScopes(allowedScopes ...string) Handler
	AllowAnonymous() Handler
	InvalidTokenIsAnonymous() Handler
	InvalidScopeIsAnonymous() Handler
}

// AppHandler is handler that will fail if user is not authorized (based on token + required scope)
func AppHandler(fn HandlerFn) Handler {
	return &apphandler{
		fn: fn,
	}
}

// Satisfies the http.Handler interface
func (ah apphandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	var err error

	logger, _ := r.Context().Value(contextTypeLogger).(*logrus.Logger)

	a, hasAuth := r.Context().Value(contextTypeAuthorizationMiddleware).(*authorization)
	if hasAuth && a == nil {
		err = ServerError(nil, http.StatusInternalServerError, "Authorization info not available")
		processHTTPError(err, w, r, logger, nil)
		return
	}

	var userInfo *UserInfo = nil

	if hasAuth {

		authorizationEnabled := !a.disabled

		allowAnonymous := a.allowAnonymous
		if ah.allowAnonymous != nil {
			allowAnonymous = *ah.allowAnonymous
		}

		invalidTokenIsAnonymous := a.invalidScopeIsAnonymous
		if ah.invalidTokenIsAnonymous != nil {
			invalidTokenIsAnonymous = *ah.invalidTokenIsAnonymous
		}

		invalidScopeIsAnonymous := a.invalidScopeIsAnonymous
		if ah.invalidScopeIsAnonymous != nil {
			invalidScopeIsAnonymous = *ah.invalidScopeIsAnonymous
		}

		allowedScopes := []string{a.requiredScope}
		if ah.allowedScopes != nil {
			allowedScopes = *ah.allowedScopes
		}

		if authorizationEnabled {
			var ok bool
			userInfo, ok = r.Context().Value(contextTypeUserInfo).(*UserInfo)
			if (!ok || userInfo == nil) && !allowAnonymous {
				err = ServerError(nil, http.StatusInternalServerError, "Unable to get user info")
				processHTTPError(err, w, r, logger, nil)
				return
			}
			unauthorized := false
			if userInfo == unauthenticatedUser {
				if allowAnonymous {
					userInfo = nil
				} else {
					unauthorized = true
				}
			} else if userInfo == userWithInvalidToken {
				if invalidTokenIsAnonymous {
					userInfo = nil
				} else {
					unauthorized = true
				}
			} else {

				// Check permissions

				hasValidScope := false

				for _, scp := range allowedScopes {
					if scp == "" || scp == "*" || userInfo.HasScope(scp) {
						hasValidScope = true
						break
					}
				}

				if !hasValidScope {
					if invalidScopeIsAnonymous {
						userInfo = nil
					} else {
						err = ServerError(nil, http.StatusForbidden, "Forbidden")
						processHTTPError(err, w, r, logger, nil)
						return
					}
				}

			}

			if unauthorized {
				err = ServerError(nil, http.StatusUnauthorized, "Unauthorized")
				processHTTPError(err, w, r, logger, nil)
				return
			}
		}
	}
	err = ah.fn(w, r, userInfo)
	processHTTPError(err, w, r, logger, ah.fn)
}
