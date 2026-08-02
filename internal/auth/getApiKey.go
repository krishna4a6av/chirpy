package auth

import (
	"fmt"
	"net/http"
	"strings"
)

func GetAPIKey(headers http.Header) (string, error) {
	apiHeader := headers.Get("Authorization")

	if apiHeader == "" {
		return "", fmt.Errorf("missing authorization header")
	}

	const prefix = "ApiKey "

	if !strings.HasPrefix(apiHeader, prefix) {
		return "", fmt.Errorf("invalid authorization header")
	}

	return strings.TrimPrefix(apiHeader, prefix), nil
}
