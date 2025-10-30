package utils

import (
	"Gateway/pkg/config"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserName string `json:"username"`
	UserID   int64  `json:"user_id"`
	jwt.RegisteredClaims
}

// set jwt secret key empty for now, will be set from config file later
var jwtSecretKey = []byte("your_jwt_secret_key")

var expireDuration = time.Hour * 2 // default 2 hours

func SetJWTConfig(cfg *config.JWTConfig) {
	jwtSecretKey = []byte(cfg.Secret)
	expireDuration = time.Duration(cfg.ExpireDuration) * time.Second
}

func GenerateToken(username string, userID int64) (string, error) {
	claims := JWTClaims{
		UserName: username,
		UserID:   userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(expireDuration)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecretKey)
}

func ParseToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecretKey, nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}
