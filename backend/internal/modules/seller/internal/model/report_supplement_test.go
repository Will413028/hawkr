package model

import "testing"

func TestSupplementBlockOfIsTheOnlyRule(t *testing.T) {
	outcome := OutcomeInsufficient
	for _, c := range []struct {
		name      string
		outcome   *Outcome
		count     int
		want      SupplementBlock
		wantCanDo bool
	}{
		{"沒有結果、沒有補充", nil, 0, SupplementAllowed, true},
		{"沒有結果、差一則到上限", nil, MaxSupplements - 1, SupplementAllowed, true},
		{"沒有結果、已達上限", nil, MaxSupplements, SupplementBlockedByLimit, false},
		{"有結果", &outcome, 0, SupplementBlockedByOutcome, false},
		{"有結果又達上限，先判結果", &outcome, MaxSupplements, SupplementBlockedByOutcome, false},
	} {
		if got := SupplementBlockOf(c.outcome != nil, c.count); got != c.want {
			t.Errorf("%s：SupplementBlockOf = %v，預期 %v", c.name, got, c.want)
		}
		r := Report{Outcome: c.outcome, SupplementCount: c.count}
		if r.SupplementBlock() != c.want || r.CanSupplement() != c.wantCanDo {
			t.Errorf("%s：Report 的判斷（%v、%v）與 SupplementBlockOf 不同", c.name, r.SupplementBlock(), r.CanSupplement())
		}
	}
}
