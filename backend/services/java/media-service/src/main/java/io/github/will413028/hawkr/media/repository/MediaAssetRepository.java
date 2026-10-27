package io.github.will413028.hawkr.media.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.media.entity.MediaAssetEntity;
import org.apache.ibatis.annotations.Delete;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 資源資產 Repository
 */
@Mapper
public interface MediaAssetRepository extends BaseMapper<MediaAssetEntity> {
    /**
     * 法定保存的引用類型：GC <b>永不</b>觸碰（設計書 58 §0 表「憑證與稽核保存 1826 天（5 年）」、
     * G6「與兩個既有自動刪除機制衝突（媒體 GC 7 天、帳號 PII T+30）⇒ <b>必須例外化</b>」）。
     *
     * <p>🔴 <b>在此之前這個例外不存在，發票 PDF 只是「剛好有 asset_reference」才沒被刪掉</b>
     * （2026-09-02 實查 dev 10 份全部 refs=1）。那是<b>偶然的保護不是結構的保護</b>：
     * {@code ReleaseReferences} / {@code UnlinkImages} 是對外開放的 gRPC，任何呼叫端帶著
     * {@code ref_type=7120} 進來就會把那筆引用拿掉 ⇒ 7 天後這裡把 R2 物件實體刪除。
     *
     * <p>而後果<b>不可回復</b>：AMEGO 的 PDF 只留 180 天（58 §8.1.5 P8），過了那條線
     * 我方是唯一持有者。刪除全程無錯誤訊息，症狀是幾個月後某位買家點下載拿到 404。
     *
     * <p>清單只放<b>法規要求長期保存</b>的類型，不是「重要的都放進來」——
     * 放寬它等於讓真正的孤兒永遠留在 R2。目前兩個：
     * <ul>
     *   <li>{@code 7120} INVOICE_DOCUMENT：電子發票／收據／折讓單 PDF</li>
     *   <li>{@code 7111} CERTIFICATION：商家認證與稅籍證明文件（爭議舉證用）</li>
     * </ul>
     */
    String STATUTORY_GC_EXCLUSION =
        " AND (ma.ref_type IS NULL OR ma.ref_type NOT IN ('7120','7111')) ";


    /**
     * 依 ID 列表批次查詢資產
     */
    @Select("""
        <script>
        SELECT * FROM media_asset WHERE id IN
        <foreach item="id" collection="ids" open="(" separator="," close=")">
            #{id}
        </foreach>
        </script>
        """)
    List<MediaAssetEntity> selectByIds(@Param("ids") List<Long> ids);

    /**
     * 統計指向同一個 object key 的資產列數（轉正失敗補償刪除前的安全檢查）。
     *
     * <p>{@code media_asset.object_key} 有唯一索引（V7），正常情況回 0 或 1；
     * 回 1 代表該物件已被某一列認領，補償刪除必須跳過它。</p>
     */
    @Select("SELECT count(*) FROM media_asset WHERE object_key = #{objectKey}")
    int countByObjectKey(@Param("objectKey") String objectKey);

    /**
     * 依 object key 查詢資產。
     *
     * <p>服務端讀取機敏資產（{@code ServerReadSecureAsset}）的入口：呼叫端業務表存的是
     * {@code object_key} 而不是 {@code asset_id}，所以查詢鍵是它。{@code media_asset.object_key}
     * 有唯一索引（V7），最多一列。
     */
    @Select("SELECT * FROM media_asset WHERE object_key = #{objectKey}")
    MediaAssetEntity selectByObjectKey(@Param("objectKey") String objectKey);

    /**
     * 查詢無引用且建立超過指定天數的資產（用於 GC）
     *
     * <p>issue #2294 L8：加上 {@code gc_lease_until} 租約述詞——租約未到期的列由別的 pod 持有，
     * 本輪不撈。這同時是 {@code cleanOrphanedAssets} 那個 do/while 迴圈的<b>終止條件</b>。
     *
     * <p><b>R4-P1-2（2026-08-08）：{@code ORDER BY} 不是可有可無的排版</b>。沒有它時
     * PostgreSQL 的 {@code synchronize_seqscans} 會讓並行的 seq scan <b>從不同位置起掃</b>，
     * 兩個 pod 拿到的候選順序因此可以相反 ⇒ 取鎖順序相反 ⇒ <b>deadlock</b>
     * （兩 pod 的 GC cron 同為每日 03:00，碰撞是必然）。有序取鎖之後，落後的 pod 只會在
     * 同一個位置等待，等待圖不成環。{@code (created_at, gc_lease_until)} 索引（V8）
     * 直接吃 {@code created_at ASC} 這個排序前綴；{@code id} 只是打破同秒併列的 tie-break。
     */
    @Select("""
        SELECT ma.* FROM media_asset ma
        LEFT JOIN asset_reference ar ON ma.id = ar.asset_id
        WHERE ar.id IS NULL
          AND ma.created_at < #{threshold}
          AND (ma.gc_lease_until IS NULL OR ma.gc_lease_until <= #{now})
        """ + STATUTORY_GC_EXCLUSION + """
        ORDER BY ma.created_at ASC, ma.id ASC
        LIMIT #{limit}
        """)
    List<MediaAssetEntity> selectOrphanedAssets(
        @Param("threshold") LocalDateTime threshold,
        @Param("now") LocalDateTime now,
        @Param("limit") int limit
    );

    /**
     * issue #2294 L8：孤兒資產 GC 的 <b>lease claim</b>——搶到才可以刪 R2/OSS 物件。
     *
     * <p>副作用是<b>外部儲存刪除（不可逆）</b>，所以閘門必須在 {@code deleteObject} 之前。
     * affected=0 表示另一個 pod 已搶走且租約未到期。
     *
     * <p><b>R4-P2-1（2026-08-08）：{@code NOT EXISTS asset_reference} 是這裡的 TOCTOU 關閉點。</b>
     * 候選查詢的「無引用」條件靠 {@code LEFT JOIN asset_reference … WHERE ar.id IS NULL} 成立，
     * 但 SELECT 與 claim 之間有一段真實的時間差（同批前幾筆的 OSS DELETE），
     * 期間若有人替這個 asset 建了 reference（商品綁圖／匯入轉正），claim 少了這個條件就會照樣搶到，
     * 然後把一個<b>已被引用</b>的物件從儲存端刪掉——不可逆，且 DB 列也一併刪除。
     * claim 是這條路徑上<b>最後一個能重新驗證候選條件</b>的位置，候選述詞的每一個條件都要在這裡重述。
     * <p>用 {@code NOT EXISTS} 而非 {@code LEFT JOIN}：UPDATE 沒有 join 語法，且
     * {@code NOT EXISTS} 走 {@code asset_reference(asset_id)} 索引後可以 semi-join 早停。
     */
    @Update("""
        UPDATE media_asset ma
        SET gc_lease_until = #{leaseUntil}
        WHERE ma.id = #{id}
          AND ma.created_at < #{threshold}
          AND (ma.gc_lease_until IS NULL OR ma.gc_lease_until <= #{now})
          AND NOT EXISTS (
            SELECT 1 FROM asset_reference ar WHERE ar.asset_id = ma.id
          )
        """ + STATUTORY_GC_EXCLUSION + """
        """)
    int claimAssetForGc(
        @Param("id") Long id,
        @Param("threshold") LocalDateTime threshold,
        @Param("now") LocalDateTime now,
        @Param("leaseUntil") LocalDateTime leaseUntil
    );

    /**
     * 批次刪除資產
     */
    @Delete("""
        <script>
        DELETE FROM media_asset WHERE id IN
        <foreach item="id" collection="ids" open="(" separator="," close=")">
            #{id}
        </foreach>
        </script>
        """)
    int deleteByIds(@Param("ids") List<Long> ids);
}
