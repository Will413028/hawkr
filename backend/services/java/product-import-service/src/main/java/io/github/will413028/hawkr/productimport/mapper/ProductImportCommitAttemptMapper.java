package io.github.will413028.hawkr.productimport.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.productimport.entity.ProductImportCommitAttemptEntity;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;

/**
 * product_import_commit_attempt mapper。
 */
public interface ProductImportCommitAttemptMapper extends BaseMapper<ProductImportCommitAttemptEntity> {

    /** 取最新一次 attempt（用於 GetImportItem 顯示）。 */
    @Select("""
            SELECT * FROM product_import_commit_attempt
            WHERE import_item_id = #{importItemId}
            ORDER BY started_at DESC LIMIT 1
            """)
    ProductImportCommitAttemptEntity findLatestByItem(@Param("importItemId") Long importItemId);
}
