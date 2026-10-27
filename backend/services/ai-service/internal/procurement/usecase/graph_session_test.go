package usecase

import "testing"

func TestProcurementSessionID_StableAcrossCalls(t *testing.T) {
	a := ProcurementSessionID(12345)
	b := ProcurementSessionID(12345)
	if a != b {
		t.Errorf("ProcurementSessionID(12345) not stable: got %q then %q", a, b)
	}
	if a == ProcurementSessionID(67890) {
		t.Errorf("different requestID produced same SessionID: %q", a)
	}
}
