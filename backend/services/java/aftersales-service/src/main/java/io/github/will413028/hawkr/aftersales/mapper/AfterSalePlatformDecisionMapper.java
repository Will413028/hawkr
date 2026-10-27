package io.github.will413028.hawkr.aftersales.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSalePlatformDecisionEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

@Mapper
public interface AfterSalePlatformDecisionMapper extends BaseMapper<AfterSalePlatformDecisionEntity> {

    @Select("SELECT * FROM aftersale_platform_decisions WHERE case_no = #{caseNo}")
    AfterSalePlatformDecisionEntity selectByCaseNo(@Param("caseNo") String caseNo);
}
