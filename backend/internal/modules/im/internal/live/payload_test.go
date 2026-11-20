package live

import (
	"errors"
	"math"
	"strings"
	"testing"
	"uuid"
)

func TestEncodedPayloadFitsFarBelowTheNotifyLimit(t *testing.T) {
	t.Parallel()
	c := uuid.Max()
	for name, p := range map[string]Payload{
		"message": Message(c, math.MaxInt64, math.MaxInt64, math.MaxInt64),
		"visible": Visible(c, math.MaxInt64, math.MaxInt64, math.MaxInt64),
		"read":    Read(c, math.MaxInt64, math.MaxInt64, math.MaxInt64, SideStore),
		"batch":   ReadBatch(math.MaxInt64, 0),
	} {
		if n := len(p.Encode()); n >= MaxPayloadBytes/10 {
			t.Errorf("%s：payload %d bytes，預期遠小於 %d", name, n, MaxPayloadBytes)
		}
	}
}

func TestPayloadRoundTrip(t *testing.T) {
	t.Parallel()
	c := uuid.NewV7()
	for _, want := range []Payload{Message(c, 5, 9, 17), Visible(c, 5, 9, 3), Read(c, 5, 9, 4, SideBuyer), ReadBatch(0, 9)} {
		got, conv, err := Decode(want.Encode())
		if err != nil {
			t.Fatalf("%+v：%v", want, err)
		}
		if got != want {
			t.Fatalf("round trip：%+v ≠ %+v", got, want)
		}
		if want.Kind != KindReadBatch && conv != c {
			t.Fatalf("對話 ID 不符：%v", conv)
		}
	}
}

func TestUnknownVersionBadJSONAndMissingFieldsAreDroppedAndCountedWithoutBreakingTheHub(t *testing.T) {
	t.Parallel()
	c := uuid.NewV7().String()
	bad := []string{
		"",
		"not json",
		`{"v":2,"k":"m","c":"` + c + `","b":1,"s":2,"q":3}`,
		`{"v":1,"k":"x","c":"` + c + `","b":1,"s":2,"q":3}`,
		`{"v":1,"k":"m","c":"` + c + `","b":1,"s":2}`,
		`{"v":1,"k":"m","c":"` + c + `","s":2,"q":3}`,
		`{"v":1,"k":"m","c":"not-a-uuid","b":1,"s":2,"q":3}`,
		`{"v":1,"k":"m","b":1,"s":2,"q":3}`,
		`{"v":1,"k":"r","c":"` + c + `","b":1,"s":2,"q":3}`,
		`{"v":1,"k":"r","c":"` + c + `","b":1,"s":2,"q":3,"d":"X"}`,
		`{"v":1,"k":"u","b":1,"s":2}`,
		`{"v":1,"k":"u"}`,
		`{"v":1,"k":"u","b":-1}`,
	}
	for _, s := range bad {
		if _, _, err := Decode(s); !errors.Is(err, ErrPayload) {
			t.Errorf("%q 應回 ErrPayload，得到 %v", s, err)
		}
	}
	h := newTestHub(t, newFakeReader(), Config{})
	sub, err := h.Subscribe(t.Context(), buyerParty(5), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range bad {
		h.dispatch(s)
	}
	if got := h.Dropped(); got != int64(len(bad)) {
		t.Fatalf("丟棄數 %d，預期 %d", got, len(bad))
	}
	h.dispatch(Message(uuid.NewV7(), 5, 9, 1).Encode())
	sub.mu.Lock()
	n := len(sub.pending)
	sub.mu.Unlock()
	if n != 1 {
		t.Fatalf("丟棄之後 hub 還要能處理合法通知：pending %d", n)
	}
	if strings.Contains(Message(uuid.NewV7(), 5, 9, 1).Encode(), "content") {
		t.Fatal("payload 不帶內容")
	}
}
