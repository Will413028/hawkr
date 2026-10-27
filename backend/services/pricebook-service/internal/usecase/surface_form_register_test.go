package usecase

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain"
)

func TestSurfaceFormsForCommodity_CanonicalBeforeSourceTerms_SkipsBlank(t *testing.T) {
	regs := surfaceFormsForCommodity(7, "白肉雞（2.0 公斤以上）產地價", []string{"白肉雞", "", "  "}, nil)
	require.Len(t, regs, 2)
	require.Equal(t, domain.FormKindCanonical, regs[0].FormKind)
	require.Equal(t, "白肉雞（2.0 公斤以上）產地價", regs[0].FormRaw)
	require.Equal(t, "7", regs[0].EntityCode)
	require.Equal(t, domain.EntityKindCommodity, regs[0].EntityKind)
	require.Equal(t, domain.FormKindSourceTerm, regs[1].FormKind)
	require.Equal(t, "白肉雞", regs[1].FormRaw)
}

func TestSurfaceFormsForCommodity_BuyerAliasesAfterSourceTerms(t *testing.T) {
	regs := surfaceFormsForCommodity(9, "白米零售價（稉種）", []string{"稉種"}, []string{"糯米", "", "  "})
	require.Len(t, regs, 3)
	require.Equal(t, domain.FormKindCanonical, regs[0].FormKind)
	require.Equal(t, domain.FormKindSourceTerm, regs[1].FormKind)
	require.Equal(t, "稉種", regs[1].FormRaw)
	require.Equal(t, domain.FormKindAlias, regs[2].FormKind)
	require.Equal(t, "糯米", regs[2].FormRaw)
	require.Equal(t, "9", regs[2].EntityCode)
	require.Equal(t, domain.EntityKindCommodity, regs[2].EntityKind)
}

func TestSurfaceFormsForCrop_VariantOnlyWhenPresent(t *testing.T) {
	withVariant := surfaceFormsForCrop("LA1", "甘藍-初秋", "初秋")
	require.Len(t, withVariant, 2)
	require.Equal(t, domain.FormKindCanonical, withVariant[0].FormKind)
	require.Equal(t, domain.FormKindVariant, withVariant[1].FormKind)
	require.Equal(t, "初秋", withVariant[1].FormRaw)

	require.Len(t, surfaceFormsForCrop("LA9", "甘藍-進口", ""), 1)
}

func TestSurfaceFormsForCropGroup(t *testing.T) {
	regs := surfaceFormsForCropGroup("moa-group:甘藍", "甘藍")
	require.Len(t, regs, 1)
	require.Equal(t, domain.EntityKindCropGroup, regs[0].EntityKind)
	require.Equal(t, "moa-group:甘藍", regs[0].EntityCode)
	require.Empty(t, surfaceFormsForCropGroup("moa-group:blank", "  "))
}
