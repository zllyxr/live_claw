package idgen

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"io"
	"strings"
	"time"
)

var encoding = base32.NewEncoding("0123456789ABCDEFGHJKMNPQRSTVWXYZ").WithPadding(base32.NoPadding)

func New() (string, error) {
	return NewAt(time.Now(), rand.Reader)
}

func NewAt(now time.Time, source io.Reader) (string, error) {
	var raw [16]byte
	milliseconds := uint64(now.UnixMilli())
	raw[0] = byte(milliseconds >> 40)
	raw[1] = byte(milliseconds >> 32)
	raw[2] = byte(milliseconds >> 24)
	raw[3] = byte(milliseconds >> 16)
	raw[4] = byte(milliseconds >> 8)
	raw[5] = byte(milliseconds)
	if _, err := io.ReadFull(source, raw[6:]); err != nil {
		return "", fmt.Errorf("read id entropy: %w", err)
	}
	return strings.ToLower(encoding.EncodeToString(raw[:])), nil
}
