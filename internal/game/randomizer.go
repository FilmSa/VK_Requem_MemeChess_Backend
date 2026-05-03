package game

import (
	"crypto/rand"
	"encoding/binary"
	"fmt"
)

type randomizer interface {
	Intn(n int) (int, error)
}

type cryptoRandomizer struct{}

func (cryptoRandomizer) Intn(n int) (int, error) {
	if n <= 0 {
		return 0, fmt.Errorf("invalid bound %d", n)
	}

	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0, err
	}

	return int(binary.BigEndian.Uint64(buf[:]) % uint64(n)), nil
}
