package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

func GenerateUniqueSuffix() string {
	b := make([]byte, 4)
	_, err := rand.Read(b)
	if err != nil {
		return fmt.Sprintf("-%d", time.Now().UnixNano())
	}
	return "-" + hex.EncodeToString(b)
}
