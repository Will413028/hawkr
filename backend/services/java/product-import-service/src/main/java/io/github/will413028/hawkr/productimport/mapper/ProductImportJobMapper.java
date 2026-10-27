package io.github.will413028.hawkr.productimport.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.productimport.entity.ProductImportJobEntity;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * product_import_job mapper。
 */
public interface ProductImportJobMapper extends BaseMapper<ProductImportJobEntity> {

    /**
     * 24h 內同 account + hash 的 dedup 查詢（最新一筆）。
     *
     * <p>40 §5.1.3.5：dedup 限 active 集合（CREATED 9101 / PARSING 9102 / READY 9104 /
     * COMMITTING 9105 / CSV_INVALID 9111）。{@code CANCELLED / EXPIRED / COMMITTED / PARSE_FAILED}
     * 視為已結束，賣家可重新上傳同份 CSV 跑新一輪（COMMITTED 代表 csv 完成使命；
     * CANCELLED / EXPIRED / PARSE_FAILED 已死亡無需 dedup）。
     */
    @Select("""
            SELECT * FROM product_import_job
            WHERE store_id = #{storeId}
              AND source_file_hash = #{sourceFileHash}
              AND created_at >= #{since}
              AND status IN ('9101','9102','9104','9105','9111')
            ORDER BY created_at DESC LIMIT 1
            """)
    ProductImportJobEntity findRecentByHash(@Param("storeId") Long storeId,
                                            @Param("sourceFileHash") String sourceFileHash,
                                            @Param("since") LocalDateTime since);

    /** 賣家 active 任務數（active set：9101/9102/9104/9105/9111）。 */
    @Select("""
            SELECT COUNT(*) FROM product_import_job
            WHERE store_id = #{storeId}
              AND status IN ('9101','9102','9104','9105','9111')
            """)
    int countActiveByAccount(@Param("storeId") Long storeId);

    /** active 集合內同 filename 是否衝突。 */
    @Select("""
            SELECT COUNT(*) FROM product_import_job
            WHERE store_id = #{storeId}
              AND source_filename = #{filename}
              AND status IN ('9101','9102','9104','9105','9111')
            """)
    int countActiveByFilename(@Param("storeId") Long storeId,
                              @Param("filename") String filename);

    /**
     * 賣家當日（UTC CURRENT_DATE 之後）累積已建 job 數，含已 cancel / committed / failed —
     * 對應 §5.1 step 3.1 max-daily 限制（防 DoS / cost overrun）。
     */
    @Select("""
            SELECT COUNT(*) FROM product_import_job
            WHERE store_id = #{storeId}
              AND created_at >= CURRENT_DATE
            """)
    int countCreatedTodayByAccount(@Param("storeId") Long storeId);

    /**
     * GC 用：掃過期 job。
     *
     * <p>issue #2294 L9：加上 {@code gc_lease_until} 租約述詞——租約未到期的列由別的 pod 持有，
     * 本輪不撈。
     */
    @Select("""
            SELECT * FROM product_import_job
            WHERE expires_at < #{now}
              AND status NOT IN ('9106','9107','9108','9103')
              AND (gc_lease_until IS NULL OR gc_lease_until <= #{now})
            ORDER BY expires_at ASC LIMIT #{limit}
            """)
    List<ProductImportJobEntity> findExpiredCandidates(@Param("now") LocalDateTime now,
                                                       @Param("limit") int limit);

    /**
     * issue #2294 L9：GC tierA 的 <b>lease claim</b>。
     *
     * <p>tierA 的副作用有三個（job 轉 EXPIRED、item 批次轉 REMOVED、發 {@code gc.expire_job} MQ），
     * 而<b>把該列推出候選集合的原本是 {@code status} 轉終態</b>——那是業務狀態轉換本身，
     * 拿它當閘門就是永久旗標：claim 之後 pod 崩潰（不是拋例外，是進程消失）⇒ 該列再也回不到
     * 候選集合 ⇒ {@code expire_job} 事件永久遺失且無告警。所以在它前面補一道會過期的租約。
     *
     * <p>⚠️ {@code expires_at} <b>不可</b>挪用：它是業務時戳（「這個匯入任務何時失效」），
     * 推它等於延長任務壽命。
     */
    @Update("""
            UPDATE product_import_job
            SET gc_lease_until = #{leaseUntil}
            WHERE id = #{id}
              AND expires_at < #{now}
              AND status NOT IN ('9106','9107','9108','9103')
              AND (gc_lease_until IS NULL OR gc_lease_until <= #{now})
            """)
    int claimExpiredJobForGc(@Param("id") Long id,
                             @Param("now") LocalDateTime now,
                             @Param("leaseUntil") LocalDateTime leaseUntil);
}
