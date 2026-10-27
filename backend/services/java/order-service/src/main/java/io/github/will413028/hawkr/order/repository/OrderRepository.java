package io.github.will413028.hawkr.order.repository;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.OrderStatus;
import io.github.will413028.hawkr.common.utils.code.ReceiptStatus;
import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.BuyerMerchantSummaryResult;
import io.github.will413028.hawkr.order.entity.BuyerSellerOrderResult;
import io.github.will413028.hawkr.order.entity.OrderAggregateResult;
import io.github.will413028.hawkr.order.entity.OrderEntity;
import io.github.will413028.hawkr.order.entity.OrderStatusCountResult;
import io.github.will413028.hawkr.order.entity.ReviewSnapshot;
import io.github.will413028.hawkr.order.entity.SellerOrderQuery;
import io.github.will413028.hawkr.order.entity.SellerRevenueResult;
import io.github.will413028.hawkr.order.entity.StoreOrderStatsResult;
import io.github.will413028.hawkr.order.entity.StorePeriodOrderStatsResult;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDateTime;
import java.util.List;

/**
 * 訂單主表 Mapper
 *
 * <p><b>取多列的查詢一律以唯一鍵收尾排序</b>：{@code created_at} 等時間欄不唯一（同秒下單、
 * 同一交易內建立的多列時間戳完全相同），只靠它排序在 PostgreSQL 上仍然沒有全序，而
 * {@code LIMIT/OFFSET} 分頁是逐頁重新執行的——相鄰兩頁對同一批同鍵值的列各自挑一種順序，
 * 就會有訂單在兩頁重複出現、也會有訂單一頁都不出現。收尾鍵取主鍵 {@code id}（TSID，時間有序，
 * 與 {@code created_at} 同向）；{@code GROUP BY} 的查詢取不到 {@code id}，改以分組鍵收尾。
 */
public interface OrderRepository extends BaseMapper<OrderEntity> {

    @Select("SELECT * FROM orders WHERE order_no = #{orderNo}")
    OrderEntity findByOrderNo(@Param("orderNo") String orderNo);

    @Select("SELECT * FROM orders WHERE order_no = #{orderNo} FOR UPDATE")
    OrderEntity findByOrderNoForUpdate(@Param("orderNo") String orderNo);

    /** 取賣家 store_id（[B-207] #1181：seller workspace realtime emit 用，scalar projection 非 Entity） */
    @Select("SELECT store_id FROM orders WHERE order_no = #{orderNo}")
    Long findStoreIdByOrderNo(@Param("orderNo") String orderNo);

    /**
     * 取買家 buyer_id（order.delivered 的 notify_buyer_account_id 解析用，scalar projection 非 Entity）。
     * 一律查自己 order 記錄取得，不採信上游 shipping-service shipment.delivered payload 同名欄位
     * （跨服務身份欄位不可信；鏡像上方 findStoreIdByOrderNo）。
     */
    @Select("SELECT buyer_id FROM orders WHERE order_no = #{orderNo}")
    Long findBuyerIdByOrderNo(@Param("orderNo") String orderNo);

    /**
     * Backstop（03 §8.11）：付款後逾寬限期仍無物流單的待出貨訂單。
     * status='10' = OrderStatus.PAID；shipment_no 空 ⟺ 尚未建立物流單。
     */
    @Select("""
            SELECT * FROM orders
            WHERE status = '10'
              AND (shipment_no IS NULL OR shipment_no = '')
              AND pay_time < #{graceCutoff}
              AND (shipment_create_next_retry_at IS NULL OR shipment_create_next_retry_at <= #{now})
            ORDER BY pay_time ASC, id ASC
            LIMIT #{limit}
            """)
    List<OrderEntity> findPaidOrdersWithoutShipmentPastGrace(
            @Param("now") LocalDateTime now,
            @Param("graceCutoff") LocalDateTime graceCutoff,
            @Param("limit") int limit);

    /**
     * Backstop（03 §8.11）：<b>搶佔</b>一筆待重試建單的訂單，同時累加重試次數並推進下次重試時間。
     *
     * <p><b>issue #2294 H4 缺陷 A</b>：原述詞是 {@code WHERE order_no=?} 純 PK 更新 ⇒
     * 兩個 pod <b>各得 affected=1</b>、各自 publish 一次 {@code shipment.create.requested}，
     * 且 {@code attempts} 一次 tick 跳 +2（重試預算被腰斬）。
     *
     * <p>改為 <b>lease on {@code shipment_create_next_retry_at}</b>：述詞就是候選 SQL
     * （{@link #findPaidOrdersWithoutShipmentPastGrace}）的第 4 條，SET 的也是同一欄
     * ⇒ claim 成功即暫時離開候選集合，{@code backoffMinutes} 到期自動回來
     * （《多副本排程搶佔規範》§二 六步 → lease）。
     *
     * <p>另加 {@code status = PAID} 守住業務前提（訂單已離開 PAID 就不該再重試建單）。
     *
     * <p><b>為什麼不用 {@code AND shipment_create_attempts = #{expectedAttempts}}</b>：
     * 兩者都擋得住第二個 pod，但 lease 與其餘命中同形、且能表達「租約到期自動回來」；
     * 計數器 CAS 的讀值來自 SELECT 快照，多一次 ABA 推理。
     *
     * @return 1 = 搶到（呼叫端才可以 publish）；0 = 已被另一個 pod 認領或訂單已離開 PAID
     */
    default int bumpShipmentCreateAttempt(String orderNo, LocalDateTime nextRetryAt, LocalDateTime now) {
        return bumpShipmentCreateAttemptByStatus(orderNo, nextRetryAt, now, OrderStatus.PAID.value());
    }

    @Update("""
            UPDATE orders
            SET shipment_create_attempts = shipment_create_attempts + 1,
                shipment_create_next_retry_at = #{nextRetryAt},
                updated_at = NOW()
            WHERE order_no = #{orderNo}
              AND status = #{paid}
              AND (shipment_create_next_retry_at IS NULL
                   OR shipment_create_next_retry_at <= #{now})
            """)
    int bumpShipmentCreateAttemptByStatus(
            @Param("orderNo") String orderNo,
            @Param("nextRetryAt") LocalDateTime nextRetryAt,
            @Param("now") LocalDateTime now,
            @Param("paid") String paid);

    /**
     * Backstop（03 §8.11）：<b>搶佔</b>一筆「重試上限已耗盡、要送取消+退款」的訂單。
     *
     * <p><b>issue #2294 H4 缺陷 B（單 pod 也壞，issue 原文未列）</b>：give-up 分支原本
     * <b>完全不寫任何狀態</b> ⇒ 候選 SQL 的四個條件全部維持成立 ⇒ 該列
     * <b>每 5 分鐘、每個 pod 各發一次「取消 + 退款」請求</b>，直到訂單狀態離開 PAID 為止。
     *
     * <p><b>為什麼 give-up 也走租約而不是永久旗標</b>（本案的核心取捨）：
     * <ul>
     *   <li>永久旗標版：publish 拋例外或 pod 在兩者之間崩潰 ⇒ 該訂單<b>永遠停在 PAID、
     *       永遠沒有物流單、永遠不會被取消退款，且沒有任何告警</b>——那正是本 issue 要修的
     *       靜默失效，只是換了位置。</li>
     *   <li>租約版<b>會自己終止</b>，不需要任何旗標：取消一旦生效，訂單 {@code status} 就離開
     *       {@code '10'}(PAID)，候選 SQL 第一個條件即不成立 ⇒ 該列自然退出。
     *       1 天的租約只在「取消沒生效」時才會再響一次——那正是我們想知道的事。</li>
     * </ul>
     *
     * <p>{@code shipment_backstop_given_up_at} 是<b>純觀測欄</b>（{@code COALESCE} 只寫第一次），
     * <b>不進候選述詞</b>；用途是給 {@code sql_exporter} 一條「{@code given_up_at IS NOT NULL}
     * 且 {@code status='10'}」的告警。
     *
     * @return 1 = 搶到（呼叫端才可以 publish）；0 = 已被另一個 pod 認領或訂單已離開 PAID
     */
    default int claimShipmentBackstopGiveUp(String orderNo, LocalDateTime giveUpRetryAt, LocalDateTime now) {
        return claimShipmentBackstopGiveUpByStatus(orderNo, giveUpRetryAt, now, OrderStatus.PAID.value());
    }

    @Update("""
            UPDATE orders
            SET shipment_create_next_retry_at = #{giveUpRetryAt},
                shipment_backstop_given_up_at = COALESCE(shipment_backstop_given_up_at, #{now}),
                updated_at = NOW()
            WHERE order_no = #{orderNo}
              AND status = #{paid}
              AND (shipment_create_next_retry_at IS NULL
                   OR shipment_create_next_retry_at <= #{now})
            """)
    int claimShipmentBackstopGiveUpByStatus(
            @Param("orderNo") String orderNo,
            @Param("giveUpRetryAt") LocalDateTime giveUpRetryAt,
            @Param("now") LocalDateTime now,
            @Param("paid") String paid);

    @Select("""
        <script>
        SELECT * FROM orders
        WHERE buyer_id = #{buyerId}
        <if test="statusList != null and !statusList.isEmpty()">
            AND status IN
            <foreach collection="statusList" item="s" open="(" separator="," close=")">#{s}</foreach>
        </if>
        ORDER BY created_at DESC, id DESC
        LIMIT #{size} OFFSET #{offset}
        </script>
    """)
    List<OrderEntity> findByUserIdAndStatus(
            @Param("buyerId") long buyerId,
            @Param("statusList") List<String> statusList,
            @Param("size") int size,
            @Param("offset") int offset
    );

    @Select("""
        <script>
        SELECT COUNT(*) FROM orders
        WHERE buyer_id = #{buyerId}
        <if test="statusList != null and !statusList.isEmpty()">
            AND status IN
            <foreach collection="statusList" item="s" open="(" separator="," close=")">#{s}</foreach>
        </if>
        </script>
    """)
    long countByUserIdAndStatus(@Param("buyerId") long buyerId, @Param("statusList") List<String> statusList);

    /**
     * 買家各 OrderStatus 原始筆數（GROUP BY status，筆數為 0 的狀態不回）
     * WHERE 條件必須與 countByUserIdAndStatus 完全一致（僅 buyer_id），SSOT 見 03 §2.1
     */
    @Select("""
        SELECT status, COUNT(*) AS count
        FROM orders
        WHERE buyer_id = #{buyerId}
        GROUP BY status
        """)
    List<OrderStatusCountResult> countMyOrdersGroupByStatus(@Param("buyerId") long buyerId);

    /**
     * 賣家訂單列表與筆數共用的 WHERE 片段（56 號設計書 §6.1.10）。
     * <p>
     * 分頁是手寫 LIMIT/OFFSET ＋ 獨立 COUNT，兩句 SQL 的條件一旦分岔，total 就會與 records
     * 對不上（前端翻頁看到空白頁，且不會報錯）。這裡用編譯期常數把片段共用出去，讓「兩邊一致」
     * 由結構保證而不是靠人眼比對；{@code OrderRepositorySellerFilterSqlTest} 另從渲染後的 SQL
     * 反向釘住這條不變式，防止之後有人把片段就地展開回其中一句。
     * <p>
     * ⚠️ <b>keyword 的比對範圍恰為 order_no、recipient_name，以及 order_items 的 product_name／
     * sku_name 四者，不得加入 recipient_phone 或完整 shipping_address。</b>
     * 判準是「列表投影看得到的才可以搜」：收件人姓名在賣家與代行的列表投影裡本來就未遮罩顯示
     * （代行投影只把地址遮到縣市區），搜尋它只是省去翻頁；而電話這支列表 RPC 從不回傳、
     * 地址也只給到縣市區 ⇒ 拿它們當搜尋條件等於造出一支 oracle——輸入一個值看有沒有命中，
     * 就能在完全不揭露該值的前提下確認它存在，而且不會留下 ORDER_PII_REVEAL 稽核紀錄
     * （56 號設計書 §8.1.5／A16）。
     * <p>
     * 訂單編號是 "O" + 9 位數字（TSID 取模，不含日期、不遞增），故只能 {@code ILIKE '%q%'}，
     * prefix 匹配沒有意義。品名走 order_items 半連接而非 JOIN，避免一單多品時列表出現重複列。
     */
    String SELLER_ORDER_FILTER = """
            WHERE store_id = #{q.storeId}
              AND buyer_id != #{q.storeId}
            <if test="q.buyerId != 0">
                AND buyer_id = #{q.buyerId}
            </if>
            <if test="q.statusList != null and !q.statusList.isEmpty()">
                AND status IN
                <foreach collection="q.statusList" item="s" open="(" separator="," close=")">#{s}</foreach>
            </if>
            <if test="q.keywordPattern != null">
                AND (
                    order_no ILIKE #{q.keywordPattern}
                    OR recipient_name ILIKE #{q.keywordPattern}
                    OR EXISTS (
                        SELECT 1 FROM order_items oi
                        WHERE oi.order_no = orders.order_no
                          AND (oi.product_name ILIKE #{q.keywordPattern}
                               OR oi.sku_name ILIKE #{q.keywordPattern})
                    )
                )
            </if>
            <if test="q.dateFrom != null">
                AND created_at &gt;= #{q.dateFrom}
            </if>
            <if test="q.dateTo != null">
                AND created_at &lt;= #{q.dateTo}
            </if>
            """;

    @Select("<script>SELECT * FROM orders " + SELLER_ORDER_FILTER
            + "ORDER BY created_at DESC, id DESC LIMIT #{size} OFFSET #{offset}</script>")
    List<OrderEntity> findSellerOrders(
            @Param("q") SellerOrderQuery query,
            @Param("size") int size,
            @Param("offset") int offset
    );

    @Select("<script>SELECT COUNT(*) FROM orders " + SELLER_ORDER_FILTER + "</script>")
    long countSellerOrders(@Param("q") SellerOrderQuery query);

    @Select("""
        SELECT * FROM orders
        WHERE (#{orderNo} = '' OR order_no LIKE CONCAT('%', #{orderNo}, '%'))
        AND (#{buyerId} = 0 OR buyer_id = #{buyerId})
        AND (#{storeId} = 0 OR store_id = #{storeId})
        AND (#{status} = '' OR status = #{status})
        ORDER BY created_at DESC, id DESC
        LIMIT #{size} OFFSET #{offset}
    """)
    List<OrderEntity> searchOrders(
            @Param("orderNo") String orderNo,
            @Param("buyerId") long buyerId,
            @Param("storeId") long storeId,
            @Param("status") String status,
            @Param("size") int size,
            @Param("offset") int offset
    );

    @Select("""
        SELECT COUNT(*) FROM orders
        WHERE (#{orderNo} = '' OR order_no LIKE CONCAT('%', #{orderNo}, '%'))
        AND (#{buyerId} = 0 OR buyer_id = #{buyerId})
        AND (#{storeId} = 0 OR store_id = #{storeId})
        AND (#{status} = '' OR status = #{status})
    """)
    long countSearchOrders(
            @Param("orderNo") String orderNo,
            @Param("buyerId") long buyerId,
            @Param("storeId") long storeId,
            @Param("status") String status
    );

    /**
     * 付款成功：原子更新狀態 + 付款資訊（status/transaction_id/pay_time/payment_method）
     * 僅當 status = PENDING_PAYMENT('0') 時才更新，避免 updateById 全欄位覆蓋問題
     */
    @Update("""
        UPDATE orders
        SET status = #{newStatus},
            transaction_id = #{transactionId},
            pay_time = #{payTime},
            payment_method = #{paymentMethod},
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND status = #{expectedOldStatus}
        """)
    int updatePaymentConfirmed(
            @Param("orderNo") String orderNo,
            @Param("expectedOldStatus") String expectedOldStatus,
            @Param("newStatus") String newStatus,
            @Param("transactionId") String transactionId,
            @Param("payTime") LocalDateTime payTime,
            @Param("paymentMethod") String paymentMethod
    );

    /**
     * 履約責任判定 financially final ＝ {@code BUYER_CAUSED} ⇒ {@code SHIPPING_ABNORMAL(35)}
     * → {@code DELIVERY_FAILED_NO_REFUND(54)}（03 §2.1「唯一入邊、執行者與下游讀者」）。
     *
     * <p>🔴 <b>為什麼要一支專屬 CAS，不能沿用 {@link #updateStatusIfMatchAny}</b>：
     * 那支的 allowed-from 清單由 {@code OrderShippingProgress.allowedFromFor} 產生，
     * 而 {@code 54} <b>刻意不在</b> 它的 {@code RANK} 表內（03 §2.1 逐字：{@code 54}
     * 不由物流貨態驅動，<b>不得</b>把它加進任何一列的 allowed-from）
     * ⇒ {@code allowedFromFor("54")} 回<b>空清單</b> ⇒ {@code <foreach>} 展開成
     * {@code status IN ()} ⇒ <b>SQL 語法錯誤</b>。
     * ⚠️ 那不是「擋下來了」而是<b>整個交易炸掉</b>，而交易裡還有撥款任務那一列。
     *
     * <p>🔴 <b>唯一入邊 {@code 35 → 54} 寫死在述詞裡</b>，不接受呼叫端指定 allowed-from：
     * 嚴禁自 {@code 40}／{@code 50}／{@code 70}／{@code 80} 進入（那些是已送達或已結案語意，
     * 不是配送失敗）。{@code 54} 是 immutable financial terminal（無出邊、不可逆），
     * 而同一個交易還會建 {@code grant_payment_tasks} ⇒ 入邊放寬一格，錢就從錯的地方流出去。
     *
     * @return 影響行數；{@code 0} ＝ 訂單不在 {@code 35}（重送或競態）。
     *         🔴 呼叫端<b>不得</b>把 {@code 0} 逕行當成「已處理」就 ACK——必須驗證既有終態
     *         （03 §5.7.3：已是 {@code 54} ⇒ 確認撥款任務存在；其他狀態 ⇒ 建案件且不得靜默 ACK）
     */
    @Update("""
        UPDATE orders SET status = #{newStatus}, updated_at = NOW()
        WHERE order_no = #{orderNo} AND status = #{expectedOldStatus}
        """)
    int markDeliveryFailedNoRefund(
            @Param("orderNo") String orderNo,
            @Param("expectedOldStatus") String expectedOldStatus,
            @Param("newStatus") String newStatus
    );

    /**
     * 條件式狀態更新（多種合法舊狀態）：僅當當前狀態在允許清單內時才更新
     * 用於退款等可從多種狀態轉換的場景
     */
    @Update("""
        <script>
        UPDATE orders SET status = #{newStatus}, updated_at = NOW()
        WHERE order_no = #{orderNo} AND status IN
        <foreach collection="oldStatuses" item="s" open="(" separator="," close=")">#{s}</foreach>
        </script>
        """)
    int updateStatusIfMatchAny(
            @Param("orderNo") String orderNo,
            @Param("newStatus") String newStatus,
            @Param("oldStatuses") List<String> oldStatuses
    );

    /**
     * 部分退款完成（03 §5.7.1 PARTIAL 分支，強制單一原子 UPDATE，CASE 運算式）：
     * 累加 refund_total_amount，且僅當 status=COMPLETED(50) 時轉 PARTIALLY_REFUNDED(60)，
     * 其餘進行中狀態（10~40）不改變（物流推進 / 自動完成 / 確認收貨路徑照走）；60 維持 60 僅累加。
     * <p>
     * 禁止 SELECT-then-UPDATE（TOCTOU：與 confirmReceipt 40→50 CAS 並行時帶舊狀態條件的
     * UPDATE affected=0，累加會靜默失敗）。終態 FULLY_REFUNDED(70) / CANCELLED(80) 不累加。
     *
     * @return 影響行數；0 = 訂單不存在或已處終態（70/80），caller 記 log skip
     */
    default int applyPartialRefund(String orderNo, long refundAmount) {
        return applyPartialRefundByStatus(orderNo, refundAmount,
                OrderStatus.COMPLETED.value(), OrderStatus.PARTIALLY_REFUNDED.value(),
                OrderStatus.FULLY_REFUNDED.value(), OrderStatus.CANCELLED.value());
    }

    @Update("""
        UPDATE orders
        SET refund_total_amount = refund_total_amount + #{refundAmount},
            status = CASE WHEN status = #{completed} THEN #{partiallyRefunded} ELSE status END,
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND status NOT IN (#{fullyRefunded}, #{cancelled})
        """)
    int applyPartialRefundByStatus(
            @Param("orderNo") String orderNo,
            @Param("refundAmount") long refundAmount,
            @Param("completed") String completed,
            @Param("partiallyRefunded") String partiallyRefunded,
            @Param("fullyRefunded") String fullyRefunded,
            @Param("cancelled") String cancelled
    );

    @Select("SELECT * FROM orders WHERE shipment_no = #{shipmentNo}")
    OrderEntity findByShipmentNo(@Param("shipmentNo") String shipmentNo);

    /**
     * 根據 checkout_id 查詢訂單（冪等檢查用）
     * 排除已取消訂單，允許使用者取消後以相同 checkout_id 重新下單
     * <p>
     * checkout_id 上沒有唯一約束（同一個 checkout_id 取消後可再建單），所以 LIMIT 1 是從一個
     * 可能不只一列的集合裡挑一列——沒有 ORDER BY 就是任意挑，冪等重試會回到不同的訂單號。
     * 取最新一筆才對應「這個結帳最後成立的訂單」。
     */
    @Select("""
        SELECT * FROM orders
        WHERE checkout_id = #{checkoutId} AND status != '80'
        ORDER BY created_at DESC, id DESC
        LIMIT 1
        """)
    OrderEntity findByCheckoutId(@Param("checkoutId") String checkoutId);

    /**
     * EXTERNAL lane RFQ 建單的冪等回查（64 號 §6.4.7）。
     *
     * <p>⚠️ 與 {@link #findByCheckoutId} 不同，這裡<b>不排除已取消</b>（{@code status != '80'}）：
     * 那一條是為了「同一個購物車結帳重試要拿到最後成立的那張」而存在的；acceptance 的
     * 語意相反——一筆成交至多一張訂單，{@code uk_orders_acceptance_id} 也是這樣約束的。
     * 濾掉已取消的會讓 worker 對一筆已取消的成交再建一張，繞過那個唯一索引的意圖。
     */
    @Select("SELECT * FROM orders WHERE acceptance_id = #{acceptanceId}")
    OrderEntity findByAcceptanceId(@Param("acceptanceId") Long acceptanceId);

    /**
     * 反查 buyer 是否已有 PENDING_PAYMENT 訂單包含當前任一 sku（03 §5.1.2）
     * 用於 CartService.CheckoutLock 失敗時分流：有 → ORDER_ALREADY_CREATED(4092)；無 → CHECKOUT_LOCK_FAILED(6005)
     */
    @Select("""
        <script>
        SELECT o.* FROM orders o
        WHERE o.buyer_id = #{buyerId}
          AND o.status = '0'
          AND EXISTS (
              SELECT 1 FROM order_items oi
              WHERE oi.order_no = o.order_no
                AND oi.sku_id IN
                <foreach collection="skuIds" item="s" open="(" separator="," close=")">#{s}</foreach>
          )
        ORDER BY o.created_at DESC, o.id DESC
        LIMIT 1
        </script>
        """)
    OrderEntity findLatestPendingPaymentByBuyerAndSkus(
            @Param("buyerId") long buyerId,
            @Param("skuIds") List<Long> skuIds
    );

    /**
     * 查詢已超時的待付款訂單
     */
    @Select("""
        SELECT * FROM orders
        WHERE status = '0'
          AND payment_deadline IS NOT NULL
          AND payment_deadline < NOW()
        ORDER BY payment_deadline ASC, id ASC
        LIMIT 100
        """)
    List<OrderEntity> findExpiredPendingOrders();

    /**
     * 物流建單事件：更新物流單號、發貨時間、物流狀態、託運單 URL、賣家處理期限
     * 訂單狀態維持不變（PAID），由後續 callTcat / 物流狀態變更事件推進
     * 冪等：僅當 shipment_no IS NULL 時才更新
     * processing_deadline：09 §5.9，由 shipment.created event payload 帶入。Uber 即時配送時為 NULL
     */
    @Update("""
        UPDATE orders
        SET shipment_no = #{shipmentNo},
            ship_time = NOW(),
            shipping_status = #{shippingStatus},
            label_url = #{labelUrl},
            processing_deadline = #{processingDeadline},
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND shipment_no IS NULL
        """)
    int updateShipmentCreated(
            @Param("orderNo") String orderNo,
            @Param("shipmentNo") String shipmentNo,
            @Param("shippingStatus") String shippingStatus,
            @Param("labelUrl") String labelUrl,
            @Param("processingDeadline") LocalDateTime processingDeadline
    );

    /**
     * §5.9 賣家完成處理（CVS 取號 / 宅配叫件成功）→ 清空 processing_deadline
     * 冪等：僅當 processing_deadline IS NOT NULL 時才更新（避免無謂寫入）
     */
    @Update("""
        UPDATE orders
        SET processing_deadline = NULL,
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND processing_deadline IS NOT NULL
        """)
    int clearProcessingDeadline(@Param("orderNo") String orderNo);

    /**
     * 出貨照片更新事件：覆蓋寫入 shipment_photo_keys JSONB（存 objectKey，49 §5.3）
     */
    @Update("UPDATE orders SET shipment_photo_keys = #{photoKeys}::jsonb WHERE order_no = #{orderNo}")
    int updateShipmentPhotoKeys(
            @Param("orderNo") String orderNo,
            @Param("photoKeys") String photoKeys
    );

    /**
     * 物流狀態變更事件：更新物流狀態
     * PayUni 狀態碼不遞增（91→92→21→31），不可用數值比較防併發。
     * 直接覆蓋更新（Shipping Service 已在 Java 層驗證轉移合法性）。
     */
    @Update("""
        UPDATE orders
        SET shipping_status = #{shippingStatus},
            shipping_status_desc = #{shippingStatusDesc},
            updated_at = NOW()
        WHERE order_no = #{orderNo}
        """)
    int updateShippingStatus(
            @Param("orderNo") String orderNo,
            @Param("shippingStatus") String shippingStatus,
            @Param("shippingStatusDesc") String shippingStatusDesc
    );

    /**
     * 物流狀態變更事件：更新物流狀態 + labelUrl + labelExpireAt（宅配取號後帶 PDF URL 和下載期限）
     */
    @Update("""
        UPDATE orders
        SET shipping_status = #{shippingStatus},
            shipping_status_desc = #{shippingStatusDesc},
            label_url = #{labelUrl},
            label_expire_at = #{labelExpireAt},
            updated_at = NOW()
        WHERE order_no = #{orderNo}
        """)
    int updateShippingStatusAndLabel(
            @Param("orderNo") String orderNo,
            @Param("shippingStatus") String shippingStatus,
            @Param("shippingStatusDesc") String shippingStatusDesc,
            @Param("labelUrl") String labelUrl,
            @Param("labelExpireAt") LocalDateTime labelExpireAt
    );

    /**
     * 超商出貨便代碼回填：更新 partner_id/odno/validation_no
     * 冪等：僅當 partner_id IS NULL 時才更新
     */
    @Update("""
        UPDATE orders
        SET partner_id = #{partnerId},
            odno = #{odno},
            validation_no = #{validationNo}
        WHERE order_no = #{orderNo}
          AND partner_id IS NULL
        """)
    int updateLabelInfo(
            @Param("orderNo") String orderNo,
            @Param("partnerId") String partnerId,
            @Param("odno") String odno,
            @Param("validationNo") String validationNo
    );

    /**
     * 物流送達事件：更新訂單狀態為已送達 + 設定自動完成時間
     * 冪等：僅當 status 在 allowedStatuses 清單中時才更新
     */
    @Update("""
        <script>
        UPDATE orders
        SET status = #{status},
            shipping_status = #{shippingStatus},
            shipping_status_desc = #{shippingStatusDesc},
            auto_complete_at = #{autoCompleteAt},
            countdown_paused_at = NULL,
            delivered_at = NOW(),
            -- E 軸（收貨確認）入口。🔴 不是選配：DB CHECK chk_orders_receipt_axis_present
            -- 要求 delivered_at 非 NULL ⇒ receipt_status 必須非 NULL，少這兩行整句 23514 失敗，
            -- 而失敗會被 MQ consumer 的重試包住 ⇒ 對呼叫端是靜默的（V14 ②）。
            -- 用 COALESCE 不用直接賦值：本 SQL 的 WHERE 只鎖 status，若某天允許重投遞，
            -- 直接賦值會把已經前進的 E 軸（BUYER_CONFIRMED 等）打回 PENDING。
            -- 值由 ReceiptStatus CodeMeta enum 供給（本波已建），不在 SQL 內寫字面值。
            receipt_status = COALESCE(receipt_status, #{receiptStatusPending}),
            receipt_status_entered_at = COALESCE(receipt_status_entered_at, NOW()),
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND status IN
            <foreach collection="allowedStatuses" item="s" open="(" separator="," close=")">
                #{s}
            </foreach>
        </script>
        """)
    int updateOrderDelivered(
            @Param("orderNo") String orderNo,
            @Param("status") String status,
            @Param("shippingStatus") String shippingStatus,
            @Param("shippingStatusDesc") String shippingStatusDesc,
            @Param("autoCompleteAt") LocalDateTime autoCompleteAt,
            @Param("allowedStatuses") List<String> allowedStatuses,
            @Param("receiptStatusPending") String receiptStatusPending
    );

    /**
     * 物流送達事件（E 軸入口值由 CodeMeta 供給）。
     *
     * <p>🔴 這個多載的存在是為了讓呼叫端<b>不必知道 E 軸的初始值是什麼</b>——
     * 值域的權威是 {@link ReceiptStatus}，不是散在各處的字串。
     */
    default int updateOrderDelivered(String orderNo, String status, String shippingStatus,
                                     String shippingStatusDesc, LocalDateTime autoCompleteAt,
                                     List<String> allowedStatuses) {
        return updateOrderDelivered(orderNo, status, shippingStatus, shippingStatusDesc,
                autoCompleteAt, allowedStatuses, ReceiptStatus.PENDING.value());
    }

    /**
     * 快照付款資訊：僅更新 transaction_id / payment_method / ATM 欄位
     * 使用條件式更新避免覆蓋 MQ 事件已確認的值（僅在欄位為空時寫入）
     */
    @Update("""
        UPDATE orders
        -- §2A.7 觸發點 #5（重複付款翻轉後 snapshot）需要覆寫 payment_method，
        -- 所以移除原本的 immutability 限制（CASE WHEN payment_method IS NULL）。
        -- 業務正確性由 payment-service 呼叫端負責（每次都用「當下查到的主 row.payment_type」）。
        SET transaction_id = CASE WHEN #{transactionId} != '' THEN #{transactionId} ELSE transaction_id END,
            payment_method = CASE WHEN #{paymentMethod} != '' THEN #{paymentMethod} ELSE payment_method END,
            atm_bank_code = CASE WHEN #{atmBankCode} != '' THEN #{atmBankCode} ELSE atm_bank_code END,
            atm_virtual_account = CASE WHEN #{atmVirtualAccount} != '' THEN #{atmVirtualAccount} ELSE atm_virtual_account END,
            atm_expire_date = CASE WHEN #{atmExpireDate} != '' THEN #{atmExpireDate} ELSE atm_expire_date END,
            updated_at = NOW()
        WHERE order_no = #{orderNo}
        """)
    int snapshotPaymentInfo(
            @Param("orderNo") String orderNo,
            @Param("transactionId") String transactionId,
            @Param("paymentMethod") String paymentMethod,
            @Param("atmBankCode") String atmBankCode,
            @Param("atmVirtualAccount") String atmVirtualAccount,
            @Param("atmExpireDate") String atmExpireDate
    );

    /**
     * 查詢已到期的已送達訂單（待自動完成）。
     * 暫停中（countdown_paused_at IS NOT NULL）的訂單跳過，03 §5.8。
     */
    @Select("""
        SELECT * FROM orders
        WHERE status = '40'
          AND auto_complete_at IS NOT NULL
          AND auto_complete_at <= NOW()
          AND countdown_paused_at IS NULL
        ORDER BY auto_complete_at ASC, id ASC
        LIMIT #{limit}
        """)
    List<OrderEntity> findAutoCompleteOrders(@Param("limit") int limit);

    /**
     * 暫停自動完成倒數（03 §5.8）。
     * 冪等：僅當 status=40 且尚未暫停時生效。
     */
    @Update("""
        UPDATE orders
        SET countdown_paused_at = NOW(),
            countdown_pause_count = countdown_pause_count + 1,
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND status = '40'
          AND countdown_paused_at IS NULL
        """)
    int pauseCountdown(@Param("orderNo") String orderNo);

    /**
     * 恢復自動完成倒數（03 §5.8）。
     * stopwatch 模式：auto_complete_at += (NOW - countdown_paused_at)；恢復後清空 countdown_paused_at。
     * 冪等：僅當 status=40 且暫停中時生效。
     */
    @Update("""
        UPDATE orders
        SET auto_complete_at = auto_complete_at + (NOW() - countdown_paused_at),
            countdown_paused_at = NULL,
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND status = '40'
          AND countdown_paused_at IS NOT NULL
        """)
    int resumeCountdown(@Param("orderNo") String orderNo);

    /**
     * 對帳兜底（03 §5.8.7）：暫停早於 thresholdAt 的訂單，由 worker 反查 AfterSales 確認是否需要恢復。
     */
    @Select("""
        SELECT order_no FROM orders
        WHERE status = '40'
          AND countdown_paused_at IS NOT NULL
          AND countdown_paused_at < #{thresholdAt}
        ORDER BY countdown_paused_at ASC, id ASC
        LIMIT #{limit}
        """)
    List<String> scanStalledPausedOrders(@Param("thresholdAt") LocalDateTime thresholdAt, @Param("limit") int limit);

    /**
     * 確認收貨或自動完成：DELIVERED(40) → COMPLETED(50)
     * 冪等：僅當 status = DELIVERED(40) 時才更新
     */
    default int completeDeliveredOrder(String orderNo, String confirmType, String receiptPhotoKeys) {
        return completeDeliveredOrderByStatus(orderNo, confirmType, receiptPhotoKeys,
                OrderStatus.COMPLETED.value(), OrderStatus.DELIVERED.value(),
                receiptStatusFor(confirmType));
    }

    /**
     * 收貨確認方式 → E 軸狀態。
     *
     * <p>🔴 <b>這是 E 軸的「誰把它推走」</b>。沒有它，每一筆訂單在送達時進 {@code PENDING}，
     * 然後**永遠停在 PENDING**——訂單軸走到 COMPLETED 了，收貨確認軸一格都沒動，
     * 而撥款資格的其中一條正是讀這一軸。症狀是撥款永遠不成立，且沒有任何錯誤。
     *
     * <p>映射與 {@code V14} 回填用的 CASE 逐字一致（那支 migration 已經把存量列照這個規則填過），
     * 兩處若分歧，新舊資料會用兩套語意。
     *
     * <p>⚠️ {@code AUTO_CONFIRMED} 與 {@code BUYER_CONFIRMED} <b>刻意分開</b>：
     * 撥款資格的收貨確認條件只認買家或平台的明示確認，逾期自動完成不算。
     * 合成一個值會讓「買家從沒表示過意見」的訂單通過那一條。
     */
    static String receiptStatusFor(String confirmType) {
        if (OrderConstants.CONFIRM_TYPE_MANUAL.equals(confirmType)) {
            return ReceiptStatus.BUYER_CONFIRMED.value();
        }
        if (OrderConstants.CONFIRM_TYPE_AUTO.equals(confirmType)) {
            return ReceiptStatus.AUTO_CONFIRMED.value();
        }
        // fail-closed：未知的確認方式不猜。丟例外而不是落一個「像的」值——
        // E 軸的值直接影響撥款資格，猜錯的後果是錢的方向錯。
        throw new IllegalArgumentException(
                "未知的 confirmType=" + confirmType + "，無法決定收貨確認軸的狀態。"
                        + "新增確認方式時必須同批決定它對應哪一個 ReceiptStatus，"
                        + "以及它算不算「買家表示過意見」（那決定撥款資格）");
    }

    @Update("""
        UPDATE orders
        SET status = #{completed},
            confirmed_at = NOW(),
            confirm_type = #{confirmType},
            receipt_photo_keys = CAST(#{receiptPhotoKeys} AS jsonb),
            -- E 軸出口。與上面的 status CAS 同一句 ⇒ 訂單軸與收貨確認軸不可能分歧。
            -- ⚠️ 不另加 receipt_status 的 allowed-from：本句的 WHERE 已經鎖死
            -- status = DELIVERED，而 DELIVERED→COMPLETED 只會成功一次；
            -- 再加一層 allowed-from 只會在重投遞時多一種 rows=0 的靜默失敗。
            receipt_status = #{receiptStatus},
            receipt_status_entered_at = NOW(),
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND status = #{delivered}
        """)
    int completeDeliveredOrderByStatus(
            @Param("orderNo") String orderNo,
            @Param("confirmType") String confirmType,
            @Param("receiptPhotoKeys") String receiptPhotoKeys,
            @Param("completed") String completed,
            @Param("delivered") String delivered,
            @Param("receiptStatus") String receiptStatus
    );

    /**
     * 遞增付款嘗試次數
     */
    @Update("""
        UPDATE orders
        SET payment_attempts = COALESCE(payment_attempts, 0) + 1,
            updated_at = NOW()
        WHERE order_no = #{orderNo}
        """)
    int incrementPaymentAttempts(@Param("orderNo") String orderNo);

    /**
     * 更新 ATM 退款頁面標記
     */
    @Update("UPDATE orders SET need_refund_page = #{needRefundPage}, updated_at = NOW() WHERE order_no = #{orderNo}")
    int updateNeedRefundPage(@Param("orderNo") String orderNo, @Param("needRefundPage") boolean needRefundPage);

    /**
     * 標記訂單已評論（冪等：僅 is_reviewed = FALSE 時更新）
     */
    @Update("UPDATE orders SET is_reviewed = TRUE, updated_at = NOW() WHERE order_no = #{orderNo} AND is_reviewed = FALSE")
    int markReviewed(@Param("orderNo") String orderNo);

    /**
     * 寫入訂單評論快照（訂單層級單一）。most-recent-wins：僅當尚無快照、或新評論的 createdAt
     * 晚於現存快照時才覆寫，確保訂單詳情顯示「最近一筆」商品評論（不受 MQ 到達順序影響）。
     * 評論不可改/刪 → 凍結快照，重送同事件不覆寫（03 設計書 §6.1 / §8.9）。
     */
    @Update("UPDATE orders SET review = #{review,jdbcType=OTHER,typeHandler=io.github.will413028.hawkr.order.config.typehandler.ReviewSnapshotTypeHandler}, updated_at = NOW() "
            + "WHERE order_no = #{orderNo} AND (review IS NULL OR (review->>'createdAt') < #{createdAt})")
    int updateOrderReview(@Param("orderNo") String orderNo, @Param("review") ReviewSnapshot review, @Param("createdAt") String createdAt);

    /**
     * 消費紀錄（買家會員卡 / 賣家顧客消費明細）納入的訂單狀態（SSOT，三支查詢共用）。
     * <p>
     * <b>COMPLETED(50)</b>：買家確認收貨的正常已完成消費。
     * <b>PARTIALLY_REFUNDED(60)</b>：CodeMeta「唯一進入轉移 = 50→60」，且本 repository
     * 的 {@link #applyPartialRefundByStatus} 也只有 {@code status = CASE WHEN status = '50'}
     * 這一條路徑會寫成 60——所以 60 在語意上就是「一筆已完成的消費，事後退了部分款」，
     * 漏掉它會讓買家自己退過款的那幾筆整個從消費紀錄消失。
     * <p>
     * <b>刻意不含 FULLY_REFUNDED(70)</b>，兩個獨立理由：
     * <ol>
     *   <li>70 可由 10/20/30/32/35/40/50/60 任一狀態進入
     *       （AfterSalesRefundEventHandler.applyFullRefund 的 allowedStatuses），
     *       不等於「已完成的消費」——可能是付了款根本沒收到貨就整筆退掉；</li>
     *   <li>更關鍵：FULL 路徑<b>不維護</b> {@code refund_total_amount}（applyFullRefund 只做
     *       狀態 CAS），納入只會顯示「金額 X／退回 0」這種錯誤資訊。crm-service 也是同一個
     *       判斷（{@code crm.go}「FULL_REFUND does not trust order-service's
     *       refund_total_amount (known unreliable on that path)」），且
     *       {@link #findOrderAggregates} 對 70 一律淨額計 0。</li>
     * </ol>
     */
    private static List<String> spendingRecordStatuses() {
        return List.of(OrderStatus.COMPLETED.value(), OrderStatus.PARTIALLY_REFUNDED.value());
    }

    /**
     * 查詢買家在各賣家的累計消費金額與累計退款額
     * 用於買家「會員卡」清單。狀態集合見 {@link #spendingRecordStatuses()}。
     * <p>
     * total_spending 是<b>毛額</b>（不扣退款），total_refund 另欄回傳——與明細列表的
     * 「金額 / 退回」兩欄同語意，逐列相加即等於這裡的兩個總和，前端不會加不起來。
     */
    default List<BuyerMerchantSummaryResult> findBuyerMerchantSummary(long buyerId) {
        return findBuyerMerchantSummaryByStatuses(buyerId, spendingRecordStatuses());
    }

    @Select("""
        <script>
        SELECT store_id,
               SUM(total_actual_amount) AS total_spending,
               SUM(COALESCE(refund_total_amount, 0)) AS total_refund,
               MAX(created_at) AS last_order_time
        FROM orders
        WHERE buyer_id = #{buyerId}
          AND status IN
          <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
        GROUP BY store_id
        ORDER BY MAX(created_at) DESC, store_id DESC
        </script>
        """)
    List<BuyerMerchantSummaryResult> findBuyerMerchantSummaryByStatuses(
            @Param("buyerId") long buyerId,
            @Param("statuses") List<String> statuses
    );

    /**
     * 查詢買家在特定賣家的消費紀錄（含折扣與已完成退款金額）
     * 用於買家「消費紀錄」明細與賣家 CRM 顧客消費紀錄。狀態集合見 {@link #spendingRecordStatuses()}。
     */
    default List<BuyerSellerOrderResult> findBuyerOrdersForSeller(long buyerId, long storeId, int size, int offset) {
        return findBuyerOrdersForSellerByStatuses(buyerId, storeId, spendingRecordStatuses(), size, offset);
    }

    @Select("""
        <script>
        SELECT order_no, created_at, total_actual_amount, COALESCE(discount_amount, 0) AS discount_amount,
               shipping_address, recipient_phone, COALESCE(shipping_fee, 0) AS shipping_fee,
               COALESCE(refund_total_amount, 0) AS refund_total_amount
        FROM orders
        WHERE buyer_id = #{buyerId}
          AND store_id = #{storeId}
          AND status IN
          <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
        ORDER BY created_at DESC, id DESC
        LIMIT #{size} OFFSET #{offset}
        </script>
        """)
    List<BuyerSellerOrderResult> findBuyerOrdersForSellerByStatuses(
            @Param("buyerId") long buyerId,
            @Param("storeId") long storeId,
            @Param("statuses") List<String> statuses,
            @Param("size") int size,
            @Param("offset") int offset
    );

    /** 與 {@link #findBuyerOrdersForSeller} 必須同一組狀態，否則 total 與 records 對不上、分頁破掉。 */
    default long countBuyerOrdersForSeller(long buyerId, long storeId) {
        return countBuyerOrdersForSellerByStatuses(buyerId, storeId, spendingRecordStatuses());
    }

    @Select("""
        <script>
        SELECT COUNT(*) FROM orders
        WHERE buyer_id = #{buyerId}
          AND store_id = #{storeId}
          AND status IN
          <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
        </script>
        """)
    long countBuyerOrdersForSellerByStatuses(
            @Param("buyerId") long buyerId,
            @Param("storeId") long storeId,
            @Param("statuses") List<String> statuses
    );

    @Select("""
        SELECT COUNT(*) FROM orders
        WHERE store_id = #{storeId}
          AND status = #{status}
        """)
    long countBySellerAndStatus(
            @Param("storeId") long storeId,
            @Param("status") String status
    );

    @Select("""
        <script>
        SELECT COUNT(*) FROM orders
        WHERE store_id = #{storeId}
          AND status IN
            <foreach collection="statuses" item="s" open="(" separator="," close=")">
                #{s}
            </foreach>
        </script>
        """)
    long countBySellerInProgress(
            @Param("storeId") long storeId,
            @Param("statuses") List<String> statuses
    );

    /**
     * 賣家工作台 Dashboard 桶 count（03 §2.1 SSOT）：status IN(statuses) 且 order_no 不在 excludeOrderNos。
     * <p>
     * excludeOrderNos 為被售後覆蓋（active ∪ terminal-override）需從生命週期三桶排除的訂單；
     * 空集合時不加 NOT IN 條件。用於 pending / in_progress / completed 第一段。
     */
    @Select("""
        <script>
        SELECT COUNT(*) FROM orders
        WHERE store_id = #{storeId}
          AND buyer_id != #{storeId}
          AND status IN
            <foreach collection="statuses" item="s" open="(" separator="," close=")">
                #{s}
            </foreach>
          <if test="excludeOrderNos != null and excludeOrderNos.size() > 0">
            AND order_no NOT IN
            <foreach collection="excludeOrderNos" item="o" open="(" separator="," close=")">
                #{o}
            </foreach>
          </if>
        </script>
        """)
    long countBySellerStatusExcludingOrders(
            @Param("storeId") long storeId,
            @Param("statuses") List<String> statuses,
            @Param("excludeOrderNos") List<String> excludeOrderNos
    );

    /**
     * 賣家工作台 Dashboard completed 第二段（03 §2.1 SSOT）：order_no IN(includeOrderNos) 且 status IN(statuses)。
     * <p>
     * includeOrderNos 為 terminal-override 售後（已排除 active）的訂單；statuses 傳生命週期非終態集
     * {10,20,30,32,35,40}，把「售後已結但 order.status 尚未達終態」的訂單補入 completed。
     * includeOrderNos 空集合時回 0（AND 1=0）。
     */
    @Select("""
        <script>
        SELECT COUNT(*) FROM orders
        WHERE store_id = #{storeId}
          AND buyer_id != #{storeId}
          AND status IN
            <foreach collection="statuses" item="s" open="(" separator="," close=")">
                #{s}
            </foreach>
          <choose>
            <when test="includeOrderNos != null and includeOrderNos.size() > 0">
              AND order_no IN
              <foreach collection="includeOrderNos" item="o" open="(" separator="," close=")">
                  #{o}
              </foreach>
            </when>
            <otherwise>
              AND 1 = 0
            </otherwise>
          </choose>
        </script>
        """)
    long countBySellerOrdersInStatus(
            @Param("storeId") long storeId,
            @Param("includeOrderNos") List<String> includeOrderNos,
            @Param("statuses") List<String> statuses
    );

    /**
     * 查詢近 N 天營收排名前 N 的賣家
     * 用於推薦商家 API
     */
    @Select("""
        <script>
        SELECT store_id,
               SUM(total_actual_amount) AS total_revenue,
               COUNT(*)                 AS order_count
        FROM orders
        WHERE status IN
            <foreach collection="statuses" item="s" open="(" separator="," close=")">
                #{s}
            </foreach>
          AND created_at >= NOW() - make_interval(days => #{days})
        GROUP BY store_id
        ORDER BY total_revenue DESC, store_id DESC
        LIMIT #{limit}
        </script>
        """)
    List<SellerRevenueResult> findTopSellersByRevenue(
            @Param("days") int days,
            @Param("limit") int limit,
            @Param("statuses") List<String> statuses
    );

    /**
     * 近 N 天各店的訂單統計（ListStoreOrderStats，單體 discovery 推薦排行用）。
     * 成立訂單（qualifying）= status IN #{qualifyingStatuses}（已付款、未取消、未全額退款；與
     * findTopSellersByRevenue 同一組狀態集合，部分退款仍是成立的購買），時間窗以付款時間 {@code pay_time} 計；
     * 賣家取消 = status = #{cancelledStatus} 且 cancel_by_role = #{sellerRole}，時間窗以取消時間
     * {@code cancelled_at} 計。兩個條件互斥（一筆訂單只有一個狀態），所以 FILTER 只看狀態、時間窗在 WHERE。
     * 只回至少有一筆成立或賣家取消訂單的店；以 store_id 升冪做 keyset 分頁（store_id &gt; #{afterStoreId}，
     * 0＝從頭；store_id 是 BIGINT，游標與排序同為數值比較）。
     */
    @Select("""
        <script>
        SELECT store_id,
               COUNT(DISTINCT buyer_id) FILTER (WHERE status IN
                   <foreach collection="qualifyingStatuses" item="s" open="(" separator="," close=")">
                       #{s}
                   </foreach>
               ) AS distinct_buyers,
               COUNT(*) FILTER (WHERE status IN
                   <foreach collection="qualifyingStatuses" item="s" open="(" separator="," close=")">
                       #{s}
                   </foreach>
               ) AS qualifying_orders,
               COUNT(*) FILTER (WHERE status = #{cancelledStatus} AND cancel_by_role = #{sellerRole})
                   AS seller_cancelled_orders
        FROM orders
        WHERE store_id IS NOT NULL
          AND store_id &gt; #{afterStoreId}
          AND ((status IN
                   <foreach collection="qualifyingStatuses" item="s" open="(" separator="," close=")">
                       #{s}
                   </foreach>
                AND pay_time >= NOW() - make_interval(days => #{days}))
               OR (status = #{cancelledStatus} AND cancel_by_role = #{sellerRole}
                AND cancelled_at >= NOW() - make_interval(days => #{days})))
        GROUP BY store_id
        ORDER BY store_id ASC
        LIMIT #{limit}
        </script>
        """)
    List<StoreOrderStatsResult> findStoreOrderStats(
            @Param("days") int days,
            @Param("afterStoreId") long afterStoreId,
            @Param("limit") int limit,
            @Param("qualifyingStatuses") List<String> qualifyingStatuses,
            @Param("cancelledStatus") String cancelledStatus,
            @Param("sellerRole") String sellerRole
    );

    /**
     * 單店在時間窗 [startMs, endMs)（Unix ms）內的成立訂單統計（GetStorePeriodOrderStats，單體賣家儀表板用）。
     * 成立訂單與 findStoreOrderStats 同一組狀態集合（#{qualifyingStatuses}），時間窗以付款時間 {@code pay_time} 計；
     * 不排除自買單（與 findStoreOrderStats 一致）。營收 = 實付金額 total_actual_amount 減去已完成的退款金額
     * refund_total_amount（NOT NULL DEFAULT 0；部分退款在進行中狀態也會累加），所以部分退款的訂單只計實際收到的部分。
     * 時間以毫秒傳入、由資料庫轉成 timestamptz，不依賴 JDBC 連線的時區。沒有符合的訂單時回一列全為 0。
     */
    @Select("""
        <script>
        SELECT COUNT(*)                                                   AS qualifying_orders,
               COUNT(DISTINCT buyer_id)                                   AS distinct_buyers,
               COALESCE(SUM(total_actual_amount - refund_total_amount), 0) AS revenue
        FROM orders
        WHERE store_id = #{storeId}
          AND status IN
            <foreach collection="qualifyingStatuses" item="s" open="(" separator="," close=")">
                #{s}
            </foreach>
          AND pay_time >= to_timestamp(CAST(#{startMs} AS double precision) / 1000.0)
          AND pay_time &lt; to_timestamp(CAST(#{endMs} AS double precision) / 1000.0)
        </script>
        """)
    StorePeriodOrderStatsResult findStorePeriodOrderStats(
            @Param("storeId") long storeId,
            @Param("startMs") long startMs,
            @Param("endMs") long endMs,
            @Param("qualifyingStatuses") List<String> qualifyingStatuses
    );

    /**
     * CRM 訂單聚合（issue #1519，GetOrderAggregates）：單一 GROUP BY，非逐單掃描。
     * status 排除 PENDING_PAYMENT('0') 與 CANCELLED('80')；PARTIALLY_REFUNDED('60') 扣
     * refund_total_amount；FULLY_REFUNDED('70') 淨額計 0。storeId=0 表示不篩選。
     */
    @Select("""
        WITH agg AS (
          SELECT store_id, buyer_id,
                 COUNT(*) AS order_count,
                 SUM(CASE
                       WHEN status = '70' THEN 0
                       WHEN status = '60' THEN total_actual_amount - refund_total_amount
                       ELSE total_actual_amount
                     END) AS total_net_amount
          FROM orders
          WHERE status IN ('10','20','30','32','35','40','50','60','70')
            AND (#{storeId} = 0 OR store_id = #{storeId})
          GROUP BY store_id, buyer_id
        )
        SELECT * FROM agg ORDER BY store_id, buyer_id LIMIT #{size} OFFSET #{offset}
        """)
    List<OrderAggregateResult> findOrderAggregates(
            @Param("storeId") long storeId, @Param("size") int size, @Param("offset") int offset);

    @Select("""
        SELECT COUNT(*) FROM (
          SELECT store_id, buyer_id
          FROM orders
          WHERE status IN ('10','20','30','32','35','40','50','60','70')
            AND (#{storeId} = 0 OR store_id = #{storeId})
          GROUP BY store_id, buyer_id
        ) t
        """)
    long countOrderAggregates(@Param("storeId") long storeId);

    /**
     * 售後內部取消訂單：CAS 狀態更新 + 取消原因/備註/角色 + 清 processing_deadline（03 §5.3.1）
     * 允許取消的狀態：PENDING_PAYMENT(0) / PAID(10) / PENDING_SHIPMENT(20)
     * cancelByRole：賣家透過售後 path 為 'SELLER'；系統 timeout path 為 'SYSTEM'
     *
     * <p>issue #2471：與另兩支取消 CAS 一樣在同一句寫 {@code cancelled_at}——狀態與結案時間
     * 必須原子同落，分兩句會出現「已取消但沒有取消時間」的窗口。CAS 保證只有搶到的那次會寫，
     * 重送不會把時間往後推。
     */
    @Update("""
        UPDATE orders SET status = #{newStatus}, cancel_reason = #{cancelReason},
            cancel_remark = #{cancelRemark}, cancel_by_role = #{cancelByRole},
            cancelled_at = NOW(), processing_deadline = NULL, updated_at = NOW()
        WHERE order_no = #{orderNo} AND status IN ('0', '10', '20')
        """)
    int cancelOrderInternalCas(
            @Param("orderNo") String orderNo,
            @Param("newStatus") String newStatus,
            @Param("cancelReason") String cancelReason,
            @Param("cancelRemark") String cancelRemark,
            @Param("cancelByRole") String cancelByRole
    );

    /**
     * Buyer 自願取消未付款訂單：CAS 狀態更新 + cancel_by_role='BUYER'（cancel_reason / cancel_remark 留 NULL，03 §5.3.1）
     * ＋ cancelled_at 結案時間（issue #2471，理由見 {@link #cancelOrderInternalCas}）
     */
    @Update("""
        UPDATE orders SET status = #{newStatus}, cancel_by_role = 'BUYER',
            cancelled_at = NOW(), updated_at = NOW()
        WHERE order_no = #{orderNo} AND status = #{expectedOldStatus}
        """)
    int cancelOrderByBuyerCas(
            @Param("orderNo") String orderNo,
            @Param("expectedOldStatus") String expectedOldStatus,
            @Param("newStatus") String newStatus
    );

    /**
     * 系統批次取消（CAS）：CAS 狀態更新 + cancel_reason（呼叫端傳入）+ cancel_by_role='SYSTEM'（03 §5.3.1）。
     *
     * <p>原僅供付款逾時（硬編 '9101'）使用；賣家停權批次取消（§5.1a.2）複用同一 CAS + 悲觀鎖防護，
     * 需要不同 cancel_reason（{@code OrderCancelReason.SELLER_SUSPENDED}），故參數化。
     * 呼叫端（{@code PaymentEventHandler.cancelExpiredOrderInTx}）已用 {@code CodeMetaValidator} 驗證合法值。
     *
     * <p>issue #2471：同句寫 {@code cancelled_at}（理由見 {@link #cancelOrderInternalCas}）。
     */
    @Update("""
        UPDATE orders SET status = #{newStatus}, cancel_reason = #{cancelReason},
            cancel_by_role = 'SYSTEM', cancelled_at = NOW(), updated_at = NOW()
        WHERE order_no = #{orderNo} AND status = #{expectedOldStatus}
        """)
    int cancelOrderByPaymentExpireCas(
            @Param("orderNo") String orderNo,
            @Param("expectedOldStatus") String expectedOldStatus,
            @Param("newStatus") String newStatus,
            @Param("cancelReason") String cancelReason
    );

    /**
     * 賣家停權批次取消未付款單掃描（03 §5.1a.2）：查該賣家全部 {@code PENDING_PAYMENT(0)} 訂單號。
     * 逐筆走 {@code PaymentEventHandler.cancelExpiredOrderInTx}，繼承悲觀鎖 + CAS 全部既有防護，
     * 本查詢僅負責「挑出候選 orderNo」，非最終取消判定（CAS 於取消當下才鎖定真實狀態）。
     */
    @Select("""
        SELECT order_no FROM orders
        WHERE store_id = #{storeId}
          AND status = #{status}
        """)
    List<String> findPendingPaymentOrderNosBySeller(
            @Param("storeId") long storeId,
            @Param("status") String status);

    /**
     * Coupon Reserve 成功後回填 coupon_instance_id（冪等：僅當欄位為 NULL 時寫入）。
     * 03 §2.1 INV-3：只動 coupon_instance_id 一個欄位，避免 updateById 全欄位覆蓋。
     */
    @Update("""
        UPDATE orders
        SET coupon_instance_id = #{instanceId},
            updated_at = NOW()
        WHERE order_no = #{orderNo}
          AND coupon_instance_id IS NULL
        """)
    int updateCouponInstanceIdIfAbsent(
            @Param("orderNo") String orderNo,
            @Param("instanceId") String instanceId
    );

    /**
     * 賣家停權 IM 訂單關聯回填（14 號設計書 §6.5.1 路徑②／§6.5.2）：
     * 反查買賣雙方之間「關聯」訂單的最近一筆 order_no。
     * 判準 status NOT IN (0,80)：排除 PENDING_PAYMENT(0) 與 CANCELLED(80)，
     * 其餘（PAID 10 ～ FULLY_REFUNDED 70）皆算關聯。取最近一筆即可（HasOrderBetween 只需 bool + 參考單號）。
     */
    @Select("""
        SELECT order_no FROM orders
        WHERE buyer_id = #{buyerId}
          AND store_id = #{storeId}
          AND status NOT IN ('0', '80')
        ORDER BY created_at DESC, id DESC
        LIMIT 1
        """)
    String findLatestRelatedOrderNoBetween(
            @Param("buyerId") long buyerId,
            @Param("storeId") long storeId
    );
}
