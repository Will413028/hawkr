package domain

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCategoryOrigin_FromSelection(t *testing.T) {
	sel := &CropSelection{Stage: CropSelectionItem, ParentCategoryID: "198", ParentLabel: "豬肉"}
	o := CategoryFromSelection(sel)
	require.True(t, o.Declared())
	require.Equal(t, "198", o.ID())
}

func TestCategoryOrigin_FromSelectionWithoutConfirmedCategoryIsStillDeclared(t *testing.T) {
	o := CategoryFromSelection(&CropSelection{Stage: CropSelectionGroup})
	require.True(t, o.Declared(), "問過 selection 而它沒有分類，跟從來沒問過是兩件事")
	require.Empty(t, o.ID())
}

func TestCategoryOrigin_Absent(t *testing.T) {
	o := CategoryAbsent("自填價沒有對應實體")
	require.True(t, o.Declared())
	require.Empty(t, o.ID())
	require.Equal(t, "自填價沒有對應實體", o.AbsentReason())
}

func TestCategoryOrigin_ZeroValueIsUndeclared(t *testing.T) {
	var o CategoryOrigin
	require.False(t, o.Declared(),
		"零值代表建構點沒有表態，這是 2026-09-18 兩次漏接的形狀；不變式測試靠這個抓")
}

func TestNewSelfPricedItemResolution_UndeclaredOriginProducesNothing(t *testing.T) {
	got := NewSelfPricedItemResolution(95, "高麗菜", CategoryOrigin{})

	require.Nil(t, got,
		"沒表態分類來源就建不出 resolution——寧可整個解析不見（買家會看到逃生艙），也不要靜默掉一個空的 category_id")
}

func TestNewSelfPricedItemResolution_DeclaredAbsentStillProducesResolution(t *testing.T) {
	got := NewSelfPricedItemResolution(95, "", CategoryAbsent("測試"))

	require.NotNil(t, got)
	require.Empty(t, got.CategoryID)
}

func newSelfPricedForTest(pricePerKg int) *ItemResolution {
	return NewSelfPricedItemResolution(pricePerKg, "", CategoryAbsent("測試"))
}
