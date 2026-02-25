package protocol

import (
	"encoding/base64"
)

func init() {
	// Set default encoding functions
	SetEncodingFunctions(
		func(data []byte) string { return base64.StdEncoding.EncodeToString(data) },
		func(s string) ([]byte, error) { return base64.StdEncoding.DecodeString(s) },
	)
}
