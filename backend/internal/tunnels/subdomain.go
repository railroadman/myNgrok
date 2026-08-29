package tunnels

import (
	"crypto/rand"
	"fmt"
)

const subdomainLength = 10

var alphabet = []byte("abcdefghijklmnopqrstuvwxyz0123456789")

// GenerateSubdomain returns a random DNS-label-safe identifier for a public
// tunnel address. Database uniqueness is enforced when the tunnel is created.
func GenerateSubdomain() (string, error) {
	bytes := make([]byte, subdomainLength)
	for index := range bytes {
		limit := byte(256 - (256 % len(alphabet)))
		for {
			var randomByte [1]byte
			if _, err := rand.Read(randomByte[:]); err != nil {
				return "", fmt.Errorf("generate subdomain: %w", err)
			}
			if randomByte[0] < limit {
				bytes[index] = alphabet[int(randomByte[0])%len(alphabet)]
				break
			}
		}
	}
	return string(bytes), nil
}
