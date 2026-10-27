package io.github.will413028.hawkr.payment.logistics.repository.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.logistics.domain.ReceivableStatusRules;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import java.time.LocalDateTime;
import java.util.List;
import org.apache.ibatis.annotations.Insert;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

/**
 * {@code logistics_receivables} 的 mapper。
 *
 * <p>🔴 <b>狀態轉換一律 CAS，allowed-from 一律從 {@link ReceivableStatusRules#allowedFromValues}
 * 取，不在 SQL 裡重打一份</b>：兩份必然漂移，而漂移的症狀是 rows=0 這種看起來像
 * 「別人先做過了」的靜默卡死。所以 {@link #transitionStatus} 是 {@code default} 方法，
 * SQL 那一支只接受<b>呼叫端傳進來的清單</b>，自己不知道任何狀態值。
 *
 * <p>🔴 <b>claim 用 lease 不用永久旗標</b>（10 檔 §29）：租約基準在 claim 當下<b>逐筆</b>取
 * （{@code NOW() + make_interval}），不在批次迴圈外算一次——後者會讓不變式從
 * 「單筆最壞耗時 &lt; 租約」退化成「批次總耗時 &lt; 租約」，隨 batch size 一起破。
 */
@Mapper
public interface LogisticsReceivableMapper extends BaseMapper<LogisticsReceivableEntity> {

    /**
     * 建立應收，以 {@code idempotency_key} 去重。
     *
     * <p>🔴 {@code ON CONFLICT DO NOTHING} 而不是 catch {@code DuplicateKeyException}：
     * PostgreSQL 交易內任一句 SQL 失敗 ⇒ 整個交易 aborted（{@code 25P02}），catch 得到例外
     * 但下一句 SELECT 會炸成 500（10 檔 §11.1）。衝突代表送達事件重放，正確行為是
     * 「什麼都不做」而不是「報錯」。
     *
     * @return 1 = 新建；0 = 已存在（呼叫端據此 ACK，不是錯誤）
     */
    @Insert("""
            INSERT INTO logistics_receivables (
                id, store_id, order_no, shipment_id, provider, provider_delivery_ref,
                estimated_cost, platform_subsidy, target_net, carried_residual,
                status, charge_basis_frozen_at, idempotency_key
            ) VALUES (
                #{e.id}, #{e.storeId}, #{e.orderNo,jdbcType=VARCHAR}, #{e.shipmentId,jdbcType=BIGINT},
                #{e.provider}, #{e.providerDeliveryRef,jdbcType=VARCHAR},
                #{e.estimatedCost}, #{e.platformSubsidy}, #{e.targetNet}, #{e.carriedResidual},
                #{pending}, #{e.chargeBasisFrozenAt}, #{e.idempotencyKey}
            )
            ON CONFLICT (idempotency_key) DO NOTHING
            """)
    int insertIfAbsentWithStatus(@Param("e") LogisticsReceivableEntity entity,
                                 @Param("pending") String pending);

    /** {@link #insertIfAbsentWithStatus} 的具名入口：初始狀態一律 PENDING，碼值來自 CodeMeta。 */
    default int insertIfAbsent(LogisticsReceivableEntity entity) {
        return insertIfAbsentWithStatus(entity, LogisticsReceivableStatus.PENDING.value());
    }

    @Select("SELECT * FROM logistics_receivables WHERE id = #{id}")
    LogisticsReceivableEntity findById(@Param("id") Long id);

    @Select("SELECT * FROM logistics_receivables WHERE idempotency_key = #{key}")
    LogisticsReceivableEntity findByIdempotencyKey(@Param("key") String idempotencyKey);

    @Select("""
            SELECT * FROM logistics_receivables
            WHERE store_id = #{storeId}
            ORDER BY created_at DESC
            LIMIT #{limit} OFFSET #{offset}
            """)
    List<LogisticsReceivableEntity> listByStore(@Param("storeId") Long storeId,
                                                @Param("limit") int limit,
                                                @Param("offset") int offset);

    @Select("SELECT COUNT(*) FROM logistics_receivables WHERE store_id = #{storeId}")
    long countByStore(@Param("storeId") Long storeId);

    /** 執行者：ADM 例外佇列（UNCONFIRMED／FAILED）。狀態值由呼叫端給，SQL 不持有任何碼值。 */
    @Select("""
            <script>
            SELECT * FROM logistics_receivables
            WHERE status IN
              <foreach collection="statuses" item="s" open="(" separator="," close=")">#{s}</foreach>
            ORDER BY created_at
            LIMIT #{limit}
            </script>
            """)
    List<LogisticsReceivableEntity> listByStatus(@Param("statuses") List<String> statuses,
                                                 @Param("limit") int limit);

    /**
     * 🔴 CAS ＋ allowed-from 的<b>唯一</b>入口。
     *
     * <p>{@code allowedFrom} 一律由 {@link ReceivableStatusRules} 給；本方法自己不知道任何
     * 狀態值，所以「SQL 裡另有一份 allowed-from」這件事在結構上不可能發生。
     *
     * @return 1 = 推進成功；0 = 這筆已經不在允許的來源狀態。
     *         🔴 呼叫端<b>不得</b>把 0 讀成「別人先做過了」就靜默放行——那正是 allowed-from
     *         漏起點時的症狀。
     */
    default int transitionStatus(Long id, LogisticsReceivableStatus to) {
        return transitionStatusTo(id, to.value(), ReceivableStatusRules.allowedFromValues(to));
    }

    @Update("""
            <script>
            UPDATE logistics_receivables
            SET status = #{toStatus},
                updated_at = NOW()
            WHERE id = #{id}
              AND status IN
              <foreach collection="allowedFrom" item="s" open="(" separator="," close=")">#{s}</foreach>
            </script>
            """)
    int transitionStatusTo(@Param("id") Long id,
                           @Param("toStatus") String toStatus,
                           @Param("allowedFrom") List<String> allowedFrom);

    /**
     * 🔴 提交 worker 的<b>租約式</b>搶佔（10 檔 §29）。
     *
     * <p>沒有它的後果很具體：兩個 pod 同時撈到同一列 PENDING ⇒ 各自生一個 TSID {@code trade_no}
     * ⇒ 各自 pre-commit ⇒ <b>各自呼叫 PayUni</b> ⇒ 同一筆應收被收兩次錢。
     * ⚠️ {@code trade_no} 的 UNIQUE 擋不住（兩個 pod 的 trade_no 本來就不同）；
     * ⚠️ {@code PENDING → SUBMITTED} 的 CAS 也擋不住（它發生在呼叫 PayUni <b>之後</b>）。
     *
     * <p>🔴 租約基準 {@code NOW() + make_interval(secs => …)} 在 UPDATE 內<b>逐筆</b>求值，
     * 不由呼叫端在批次迴圈外算一次。
     *
     *
     * <p>⚠️ <b>實測出來的限制，寫在這裡而不是假裝它不存在</b>：{@code NOW()} 在 PostgreSQL 是
     * <b>交易開始時間</b>，所以<b>同一次 claim 撈到的整批列共用同一個租約起點</b>
     * （實測：{@code batchSize=10} 一次撈 2 筆，兩筆的 {@code *_lease_until} 位元相同）。
     * ⇒ 本方法保證的是「租約起點由 DB 在 claim 當下產生」，<b>不是</b>「每一列各自從它被處理的
     * 那一刻開始算」。不變式因此是：
     * <pre>批次內所有列的處理總耗時 &lt; 租約長度</pre>
     * 🔴 呼叫端必須二選一，否則 10 檔 §29 警告的那個退化就在這裡發生：
     * ① {@code batchSize = 1}（不變式回到「單筆最壞耗時 &lt; 租約」，最穩）；
     * ② 租約長度 &gt; {@code batchSize × 單筆最壞耗時}（含每個外部呼叫的 deadline）。
     * <b>兩者都不做就是「隨 batch size 一起破」。</b>
     * <p>⚠️ {@code FOR UPDATE SKIP LOCKED} 讓並行的第二個 pod 直接跳過被壓住的列而不等待；
     * 少了它，兩個 pod 會排隊拿到同一批候選。
     *
     * <p>⚠️ 用 {@code @Select} 執行 {@code UPDATE … RETURNING} 是本 repo 既有寫法
     * （{@code ProductImportItemMapper.adjustBlockingErrorCount}）：PgJDBC 對它回 ResultSet。
     */
    default List<LogisticsReceivableEntity> claimPendingForSubmit(int leaseSeconds, int batchSize) {
        return claimForSubmitByStatus(LogisticsReceivableStatus.PENDING.value(), leaseSeconds, batchSize);
    }

    @Select("""
            UPDATE logistics_receivables
            SET submit_lease_until = NOW() + make_interval(secs => #{leaseSeconds}),
                updated_at = NOW()
            WHERE id IN (
                SELECT r.id
                FROM logistics_receivables r
                WHERE r.status = #{pending}
                  AND (r.submit_lease_until IS NULL OR r.submit_lease_until <= NOW())
                ORDER BY r.created_at
                LIMIT #{batchSize}
                FOR UPDATE SKIP LOCKED
            )
            RETURNING *
            """)
    List<LogisticsReceivableEntity> claimForSubmitByStatus(@Param("pending") String pending,
                                                           @Param("leaseSeconds") int leaseSeconds,
                                                           @Param("batchSize") int batchSize);

    /**
     * 提前歸還租約（送出流程走完或明確失敗時），讓該列立刻回到候選集合而不必等租約自然到期。
     *
     * <p>⚠️ 這是<b>最佳化</b>不是正確性來源：不呼叫它也只是多等一個租約，呼叫它不得成為
     * 任何不變式的前提（10 檔 §29「租約不是鎖」）。
     */
    @Update("""
            UPDATE logistics_receivables
            SET submit_lease_until = NULL,
                updated_at = NOW()
            WHERE id = #{id}
            """)
    int releaseSubmitLease(@Param("id") Long id);

    /**
     * 對帳回填。
     *
     * <p>🔴 {@code actual_cost} 與 {@code statement_line_id} <b>必須同時寫</b>
     * （{@code chk_logrecv_recon_pair} 是雙向等式，單邊寫會被 DB 擋下）。
     *
     * @return 1 = 回填成功；0 = 已經對過帳（{@code actual_cost} 非 NULL）
     */
    @Update("""
            UPDATE logistics_receivables
            SET actual_cost = #{actualCost},
                statement_line_id = #{statementLineId},
                updated_at = NOW()
            WHERE id = #{id}
              AND actual_cost IS NULL
            """)
    int applyTrueUp(@Param("id") Long id,
                    @Param("actualCost") Long actualCost,
                    @Param("statementLineId") Long statementLineId);

    /**
     * 寫入責任成因的 audit snapshot（{@code V15}）。
     *
     * <p>🔴 <b>{@code AND responsibility_cause IS NULL} 不可省</b>：它讓這個寫入變成
     * <b>一次性</b>的。少了它，N16 重投會把既有快照無聲覆蓋掉；而若兩次投遞帶了不同的成因
     * （代表 producer 側的 finality 不變式壞了），後到的那個會直接勝出而沒有任何人知道。
     * 有了它，第二次寫入 {@code rows = 0}，呼叫端才有機會分辨「重投（值相同）」與
     * 「衝突（值不同）」——兩者的處置完全相反。
     *
     * <p>⚠️ 本方法<b>不</b>建立應收，也不改金額與狀態。它只補一個稽核欄
     * （70 §3.6.2：這一欄是 snapshot，不是 responsibility 的 SSOT）。
     *
     * @return 影響行數；{@code 0} ＝ 該列已經有成因（值相同或不同，由呼叫端讀回判斷）
     */
    @Update("""
            UPDATE logistics_receivables
            SET responsibility_cause = #{responsibilityCause},
                updated_at = NOW()
            WHERE id = #{id}
              AND responsibility_cause IS NULL
            """)
    int snapshotResponsibilityCause(@Param("id") Long id,
                                    @Param("responsibilityCause") String responsibilityCause);

    /**
     * 🔴 <b>對帳關聯鍵</b>查詢：{@code provider_settlement_lines.provider_shipment_ref}
     * ↔ {@code logistics_receivables.provider_delivery_ref}（V9 欄註解逐字「對帳的關聯鍵」）。
     *
     * <p>🔴 <b>回 List 不回單筆，而且那不是防禦性寫法</b>：該欄<b>沒有</b> UNIQUE
     * （V9 只有 partial index {@code idx_logrecv_delivery_ref}）⇒ 寫成單筆回傳，
     * 多筆命中時會靜默取到「任意一筆」，而症狀是<b>帳單金額對到了錯的賣家</b>。
     * 同形教訓：sqlc 的 {@code :one} 命中多列取任意一列（零錯誤零 log）。
     * 由呼叫端決定「命中不是 1 筆」要怎麼辦（W4：進爭議，不猜）。
     */
    @Select("""
            SELECT * FROM logistics_receivables
            WHERE provider_delivery_ref = #{providerDeliveryRef}
            ORDER BY created_at
            """)
    List<LogisticsReceivableEntity> findByProviderDeliveryRef(
            @Param("providerDeliveryRef") String providerDeliveryRef);

    /** 回填 58 號憑證關聯（方向二：平台 → 賣家）。 */
    @Update("""
            UPDATE logistics_receivables
            SET invoice_document_id = #{invoiceDocumentId},
                updated_at = NOW()
            WHERE id = #{id}
              AND invoice_document_id IS NULL
            """)
    int bindInvoiceDocument(@Param("id") Long id,
                            @Param("invoiceDocumentId") Long invoiceDocumentId);

    /** 診斷用：目前被租約壓住、且租約尚未到期的列數（給告警看「claim 是不是卡住了」）。 */
    @Select("""
            SELECT COUNT(*) FROM logistics_receivables
            WHERE status = #{pending}
              AND submit_lease_until IS NOT NULL
              AND submit_lease_until > #{now}
            """)
    long countLeasedPending(@Param("pending") String pending, @Param("now") LocalDateTime now);
}
