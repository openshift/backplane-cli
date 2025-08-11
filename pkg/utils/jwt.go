package utils

import (
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v4"
)

// GetStringFieldFromJWT extracts a string field from a JWT token without verification.
// It parses the token and returns the specified field value as a string.
// Returns an error if the token cannot be parsed, the field doesn't exist, or the field is not a string.
func GetStringFieldFromJWT(token string, field string) (string, error) {
	var jwtToken *jwt.Token
	var err error

	parser := new(jwt.Parser)
	jwtToken, _, err = parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return "", fmt.Errorf("failed to parse jwt")
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

// GetUsernameFromJWT returns the username extracted from JWT token
func GetUsernameFromJWT(token string) string {
	var jwtToken *jwt.Token
	var err error
	parser := new(jwt.Parser)
	jwtToken, _, err = parser.ParseUnverified(token, jwt.MapClaims{})
	if err != nil {
		return "anonymous"
	}
	claims, ok := jwtToken.Claims.(jwt.MapClaims)
	if !ok {
		return "anonymous"
	}
	claim, ok := claims["username"]
	if !ok {
		return "anonymous"
	}
	return claim.(string)
}

// GetContextNickname returns a nickname of a context
func GetContextNickname(namespace, clusterNick, userNick string) string {
	tokens := strings.SplitN(userNick, "/", 2)
	return namespace + "/" + clusterNick + "/" + tokens[0]
}
