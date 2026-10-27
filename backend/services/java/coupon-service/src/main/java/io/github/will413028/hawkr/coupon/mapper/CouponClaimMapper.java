package io.github.will413028.hawkr.coupon.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.coupon.entity.CouponClaimEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;
import java.util.Set;

public interface CouponClaimMapper extends BaseMapper<CouponClaimEntity> {

    /**
     * 查詢買家已領取的所有 template_id
     */
    @Select("""
        SELECT template_id FROM coupon_claims
        WHERE buyer_id = #{buyerId}
        """)
    Set<Long> selectTemplateIdsByBuyerId(@Param("buyerId") Long buyerId);

    /**
     * 統計某模板的領取數量
     */
    @Select("""
        SELECT COUNT(*) FROM coupon_claims
        WHERE template_id = #{templateId}
        """)
    int countByTemplateId(@Param("templateId") Long templateId);

    /**
     * 新增領取紀錄，若已存在則忽略（冪等）
     *
     * @return 影響列數（1=新增成功, 0=已存在）
     */
    @Insert("""
        INSERT INTO coupon_claims (template_id, buyer_id, claimed_at)
        VALUES (#{templateId}, #{buyerId}, NOW())
        ON CONFLICT (template_id, buyer_id) DO NOTHING
        """)
    int insertIgnore(@Param("templateId") Long templateId, @Param("buyerId") Long buyerId);

    /**
     * 批量新增領取紀錄，已存在則忽略
     *
     * @return 實際新增的列數
     */
    @Insert("""
        <script>
        INSERT INTO coupon_claims (template_id, buyer_id, claimed_at)
        VALUES
        <foreach collection="buyerIds" item="buyerId" separator=",">
            (#{templateId}, #{buyerId}, NOW())
        </foreach>
        ON CONFLICT (template_id, buyer_id) DO NOTHING
        </script>
        """)
    int batchInsertIgnore(@Param("templateId") Long templateId, @Param("buyerIds") List<Long> buyerIds);
}
