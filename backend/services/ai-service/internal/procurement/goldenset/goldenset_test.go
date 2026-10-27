package goldenset

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoad_ParsesQueries(t *testing.T) {
	raw := []byte(`
- query: 白肉雞
  expect: { category: "肉類 > 雞肉 > 雞肉" }
  note: 別名
`)
	qs, err := Load(raw)
	require.NoError(t, err)
	require.Len(t, qs, 1)
	require.Equal(t, "白肉雞", qs[0].Text)
	require.Equal(t, "肉類 > 雞肉 > 雞肉", qs[0].ExpectCategory)
}

func TestLoad_KnownGapRequiresTrackedBy(t *testing.T) {
	raw := []byte(`
- query: 糯米
  expect: { category: "穀物 > 米 > 米" }
  status: known_gap
`)
	_, err := Load(raw)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tracked_by")
}

func TestLoad_NilExpectMeansNIL(t *testing.T) {
	raw := []byte(`
- query: 鑽石
  expect: NIL
`)
	qs, err := Load(raw)
	require.NoError(t, err)
	require.True(t, qs[0].ExpectsNIL())
}

func TestLoad_RejectsEmptyQuery(t *testing.T) {
	_, err := Load([]byte("- note: 沒有 query\n"))
	require.Error(t, err)
}

func TestGoldenSet_FileIsValid(t *testing.T) {
	raw, err := os.ReadFile("testdata/golden_queries.yaml")
	require.NoError(t, err)
	qs, err := Load(raw)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(qs), 12, "起手至少 12 條")
}

func TestQueryExpectDescription(t *testing.T) {
	cases := []struct {
		name string
		q    Query
		want string
	}{
		{"分類", Query{ExpectCategory: "穀物 > 米"}, "穀物 > 米"},
		{"label", Query{ExpectLabel: "白米零售價（稉種）"}, "白米零售價（稉種）"},
		{"兩者都有", Query{ExpectCategory: "肉類 > 豬肉", ExpectLabel: "毛豬（規格豬）"}, "肉類 > 豬肉 ／ 毛豬（規格豬）"},
		{"逃生艙", Query{}, "NIL（逃生艙）"},
	}
	for _, c := range cases {
		if got := c.q.ExpectDescription(); got != c.want {
			t.Errorf("%s: ExpectDescription() = %q, want %q", c.name, got, c.want)
		}
	}
}
