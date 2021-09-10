package authorization

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/beanox/webservice"

	"github.com/dgrijalva/jwt-go"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/sirupsen/logrus"
)

// UserInfo information about authenticated user
type UserInfo struct {
	UserID string   `json:"uid,omitempty"`
	Email  string   `json:"email,omitempty"`
	Scopes []string `json:"scopes,omitempty"`
}

// HasScope returns if given scope is included in user info
func (ui *UserInfo) HasScope(scope string) bool {
	for idx := range ui.Scopes {
		if ui.Scopes[idx] == scope {
			return true
		}
	}
	return false
}

type contextType int

const (
	contextTypeUserInfo contextType = iota
)

// AppHandler is handler that will fail if user is not authorized (based on token + required scope)
type AppHandler func(w http.ResponseWriter, r *http.Request) error

// AppHandlerWithUserInfo is a handler that has extra *UserInfo as parameter
type AppHandlerWithUserInfo func(userInfo *UserInfo, w http.ResponseWriter, r *http.Request) error

// AppHandlerWithUserInfoAllowAnonymous is a handler that allows user also not to be authenticated. In this case userInfo is nil
type AppHandlerWithUserInfoAllowAnonymous func(userInfo *UserInfo, w http.ResponseWriter, r *http.Request) error

// Satisfies the http.Handler interface
func (ah AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	userInfo, ok := r.Context().Value(contextTypeUserInfo).(*UserInfo)

	var err error
	if !ok || userInfo == nil {
		err = webservice.ServerError(nil, http.StatusUnauthorized, "Unauthorized")
	} else {
		err = ah(w, r)
	}
	webservice.ProcessHTTPError(err, w, r)
}

// Satisfies the http.Handler interface
func (ah AppHandlerWithUserInfo) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	userInfo, ok := r.Context().Value(contextTypeUserInfo).(*UserInfo)

	var err error
	if !ok || userInfo == nil {
		err = webservice.ServerError(nil, http.StatusUnauthorized, "Unauthorized")
	} else {
		err = ah(userInfo, w, r)
	}
	webservice.ProcessHTTPError(err, w, r)
}

// Satisfies the http.Handler interface
func (ah AppHandlerWithUserInfoAllowAnonymous) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	userInfo, _ := r.Context().Value(contextTypeUserInfo).(*UserInfo)
	err := ah(userInfo, w, r)
	webservice.ProcessHTTPError(err, w, r)
}

// Authorization object
type Authorization struct {
	jwks          jwk.Set
	requiredScope string
}

// Middleware returns middleware function that can be used in router.Use()
func (a *Authorization) Middleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		var userInfo *UserInfo
		tokenString := r.Header.Get("Authorization")
		if tokenString != "" {
			splitToken := strings.Split(tokenString, "Bearer")
			if len(splitToken) != 2 {
				logrus.Errorf("Wrong Authorization header")
			} else {

				tokenString = strings.Trim(splitToken[1], " ")

				token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {

					keyID, ok := token.Header["kid"].(string)
					if !ok {
						return nil, fmt.Errorf("no key ID in token header")
					}

					key, keyFound := a.jwks.LookupKeyID(keyID)

					if keyFound {
						var publicKey rsa.PublicKey
						err := key.Raw(&publicKey)
						return &publicKey, err
					}

					return nil, fmt.Errorf("unable to find key with id: %s", keyID)
				})

				if err == nil {
					if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
						logrus.Tracef("Auth: UserID: %v/%v/%v", claims["sub"], claims["email"], claims["scopes"])
						var uid string
						var mail string
						var scopes []string

						if v, ok := claims["sub"].(string); ok {
							uid = v
						}

						if v, ok := claims["email"].(string); ok {
							mail = v
						}

						if v, ok := claims["scopes"].([]interface{}); ok {
							scopes = make([]string, 0)
							if ok {
								for _, vv := range v {
									if s, ok := vv.(string); ok {
										scopes = append(scopes, s)
									}
								}

							}
						}

						if uid != "" && mail != "" {
							userInfo = &UserInfo{
								UserID: uid,
								Email:  mail,
								Scopes: scopes,
							}

							// Check permissions
							if a.requiredScope != "" && a.requiredScope != "*" &&
								!userInfo.HasScope(a.requiredScope) && !userInfo.HasScope("*") {
								userInfo = nil
							}
						}
					}
				} else {
					logrus.WithError(err).Warnf("error decoding token")
				}

			}
		}

		if userInfo != nil {
			ctx := context.WithValue(r.Context(), contextTypeUserInfo, userInfo)
			h.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		h.ServeHTTP(w, r)
	})
}

// New create new AuthMiddleware object
func New(jwks jwk.Set, requiredScope string) (a *Authorization) {
	a = &Authorization{
		jwks:          jwks,
		requiredScope: requiredScope,
	}
	return
}
