package content

import (
	"crypto/rand"
	"encoding/hex"
	"time"
)

func timeNow() time.Time { return time.Now().UTC() }

func newContentID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return hex.EncodeToString([]byte(timeNow().String()))[:24]
	}
	return hex.EncodeToString(b)
}
