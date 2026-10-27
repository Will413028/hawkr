package io.github.will413028.hawkr.product.mq.dto;

import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

/**
 * {@code product.certification.verify.requested} 的 payload（19 §12 / 36 §3.6h.1，#2755）。
 *
 * <p>由 {@code ProductCertificationApplicationService} 在建立 {@code SUBMITTED} 申請、
 * 且 {@code gov_verify_state = PENDING}（＝同步快取未命中、需要背景補課）時，於<b>同一交易的
 * afterCommit hook</b> 發出；消費者是 product-service 自己（queue {@code product.certification.verify}）。
 *
 * <p>🔴 <b>所有 id 欄位一律 String</b>：TSID 是 64-bit，以 JSON number 傳給 generic-map 消費端
 * 會被 float64 截斷（本 repo 有前例）。
 *
 * <p>wire key 為 snake_case（{@code @JsonNaming} SnakeCaseStrategy），與同 exchange 的
 * {@code product.restock} 一致。
 *
 * @param eventId              UUID v4
 * @param eventType            固定 {@code PRODUCT_CERTIFICATION_VERIFY_REQUESTED}（CodeMeta {@code ProductEventType}）
 * @param applicationId        認證申請 ID（TSID 字串）
 * @param productId            商品 ID（TSID 字串）
 * @param storeId              賣家 store 代理鍵（字串；<b>不是</b>登入 account_id）
 * @param certificationType    {@code CertificationType} value（3701–3706）
 * @param certificateNo        證書編號
 * @param requiresManualReview true ⇒ 查驗通過也<b>不得</b>自動核准（§6.3 Step 3）；查驗仍要跑，ADM 重審需要政府資料當依據
 * @param occurredAt           事件發生時間（ISO 8601 UTC）
 */
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record ProductCertificationVerifyRequestedPayload(
        String eventId,
        String eventType,
        String applicationId,
        String productId,
        String storeId,
        String certificationType,
        String certificateNo,
        boolean requiresManualReview,
        String occurredAt
) {}
