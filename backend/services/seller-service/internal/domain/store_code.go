package domain

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"time"
)

func GenerateStoreCode(now time.Time) (int64, error) {
	year, err := strconv.ParseInt(now.Format("06"), 10, 64)
	if err != nil {
		return 0, err
	}
	n, err := rand.Int(rand.Reader, big.NewInt(100_000_000))
	if err != nil {
		return 0, err
	}
	return year*100_000_000 + n.Int64(), nil
}

// NewStoreCodeGenerator adapts GenerateStoreCode into the zero-arg
// func() (int64, error) shape SellerUseCase takes via constructor DI
// (mirrors pkg/tsid.NewGenerator's func() int64 pattern for StoreID).
func NewStoreCodeGenerator() func() (int64, error) {
	return func() (int64, error) {
		return GenerateStoreCode(time.Now().UTC())
	}
}
