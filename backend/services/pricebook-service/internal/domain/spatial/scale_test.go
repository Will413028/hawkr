package spatial_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Will413028/hawkr/pricebook-service/internal/domain/spatial"
	"github.com/stretchr/testify/require"
)

func writeScaleFixture(t *testing.T, csvContent string, manifestDigest string) (csvPath, manifestPath string) {
	t.Helper()
	dir := t.TempDir()
	csvPath = filepath.Join(dir, "price_scale.v1.csv")
	require.NoError(t, os.WriteFile(csvPath, []byte(csvContent), 0o600))
	manifestPath = filepath.Join(dir, "price_scale.v1.manifest.json")
	manifest := `{"sha256":"` + manifestDigest + `","row_count":1}`
	require.NoError(t, os.WriteFile(manifestPath, []byte(manifest), 0o600))
	return csvPath, manifestPath
}

func TestLoadPriceScales_RealFile_LoadsWithoutError(t *testing.T) {
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "reference"))
	require.NoError(t, err)
	scales, err := spatial.LoadPriceScales(
		filepath.Join(dir, "price_scale.v1.csv"),
		filepath.Join(dir, "price_scale.v1.manifest.json"),
	)
	require.NoError(t, err)
	require.NotEmpty(t, scales)
	for key, scale := range scales {
		require.NotEmpty(t, scale.ScaleID, "scale for %s must have a scaleId", key)
		require.NotEmpty(t, scale.PolicyVersion)
		require.GreaterOrEqual(t, len(scale.Breaks), 2, "issue #2248 requires 2-7 fixed breaks")
		require.LessOrEqual(t, len(scale.Breaks), 7)
		for i := 1; i < len(scale.Breaks); i++ {
			require.Greater(t, scale.Breaks[i], scale.Breaks[i-1], "breaks must be strictly increasing")
		}
	}
}

func TestLoadPriceScales_DigestMismatch_ReturnsError(t *testing.T) {
	csvPath, manifestPath := writeScaleFixture(t,
		"scale_version,selection_kind,selection_code,scale_id,policy_version,breaks\nv1,group,N04,N04-v1,v1,\"20|30\"\n",
		"0000000000000000000000000000000000000000000000000000000000000000")
	_, err := spatial.LoadPriceScales(csvPath, manifestPath)
	require.ErrorIs(t, err, spatial.ErrScaleDigestMismatch)
}
