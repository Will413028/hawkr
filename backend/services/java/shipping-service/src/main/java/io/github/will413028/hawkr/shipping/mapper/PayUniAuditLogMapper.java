package io.github.will413028.hawkr.shipping.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.shipping.entity.PayUniAuditLogEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;

import java.time.LocalDateTime;

/**
 * PayUni 互動 audit Mapper（payuni_audit_logs 專表）。
 * <p>
 * append-only：只 INSERT + 保留期清理 DELETE，無 UPDATE。
 */
@Mapper
public interface PayUniAuditLogMapper extends BaseMapper<PayUniAuditLogEntity> {

    /**
     * 刪除指定時間前的稽核記錄（保留期清理排程用）。
     */
    @Delete("DELETE FROM payuni_audit_logs WHERE created_at < #{before}")
    int deleteBefore(@Param("before") LocalDateTime before);
}
