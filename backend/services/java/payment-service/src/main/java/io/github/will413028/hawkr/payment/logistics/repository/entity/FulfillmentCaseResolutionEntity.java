package io.github.will413028.hawkr.payment.logistics.repository.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableField;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import io.github.will413028.hawkr.payment.infrastructure.persistence.typehandler.JsonbStringTypeHandler;
import java.time.LocalDateTime;
import lombok.Data;

/**
 * {@code fulfillment_case_resolutions}（V9）—— 例外案件的處置提案與覆核。
 *
 * <p>🔴 {@code impactPreview} 是<b>快照，不是即時算</b>（A-4 request 逐字）：覆核當下重算並比對，
 * 不一致 ⇒ 9575。掛 {@link JsonbStringTypeHandler} ⇒ 本類別必須 {@code autoResultMap = true}。
 *
 * <p>🔴 {@code reviewedByAccountId} 不得等於 {@code proposedByAccountId}
 * （{@code chk_fcr_two_person}，沿用 {@code chk_invmu_two_person} 的形狀）。
 *
 * <p>⚠️ {@code requiresReview} 由 {@code resolutionType} 完全決定
 * （{@code chk_fcr_review_required}：只有 {@code ANOMALY_RESUME} 免覆核）——
 * 它<b>不是</b>可以自由設定的旗標，傳錯值會被 DB 直接擋下。
 */
@Data
@TableName(value = "fulfillment_case_resolutions", autoResultMap = true)
public class FulfillmentCaseResolutionEntity {

    /** TSID（＝對外的 resolution_id，A-5 的 path param）。 */
    @TableId(type = IdType.INPUT)
    private Long id;

    private Long caseId;

    /**
     * 所屬案件的 {@code category} 投影（V12）。
     *
     * <p>🔴 <b>不是可自由填寫的欄位</b>：複合外鍵 {@code fk_fcr_case_category}
     * （{@code (case_id, case_category) REFERENCES fulfillment_exception_cases (id, category)}）
     * 保證它忠實反映該 case 真正的 category——填錯會被 DB 以 23503 擋下，而不是靜默存進一個謊。
     *
     * <p>它存在的理由是把<b>跨表</b>判定變成同列判定：{@code chk_fcr_responsibility_type_binding}
     * 因此得以在 DB 層雙向綁定 {@code resolutionType} 與案件 category（65 §5.5.2 E0.2 第 5 項的
     * 「第二扇門」）。少了這個投影，那道綁定只能留在應用層，而應用層的 if 會在下一次重構被抽掉。
     *
     * <p>⚠️ 附帶保證：複合 FK 的 {@code ON UPDATE} 預設 {@code NO ACTION} ⇒ 案件一旦有提案，
     * 它的 category 就改不動了。
     */
    private String caseCategory;

    /**
     * {@code CLAIM_SETTLE} / {@code CLAIM_DISMISS} / {@code SHIPMENT_CANCEL} /
     * {@code SHIPMENT_CANCEL_CONFIRM} / {@code SHIPMENT_CANCEL_AND_REFUND} / {@code ANOMALY_RESUME}
     * / {@code RESPONSIBILITY_ATTRIBUTE}（V12 新增，CodeMeta {@code FulfillmentCaseResolutionType}）。
     *
     * <p>🔴 {@code RESPONSIBILITY_ATTRIBUTE} 與 {@link #caseCategory} 是<b>雙向</b>綁定：
     * 它只能用在責任判定案件上，責任判定案件也只接受它。
     */
    private String resolutionType;

    /**
     * 🔴 <b>Maker 提出的責任成因</b>（V13）——覆核者批准的就是這個值，他<b>提供不了</b>別的。
     *
     * <p>責任判定型別 ⇔ 本欄非 null（{@code chk_fcr_proposed_cause_binding}，雙向）；
     * 非 null 時必為四個 resolved 的 {@code ResponsibilityCause} 之一
     * （{@code chk_fcr_proposed_cause_domain}）。
     *
     * <p>⚠️ 這一欄存在的理由是雙人制：沒有它，{@code chk_fcr_two_person} 只保證
     * 「覆核者 ≠ 提案者」，不保證「覆核者批准的正是提案者提出的那一個」——
     * 而責任成因決定買家拿不拿得到退款。
     */
    private String proposedResponsibilityCause;

    private String reason;

    /** 🔴 JSONB object 快照。 */
    @TableField(typeHandler = JsonbStringTypeHandler.class)
    private String impactPreview;

    private LocalDateTime impactComputedAt;

    /** ⚠️ 由 {@code resolutionType} 決定，不是自由旗標。 */
    private Boolean requiresReview;

    /** {@code AWAITING_REVIEW} / {@code APPROVED} / {@code REJECTED}。 */
    private String status;

    /** ⚠️ account_id（ADM 操作者），不是 store_id。 */
    private Long proposedByAccountId;

    private LocalDateTime proposedAt;
    private Long reviewedByAccountId;
    private String reviewComment;
    private LocalDateTime reviewedAt;
    private LocalDateTime createdAt;
    private LocalDateTime updatedAt;
}
