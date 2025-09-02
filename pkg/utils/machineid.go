package utils

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

// GenerateMachineID generates a random machine ID
func GenerateMachineID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "unknown-machine-id"
	}

	return hex.EncodeToString(bytes)
}

// GenerateUUID generates a UUID v4
func GenerateUUID() string {
	bytes := make([]byte, 16)
	_, err := rand.Read(bytes)
	if err != nil {
		return "00000000-0000-0000-0000-000000000000"
	}

	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80

	return fmt.Sprintf("%x-%x-%x-%x-%x",
		bytes[0:4],
		bytes[4:6],
		bytes[6:8],
		bytes[8:10],
		bytes[10:16])
}
