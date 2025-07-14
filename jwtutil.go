package main

import (
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
)

var jwtKey = []byte(os.Getenv("JWT_KEY"))

type Claims struct {
	UserID uint `json:"user_id"`
	jwt.StandardClaims
}

func GenerateToken(userID uint) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID: userID,
		StandardClaims: jwt.StandardClaims{
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	return token.SignedString(jwtKey)
}

func ValidateToken(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	tkn, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (any, error) {
		return jwtKey, nil
	})

	if err != nil {
		return nil, err
	}

	if !tkn.Valid {
		return nil, jwt.ErrSignatureInvalid
	}

	return claims, nil
}

func GetCurrentUser(r *http.Request) (uint, error) {
	c, err := r.Cookie("token")
	if err != nil {
		if err == http.ErrNoCookie {
			return 0, err
		}
		return 0, err
	}

	tokenStr := c.Value
	claims, err := ValidateToken(tokenStr)
	if err != nil {
		return 0, err
	}

	return claims.UserID, nil
}
