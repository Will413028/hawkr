package io.github.will413028.hawkr.product.converter;

import io.github.will413028.hawkr.common.utils.proto.ProtoConversionUtils;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.domain.ProductPublicationPolicy;
import io.github.will413028.hawkr.product.entity.projection.ProductPublicationBriefProjection;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity.GovVerifyPayload;
import io.github.will413028.hawkr.product.entity.ProductCertificationReviewEventEntity;
import io.github.will413028.hawkr.product.grpc.GovVerifyResult;
import io.github.will413028.hawkr.product.grpc.ProductCertificationApplication;
import io.github.will413028.hawkr.product.grpc.ProductCertificationReviewEvent;
import io.github.will413028.hawkr.product.grpc.ProductCertificationSummary;
import org.springframework.stereotype.Component;

/**
 * 商品認證申請 Entity ↔ Proto 轉換器（19 號設計書《農業認證驗證》§10）。
 *
 * <p><b>型別規範</b>：所有 ID、{@code version}、{@code certification_type} 對外一律字串
 * （{@code formatLong} / {@code formatInt}），時間一律 ISO 8601 UTC（{@code formatIso8601}）。
 * TSID 以 int64 出 JSON 是本 repo 有前例的精度事故。
 *
 * <p>🔴 <b>賣家版與 Admin 版是兩個方法，不是同一個方法加旗標。</b>
 * {@code govVerifyPayload}／{@code internalNote}／{@code submittedByAccountId}／{@code currentApproved}
 * 只出現在 {@link #toAdminProto}。賣家路徑呼叫 {@link #toSellerProto} 就<b>結構上</b>拿不到它們，
 * 而不是靠「記得別填」。
 */
@Component
public class ProductCertificationApplicationConverter {

    /**
     * Entity → Proto（賣家版）。
     *
     * <p>不映射 {@code gov_verify_payload}（政府查驗原文是 ADM 的判斷依據）、
     * {@code internal_note}（僅 ADM 可見）、{@code submitted_by_account_id}（登入 principal）
     * 與 {@code current_approved}／{@code product_name}（Admin 佇列才需要）。
     */
    public ProductCertificationApplication toSellerProto(ProductCertificationApplicationEntity entity) {
        return baseBuilder(entity).build();
    }

    /**
     * Entity → Proto（Admin 版）。
     *
     * @param productBrief    商品輕量資訊；由 Service 以
     *                        {@code ProductRepository.findPublicationBriefByIdsIncludingDeleted} 補齊
     *                        （Repository 不把它 JOIN 進 Entity，見 partial projection 禁令）。
     *                        <b>可能為 null</b>：商品列被實體刪除時查不到，此時商品名為空字串、
     *                        公開狀態不設（proto3 預設空字串），ADM 至少還看得到 productId。
     * @param currentApproved 同 type 現行有效那筆；首次申請時傳 null ⇒ proto3 message 未設
     *                        ⇒ Gateway DTO 輸出 null 而不是空物件
     */
    public ProductCertificationApplication toAdminProto(ProductCertificationApplicationEntity entity,
                                                        ProductPublicationBriefProjection productBrief,
                                                        ProductCertificationApplicationEntity currentApproved) {
        ProductCertificationApplication.Builder builder = baseBuilder(entity);

        builder.setSubmittedByAccountId(ProtoConversionUtils.formatLong(entity.getSubmittedByAccountId()));
        builder.setInternalNote(nullToEmpty(entity.getInternalNote()));
        builder.setProductName(productBrief == null ? "" : nullToEmpty(productBrief.name()));
        // 公開生命週期一律走 ProductPublicationPolicy——這裡不得自己寫
        // 「deletedAt != null || sellerDeletedAt != null」，那會變成同一條規則的第二份定義。
        if (productBrief != null) {
            builder.setProductPublicationState(ProductPublicationPolicy.deriveState(
                    productBrief.firstPublishedAt(),
                    productBrief.status(),
                    productBrief.deletedAt(),
                    productBrief.sellerDeletedAt(),
                    productBrief.storeExposable()).value());
        }

        if (entity.getGovVerifyPayload() != null) {
            builder.setGovVerifyPayload(toGovVerifyResultProto(entity.getGovVerifyPayload()));
        }
        if (currentApproved != null) {
            builder.setCurrentApproved(toSummaryProto(currentApproved));
        }
        return builder.build();
    }

    /** 同 type 現行有效那筆的精簡摘要（§9.2 的 {@code currentApproved}）。 */
    public ProductCertificationSummary toSummaryProto(ProductCertificationApplicationEntity entity) {
        return ProductCertificationSummary.newBuilder()
                .setId(ProtoConversionUtils.formatLong(entity.getId()))
                .setCertificationType(ProtoConversionUtils.formatInt(entity.getCertificationType()))
                .setCertificateNo(nullToEmpty(entity.getCertificateNo()))
                .setStatus(nullToEmpty(entity.getStatus()))
                .setGovVerifyState(nullToEmpty(entity.getGovVerifyState()))
                .setReviewedAt(ProtoConversionUtils.formatIso8601(entity.getReviewedAt()))
                .build();
    }

    /**
     * 政府查驗結果 JSONB → Proto。
     *
     * <p>{@code matchedCount} 在 DB 是數值、對外是字串（§10.3 型別規範）。
     */
    public GovVerifyResult toGovVerifyResultProto(GovVerifyPayload payload) {
        return GovVerifyResult.newBuilder()
                .setCertType(nullToEmpty(payload.getCertType()))
                .setCertNo(nullToEmpty(payload.getCertNo()))
                .setQueriedAt(nullToEmpty(payload.getQueriedAt()))
                .setFound(Boolean.TRUE.equals(payload.getFound()))
                .setValidity(nullToEmpty(payload.getValidity()))
                .setStatusRaw(nullToEmpty(payload.getStatusRaw()))
                .setExpiresOn(nullToEmpty(payload.getExpiresOn()))
                .setHolderName(nullToEmpty(payload.getHolderName()))
                .setHolderAddress(nullToEmpty(payload.getHolderAddress()))
                .setIssuerName(nullToEmpty(payload.getIssuerName()))
                .setProductSummary(nullToEmpty(payload.getProductSummary()))
                .setMatchedCount(ProtoConversionUtils.formatInt(payload.getMatchedCount()))
                .setRawExcerptJson(nullToEmpty(payload.getRawExcerptJson()))
                .build();
    }

    /**
     * 審核事件 Entity → Proto（§9.2 audit-logs）。
     *
     * <p>該端點只有 Admin 入口（{@code ListProductCertificationAuditLogsForAdmin}），
     * 故 {@code internal_note} 在此映射；<b>若日後開放賣家看歷程，必須另寫一個不映射它的方法</b>，
     * 不是在這裡加旗標。
     */
    public ProductCertificationReviewEvent toAdminProto(ProductCertificationReviewEventEntity entity) {
        return ProductCertificationReviewEvent.newBuilder()
                .setId(ProtoConversionUtils.formatLong(entity.getId()))
                .setApplicationId(ProtoConversionUtils.formatLong(entity.getApplicationId()))
                .setAction(nullToEmpty(entity.getAction()))
                .setFromStatus(nullToEmpty(entity.getFromStatus()))
                .setToStatus(nullToEmpty(entity.getToStatus()))
                .setActorRole(nullToEmpty(entity.getActorRole()))
                .setActorAccountId(ProtoConversionUtils.formatLong(entity.getActorAccountId()))
                .setCertificationType(ProtoConversionUtils.formatInt(entity.getCertificationType()))
                .setCertificateNo(nullToEmpty(entity.getCertificateNo()))
                .setReasonCode(nullToEmpty(entity.getReasonCode()))
                .setPublicReason(nullToEmpty(entity.getPublicReason()))
                .setInternalNote(nullToEmpty(entity.getInternalNote()))
                .setApplicationVersion(ProtoConversionUtils.formatLong(entity.getApplicationVersion()))
                .setTraceId(nullToEmpty(entity.getTraceId()))
                .setCreatedAt(ProtoConversionUtils.formatIso8601(entity.getCreatedAt()))
                .build();
    }

    /** 賣家與 Admin 共通的欄位；Admin-only 欄位一律不在這裡填。 */
    private ProductCertificationApplication.Builder baseBuilder(ProductCertificationApplicationEntity entity) {
        return ProductCertificationApplication.newBuilder()
                .setId(ProtoConversionUtils.formatLong(entity.getId()))
                .setProductId(ProtoConversionUtils.formatLong(entity.getProductId()))
                .setStoreId(ProtoConversionUtils.formatLong(entity.getStoreId()))
                .setCertificationType(ProtoConversionUtils.formatInt(entity.getCertificationType()))
                .setCertificateNo(nullToEmpty(entity.getCertificateNo()))
                .setStatus(nullToEmpty(entity.getStatus()))
                .setSource(nullToEmpty(entity.getSource()))
                .setRequiresManualReview(Boolean.TRUE.equals(entity.getRequiresManualReview()))
                .setGovVerifyState(nullToEmpty(entity.getGovVerifyState()))
                .setGovVerifiedAt(ProtoConversionUtils.formatIso8601(entity.getGovVerifiedAt()))
                .setSubmittedAt(ProtoConversionUtils.formatIso8601(entity.getSubmittedAt()))
                .setReviewedAt(ProtoConversionUtils.formatIso8601(entity.getReviewedAt()))
                .setReviewedByAccountId(ProtoConversionUtils.formatLong(entity.getReviewedByAccountId()))
                .setReasonCode(nullToEmpty(entity.getReasonCode()))
                .setPublicReason(nullToEmpty(entity.getPublicReason()))
                .setVersion(ProtoConversionUtils.formatLong(entity.getVersion()));
    }

    /** proto3 的 setter 不接受 null。 */
    private String nullToEmpty(String value) {
        return value == null ? "" : value;
    }
}
