package membershipkernel

import (
	"errors"
	"testing"
)

func TestNewSubjectRefAcceptsOnlyDomainIdentities(t *testing.T) {
	tests := []struct {
		name        string
		subjectType SubjectType
		id          string
		want        SubjectRef
	}{
		{name: "buyer uses buyer identity", subjectType: SubjectTypeBuyer, id: "123", want: SubjectRef{SubjectType: SubjectTypeBuyer, SubjectID: "123"}},
		{name: "seller uses store identity", subjectType: SubjectTypeSeller, id: "456", want: SubjectRef{SubjectType: SubjectTypeSeller, SubjectID: "456"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewSubjectRef(tt.subjectType, tt.id)
			if err != nil {
				t.Fatalf("NewSubjectRef() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("NewSubjectRef() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestSubjectRefRejectsImplicitOrInvalidIdentity(t *testing.T) {
	tests := []struct {
		name string
		ref  SubjectRef
		want error
	}{
		{name: "unknown subject type", ref: SubjectRef{SubjectType: "ACCOUNT", SubjectID: "123"}, want: ErrInvalidSubjectType},
		{name: "empty subject id", ref: SubjectRef{SubjectType: SubjectTypeBuyer}, want: ErrInvalidSubjectID},
		{name: "whitespace subject id", ref: SubjectRef{SubjectType: SubjectTypeSeller, SubjectID: " 123 "}, want: ErrInvalidSubjectID},
		{name: "zero subject id", ref: SubjectRef{SubjectType: SubjectTypeBuyer, SubjectID: "0"}, want: ErrInvalidSubjectID},
		{name: "negative subject id", ref: SubjectRef{SubjectType: SubjectTypeBuyer, SubjectID: "-1"}, want: ErrInvalidSubjectID},
		{name: "account id cannot be implicit", ref: SubjectRef{SubjectType: "", SubjectID: "999"}, want: ErrInvalidSubjectType},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.ref.Validate(); !errors.Is(err, tt.want) {
				t.Fatalf("Validate() error = %v, want errors.Is(..., %v)", err, tt.want)
			}
		})
	}
}

func TestSubjectRefRejectsLeadingZeroIDs(t *testing.T) {
	if err := (SubjectRef{SubjectType: SubjectTypeBuyer, SubjectID: "001"}).Validate(); !errors.Is(err, ErrInvalidSubjectID) {
		t.Fatalf("Validate() error = %v, want ErrInvalidSubjectID", err)
	}
}
