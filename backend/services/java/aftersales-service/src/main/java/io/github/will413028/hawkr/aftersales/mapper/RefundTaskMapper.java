package io.github.will413028.hawkr.aftersales.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.aftersales.entity.RefundTaskEntity;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDate;

/**
 * refund_tasks 自訂 SQL。
 *
 * <p>INV-3 對齊（09 §2.2 / 06-Shipping §2.4）：
 * 所有 transition 必須走附帶 WHERE 前置狀態的 conditional update，禁止 updateById 全欄位覆蓋。
 */
@Mapper
public interface RefundTaskMapper extends BaseMapper<RefundTaskEntity> {

    /**
     * ATM 同步 SUCCESS：transition AWAITING_BUYER_REFUND_PAGE(5) → AWAITING_WEBHOOK(15)
     * 同步寫入 estimated_refund_date + payuni_status，清空 payuni_message。
     * 09 §2.2 對應路徑：「同步 SUCCESS → AWAITING_WEBHOOK + 寫入 estimated_refund_date」
     */
    @Update("""
        UPDATE refund_tasks
        SET status = '15',
            payuni_status = #{payuniStatus},
            payuni_message = NULL,
            estimated_refund_date = #{estimatedRefundDate},
            updated_at = NOW()
        WHERE id = #{id}
          AND status = '5'
        """)
    int markAtmRefundPageSucceeded(
            @Param("id") Long id,
            @Param("payuniStatus") String payuniStatus,
            @Param("estimatedRefundDate") LocalDate estimatedRefundDate
    );

    /**
     * ATM 同步失敗：snapshot payuni_status / payuni_message，state 保持 AWAITING_BUYER_REFUND_PAGE(5)
     * 09 §2.2 對應路徑：「同步 錯誤碼 → 保持 5 + 只更新 snapshot」
     */
    @Update("""
        UPDATE refund_tasks
        SET payuni_status = #{payuniStatus},
            payuni_message = #{payuniMessage},
            updated_at = NOW()
        WHERE id = #{id}
          AND status = '5'
        """)
    int stampAtmRefundPageError(
            @Param("id") Long id,
            @Param("payuniStatus") String payuniStatus,
            @Param("payuniMessage") String payuniMessage
    );

    /**
     * 每次買家點「前往退款」覆寫 client_type / app_type；
     * 買家可能在不同次嘗試切換 App / Web，最後一次點擊決定 PayUni 回調的 redirect_url 出口。
     */
    @Update("""
        UPDATE refund_tasks
        SET client_type = #{clientType},
            app_type = #{appType},
            updated_at = NOW()
        WHERE id = #{id}
        """)
    int markRefundPageClient(
            @Param("id") Long id,
            @Param("clientType") String clientType,
            @Param("appType") String appType
    );

    /**
     * §2.2 業務 pivot (2026-05-14)：ATM 退款任何失敗來源（sync return / 合規 async webhook RefundStatus=2 /
     * UNKNOWN 變體）統一退回 AWAITING_BUYER_REFUND_PAGE(5)。
     *
     * <p>原子操作（含 attempt++）：
     * <ul>
     *   <li>當 status=5（從未真正同步成功過，例如 sync 失敗）：stamp message + attempt++</li>
     *   <li>當 status=15（受理後 async 匯款失敗）：退回 5 + stamp message + attempt++</li>
     * </ul>
     *
     * <p>WHERE 樂觀鎖：{@code status IN ('5','15')}，其他狀態（DONE/CANCELLED/...）不動。
     *
     * <p>清空 {@code estimated_refund_date}：避免下次 buyer 重填成功時 markAtmRefundPageSucceeded 寫入新日期前
     * 仍殘留上次的舊日期被前端誤讀。</p>
     */
    @Update("""
        UPDATE refund_tasks
        SET status = '5',
            payuni_status = #{payuniStatus},
            payuni_message = #{payuniMessage},
            estimated_refund_date = NULL,
            current_attempt = current_attempt + 1,
            updated_at = NOW()
        WHERE id = #{id}
          AND status IN ('5', '15')
        """)
    int revertFromAnyAwaitingToAwaitingPage(
            @Param("id") Long id,
            @Param("payuniStatus") String payuniStatus,
            @Param("payuniMessage") String payuniMessage
    );

    /**
     * §2.2 業務 pivot (2026-05-14)：UnknownRefundFallbackConsumer 用 MerID 找最近一筆等待中 refund_task。
     *
     * <p>查 status IN (5, 15) 兩種等待狀態，依 updated_at DESC 取最近一筆。
     * 不設時間窗：ATM 匯款異步 webhook 可能延遲多日，限制時間窗反而漏綁。</p>
     */
    @Select("""
        SELECT * FROM refund_tasks
        WHERE mer_id = #{merId}
          AND status IN ('5', '15')
        ORDER BY updated_at DESC
        LIMIT 1
        """)
    RefundTaskEntity findLatestAwaitingByMerId(@Param("merId") String merId);

    /**
     * issue #1288 對帳推進：payment 端該退款已 SUCCEEDED → 等待中 task（5/15）推進 DONE(20)。
     * <p>WHERE status IN ('5','15')：其他狀態（DONE/CANCELLED/40...）不動，天然冪等。
     * 由 caller 於同一 tx 內接著 close case + 發 aftersales.refund.completed（等同收到成功）。
     */
    @Update("""
        UPDATE refund_tasks
        SET status = '20',
            payuni_status = #{payuniStatus},
            payuni_message = #{payuniMessage},
            updated_at = NOW()
        WHERE id = #{id}
          AND status IN ('5', '15')
        """)
    int advanceAnyAwaitingToDone(
            @Param("id") Long id,
            @Param("payuniStatus") String payuniStatus,
            @Param("payuniMessage") String payuniMessage
    );

    /**
     * issue #1288 對帳推進：payment 端已 PROCESSING（受理待 webhook）→ task 推進 AWAITING_WEBHOOK(15)。
     * <p>WHERE status = '5'：已在 15 者不動（天然冪等），其他終態不動。
     */
    @Update("""
        UPDATE refund_tasks
        SET status = '15',
            payuni_status = #{payuniStatus},
            payuni_message = #{payuniMessage},
            updated_at = NOW()
        WHERE id = #{id}
          AND status = '5'
        """)
    int advanceAwaitingPageToAwaitingWebhook(
            @Param("id") Long id,
            @Param("payuniStatus") String payuniStatus,
            @Param("payuniMessage") String payuniMessage
    );

    /**
     * issue #1288 對帳無法判定：等待中 task（5/15）轉 MANUAL_INTERVENTION(40) 打斷重填迴圈，等維運處理。
     * <p>WHERE status IN ('5','15')：其他狀態不動。
     */
    @Update("""
        UPDATE refund_tasks
        SET status = '40',
            payuni_status = #{payuniStatus},
            payuni_message = #{payuniMessage},
            updated_at = NOW()
        WHERE id = #{id}
          AND status IN ('5', '15')
        """)
    int markManualIntervention(
            @Param("id") Long id,
            @Param("payuniStatus") String payuniStatus,
            @Param("payuniMessage") String payuniMessage
    );

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
