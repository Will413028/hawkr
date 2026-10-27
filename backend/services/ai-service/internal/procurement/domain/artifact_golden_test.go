package domain

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Will413028/hawkr/go-pkg/codemeta"
)

// updateArtifactGolden 重新產生 testdata/artifacts 下的 fixture，用法同
// gateway agui_golden_test.go 的 -update-agui-golden：
// go test -run TestArtifactGoldenFixtures -update-artifact-golden
var updateArtifactGolden = flag.Bool("update-artifact-golden", false, "重新產生 testdata/artifacts 下的 fixture")

// TestArtifactGoldenFixtures 釘住 doc 57 §4.1.3／§4.5 已展開四型（Track A 的
// REQUEST_SNAPSHOT／CLARIFICATION／CANDIDATE_RESULTS／ERROR_RECOVERY）的完整
// Artifact envelope JSON 序列化形狀，逐位元組比對版控 fixture。
//
// 這是 B1（doc 57 §12 Slice B）契約凍結的最後一項：BE-01B serializer
// （Slice D）與 #2448 的 ledger row envelope／runtime-snapshot DTO 要對著它
// 寫。其餘 9 型（doc 57 §4.5 note）data 形狀尚未展開，故本檔不產生對應
// fixture——凍結一個還沒有業務設計驗證的形狀，比暫時沒有 fixture 更危險。
//
// 目前只有 backend 一側消費這份 schema（website 的 assistant-ui artifact
// renderer 尚未開工，見 wiki agent-runtime-pipeline-plan Slice D／E）。issue
// #2446 08-20 delta 要求「Backend/Web golden fixture checksum」，比照
// gateway `agui_golden_test.go` 加 website 拷貝閘的 cross-repo gate 留待
// website 端真的開始消費這份 schema 時再補，現在加只是一個永遠 SKIP 的殼。
func TestArtifactGoldenFixtures(t *testing.T) {
	asOf := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name     string
		artifact Artifact
	}{
		{"request-snapshot", func() Artifact {
			item := "台灣鯛"
			qty := 20
			unit := "KG"
			city := "台中市"
			exact := true
			data := RequestSnapshotData{
				RevisionID: 42,
				RevisionNo: 3,
				Slots: Slots{
					Item: &item, Quantity: &qty, Unit: &unit,
					DeliveryCity: &city, ExactMatchRequired: &exact,
				},
				SlotsHash: "sha256:slots-golden",
			}
			return Artifact{
				ArtifactID: 1001,
				Type:       ArtifactRequestSnapshot,
				Version:    1,
				Checksum:   "sha256:request-snapshot-golden",
				Status:     codemeta.ProcurementArtifactStatusReady.Value,
				AsOf:       asOf,
				DomainRef:  "42",
				Data:       mustMarshalGolden(t, data),
			}
		}()},
		{"clarification", func() Artifact {
			// 用真正的 BuildQuestionFor，不手刻 ClarifyQuestion——理由同
			// gateway golden test 的 stepDetailPayload：production 就是這條
			// 路徑產生的值，手刻會把 fixture 釘在不會出現的形狀上。
			question := BuildQuestionFor(SlotUnit, Slots{})
			data := ClarificationData{Question: *question}
			return Artifact{
				ArtifactID: 2001,
				Type:       ArtifactClarification,
				Version:    1,
				Checksum:   "sha256:clarification-golden",
				Status:     codemeta.ProcurementArtifactStatusReady.Value,
				AsOf:       asOf,
				DomainRef:  "42",
				Data:       mustMarshalGolden(t, data),
			}
		}()},
		{"candidate-results", func() Artifact {
			processed, total := 5, 12
			data := CandidateResultsData{
				PlatformLane: CandidateLaneResult{
					Status:         codemeta.ProcurementLaneStatusSucceeded.Value,
					ProcessedCount: &processed,
					TotalCount:     &total,
					Candidates: []CandidateSummary{{
						CandidateID: 5001,
						SourceID:    "sku-5001",
						Lane:        LanePlatform,
						DisplayName: "陽光小農創意鮮蔬",
						Score:       0.92,
						MatchedFacts: []MatchFact{
							{Key: SlotItem, Label: "品項"},
							{Key: SlotDeliveryCity, Label: "配送城市"},
						},
						UnverifiableFacts: []UnverifiableFact{
							{Key: SlotDeliveryDate, Reason: "賣家未提供到貨時間，需向賣家確認"},
						},
						FactsAsOf:  asOf,
						Selected:   false,
						StoreID:    "store-01",
						SellerName: "陽光小農",
						StoreCode:  "sunfarm",
						LogoKey:    "sellers/sunfarm/logo.png",
					}},
				},
				// EXTERNAL lane 刻意示範兩件事同時成立：processedCount／
				// totalCount 在 unknown total 時是 nil（doc 57 §6.3，不得
				// 顯示假 percent），而 matchedFacts／unverifiableFacts 即使
				// 為空也是非 nil 空 slice（doc 57 §4.1「保證非 null」）。
				ExternalLane: CandidateLaneResult{
					Status:         codemeta.ProcurementLaneStatusRunning.Value,
					ProcessedCount: nil,
					TotalCount:     nil,
					Candidates: []CandidateSummary{{
						CandidateID:       6001,
						SourceID:          "lead-6001",
						Lane:              LaneExternal,
						DisplayName:       "阿明蔬果行",
						Score:             0.81,
						MatchedFacts:      []MatchFact{},
						UnverifiableFacts: []UnverifiableFact{},
						FactsAsOf:         asOf,
						Selected:          false,
					}},
				},
			}
			return Artifact{
				ArtifactID: 3001,
				Type:       ArtifactCandidateResults,
				Version:    2,
				Checksum:   "sha256:candidate-results-golden",
				Status:     codemeta.ProcurementArtifactStatusReady.Value,
				AsOf:       asOf,
				DomainRef:  "candidate-set-77",
				Data:       mustMarshalGolden(t, data),
			}
		}()},
		{"error-recovery", func() Artifact {
			data := ErrorRecoveryData{
				Reason:    codemeta.ProcurementErrorReasonStaleRevision.Value,
				Message:   "版本已過期，請重新整理後再試",
				Retryable: true,
			}
			// 刻意不帶 DomainRef（omitempty）：示範失敗態 artifact 不一定有
			// 可回溯的來源 domain 物件。
			return Artifact{
				ArtifactID: 4001,
				Type:       ArtifactErrorRecovery,
				Version:    1,
				Checksum:   "sha256:error-recovery-golden",
				Status:     codemeta.ProcurementArtifactStatusFailed.Value,
				AsOf:       asOf,
				Data:       mustMarshalGolden(t, data),
			}
		}()},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wire, err := json.MarshalIndent(tc.artifact, "", "  ")
			require.NoError(t, err)
			wire = append(wire, '\n')

			path := filepath.Join("testdata", "artifacts", tc.name+".json")

			if *updateArtifactGolden {
				require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
				require.NoError(t, os.WriteFile(path, wire, 0o644))
				return
			}

			want, err := os.ReadFile(path)
			require.NoErrorf(t, err, "讀不到 fixture（第一次請跑 go test -run TestArtifactGoldenFixtures -update-artifact-golden）")
			require.Truef(t, bytes.Equal(want, wire),
				"Artifact JSON 與 fixture 不符。若是預期的欄位變更，跑 -update-artifact-golden 重新產生。\ngot:  %s\nwant: %s",
				wire, want)
		})
	}
}

func mustMarshalGolden(t *testing.T, v any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(v)
	require.NoError(t, err)
	return raw
}
