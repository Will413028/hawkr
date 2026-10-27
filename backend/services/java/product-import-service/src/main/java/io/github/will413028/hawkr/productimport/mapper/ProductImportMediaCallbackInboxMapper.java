package io.github.will413028.hawkr.productimport.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.productimport.entity.ProductImportMediaCallbackInboxEntity;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Param;

/**
 * product_import_media_callback_inbox mapper（dedup inbox）。
 */
public interface ProductImportMediaCallbackInboxMapper extends BaseMapper<ProductImportMediaCallbackInboxEntity> {

    /**
     * INSERT ON CONFLICT DO NOTHING — 影響筆數 0 表示重複事件。
     * 由呼叫端判斷返回值是否為 1（首次）以決定是否處理 callback。
     */
    @Insert("""
            INSERT INTO product_import_media_callback_inbox
                (event_id, import_asset_id, result_status, processed_at)
            VALUES (#{eventId}, #{importAssetId}, #{resultStatus}, NOW())
            ON CONFLICT (event_id) DO NOTHING
            """)
    int insertIfAbsent(@Param("eventId") String eventId,
                       @Param("importAssetId") String importAssetId,
                       @Param("resultStatus") String resultStatus);
}
