package auth

import (
	"crypto/rand"
	"encoding/hex"
)

func MakeRefreshToken() string {
	key := make([]byte, 32)

	if _, err := rand.Read(key); err != nil {
		panic(err) // or return an error if you change the function signature
	}

	return hex.EncodeToString(key)
}
