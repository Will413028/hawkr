package revalidate

import (
	"regexp"
	"slices"
	"testing"
)

// entityTagPattern mirrors ENTITY_TAG in
// hawkr-website/lib/api/revalidate-request.ts. A tag that fails it is
// rejected with 400 — and because the endpoint validates the whole payload
// before acting, one bad tag drops every other tag sent with it. Pinning the
// pattern here makes that contract break a red test instead of a silent
// never-invalidated cache.
var entityTagPattern = regexp.MustCompile(
	`^(?:product|seller|post|category|hashtag):[A-Za-z0-9][A-Za-z0-9._-]{0,127}$`)

func TestTagBuildersMatchWebsiteAllowList(t *testing.T) {
	tags := []string{
		ProductTag(2623900520),
		SellerTag(2685914682),
		PostTag(2700000001),
		HashtagTag(2700000003),
		CategoryTag(9001),
	}
	for _, tag := range tags {
		if !entityTagPattern.MatchString(tag) {
			t.Errorf("tag %q is rejected by the website allow-list regex", tag)
		}
	}
}

func TestTagBuilderFormats(t *testing.T) {
	cases := []struct {
		got  string
		want string
	}{
		{PostTag(123), "post:123"},
		{HashtagTag(789), "hashtag:789"},
		{CategoryTag(9001), "category:9001"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("got %q, want %q", c.got, c.want)
		}
	}
}

func TestHashtagTags(t *testing.T) {
	tests := []struct {
		name string
		ids  []int64
		want []string
	}{
		{
			name: "distinct ids each get a tag, order preserved",
			ids:  []int64{300, 100, 200},
			want: []string{"hashtag:300", "hashtag:100", "hashtag:200"},
		},
		{
			name: "repeats collapse (a tag added and removed across posts)",
			ids:  []int64{100, 200, 100},
			want: []string{"hashtag:100", "hashtag:200"},
		},
		{
			name: "non-positive ids are skipped rather than emitting hashtag:0",
			ids:  []int64{100, 0, -1},
			want: []string{"hashtag:100"},
		},
		{
			name: "no id yields no tag (BustAsync treats it as a no-op)",
			ids:  nil,
			want: []string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HashtagTags(tt.ids...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("HashtagTags(%v) = %v, want %v", tt.ids, got, tt.want)
			}
		})
	}
}

func TestSellerTags(t *testing.T) {
	tests := []struct {
		name string
		ids  []int64
		want []string
	}{
		{
			name: "diverged ids each get their own tag, order preserved",
			ids:  []int64{2623900520, 700700700, 2600000001},
			want: []string{"seller:2623900520", "seller:700700700", "seller:2600000001"},
		},
		{
			name: "grandfathered seller whose ids coincide collapses to one tag",
			ids:  []int64{2623900520, 2623900520, 2623900520},
			want: []string{"seller:2623900520"},
		},
		{
			name: "partial overlap keeps the distinct ids only once each",
			ids:  []int64{100, 200, 100},
			want: []string{"seller:100", "seller:200"},
		},
		{
			name: "unresolved (zero / negative) ids are skipped, the rest still emit",
			ids:  []int64{2623900520, 0, -1},
			want: []string{"seller:2623900520"},
		},
		{
			name: "no resolvable id yields no tag (BustAsync treats it as a no-op)",
			ids:  []int64{0},
			want: []string{},
		},
		{
			name: "no id at all yields no tag",
			ids:  nil,
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SellerTags(tt.ids...)
			if !slices.Equal(got, tt.want) {
				t.Errorf("SellerTags(%v) = %v, want %v", tt.ids, got, tt.want)
			}
		})
	}
}
