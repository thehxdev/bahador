package utils

import (
	"crypto/rand"
	"math/big"
)

func GenRandInt64(low, high int64) (int64, error) {
	delta := high - low + 1
	n := big.NewInt(delta)
	randNum, err := rand.Int(rand.Reader, n)
	if err != nil {
		return 0, err
	}
	return randNum.Int64(), nil
}
