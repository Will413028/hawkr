package io.github.will413028.hawkr.aftersales.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Update;

/**
 * refund_tasks 自訂 SQL。
 *
 * <p>INV-3 對齊（09 §2.2 / 06-Shipping §2.4）：
 * 所有 transition 必須走附帶 WHERE 前置狀態的 conditional update，禁止 updateById 全欄位覆蓋。
 */
@Mapper
public interface RefundTaskMapper extends BaseMapper<RefundTaskEntity> {

    /**
     * §8.4 issue #2294 L5：{@code RefundSyncWorker}（補發遺漏 MQ 通知）的 <b>lease claim</b>。
     *
     * <p>把 {@code resync_at} 推到 {@code leaseUntil}（= now + 3 小時 = 排程週期 × 3）：
     * affected=1 表示本 pod 搶到這一輪的補發權；affected=0 表示另一個 pod 已搶走且租約未到期。
     *
     * <p>⚠️ 這支是 lease 不是「已補發旗標」：租約到期後該列仍會被重新撈到並再次補發，
     * 那是刻意的（本 worker 的職責就是無條件重發，去重交給下游冪等
     * {@code financial_log (type, external_txn_id)} UNIQUE）。
     * <p>⚠️ 刻意<b>不寫</b> {@code updated_at}：本 worker 的候選述詞是
     * {@code updated_at < NOW()-1h}，寫它會讓該列立刻退出候選集合——那會把 lease 變成永久旗標。
     * 這也是為什麼租約欄不能直接挪用 {@code updated_at}。
     */
    @Update("""
        UPDATE refund_tasks
        SET resync_at = #{leaseUntil}
        WHERE id = #{id}
          AND status = #{status}
          AND (resync_at IS NULL OR resync_at <= #{now})
        """)
    int claimForResync(
            @Param("id") Long id,
            @Param("status") String status,
            @Param("now") java.time.LocalDateTime now,
            @Param("leaseUntil") java.time.LocalDateTime leaseUntil
    );
}
