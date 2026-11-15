package rpc

import "testing"

func TestParseRequestIDAcceptsOnlyTheHyphenatedUUIDv4(t *testing.T) {
	const ok = "3f2b8c1e-9a4d-4e6f-8b7a-1c2d3e4f5a6b"
	for _, raw := range []string{ok, "3F2B8C1E-9A4D-4E6F-8B7A-1C2D3E4F5A6B"} {
		if id, err := ParseRequestID(raw); err != nil || id == nil {
			t.Errorf("ParseRequestID(%q) = %v, %v，預期接受", raw, id, err)
		}
	}
	if id, err := ParseRequestID(""); id != nil || err != nil {
		t.Errorf("空字串應是不設：%v, %v", id, err)
	}
	for _, raw := range []string{
		"{" + ok + "}", "urn:uuid:" + ok, "3f2b8c1e9a4d4e6f8b7a1c2d3e4f5a6b",
		"3f2b8c1e-9a4d-1e6f-8b7a-1c2d3e4f5a6b", "3f2b8c1e-9a4d-4e6f-cb7a-1c2d3e4f5a6b", "not-a-uuid",
	} {
		if id, err := ParseRequestID(raw); err == nil || id != nil {
			t.Errorf("ParseRequestID(%q) = %v, %v，預期拒絕", raw, id, err)
		}
	}
}
