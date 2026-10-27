package io.github.will413028.hawkr.product.entity;

import com.baomidou.mybatisplus.annotation.IdType;
import com.baomidou.mybatisplus.annotation.TableId;
import com.baomidou.mybatisplus.annotation.TableName;
import lombok.Data;

import java.io.Serializable;
import java.time.LocalDateTime;

/**
 * 商品認證審核事件 Entity（19 號設計書《農業認證驗證》§4.2，Flyway V18）。
 *
 * <p><b>append-only：禁止 UPDATE / DELETE。</b>
 * {@link io.github.will413028.hawkr.product.repository.ProductCertificationReviewEventRepository}
 * 刻意不繼承 {@code BaseMapper}，只暴露 INSERT 與 SELECT，讓 append-only 由型別擋住而非靠紀律。
 *
 * <p>🔴 <b>自動核准也必須寫事件</b>（{@code actorRole = SYSTEM}、{@code action = AUTO_APPROVE}）。
 * 缺了它，事後查不出「這張標章是誰放行的」，而自動核准正是最需要事後可稽核的路徑。
 *
 * <p>另外 §6.3 Step 4 的每一列 SUPERSEDE 都要寫一筆（含併發競態造成的退場），
 * 否則賣家的申請憑空消失且查不到是誰動的。
 */
@Data
@TableName("product_certification_review_event")
public class ProductCertificationReviewEventEntity implements Serializable {

    private static final long serialVersionUID = 1L;

    /** 事件 ID（BIGSERIAL；內部表不用 TSID） */
    @TableId(type = IdType.AUTO)
    private Long id;

    /** 對應的申請 ID（FK 語意，不建 FK 約束，與本服務既有慣例一致） */
    private Long applicationId;

    /**
     * 動作（{@link io.github.will413028.hawkr.common.utils.code.ProductCertificationReviewAction}：
     * SUBMIT／AUTO_APPROVE／APPROVE／REJECT／WITHDRAW／SUPERSEDE／REVOKE）。
     */
    private String action;

    /** 轉移前狀態（ProductCertificationStatus）；建立時為 null */
    private String fromStatus;

    /** 轉移後狀態（ProductCertificationStatus） */
    private String toStatus;

    /** SYSTEM / SELLER / ADMIN */
    private String actorRole;

    /** 動作發起者 account id；SYSTEM 時為 null */
    private Long actorAccountId;

    /** 認證類型快照（申請列日後被改也查得到當時值） */
    private Integer certificationType;

    /** 證書編號快照 */
    private String certificateNo;

    /** 原因碼快照（ProductCertificationRejectReason） */
    private String reasonCode;

    /** 對賣家公開的原因快照 */
    private String publicReason;

    /** 內部備註快照；<b>僅 ADM 可見</b> */
    private String internalNote;

    /** 事件當下的 application version */
    private Long applicationVersion;

    /** 由 observability context 帶入的 trace id */
    private String traceId;

    /**
     * 事件時間。
     *
     * <p>由 DB {@code DEFAULT NOW()} 填：本表走自訂 {@code @Insert}（不列 created_at 欄），
     * 而 MyBatis-Plus 的 MetaObjectHandler 只在 BaseMapper 的 insert/update 路徑上生效
     * ⇒ 在這裡標 {@code @TableField(fill = INSERT)} 是<b>不會執行的</b>假保證，故刻意不標。
     */
    private LocalDateTime createdAt;
}
