package autoscaler

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
)

// Sha256 is a hash function that returns a sha256 hash of the input string.
func Sha256(input string) string {
	h := sha256.New()
	_, err := io.WriteString(h, input)
	if err != nil {
		panic(err)
	}
	hash := hex.EncodeToString(h.Sum(nil))
	return hash
}
