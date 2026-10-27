package membershipkernel

import (
	"errors"
	"fmt"
	"strings"
)

// FeatureValueType is the intentionally small typed union in the first
// contract. Adding enum/string/decimal later requires a new contract and
// validator rather than falling back to arbitrary JSON.
type FeatureValueType string

const (
	FeatureValueTypeBoolean      FeatureValueType = "BOOLEAN"
	FeatureValueTypeIntegerValue FeatureValueType = "INTEGER_VALUE"
	FeatureValueTypeIntegerLimit FeatureValueType = "INTEGER_LIMIT"
)

var (
	ErrInvalidFeature       = errors.New("invalid membership feature")
	ErrInvalidFeatureType   = errors.New("invalid membership feature value type")
	ErrInvalidFeatureValue  = errors.New("invalid membership feature value")
	ErrDuplicateFeatureCode = errors.New("duplicate membership feature code")
)

// FeatureValue is a typed entitlement value. Pointer fields preserve the
// distinction between an absent integer and the valid value zero.
type FeatureValue struct {
	FeatureCode  string
	ValueType    FeatureValueType
	Enabled      bool
	IntegerValue *int64
	LimitValue   *int64
	IsUnlimited  bool
	Unit         string
}

func NewBooleanFeature(code string, enabled bool) FeatureValue {
	return FeatureValue{FeatureCode: code, ValueType: FeatureValueTypeBoolean, Enabled: enabled}
}

func NewIntegerValueFeature(code string, value int64, unit string) FeatureValue {
	return FeatureValue{FeatureCode: code, ValueType: FeatureValueTypeIntegerValue, IntegerValue: int64Pointer(value), Unit: unit}
}

func NewIntegerLimitFeature(code string, limit int64, unit string) FeatureValue {
	return FeatureValue{FeatureCode: code, ValueType: FeatureValueTypeIntegerLimit, LimitValue: int64Pointer(limit), Unit: unit}
}

func NewUnlimitedIntegerLimitFeature(code string, unit string) FeatureValue {
	return FeatureValue{FeatureCode: code, ValueType: FeatureValueTypeIntegerLimit, IsUnlimited: true, Unit: unit}
}

func int64Pointer(value int64) *int64 { return &value }

// Validate checks the union arm and rejects ambiguous representations.
func (f FeatureValue) Validate() error {
	if strings.TrimSpace(f.FeatureCode) == "" || f.FeatureCode != strings.TrimSpace(f.FeatureCode) {
		return fmt.Errorf("%w: feature_code=%q", ErrInvalidFeature, f.FeatureCode)
	}
	switch f.ValueType {
	case FeatureValueTypeBoolean:
		if f.IntegerValue != nil || f.LimitValue != nil || f.IsUnlimited {
			return fmt.Errorf("%w: boolean feature carries integer fields", ErrInvalidFeatureValue)
		}
	case FeatureValueTypeIntegerValue:
		if f.IntegerValue == nil || f.LimitValue != nil || f.IsUnlimited {
			return fmt.Errorf("%w: integer value requires exactly integer_value", ErrInvalidFeatureValue)
		}
	case FeatureValueTypeIntegerLimit:
		if f.IntegerValue != nil {
			return fmt.Errorf("%w: integer limit cannot carry integer_value", ErrInvalidFeatureValue)
		}
		if f.IsUnlimited {
			if f.LimitValue != nil {
				return fmt.Errorf("%w: unlimited limit cannot carry limit_value", ErrInvalidFeatureValue)
			}
			break
		}
		if f.LimitValue == nil || *f.LimitValue < 0 {
			return fmt.Errorf("%w: finite integer limit must be non-negative", ErrInvalidFeatureValue)
		}
	default:
		return fmt.Errorf("%w: %q", ErrInvalidFeatureType, f.ValueType)
	}
	return nil
}

// ValidateFeatureSet validates every item and ensures one canonical value per
// feature code. Ordering is left to the caller's immutable revision snapshot;
// the set itself never uses a map as a source of truth.
func ValidateFeatureSet(features []FeatureValue) error {
	seen := make(map[string]struct{}, len(features))
	for _, feature := range features {
		if err := feature.Validate(); err != nil {
			return err
		}
		if _, exists := seen[feature.FeatureCode]; exists {
			return fmt.Errorf("%w: %s", ErrDuplicateFeatureCode, feature.FeatureCode)
		}
		seen[feature.FeatureCode] = struct{}{}
	}
	return nil
}
