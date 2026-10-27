package io.github.will413028.hawkr.payment.application.dto;

import java.time.LocalDateTime;

/**
 * 撥款指示結果 DTO
 *
 * <p>§5.6 去重硬化 A（P0-3a）：{@code grantStatus} / {@code grantMessage} 承載撥款成敗，
 * 供 gRPC adapter 於 {@code @Transactional} 外依已落地的 grant_status 決定回 success 或 throw
 * gRPC error（write-then-return 讓失敗終態隨交易 commit 存活，失敗訊號於 commit 後由 adapter 產生）。
 *
 * <p>§5.6.1：{@code grantOutcome} 承載結構化結果語意，供 adapter 組
 * {@code GrantPaymentResponse.grant_outcome}——{@code GRANTED}（指示成功發動）/
 * {@code PROVIDER_TOOK_OVER}（PayUni 已接管撥款，屬正常回應非錯誤）；失敗路徑為 {@code null}
 * （adapter 照舊 throw gRPC error）。
 *
 * @param grantStatus  {@code SUCCESS} / {@code PENDING_CONFIRM} / PayUni 錯誤碼
 * @param grantMessage PayUni 回應訊息或失敗原因
 * @param grantOutcome {@code GrantResponseRules.GRANT_OUTCOME_*}；失敗（要 throw）時為 null
 */
public record GrantPaymentResult(
        String tradeNo,
        String orderId,
        String providerTradeNo,
        long amount,
        LocalDateTime grantedAt,
        String grantStatus,
        String grantMessage,
        String grantOutcome
) {}
