package gases

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"air"

	"github.com/dgrijalva/jwt-go"
)

type (
	// JWTConfig defines the config for JWT auth gas.
	JWTConfig struct {
		// Signing key to validate token.
		// Required.
		SigningKey []byte `json:"signing_key"`

		// Signing method, used to check token signing method.
		// Optional. Default value HS256.
		SigningMethod string `json:"signing_method"`

		// Context key to store user information from the token into context.
		// Optional. Default value "user".
		ContextKey string `json:"context_key"`

		// TokenLookup is a string in the form of "<source>:<name>" that is used
		// to extract token from the request.
		// Optional. Default value "header:Authorization".
		// Possible values:
		// - "header:<name>"
		// - "query:<name>"
		TokenLookup string `json:"token_lookup"`
	}

	jwtExtractor func(air.Context) (string, error)
)

const (
	bearer = "Bearer"
)

// Algorithims
const (
	AlgorithmHS256 = "HS256"
)

var (
	// DefaultJWTConfig is the default JWT auth gas config.
	DefaultJWTConfig = JWTConfig{
		SigningMethod: AlgorithmHS256,
		ContextKey:    "user",
		TokenLookup:   "header:" + air.HeaderAuthorization,
	}
)

// JWT returns a JSON Web Token (JWT) auth gas.
//
// For valid token, it sets the user in context and calls next handler.
// For invalid token, it sends "401 - Unauthorized" response.
// For empty or invalid `Authorization` header, it sends "400 - Bad Request".
//
// See: https://jwt.io/introduction
func JWT(key []byte) air.GasFunc {
	c := DefaultJWTConfig
	c.SigningKey = key
	return JWTWithConfig(c)
}

// JWTWithConfig returns a JWT auth gas from config.
// See: `JWT()`.
func JWTWithConfig(config JWTConfig) air.GasFunc {
	// Defaults
	if config.SigningKey == nil {
		panic("jwt gas requires signing key")
	}
	if config.SigningMethod == "" {
		config.SigningMethod = DefaultJWTConfig.SigningMethod
	}
	if config.ContextKey == "" {
		config.ContextKey = DefaultJWTConfig.ContextKey
	}
	if config.TokenLookup == "" {
		config.TokenLookup = DefaultJWTConfig.TokenLookup
	}

	// Initialize
	parts := strings.Split(config.TokenLookup, ":")
	extractor := jwtFromHeader(parts[1])
	switch parts[0] {
	case "query":
		extractor = jwtFromQuery(parts[1])
	}

	return func(next air.HandlerFunc) air.HandlerFunc {
		return func(c air.Context) error {
			auth, err := extractor(c)
			if err != nil {
				return air.NewHTTPError(http.StatusBadRequest, err.Error())
			}
			token, err := jwt.Parse(auth, func(t *jwt.Token) (interface{}, error) {
				// Check the signing method
				if t.Method.Alg() != config.SigningMethod {
					return nil, fmt.Errorf("unexpected jwt signing method=%v", t.Header["alg"])
				}
				return config.SigningKey, nil

			})
			if err == nil && token.Valid {
				// Store user information from token into context.
				c.Set(config.ContextKey, token)
				return next(c)
			}
			return air.ErrUnauthorized
		}
	}
}

// jwtFromHeader returns a `jwtExtractor` that extracts token from the provided
// request header.
func jwtFromHeader(header string) jwtExtractor {
	return func(c air.Context) (string, error) {
		auth := c.Request().Header().Get(header)
		l := len(bearer)
		if len(auth) > l+1 && auth[:l] == bearer {
			return auth[l+1:], nil
		}
		return "", errors.New("empty or invalid jwt in authorization header")
	}
}

// jwtFromQuery returns a `jwtExtractor` that extracts token from the provided query
// parameter.
func jwtFromQuery(param string) jwtExtractor {
	return func(c air.Context) (string, error) {
		token := c.QueryParam(param)
		if token == "" {
			return "", errors.New("empty jwt in query param")
		}
		return token, nil
	}
}
