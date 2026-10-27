package io.github.will413028.hawkr.product.mq.dto;

import com.fasterxml.jackson.annotation.JsonInclude;
import com.fasterxml.jackson.databind.PropertyNamingStrategies;
import com.fasterxml.jackson.databind.annotation.JsonNaming;

/**
 * {@code product.certification.approved} / {@code .rejected} / {@code .revoked} 三事件共用的
 * payload（19 §12.4 / 36 §3.6h.1，#2755）。消費者是 notification-service（既有 queue
 * {@code notification.product}，binding key {@code '#'} 全收）。
 *
 * <p>🔴 <b>{@code internalNote} 絕不進 payload</b>（僅 ADM 可見，19 §4.1）。
 *
 * <p>🔴 <b>{@code sellerAccountId} 仍須 fail-closed 解析</b>（{@code SellerGrpcClient.getSellerMetaOrThrow}）：
 * 用失敗回空 Map 的批次版本會發出缺這個欄位的 payload —— 事件已「成功」發出，這個欄位卻永久
 * 缺值，重送救不回來。⚠️ 但自 2026-08-30（{@code 20260830100000_seller_target_field_store_id_rekey_batch1.sql}）起，
 * 本檔的三支模板（{@code product.certification.approved}／{@code .rejected}／{@code .revoked}）
 * 的 {@code notification_templates.target_user_field} 已改指向 {@link #storeId}，
 * <b>{@code sellerAccountId} 不再是通知收件人的判斷依據</b>——它僅留供對照／稽核；fail-closed
 * 這條規則對這個欄位本身依然成立，只是理由已經變了。通知收件人身分模型（賣家側逐
 * event_type 漸進遷移 vs. device_tokens／notification_preferences 的原子遷移）見 13 號設計書
 * §12.1.2／§12.1.4，本檔不重複展開。
 *
 * <p>🔴 <b>所有 id 欄位一律 String</b>（TSID 精度）。wire key 為 snake_case。
 *
 * <p>{@code reasonCode} / {@code publicReason} 只有 rejected／revoked 有；approved 為 null，
 * 由類別層的 {@link JsonInclude.Include#NON_NULL} 讓那兩個 key 直接不出現在 wire 上。
 *
 * @param eventId           UUID v4
 * @param eventType         {@code PRODUCT_CERTIFICATION_APPROVED} / {@code _REJECTED} / {@code _REVOKED}
 * @param applicationId     認證申請 ID（TSID 字串）＝ notification 端 {@code reference_id}（一次裁決一則通知）
 * @param productId         商品 ID（TSID 字串）
 * @param productName       商品名稱（推播文案變數）
 * @param storeId           賣家 store 代理鍵（字串；2026-08-30 起是本檔三支模板實際的通知收件人欄位，見上方 class doc）
 * @param sellerAccountId   賣家登入 account_id（原始設計的通知收件人；2026-08-30 起已被 {@link #storeId} 取代，
 *                          僅留供對照／稽核；欄位本身仍須 fail-closed 解析，理由見上方 class doc）
 * @param certificationType {@code CertificationType} value（3701–3706）
 * @param certificateNo     證書編號
 * @param decidedBy         {@code SYSTEM}（背景查驗自動核准）／{@code ADMIN}（人工裁決）
 * @param reasonCode        駁回／撤銷原因碼（{@code ProductCertificationRejectReason}）；approved 為 null
 * @param publicReason      給賣家看的原因說明；approved 為 null
 * @param occurredAt        事件發生時間（ISO 8601 UTC）
 */
@JsonInclude(JsonInclude.Include.NON_NULL)
@JsonNaming(PropertyNamingStrategies.SnakeCaseStrategy.class)
public record ProductCertificationDecidedPayload(
        String eventId,
        String eventType,
        String applicationId,
        String productId,
        String productName,
        String storeId,
        String sellerAccountId,
        String certificationType,
        String certificateNo,
        String decidedBy,
        String reasonCode,
        String publicReason,
        String occurredAt
) {

    /** {@link #decidedBy} 的兩個合法值（19 §4.1 的 actor 語意，與 review_event 的 actor_role 對齊）。 */
    public static final String DECIDED_BY_SYSTEM = "SYSTEM";
    public static final String DECIDED_BY_ADMIN = "ADMIN";
}
