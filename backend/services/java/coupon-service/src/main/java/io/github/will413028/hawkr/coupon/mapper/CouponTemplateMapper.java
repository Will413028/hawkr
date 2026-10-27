package io.github.will413028.hawkr.coupon.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import com.baomidou.mybatisplus.core.metadata.IPage;
import io.github.will413028.hawkr.coupon.entity.CouponTemplateEntity;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.Collection;

public interface CouponTemplateMapper extends BaseMapper<CouponTemplateEntity> {

    /**
     * 買家視角列表查詢：在 SQL 層套用 §6.7.2 可見性規則 fragment，
     * 確保「過濾先於分頁」，避免 target_type=DESIGNATED/COMPENSATION 因分頁切片漏抓。
     *
     * <p>對應設計書 §6.7、§6.8、§6.9。
     *
     * <p>⚠ 同步警告：本方法的可見性條件與 {@code CouponGrpcService#isEligible} 邏輯為兩份等價實作，
     * 若調整 4 種 targetType 規則，必須兩處同步（設計書 §6.7.1.1 接受的技術債）。
     *
     * <p>{@code storeId} 參數是買家端篩選的目標商店 store_id surrogate（account_id 命名債
     * Phase 2 rename from {@code sellerAccountId}——舊參數名沿用命名但值本來就是
     * store_id-correct，本次只正名參數名，非身分來源 bug）。
     *
     * @param page             分頁參數
     * @param storeId          null=全平台券；有值=該賣家券 + 全平台券（applicable_store_id IS NULL）
     * @param buyerId          當前買家 buyer_id
     * @param buyerCreatedAt   買家註冊時間（UTC）— 用於 target_type=ALL 的時間比對
     * @param memberSellerIds  買家有完成交易紀錄的賣家集合 — 用於 target_type=MEMBER 比對；空集合請傳入哨兵值（如 [-1L]）以保證 IN 子句合法
     */
    @Select("""
        <script>
        SELECT * FROM coupon_templates t
        WHERE t.status = '1'
          AND t.start_at &lt;= NOW()
          AND t.end_at &gt;= NOW()
          <if test="storeId != null">
            AND (t.applicable_store_id = #{storeId} OR t.applicable_store_id IS NULL)
          </if>
          AND (
               (t.target_type = '1' AND t.created_at &gt;= #{buyerCreatedAt})
            OR (t.target_type = '2' AND t.applicable_store_id IN
                <foreach collection="memberSellerIds" item="sid" open="(" separator="," close=")">#{sid}</foreach>
               )
            OR (t.target_type IN ('3','4') AND EXISTS (
                  SELECT 1 FROM coupon_target_accounts cta
                  WHERE cta.template_id = t.template_id
                    AND cta.buyer_id = #{buyerId}
               ))
          )
        ORDER BY t.created_at DESC
        </script>
        """)
    IPage<CouponTemplateEntity> selectAvailableForBuyer(
            IPage<CouponTemplateEntity> page,
            @Param("storeId") Long storeId,
            @Param("buyerId") Long buyerId,
            @Param("buyerCreatedAt") LocalDateTime buyerCreatedAt,
            @Param("memberSellerIds") Collection<Long> memberSellerIds);

    /**
     * 防超發：原子遞增 issued_count
     * status value: 1=ACTIVE
     *
     * @return 影響列數（0=已達上限）
     */
    @Update("""
        UPDATE coupon_templates
        SET issued_count = issued_count + #{count}
        WHERE template_id = #{templateId}
        AND status = '1'
        AND (total_issue_limit = -1 OR issued_count + #{count} <= total_issue_limit)
        """)
    int incrementIssuedCount(@Param("templateId") Long templateId, @Param("count") int count);

    /**
     * 賣家收回模板
     * status value: 1=ACTIVE, 4=REVOKED
     *
     * <p>{@code issuer_store_id} 自 V6 起是 SELLER 專用欄位（ADMIN 模板該欄位恆為
     * null），{@code issuer_type} 過濾維持是因為這條 UPDATE 本來就只該影響賣家自建
     * 模板；呼叫端固定傳 {@code IssuerType.SELLER.value()}，見
     * {@link io.github.will413028.hawkr.coupon.service.CouponTemplateService#revokeTemplate}）。
     */
    @Update("""
        UPDATE coupon_templates
        SET status = '4', revoked_at = NOW(), revoked_reason = #{reason}
        WHERE template_id = #{templateId}
        AND issuer_store_id = #{storeId}
        AND issuer_type = #{issuerType}
        AND status = '1'
        """)
    int revokeTemplate(@Param("templateId") Long templateId,
                       @Param("storeId") Long storeId,
                       @Param("issuerType") String issuerType,
                       @Param("reason") String reason);

    /**
     * 自動過期：將超期的 ACTIVE/INACTIVE Template 標記為 EXPIRED
     * status value: 1=ACTIVE, 2=INACTIVE, 3=EXPIRED
     */
    @Update("""
        UPDATE coupon_templates
        SET status = '3'
        WHERE status IN ('1', '2')
        AND end_at < NOW()
        """)
    int expireTemplates();

    /**
     * 賣家停權連動停用（07 §5.20）：把該賣家自建的 ACTIVE 模板停用為 INACTIVE 並標記
     * {@code disabled_by_suspension=TRUE}。範圍僅限 {@code issuer_type='SELLER'}（admin 建立、
     * 掛該賣家 {@code applicable_store_id} 的模板不在此列——賣家自建模板才是「賣家的」）。
     *
     * <p>{@code issuer_store_id} 自 V6 起是 SELLER 專用欄位，不會再讀到 ADMIN 建立模板的值。
     *
     * <p>冪等：{@code WHERE status=activeStatus AND disabled_by_suspension=FALSE}，重跑挑 0 筆。
     */
    @Update("""
        UPDATE coupon_templates
        SET status = #{inactiveStatus}, disabled_by_suspension = TRUE
        WHERE issuer_type = #{sellerIssuerType}
          AND issuer_store_id = #{storeId}
          AND status = #{activeStatus}
          AND disabled_by_suspension = FALSE
        """)
    int disableActiveTemplatesBySuspension(
            @Param("storeId") Long storeId,
            @Param("sellerIssuerType") String sellerIssuerType,
            @Param("activeStatus") String activeStatus,
            @Param("inactiveStatus") String inactiveStatus);

    /**
     * 賣家解除停權精準還原（07 §5.20）：只還原 {@code disabled_by_suspension=TRUE} 的模板
     * （因本次停權而停用者），不誤還原賣家本來就自行停用 / 被 admin 停用的模板。
     *
     * <p>冪等：{@code WHERE disabled_by_suspension=TRUE}，重跑挑 0 筆。
     */
    @Update("""
        UPDATE coupon_templates
        SET status = #{activeStatus}, disabled_by_suspension = FALSE
        WHERE issuer_type = #{sellerIssuerType}
          AND issuer_store_id = #{storeId}
          AND status = #{inactiveStatus}
          AND disabled_by_suspension = TRUE
        """)
    int restoreTemplatesFromSuspension(
            @Param("storeId") Long storeId,
            @Param("sellerIssuerType") String sellerIssuerType,
            @Param("activeStatus") String activeStatus,
            @Param("inactiveStatus") String inactiveStatus);
}
