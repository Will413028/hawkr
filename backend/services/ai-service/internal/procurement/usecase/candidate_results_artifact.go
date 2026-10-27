package usecase

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"go.uber.org/zap"

	procurementdomain "github.com/Will413028/hawkr/ai-service/internal/procurement/domain"
	"github.com/Will413028/hawkr/ai-service/internal/procurement/repository"
	"github.com/Will413028/hawkr/go-pkg/codemeta"
	pkglogger "github.com/Will413028/hawkr/go-pkg/logger"
)

// produceCandidateResultsArtifact 產製這個 run 的 CANDIDATE_RESULTS artifact
// （doc 57 §4.5 的 13 型之一，data 形狀已展開為 domain.CandidateResultsData）。
//
// 這是 artifact 寫入路徑的**第一個真 producer**：表在 schema 裡存在已久但零 Go 碼
// 碰它，13 型因此全部沒有 producer。挑 CANDIDATE_RESULTS 起手是因為它是 4 個 data
// 形狀已展開的型別之一——不需要預先發明任何未經業務設計驗證的欄位（§4.5 對未展開
// 型別的明文禁止）。
//
// 失敗只記 log：artifact 是投影，`procurement_candidate_sets` 才是這個 domain 的
// 權威狀態來源（同 CreateLedgerRunForCandidateSearch／TerminalizeRun 的既有取捨）。
// 一個新增的觀測層不得把已經寫定的候選結果拖下水。
func (uc *CandidateSearchUseCase) produceCandidateResultsArtifact(
	ctx context.Context, set repository.CandidateSet, platform, external repository.LaneResult,
) {
	runID := set.PublicRunID()

	// 候選的 ID 由 FinalizeCandidateSet 在交易內指派，lane 結果那兩份記憶體副本沒有
	// 它——而 candidateId 是 PublicCandidateV1 的必填欄位（doc 57 §4.1）。少了它前端
	// 沒有任何鍵可以勾選候選，所以這裡讀回落庫後的版本，不用手上的副本硬湊。
	_, persisted, err := uc.store.GetCandidateSetRunWithCandidates(ctx, runID, set.RequestID, set.BuyerID)
	if err != nil {
		pkglogger.Error(ctx, "procurement: failed to read persisted candidates for CANDIDATE_RESULTS artifact",
			zap.Error(err), zap.Int64("candidate_set_id", set.ID), zap.Int64("run_id", runID))
		return
	}

	data := procurementdomain.CandidateResultsData{
		PlatformLane: candidateLaneResult(platform.Status, persisted, procurementdomain.LanePlatform),
		ExternalLane: candidateLaneResult(external.Status, persisted, procurementdomain.LaneExternal),
	}
	payload, err := json.Marshal(data)
	if err != nil {
		pkglogger.Error(ctx, "procurement: failed to marshal CANDIDATE_RESULTS artifact data",
			zap.Error(err), zap.Int64("run_id", runID))
		return
	}

	if _, err := uc.store.UpsertRunArtifact(ctx, repository.UpsertRunArtifactInput{
		RunID: runID, BuyerID: set.BuyerID,
		ArtifactType:   codemeta.ProcurementArtifactTypeCandidateResults.Value,
		Status:         codemeta.ProcurementArtifactStatusReady.Value,
		PublicDataJSON: payload,
		Checksum:       procurementdomain.ArtifactChecksum(payload),
		// domainRef 指回產製來源，供 owner restore 使用（doc 57 §4.5 allowlisted：
		// candidateSetId 是這條 run 既有的 domain ref type，見
		// CreateLedgerRunForCandidateSearch 寫進 run row 的同一組值）。
		DomainRefType: "candidateSetId", DomainRefID: strconv.FormatInt(set.ID, 10),
		AsOf: candidatesAsOf(persisted),
	}); err != nil {
		pkglogger.Error(ctx, "procurement: failed to upsert CANDIDATE_RESULTS artifact",
			zap.Error(err), zap.Int64("candidate_set_id", set.ID), zap.Int64("run_id", runID))
	}
}

// candidateLaneResult 把落庫後的候選依 lane 切出一份公開安全子集。
//
// ProcessedCount／TotalCount 一律 nil：doc 57 §6.3 明文「只有 authoritative 值才回，
// unknown total 為 null，不得顯示假 percent」。候選筆數不是 processedCount——後者要
// 回答的是「掃過幾個來源」，兩條 lane 目前都拿不到那個值，填候選筆數等於編一個看起來
// 像真的分母。
func candidateLaneResult(status string, all []repository.Candidate, lane procurementdomain.Lane) procurementdomain.CandidateLaneResult {
	out := procurementdomain.CandidateLaneResult{
		Status:     status,
		Candidates: make([]procurementdomain.CandidateSummary, 0, len(all)),
	}
	for _, c := range all {
		if c.Lane != string(lane) {
			continue
		}
		out.Candidates = append(out.Candidates, candidateSummary(c, lane))
	}
	return out
}

func candidateSummary(c repository.Candidate, lane procurementdomain.Lane) procurementdomain.CandidateSummary {
	s := procurementdomain.CandidateSummary{
		CandidateID: c.ID, SourceID: c.SourceID, Lane: lane,
		DisplayName: c.DisplayName, Score: float64(c.Score),
		FactsAsOf: c.FactsAsOf,
		// Selected 恆為 false：這個 artifact 是「搜尋結束當下」的快照，而勾選是買家
		// 之後才發生的狀態，住 procurement_candidate_selections。把讀取時才成立的
		// 買家狀態凍進不可變快照，會讓同一個 artifact 版本在不同時間點自相矛盾。
		Selected: false,
	}
	// 兩個 fact 列表是 JSON 原文（repository 存的就是 jsonb），解回 typed 才能進
	// 這個 typed envelope。解析失敗時留空陣列而不是塞進無法驗證的原文——
	// evidence contract 的閘門（acceptEvidence）已經在上游擋過一次 null 與非陣列，
	// 走到這裡還失敗代表 DB 裡的值本身壞了，不該再往公開投影傳。
	if err := json.Unmarshal(c.MatchedFactsJSON, &s.MatchedFacts); err != nil || s.MatchedFacts == nil {
		s.MatchedFacts = []procurementdomain.MatchFact{}
	}
	if err := json.Unmarshal(c.UnverifiableFactsJSON, &s.UnverifiableFacts); err != nil || s.UnverifiableFacts == nil {
		s.UnverifiableFacts = []procurementdomain.UnverifiableFact{}
	}
	// #2377 的賣家身分快照只在 PLATFORM lane 有值；EXTERNAL 四欄一律省略，
	// 不偽造平台 seller identity（同 CandidateSummary 型別註解）。
	if lane == procurementdomain.LanePlatform && c.StoreID != 0 {
		s.StoreID = strconv.FormatInt(int64(c.StoreID), 10)
		s.SellerName, s.StoreCode, s.LogoKey = c.SellerName, c.StoreCode, c.LogoKey
	}
	return s
}

// candidatesAsOf 取全部候選裡**最舊**的 factsAsOf 當這版 artifact 的事實時間。
//
// 一份混合資料的快照只能宣稱它最不新鮮的那一部分的新鮮度：取最新值等於用平台商品
// 的即時性替一批離線名單背書（doc 57 §4.5 asOf 是 freshness，且 §1.6 明文 factsAsOf
// 不是查詢時間）。完全沒有候選時退回現在——沒有任何事實可引用時，這版 artifact 描述
// 的就是「此刻查下來是空的」這件事本身。
func candidatesAsOf(candidates []repository.Candidate) time.Time {
	oldest := time.Time{}
	for _, c := range candidates {
		if c.FactsAsOf.IsZero() {
			continue
		}
		if oldest.IsZero() || c.FactsAsOf.Before(oldest) {
			oldest = c.FactsAsOf
		}
	}
	if oldest.IsZero() {
		return time.Now().UTC()
	}
	return oldest
}
