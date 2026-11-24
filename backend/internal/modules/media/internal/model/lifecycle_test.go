package model_test

import (
	"errors"
	"os"
	"regexp"
	"slices"
	"testing"

	"github.com/Will413028/hawkr/backend/internal/modules/media/internal/model"
)

// checkValues 回傳 schema.sql 裡名為 constraint 的 CHECK 的 IN (...) 值清單。
func checkValues(t *testing.T, schema, constraint string) []string {
	t.Helper()
	re := regexp.MustCompile(`CONSTRAINT ` + constraint + ` CHECK \(\s*(?:[a-z_]+ )?IN \(([^)]*)\)`)
	m := re.FindStringSubmatch(schema)
	if m == nil {
		t.Fatalf("schema.sql 沒有 %s 的 IN 清單", constraint)
	}
	var out []string
	for _, v := range regexp.MustCompile(`'([A-Z_]+)'`).FindAllStringSubmatch(m[1], -1) {
		out = append(out, v[1])
	}
	slices.Sort(out)
	return out
}

func TestStoredCodesMatchSchemaChecks(t *testing.T) {
	raw, err := os.ReadFile("../../db/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	schema := string(raw)
	cases := []struct {
		constraint string
		want       []string
	}{
		{"assets_visibility_check", []string{string(model.VisibilityPublic), string(model.VisibilitySecure)}},
		{"staged_uploads_visibility_check", []string{string(model.VisibilityPublic), string(model.VisibilitySecure)}},
		{"assets_state_check", []string{string(model.AssetStaged), string(model.AssetDeleting), string(model.AssetDeleted)}},
		{"upload_claims_kind_check", []string{"CONFIRMED", "RECLAIMED"}},
		{"staged_uploads_state_check", []string{
			string(model.StatePending), string(model.StateUploading), string(model.StateUploaded), string(model.StateReady), string(model.StateRejected),
			string(model.StateConfirmed), string(model.StateDeleting), string(model.StateDeleted),
		}},
	}
	for _, c := range cases {
		want := slices.Clone(c.want)
		slices.Sort(want)
		if got := checkValues(t, schema, c.constraint); !slices.Equal(got, want) {
			t.Errorf("%s 的值是 %v，Go 常數是 %v", c.constraint, got, want)
		}
	}
	reasons := []model.RejectReason{
		model.RejectUnrecognizedFormat, model.RejectSizeMismatch, model.RejectChecksumMismatch, model.RejectProcessingFailed, model.RejectExpired,
	}
	var want []string
	for _, r := range reasons {
		if !r.Valid() {
			t.Errorf("%s 應該是有效的拒絕原因", r)
		}
		want = append(want, string(r))
	}
	slices.Sort(want)
	re := regexp.MustCompile(`CONSTRAINT staged_uploads_reject_reason_check CHECK \([^;]*?IN \(([^)]*)\)`)
	m := re.FindStringSubmatch(schema)
	if m == nil {
		t.Fatal("schema.sql 沒有 staged_uploads_reject_reason_check")
	}
	var got []string
	for _, v := range regexp.MustCompile(`'([A-Z_]+)'`).FindAllStringSubmatch(m[1], -1) {
		got = append(got, v[1])
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("拒絕原因的 CHECK 是 %v，Go 常數是 %v", got, want)
	}
	if model.RejectReason("OTHER").Valid() {
		t.Error("未知的原因不該有效")
	}
}

func TestPresignNeverOutlivesTheUpload(t *testing.T) {
	if model.PresignTTL > model.UploadTTL {
		t.Fatalf("預簽網址有效期 %v 超過暫存 TTL %v", model.PresignTTL, model.UploadTTL)
	}
}

func TestPermanentUnwrapsAndKeepsTheReason(t *testing.T) {
	cause := errors.New("bad magic")
	err := model.Permanent(model.RejectUnrecognizedFormat, cause)
	var perm *model.PermanentError
	if !errors.As(err, &perm) || perm.Reason != model.RejectUnrecognizedFormat || !errors.Is(err, cause) {
		t.Fatalf("err = %v，沒有保留原因或根因", err)
	}
}

func TestDeriveAssetState(t *testing.T) {
	cases := []struct {
		physical                   model.AssetState
		confirmed, deleteRequested bool
		want                       model.AssetState
	}{
		{model.AssetStaged, false, false, model.AssetStaged},
		{model.AssetStaged, true, false, model.AssetActive},
		{model.AssetStaged, true, true, model.AssetDeleting},
		{model.AssetStaged, false, true, model.AssetDeleting},
		{model.AssetDeleting, true, false, model.AssetDeleting},
		{model.AssetDeleting, false, false, model.AssetDeleting},
		{model.AssetDeleted, true, true, model.AssetDeleted},
		{model.AssetDeleted, false, false, model.AssetDeleted},
	}
	for _, c := range cases {
		if got := model.DeriveAssetState(c.physical, c.confirmed, c.deleteRequested); got != c.want {
			t.Errorf("DeriveAssetState(%s, %v, %v) = %s，預期 %s", c.physical, c.confirmed, c.deleteRequested, got, c.want)
		}
	}
}
