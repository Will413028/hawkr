package handler

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

// reportData 是 gateway 檢舉紀錄 REST 端點（GET /app/reports/me、GET /admin/reports、
// GET /admin/reports/{id}、POST /admin/reports/{id}/resolve）的回應 DTO。檢舉/檢舉人/
// 被檢舉對象/處理人的 id 皆為 64-bit TSID，超過 JS Number.MAX_SAFE_INTEGER (2^53-1)。
// 先前直接序列化 *pb.Report 使這些 int64 id 以 JSON number 送出 → web / admin 原生
// JSON.parse 精度丟失，查不到對應紀錄（與 notification #960 / comment 同 bug class）。
//
// 本測試鎖住 reportData 的所有 TSID id 一律序列化成 JSON 字串、且 > 2^53 不丟精度；
// *_ms 是毫秒 timestamp（< 2^53）維持 number。
func TestReportData_IDsSerializedAsString(t *testing.T) {
	const tsid int64 = 7234567890123456789 // > 2^53
	tsidStr := strconv.FormatInt(tsid, 10)

	r := reportData{
		ID:           tsidStr,
		ReporterID:   tsidStr,
		TargetID:     tsidStr,
		ResolvedBy:   tsid,              // resolved_by 為 admin id，維持 ,string-tagged int64
		ResolvedAtMs: 1_750_000_000_000, // 毫秒 timestamp，< 2^53，維持 number
		CreateTimeMs: 1_750_000_000_000,
	}

	raw, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal reportData: %v", err)
	}
	got := string(raw)

	want := strconv.FormatInt(tsid, 10)
	for _, key := range []string{"id", "reporterId", "targetId", "resolvedBy"} {
		fragment := `"` + key + `":"` + want + `"`
		if !strings.Contains(got, fragment) {
			t.Fatalf("%s 必須序列化成 JSON 字串。\n want substring: %s\n got: %s", key, fragment, got)
		}
	}

	// *_ms 應維持 JSON number（非字串）。
	for _, key := range []string{"resolvedAtMs", "createTimeMs"} {
		fragment := `"` + key + `":1750000000000`
		if !strings.Contains(got, fragment) {
			t.Fatalf("%s 應維持 number，got: %s", key, got)
		}
	}
}
