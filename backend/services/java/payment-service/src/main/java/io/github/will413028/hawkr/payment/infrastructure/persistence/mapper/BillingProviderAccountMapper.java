package io.github.will413028.hawkr.payment.infrastructure.persistence.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.BillingProviderAccountEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

@Mapper
public interface BillingProviderAccountMapper extends BaseMapper<BillingProviderAccountEntity> {

    @Select("SELECT * FROM billing_provider_accounts WHERE account_ref = #{accountRef}")
    BillingProviderAccountEntity findByAccountRef(@Param("accountRef") String accountRef);
}
