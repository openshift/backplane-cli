package backplaneapi

import (
	"errors"
	"net/http"
)

const deprecationMsg = "server indicated that this client is deprecated"

var ErrDeprecation = errors.New(deprecationMsg)

func CheckResponseDeprecation(r *http.Response) error {
	if r.Header.Get("Deprecated-Client") == "true" {
		return ErrDeprecation
	}

	return nil
}
