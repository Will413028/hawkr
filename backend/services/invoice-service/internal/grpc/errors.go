package grpc

import (
	"errors"

	pkgerrors "github.com/Will413028/hawkr/go-pkg/errors"
	"github.com/Will413028/hawkr/go-pkg/errors/bizcode"
)

var (
	errEmptyID       = errors.New("must not be empty")
	errNonPositiveID = errors.New("must be a positive integer")
	errMissingAmount = errors.New("amount breakdown is required")
)

func newBizErr(code int32) error {
	return pkgerrors.NewBusiness(code, bizcode.Message(code))
}
