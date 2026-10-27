package io.github.will413028.hawkr.aftersales.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleResolutionEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

@Mapper
public interface AfterSaleResolutionMapper extends BaseMapper<AfterSaleResolutionEntity> {

    /**
     * §10.6.6 取最新一筆 REJECT resolution（供 Admin 詳情 API 顯示賣家最後一次拒絕理由）。
     * 取最新一筆（防禦性：reopen 已於 #1228 淘汰，單案理論上至多一筆 REJECT）。
     *
     * @param rejectValue AfterSaleResolutionType.REJECT.value()（硬編 enum 名稱避免 mapper 層依賴 common-core）
     * @return 最新 REJECT resolution；若無則 null
     */
    @Select("""
            SELECT * FROM aftersale_resolutions
             WHERE case_no = #{caseNo}
               AND resolution_type = #{rejectValue}
             ORDER BY created_at DESC
             LIMIT 1
            """)
    AfterSaleResolutionEntity selectLatestRejectByCaseNo(@Param("caseNo") String caseNo,
                                                         @Param("rejectValue") String rejectValue);
}
