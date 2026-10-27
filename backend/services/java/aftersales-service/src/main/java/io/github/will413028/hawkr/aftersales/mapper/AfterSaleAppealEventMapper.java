package io.github.will413028.hawkr.aftersales.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleAppealEventEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

import java.util.List;

@Mapper
public interface AfterSaleAppealEventMapper extends BaseMapper<AfterSaleAppealEventEntity> {

    @Select("""
            SELECT * FROM aftersale_appeal_events
             WHERE case_no = #{caseNo}
             ORDER BY created_at ASC
            """)
    List<AfterSaleAppealEventEntity> selectByCaseNoOrderByCreatedAt(@Param("caseNo") String caseNo);
}
