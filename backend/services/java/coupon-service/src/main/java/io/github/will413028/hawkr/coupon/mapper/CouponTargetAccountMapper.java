package io.github.will413028.hawkr.coupon.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.coupon.entity.CouponTargetAccountEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;

public interface CouponTargetAccountMapper extends BaseMapper<CouponTargetAccountEntity> {

    /**
     * 查詢某個買家被哪些 Template 指定
     */
    @Select("""
        SELECT template_id FROM coupon_target_accounts
        WHERE buyer_id = #{buyerId}
        """)
    List<Long> selectTemplateIdsByBuyerId(@Param("buyerId") Long buyerId);

    /**
     * 查詢某個 Template 指定的所有買家 buyer_id
     */
    @Select("""
        SELECT buyer_id FROM coupon_target_accounts
        WHERE template_id = #{templateId}
        """)
    List<Long> selectBuyerIdsByTemplateId(@Param("templateId") Long templateId);

    /**
     * 檢查某個買家是否被某個 Template 指定
     */
    @Select("""
        SELECT COUNT(*) FROM coupon_target_accounts
        WHERE template_id = #{templateId} AND buyer_id = #{buyerId}
        """)
    int countByTemplateAndBuyer(@Param("templateId") Long templateId, @Param("buyerId") Long buyerId);

    /**
     * 插入指定買家（重複忽略）
     */
    @Insert("""
        INSERT INTO coupon_target_accounts (template_id, buyer_id, created_at)
        VALUES (#{templateId}, #{buyerId}, NOW())
        ON CONFLICT (template_id, buyer_id) DO NOTHING
        """)
    int insertIgnore(@Param("templateId") Long templateId, @Param("buyerId") Long buyerId);
}
