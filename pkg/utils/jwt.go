package utils

import (
	"fmt"
	"github.com/golang-jwt/jwt/v4"
)

// GetFieldFromJWT Returns the value of a field from a given JWT.
//
// WARNING: This function does not verify the token.
//
// Only use this function in cases where you know the signature is valid and want to extract values from it.
func GetFieldFromJWT(token string, field string) (string, error) {
	var jwtToken *jwt.Token
	var err error

	parser := new(jwt.Parser)
	jwtToken, _, err = parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return "", err
	}

	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return "", err
	}

	claim, ok := claims[field]
	if !ok {
		return "", fmt.Errorf("no field %v on given token", field)
	}

	return claim.(string), nil
}
