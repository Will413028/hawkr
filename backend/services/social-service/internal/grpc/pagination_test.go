package grpc

import "testing"

func TestPageToLimitOffset_NormalizesModerationBounds(t *testing.T) {
	tests := []struct {
		name       string
		page       int32
		pageSize   int32
		wantLimit  int64
		wantOffset int64
	}{
		{
			name:       "uses defaults",
			wantLimit:  20,
			wantOffset: 0,
		},
		{
			name:       "clamps and offsets",
			page:       3,
			pageSize:   999,
			wantLimit:  100,
			wantOffset: 200,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotLimit, gotOffset := pageToLimitOffset(tt.page, tt.pageSize)
			if gotLimit != tt.wantLimit || gotOffset != tt.wantOffset {
				t.Errorf("pageToLimitOffset(%d, %d) = (%d, %d), want (%d, %d)", tt.page, tt.pageSize, gotLimit, gotOffset, tt.wantLimit, tt.wantOffset)
			}
		})
	}
}
