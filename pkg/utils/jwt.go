package utils

import (
	"fmt"
	"github.com/golang-jwt/jwt/v4"
)

func GetStringFieldFromJWT(token string, field string) (string, error) {
	var jwtToken *jwt.Token
	var err error

	parser := new(jwt.Parser)
	jwtToken, _, err = parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse jwt")
	}

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

	claimString, ok := claim.(string)
	if !ok {
		return "", fmt.Errorf("field %v does not contain a string value", field)
	}

	return claimString, nil
}
