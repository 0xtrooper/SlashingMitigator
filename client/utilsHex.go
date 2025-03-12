package client

import (
	"encoding/hex"
	"strings"
)

const (
	hexPrefix string = "0x"
)

// Encode bytes to a hex string with a 0x prefix
func EncodeHexWithPrefix(value []byte) string {
	hexString := hex.EncodeToString(value)
	return hexPrefix + hexString
}

// Convert a hex-encoded string to a byte array, removing the 0x prefix if present
func DecodeHex(value string) ([]byte, error) {
	value = strings.TrimPrefix(value, hexPrefix)
	return hex.DecodeString(value)
}
