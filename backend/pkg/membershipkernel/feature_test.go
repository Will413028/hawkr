package membershipkernel

import (
	"errors"
	"testing"
)

func TestFeatureValueValidateTypedValues(t *testing.T) {
	integer := int64(10)
	limit := int64(25)
	tests := []struct {
		name  string
		value FeatureValue
	}{
		{name: "boolean enabled", value: FeatureValue{FeatureCode: "buyer.agent.search", ValueType: FeatureValueTypeBoolean, Enabled: true}},
		{name: "integer value", value: FeatureValue{FeatureCode: "seller.agent.rank_weight", ValueType: FeatureValueTypeIntegerValue, IntegerValue: &integer}},
		{name: "integer limit", value: FeatureValue{FeatureCode: "buyer.agent.answers", ValueType: FeatureValueTypeIntegerLimit, LimitValue: &limit}},
		{name: "unlimited integer limit", value: FeatureValue{FeatureCode: "buyer.agent.answers", ValueType: FeatureValueTypeIntegerLimit, IsUnlimited: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.value.Validate(); err != nil {
				t.Fatalf("Validate() error = %v", err)
			}
		})
	}
}

func TestFeatureValueRejectsTypeMismatchAndNegativeLimit(t *testing.T) {
	negative := int64(-1)
	integer := int64(4)
	tests := []struct {
		name  string
		value FeatureValue
		want  error
	}{
		{name: "empty feature code", value: FeatureValue{ValueType: FeatureValueTypeBoolean}, want: ErrInvalidFeature},
		{name: "unknown value type", value: FeatureValue{FeatureCode: "feature", ValueType: "JSON"}, want: ErrInvalidFeatureType},
		{name: "boolean carries integer", value: FeatureValue{FeatureCode: "feature", ValueType: FeatureValueTypeBoolean, IntegerValue: &integer}, want: ErrInvalidFeatureValue},
		{name: "integer value missing", value: FeatureValue{FeatureCode: "feature", ValueType: FeatureValueTypeIntegerValue}, want: ErrInvalidFeatureValue},
		{name: "integer limit negative", value: FeatureValue{FeatureCode: "feature", ValueType: FeatureValueTypeIntegerLimit, LimitValue: &negative}, want: ErrInvalidFeatureValue},
		{name: "unlimited carries finite limit", value: FeatureValue{FeatureCode: "feature", ValueType: FeatureValueTypeIntegerLimit, IsUnlimited: true, LimitValue: &integer}, want: ErrInvalidFeatureValue},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.value.Validate(); !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want errors.Is(..., %v)", err, tt.want)
			}
		})
	}
}

func TestValidateFeatureSetRejectsDuplicateFeatureCodes(t *testing.T) {
	features := []FeatureValue{
		{FeatureCode: "buyer.agent.answers", ValueType: FeatureValueTypeBoolean},
		{FeatureCode: "buyer.agent.answers", ValueType: FeatureValueTypeBoolean},
	}
	if err := ValidateFeatureSet(features); !errors.Is(err, ErrDuplicateFeatureCode) {
		t.Fatalf("ValidateFeatureSet() error = %v, want ErrDuplicateFeatureCode", err)
	}
}
