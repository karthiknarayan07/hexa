package system

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
)

/*
RandomIDGenerator is an outbound adapter for the IDGenerator port.

The application layer only asks for a new identifier.
This adapter decides how that identifier is produced in the real world.
*/
type RandomIDGenerator struct{}

/*
NewID creates a compact random hexadecimal identifier.

Using a small adapter here reinforces that even utility concerns can stay
outside the core when you want strict dependency direction.
*/
func (RandomIDGenerator) NewID() (string, error) {
	buffer := make([]byte, 12)

	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}

	return hex.EncodeToString(buffer), nil
}
