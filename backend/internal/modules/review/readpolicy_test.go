package review

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/platform/policy"
)


type query struct{ name, file, body string }

var queryHeader = regexp.MustCompile(`(?m)^-- name: (\w+) :\w+\s*$`)

func loadQueries(t *testing.T) []query {
	t.Helper()
	files, err := filepath.Glob("db/queries/*.sql")
	if err != nil || len(files) == 0 {
		t.Fatalf("找不到 db/queries/*.sql：%v", err)
	}
	var out []query
	for _, f := range files {
		raw, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		src := string(raw)
		idx := queryHeader.FindAllStringSubmatchIndex(src, -1)
		for i, m := range idx {
			end := len(src)
			if i+1 < len(idx) {
				end = idx[i+1][0]
			}
			out = append(out, query{name: src[m[2]:m[3]], file: filepath.Base(f), body: normalize(src[m[1]:end])})
		}
	}
	return out
}

func normalize(s string) string {
	var lines []string
	for _, l := range strings.Split(s, "\n") {
		if i := strings.Index(l, "--"); i >= 0 {
			l = l[:i]
		}
		lines = append(lines, l)
	}
	return strings.Join(strings.Fields(strings.Join(lines, " ")), " ")
}

func goldenPredicate(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile("testdata/public_predicate.sql")
	if err != nil {
		t.Fatal(err)
	}
	g := normalize(string(raw))
	if g == "" {
		t.Fatal("golden 是空的")
	}
	return g
}

func TestEveryPublicQueryCarriesThePublicPredicate(t *testing.T) {
	golden := goldenPredicate(t)
	public := 0
	for _, q := range loadQueries(t) {
		if q.file != "public.sql" {
			continue
		}
		public++
		if !strings.Contains(q.body, golden) {
			t.Errorf("public.sql 的 %s 沒有公開可讀述詞（testdata/public_predicate.sql）", q.name)
		}
		viewer := strings.Contains(q.body, "buyer.pub_blocked_buyers()") && strings.Contains(q.body, "buyer.pub_blocked_stores()")
		switch {
		case strings.HasSuffix(q.name, "ForViewer") && !viewer:
			t.Errorf("%s 是有觀看者的查詢，要同時帶買家封鎖與店家封鎖", q.name)
		case strings.HasSuffix(q.name, "Anonymous") && strings.Contains(q.body, "pub_blocked"):
			t.Errorf("%s 是匿名查詢，不得帶封鎖子句（匿名與有觀看者分兩支，不用開關）", q.name)
		}
	}
	if public < 9 {
		t.Fatalf("只掃到 %d 支公開查詢：檢查範圍不對", public)
	}
}

var nonPublicReads = map[string]bool{
	"InsertReview": true, "ListMyReviews": true, "ReplyToReview": true, "GetReviewForDiagnosis": true, "GetReviewStore": true, "LockReview": true,
	"LockReviewsForTakedown": true,
	"InsertReviewImage": true, "ListReviewImages": true,
	"LockBuyerReviewBatch": true, "DeleteReviewsByID": true, "ListInactiveBuyersWithReviews": true, "GetReviewByID": true, "IsReviewPublic": true,
	"RequestRatingReconcileBatch": true,
}

var nonPublicWithPredicate = map[string]bool{"ReplyToReview": true}

func TestNonPublicReviewReadsAreListed(t *testing.T) {
	golden := goldenPredicate(t)
	reviews := regexp.MustCompile(`review\.(reviews|review_images)\b`)
	seen := map[string]bool{}
	for _, q := range loadQueries(t) {
		if q.file == "public.sql" || !reviews.MatchString(q.body) {
			continue
		}
		seen[q.name] = true
		if !nonPublicReads[q.name] {
			t.Errorf("%s（%s）讀寫 review.reviews 或 review.review_images 卻不在 public.sql，也不在 nonPublicReads：公開讀取要放進 public.sql", q.name, q.file)
		}
		if nonPublicWithPredicate[q.name] && !strings.Contains(q.body, golden) {
			t.Errorf("%s 要帶公開可讀述詞", q.name)
		}
	}
	for name := range nonPublicReads {
		if !seen[name] {
			t.Errorf("nonPublicReads 的 %s 已經不存在或不再讀這兩張表，從名單移除", name)
		}
	}
}

// review.hides 的類別 CHECK 與 platform/policy.CommunityAssessable 同一個集合（評價是買家寫的內容，與 social.hides 同一個子集）。
func TestHideCategoriesMatchPolicy(t *testing.T) {
	raw, err := os.ReadFile("db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`hides_reason_category_check CHECK \(reason_category IN \(([^)]*)\)\)`).FindStringSubmatch(string(raw))
	if m == nil {
		t.Fatal("schema.sql 沒有 hides_reason_category_check")
	}
	var got []string
	for _, v := range strings.Split(m[1], ",") {
		got = append(got, strings.Trim(strings.TrimSpace(v), "'"))
	}
	var want []string
	for _, c := range policy.CommunityAssessable() {
		want = append(want, string(c))
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("CHECK %v 與 policy.CommunityAssessable %v 不同", got, want)
	}
}
