package cropidentity

import (
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestUnderstandQuery_VariantsAreDedupedAndOrdered(t *testing.T) {
	q := understandQuery(" 高麗菜。")
	require.Equal(t, " 高麗菜。", q.Text, "原文原樣保留")
	require.Equal(t, []string{"高麗菜。", "高麗菜", "甘藍"}, q.Variants, "去空白原文 → 正規化 → 別名改寫")

	require.Equal(t, []string{"白肉雞", "雞肉"}, understandQuery("白肉雞").Variants, "正規化後與原文相同就不重複；人工 synonym 白肉雞→雞肉")
	require.Equal(t, []string{"毛豬", "豬肉"}, understandQuery("毛豬").Variants)
	require.Equal(t, []string{"糯米", "米"}, understandQuery("糯米").Variants, "人工 synonym 糯米→米（跨 L1，2026-09-17 加）")

	require.Equal(t, []string{"水泥"}, understandQuery("水泥").Variants, "沒有別名時只有原文")
}

func TestUnderstandQuery_BlankHasNoVariants(t *testing.T) {
	require.Empty(t, understandQuery("   ").Variants)
	require.Empty(t, understandQuery("").Variants)
}

func TestUnderstandQuery_StripsOriginAndQualityQualifiers(t *testing.T) {
	cases := []struct {
		query  string
		expect string
	}{
		{"臺灣豬五花", "豬五花"},
		{"台灣高麗菜", "高麗菜"},
		{"本產洋蔥", "洋蔥"},
		{"進口的牛肉", "牛肉"},
		{"新鮮虱目魚", "虱目魚"},
		{"有機小白菜", "小白菜"},
		{"冷凍白蝦", "白蝦"},
	}
	for _, c := range cases {
		got := understandQuery(c.query).Variants
		require.Containsf(t, got, c.expect,
			"「%s」要展開出去掉修飾語的「%s」：2026-09-19 黃金集實測「臺灣豬五花」因為沒有這一步而零召回，"+
				"修前是靠 LLM 猜分類補救的，模型退場後就露出來了。變體只增不減，精確命中仍然優先", c.query, c.expect)
		require.Containsf(t, got, c.query, "原文必須保留在變體裡：%v", got)
	}
}

func TestUnderstandQuery_DoesNotStripWhenRemainderIsTooShort(t *testing.T) {
	for _, q := range []string{"有機", "進口", "臺灣", "冷凍米"} {
		got := understandQuery(q).Variants
		for _, v := range got {
			require.GreaterOrEqualf(t, utf8.RuneCountInString(v), 2,
				"「%s」不該被剝成只剩一個字的變體（%v）——單字變體會把召回炸開", q, got)
		}
	}
}

func TestStripQualifiers_IsIdentityWithoutAQualifierPrefix(t *testing.T) {
	for _, q := range []string{"虱目魚", "松露", "土豆", "水泥", "筆記型電腦", "二手汽車", "糯米", "稉種", "高麗菜", "毛豬"} {
		require.Equalf(t, q, stripQualifiers(q),
			"「%s」沒有修飾語前綴，剝離必須是恆等。黃金集的 NIL 條目全部落在這一類——"+
				"剝離的中立性要靠這條鎖住，不能只在 PR 描述裡論述", q)
	}
}
