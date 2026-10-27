package pagination

import "testing"

func TestNormalizePage(t *testing.T) {
	const defaultSize, maxSize int32 = 20, 100

	cases := []struct {
		name                     string
		page, pageSize           int32
		wantPage, wantSize, want int32
	}{
		{name: "zero page defaults to 1", page: 0, pageSize: 20, wantPage: 1, wantSize: 20, want: 0},
		{name: "negative page defaults to 1", page: -5, pageSize: 20, wantPage: 1, wantSize: 20, want: 0},
		{name: "zero pageSize uses default", page: 1, pageSize: 0, wantPage: 1, wantSize: defaultSize, want: 0},
		{name: "negative pageSize uses default", page: 1, pageSize: -1, wantPage: 1, wantSize: defaultSize, want: 0},
		{name: "pageSize over max clamps to max", page: 1, pageSize: 500, wantPage: 1, wantSize: maxSize, want: 0},
		{name: "pageSize within bounds is unchanged", page: 3, pageSize: 30, wantPage: 3, wantSize: 30, want: 60},
		{name: "offset for page 1 is 0", page: 1, pageSize: 20, wantPage: 1, wantSize: 20, want: 0},
		{name: "offset scales with page", page: 5, pageSize: 20, wantPage: 5, wantSize: 20, want: 80},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPage, gotSize, gotOffset := NormalizePage(tc.page, tc.pageSize, defaultSize, maxSize)
			if gotPage != tc.wantPage || gotSize != tc.wantSize || gotOffset != tc.want {
				t.Errorf("NormalizePage(%d, %d, %d, %d) = (%d, %d, %d), want (%d, %d, %d)",
					tc.page, tc.pageSize, defaultSize, maxSize,
					gotPage, gotSize, gotOffset,
					tc.wantPage, tc.wantSize, tc.want)
			}
		})
	}
}

func TestNormalizePageSupportsIntAndInt64(t *testing.T) {
	page, pageSize, offset := NormalizePage(2, 10, 20, 100)
	if page != 2 || pageSize != 10 || offset != 10 {
		t.Errorf("NormalizePage(int) = (%d, %d, %d), want (2, 10, 10)", page, pageSize, offset)
	}

	page64, pageSize64, offset64 := NormalizePage(int64(3), int64(15), int64(20), int64(100))
	if page64 != 3 || pageSize64 != 15 || offset64 != 30 {
		t.Errorf("NormalizePage(int64) = (%d, %d, %d), want (3, 15, 30)", page64, pageSize64, offset64)
	}
}

func TestNormalizePageSize(t *testing.T) {
	const defaultSize, maxSize int32 = 20, 100

	cases := []struct {
		name     string
		pageSize int32
		want     int32
	}{
		{name: "zero uses default", pageSize: 0, want: defaultSize},
		{name: "negative uses default", pageSize: -1, want: defaultSize},
		{name: "over max clamps to max", pageSize: 500, want: maxSize},
		{name: "within bounds is unchanged", pageSize: 30, want: 30},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := NormalizePageSize(tc.pageSize, defaultSize, maxSize); got != tc.want {
				t.Errorf("NormalizePageSize(%d, %d, %d) = %d, want %d",
					tc.pageSize, defaultSize, maxSize, got, tc.want)
			}
		})
	}
}
