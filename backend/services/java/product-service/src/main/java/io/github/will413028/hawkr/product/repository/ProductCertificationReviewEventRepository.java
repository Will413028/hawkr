package io.github.will413028.hawkr.product.repository;

import io.github.will413028.hawkr.product.entity.ProductCertificationReviewEventEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Options;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;

/**
 * 商品認證審核事件 Repository（19 號設計書《農業認證驗證》§4.2）。
 *
 * <p><b>append-only：只有 INSERT 與 SELECT。</b>
 * 刻意<b>不繼承 {@code BaseMapper}</b>（與本服務其他 Repository 的慣例不同）：
 * 繼承會一併帶進 {@code updateById} / {@code deleteById}，讓「禁止 UPDATE / DELETE」
 * 退化成只靠註解與紀律。稽核表的不可竄改性應該由型別擋住。
 *
 * <p>寫入時機（缺一即查不出標章是誰放行／誰讓它退場的）：
 * <ul>
 *   <li>SUBMIT：建立申請</li>
 *   <li>AUTO_APPROVE：<b>自動核准也必須寫</b>（actorRole = SYSTEM）</li>
 *   <li>APPROVE / REJECT / REVOKE：ADM 裁決</li>
 *   <li>WITHDRAW：賣家撤回，或該 type 在全量取代中完全沒送（§6.4）</li>
 *   <li>SUPERSEDE：§6.6 ① 換號退場，以及 §6.3 Step 4 的<b>每一列</b>（含併發競態造成的退場）</li>
 * </ul>
 */
@Mapper
public interface ProductCertificationReviewEventRepository {

    /**
     * 寫入一筆審核事件。{@code created_at} 交給 DB {@code DEFAULT NOW()}
     * （本表不走 BaseMapper ⇒ MetaObjectHandler 不會執行，標 fill 只會是假保證）。
     *
     * @return 1（失敗會拋例外，不回 0）
     */
    @Insert("""
        INSERT INTO product_certification_review_event
            (application_id, action, from_status, to_status, actor_role, actor_account_id,
             certification_type, certificate_no, reason_code, public_reason, internal_note,
             application_version, trace_id)
        VALUES
            (#{applicationId}, #{action}, #{fromStatus}, #{toStatus}, #{actorRole}, #{actorAccountId},
             #{certificationType}, #{certificateNo}, #{reasonCode}, #{publicReason}, #{internalNote},
             #{applicationVersion}, #{traceId})
        """)
    @Options(useGeneratedKeys = true, keyProperty = "id")
    int insert(ProductCertificationReviewEventEntity event);

    /**
     * 單筆申請的稽核歷程（§9.2 audit-logs 分頁）。
     * 排序走 {@code idx_pcre_application (application_id, id)}。
     */
    @Select("""
        SELECT * FROM product_certification_review_event
        WHERE application_id = #{applicationId}
        ORDER BY id DESC
        LIMIT #{limit} OFFSET #{offset}
        """)
    List<ProductCertificationReviewEventEntity> findByApplicationId(
            @Param("applicationId") Long applicationId,
            @Param("limit") int limit,
            @Param("offset") int offset);

    @Select("SELECT COUNT(*) FROM product_certification_review_event WHERE application_id = #{applicationId}")
    long countByApplicationId(@Param("applicationId") Long applicationId);
}
