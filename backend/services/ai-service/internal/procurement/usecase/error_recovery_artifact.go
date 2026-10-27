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

// produceErrorRecoveryArtifact 在候選搜尋 run 失敗時產製 ERROR_RECOVERY artifact。
//
// 為什麼需要它：CANDIDATE_RESULTS 刻意只在 SUCCEEDED 時產製——硬產一個空的等於對前端
// 宣稱「搜尋完成、結果是零家」，而實際上是搜尋失敗。於是在這一型落地之前，**失敗的
// run 完全不產任何 artifact**，「搜尋失敗」在 artifact 這條線上沒有任何表示。這一型
// 就是那個誠實的替代品。
//
// data 形狀不是新發明的：ErrorRecoveryData 沿用既有 procurement 錯誤 envelope
// （doc 57 §4.7「錯誤 envelope 形狀沿用既有實作，不重新設計」），四個已展開的型別之一。
//
// 失敗只記 log：artifact 是投影，procurement_candidate_sets 才是這個 domain 的權威
// 狀態來源（同 produceCandidateResultsArtifact／TerminalizeRun 的既有取捨）。
func (uc *CandidateSearchUseCase) produceErrorRecoveryArtifact(
	ctx context.Context, set repository.CandidateSet, failureReason string,
) {
	runID := set.PublicRunID()

	// Message 取 CodeMeta 的 label，與 gateway 把 RUN_FAILED 投影成 RUN_ERROR 時
	// 用的是同一個來源——同一次失敗在 artifact 與 wire 兩條路上不該長出兩套文案。
	// 值域外的原因不編造文案，退回粗粒度句子（Reason 仍原樣帶出，供除錯）。
	message := runFailureFallbackMessage
	if reason, ok := codemeta.ProcurementRunFailureReasonFromCode(failureReason); ok {
		message = reason.Label
	}

	data := procurementdomain.ErrorRecoveryData{
		Reason:    failureReason,
		Message:   message,
		Retryable: procurementdomain.RunFailureRetryable(failureReason),
	}
	payload, err := json.Marshal(data)
	if err != nil {
		pkglogger.Error(ctx, "procurement: failed to marshal ERROR_RECOVERY artifact data",
			zap.Error(err), zap.Int64("run_id", runID))
		return
	}

	if _, err := uc.store.UpsertRunArtifact(ctx, repository.UpsertRunArtifactInput{
		RunID: runID, BuyerID: set.BuyerID,
		ArtifactType:   codemeta.ProcurementArtifactTypeErrorRecovery.Value,
		Status:         codemeta.ProcurementArtifactStatusReady.Value,
		PublicDataJSON: payload,
		Checksum:       procurementdomain.ArtifactChecksum(payload),
		// domainRef 同 CANDIDATE_RESULTS：指回產製來源，供 owner restore 使用
		// （doc 57 §4.5 allowlisted）。
		DomainRefType: "candidateSetId", DomainRefID: strconv.FormatInt(set.ID, 10),
		// AsOf 是「這個 run 失敗」這件事實發生的時間。這一型沒有更早的事實時點可用
		// （不像 CANDIDATE_RESULTS 取候選的 factsAsOf），失敗當下就是事實時點。
		AsOf: time.Now().UTC(),
	}); err != nil {
		pkglogger.Error(ctx, "procurement: failed to upsert ERROR_RECOVERY artifact",
			zap.Error(err), zap.Int64("candidate_set_id", set.ID), zap.Int64("run_id", runID))
	}
}

// runFailureFallbackMessage 在 failureReason 落在 CodeMeta 值域外時頂替。
const runFailureFallbackMessage = "任務執行失敗"
