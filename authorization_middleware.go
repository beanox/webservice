package webservice

import (
	"context"
	"crypto/rsa"
	"fmt"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/spf13/viper"

	"github.com/lestrrat-go/jwx/jwk"
	"github.com/sirupsen/logrus"
)

// UserInfo information about authenticated user
type UserInfo struct {
	UserID string                 `json:"uid,omitempty"`
	Email  string                 `json:"email,omitempty"`
	Scopes []string               `json:"scopes,omitempty"`
	Claims map[string]interface{} `json:"claims,omitempty"`
}

var userWithInvalidToken = &UserInfo{UserID: "_invalid_token_"}
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
	contextTypeLogger
)

type HandlerFn func(w http.ResponseWriter, r *http.Request, userInfo *UserInfo) (err error)

// authorization object
type authorization struct {
	logger                  *logrus.Logger
	jwks                    jwk.Set
	jwksURL                 string
	autoRefresh             *jwk.AutoRefresh
	requiredScope           string
	allowAnonymous          bool
	invalidTokenIsAnonymous bool
	invalidScopeIsAnonymous bool
	disabled                bool
}

// Middleware returns middleware function that can be used in router.Use()
func (a *authorization) Middleware(h http.Handler) (handler http.Handler) {

	handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		ctx := context.WithValue(r.Context(), contextTypeAuthorizationMiddleware, a)

		var userInfo *UserInfo = unauthenticatedUser

		tokenString := r.Header.Get("Authorization")
		if tokenString != "" {
			userInfo = userWithInvalidToken

			splitToken := strings.Split(tokenString, "Bearer")
			if len(splitToken) != 2 {
				if a.logger != nil {
					a.logger.Errorf("wrong Authorization header")
				}
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
						var publicKey rsa.PublicKey
						err := key.Raw(&publicKey)
						return &publicKey, err
					}

					return nil, fmt.Errorf("unable to find key with id: %s", keyID)
				})

				if err == nil {
					if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {

						if a.logger != nil {
							a.logger.Tracef("auth: User claims: %+v", claims)
						}
						var uid string
						var mail string
						var scopes []string

						if v, ok := claims["sub"].(string); ok {
							uid = v
						}

						if v, ok := claims["email"].(string); ok {
							mail = v
						}

						if v, ok := claims["scope"].(string); ok {
							scopes = strings.Fields(v)
						}

						if uid != "" {
							userInfo = &UserInfo{
								UserID: uid,
								Email:  mail,
								Scopes: scopes,
								Claims: claims,
							}
						}
					}
				} else {
					if a.logger != nil {
						a.logger.WithError(err).Errorf("error decoding token")
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

// AuthorizationOptions is a configuration container to setup Authorization middleware.
type AuthorizationOptions struct {
	// Jwks with private key. If not set, authorization will be disabled,
	Jwks jwk.Set
	// As alternative to Jwks, JwksURL can be provided. Middleware will fetch Jwks and auto refresh.
	// If Jwks is provided, JwksURL will be ignored.
	JwksURL string
	// Required scope that needs to be present in token. If given scope is not present
	// request will be denied. Scope '*' can be set and means any - only key must match.
	RequiredScope string
	// Allowes anonymous user - user without token. User info will be null
	AllowAnonymous bool
	// Way how to treat invalid user token: anonymous or unauthorized
	InvalidTokenIsAnonymous bool
	// Way how to treat users without valid scope: anonymous or unauthorized
	InvalidScopeIsAnonymous bool
	// Disable authorization - it will allow all requests and UserInfo will be always nil
	Disabled bool
}

func AuthorizationOptionsFromViper(prefix string) (options *AuthorizationOptions) {
	return &AuthorizationOptions{
		JwksURL:                 viper.GetString(prefix + "jwks"),
		Disabled:                viper.GetBool(prefix + "disabled"),
		RequiredScope:           viper.GetString(prefix + "scope"),
		AllowAnonymous:          viper.GetBool(prefix + "allow_anonymous"),
		InvalidTokenIsAnonymous: viper.GetBool(prefix + "invalid_token_is_anonymous"),
		InvalidScopeIsAnonymous: viper.GetBool(prefix + "invalid_scope_is_anonymous"),
	}
}

// New create new AuthMiddleware object
func newAuthorizationMiddleware(options *AuthorizationOptions, logger *logrus.Logger) (a *authorization) {
	a = &authorization{
		logger:                  logger,
		jwks:                    options.Jwks,
		jwksURL:                 options.JwksURL,
		requiredScope:           options.RequiredScope,
		allowAnonymous:          options.AllowAnonymous,
		invalidTokenIsAnonymous: options.InvalidTokenIsAnonymous,
		invalidScopeIsAnonymous: options.InvalidScopeIsAnonymous,
		disabled:                options.Disabled,
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

func (a *authorization) Validate() (err error) {

	if !a.disabled && a.autoRefresh == nil && a.jwks == nil {
		err = fmt.Errorf("authorization is enabled, but not configured - Jwks or JwksURL are required")
		return
	}

	if a.autoRefresh != nil {
		_, err = a.autoRefresh.Fetch(context.Background(), a.jwksURL)
	}
	return
}
