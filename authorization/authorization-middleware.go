package authorization

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/beanox/webservice/servererror"
	"github.com/golang-jwt/jwt/v4"
	"github.com/spf13/viper"

	"github.com/lestrrat-go/jwx/jwa"
	"github.com/lestrrat-go/jwx/jwk"
	"github.com/sirupsen/logrus"
)

// UserInfo information about authenticated user
type UserInfo struct {
	UserID string        `json:"uid,omitempty"`
	Email  string        `json:"email,omitempty"`
	Scopes []string      `json:"scopes,omitempty"`
	Claims jwt.MapClaims `json:"claims,omitempty"`
}

var userWithInvalidToken = &UserInfo{UserID: "_invalid_token_"}
var userWithInvalidScope = &UserInfo{UserID: "_invalid_scope_"}
var unauthenticatedUser = &UserInfo{UserID: "_unauthenticated_user_"}

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
	contextTypeAuthorizationMiddleware
)

// AppHandler is handler that will fail if user is not authorized (based on token + required scope)
type AppHandler func(w http.ResponseWriter, r *http.Request, userInfo *UserInfo) (err error)

// Satisfies the http.Handler interface
func (ah AppHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=UTF-8")
	var err error

	a, ok := r.Context().Value(contextTypeAuthorizationMiddleware).(*Authorization)
	if !ok || a == nil {
		err = servererror.ServerError(nil, http.StatusInternalServerError, "Authorization info not available")
		servererror.ProcessHTTPError(err, w, r)
		return
	}

	var userInfo *UserInfo = nil

	if !a.disabled {
		var ok bool
		userInfo, ok = r.Context().Value(contextTypeUserInfo).(*UserInfo)
		if (!ok || userInfo == nil) && !a.allowAnonymous {
			err = servererror.ServerError(nil, http.StatusInternalServerError, "Unable to get user info")
			servererror.ProcessHTTPError(err, w, r)
			return
		}
		unauthorized := false
		if userInfo == unauthenticatedUser {
			if a.allowAnonymous {
				userInfo = nil
			} else {
				unauthorized = true
			}
		} else if userInfo == userWithInvalidToken {
			if a.invalidTokenIsAnonymous {
				userInfo = nil
			} else {
				unauthorized = true
			}
		} else if userInfo == userWithInvalidScope {
			if a.invalidScopeIsAnonymous {
				userInfo = nil
			} else {
				err = servererror.ServerError(nil, http.StatusForbidden, "Forbidden")
				servererror.ProcessHTTPError(err, w, r)
				return
			}
		}

		if unauthorized {
			err = servererror.ServerError(nil, http.StatusUnauthorized, "Unauthorized")
			servererror.ProcessHTTPError(err, w, r)
			return
		}
	}

	err = ah(w, r, userInfo)
	servererror.ProcessHTTPError(err, w, r)
}

// Authorization object
type Authorization struct {
	jwks                    jwk.Set
	jwksURL                 string
	autoRefresh             *jwk.AutoRefresh
	requiredScope           string
	allowAnonymous          bool
	invalidTokenIsAnonymous bool
	invalidScopeIsAnonymous bool
	disabled                bool
	userValidator           func(userInfo *UserInfo) bool
}

// Middleware returns middleware function that can be used in router.Use()
func (a *Authorization) Middleware(h http.Handler) (handler http.Handler) {

	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := context.WithValue(r.Context(), contextTypeAuthorizationMiddleware, a)

		var userInfo *UserInfo = unauthenticatedUser
		if !a.disabled {
			tokenString := r.Header.Get("Authorization")
			if tokenString != "" {
				userInfo = userWithInvalidToken

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

						jwks := a.jwks
						var err error
						if a.autoRefresh != nil {
							jwks, err = a.autoRefresh.Fetch(context.Background(), a.jwksURL)
							if err != nil {
								return nil, err
							}
						}

						if jwks == nil {
							return nil, fmt.Errorf("jwks not available")
						}

						key, keyFound := jwks.LookupKeyID(keyID)
						if keyFound {
							alg := key.Algorithm()
							if alg == string(jwa.RS256) {
								var publicKey rsa.PublicKey
								err := key.Raw(&publicKey)
								return &publicKey, err
							} else if alg == string(jwa.ES256) {
								var publicKey ecdsa.PublicKey
								err := key.Raw(&publicKey)
								return &publicKey, err
							} else {
								return nil, fmt.Errorf("unsupported algorithm %s", alg)
							}
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

							if uid != "" {
								userInfo = &UserInfo{
									UserID: uid,
									Email:  mail,
									Scopes: scopes,
									Claims: claims,
								}

								// Check permissions
								if a.userValidator != nil {
									if !a.userValidator(userInfo) {
										userInfo = userWithInvalidScope
									}
								} else {
									if a.requiredScope != "" && a.requiredScope != "*" &&
										!userInfo.HasScope(a.requiredScope) && !userInfo.HasScope("*") {
										userInfo = userWithInvalidScope
									}
								}
							}
						}
					} else {
						logrus.WithError(err).Errorf("error decoding token")
					}
				}
			}
		}
		if userInfo != nil {
			ctx = context.WithValue(ctx, contextTypeUserInfo, userInfo)
		}

		h.ServeHTTP(w, r.WithContext(ctx))
	})
	return
}

// Options is a configuration container to setup Authorization middleware.
type Options struct {
	// Jwks with private key. If not set, authorization will be disabled,
	Jwks jwk.Set
	// As alternative to Jwks, JwksURL can be provided. Middleware will fetch Jwks and auto refresh.
	// If Jwks is provided, JwksURL will be ignored.
	JwksURL string
	// Required scope that needs to be present in token. If given scope is not present
	// request will be denied. Scope '*' can be set and means any - only key must match.
	RequiredScope string
	// Allows anonymous user - user without token. User info will be null
	AllowAnonymous bool
	// Way how to treat invalid user token: anonymous or unauthorized
	InvalidTokenIsAnonymous bool
	// Way how to treat users without valid scope: anonymous or unauthorized©
	InvalidScopeIsAnonymous bool
	// Disable authorization - it will allow all requests and UserInfo will be always nil
	Disabled bool
}

func OptionsFromViper(prefix string) (options Options) {
	return Options{
		JwksURL:                 viper.GetString(prefix + "jwks"),
		Disabled:                viper.GetBool(prefix + "disabled"),
		RequiredScope:           viper.GetString(prefix + "scope"),
		AllowAnonymous:          viper.GetBool(prefix + "allow_anonymous"),
		InvalidTokenIsAnonymous: viper.GetBool(prefix + "invalid_token_is_anonymous"),
		InvalidScopeIsAnonymous: viper.GetBool(prefix + "invalid_scope_is_anonymous"),
	}
}

// New create new AuthMiddleware object
func New(options Options, userValidator func(userInfo *UserInfo) bool) (a *Authorization) {
	a = &Authorization{
		jwks:                    options.Jwks,
		jwksURL:                 options.JwksURL,
		requiredScope:           options.RequiredScope,
		allowAnonymous:          options.AllowAnonymous,
		invalidTokenIsAnonymous: options.InvalidTokenIsAnonymous,
		invalidScopeIsAnonymous: options.InvalidScopeIsAnonymous,
		disabled:                options.Disabled,
		userValidator:           userValidator,
	}

	if a.requiredScope == "" {
		a.requiredScope = "*"
	}

	if a.disabled {
		a.jwks = nil
		a.jwksURL = ""
	}

	if a.jwks == nil && a.jwksURL != "" {
		a.autoRefresh = jwk.NewAutoRefresh(context.TODO())
		a.autoRefresh.Configure(a.jwksURL)
	}

	return
}

func (a *Authorization) Validate() (err error) {

	if !a.disabled && a.autoRefresh == nil && a.jwks == nil {
		err = fmt.Errorf("authorization is enabled, but not configured - Jwks or JwksURL is required")
		return
	}

	if a.autoRefresh != nil {
		_, err = a.autoRefresh.Fetch(context.Background(), a.jwksURL)
	}
	return
}
