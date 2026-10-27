package io.github.will413028.hawkr.shipping.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.List;

/**
 * shipments 表的自訂 SQL 與 ProviderCallStatus 狀態機 transition 入口。
 *
 * <p>所有狀態機 transition 都帶 WHERE 前置狀態，作為樂觀鎖兼狀態合法性檢查。
 * <p>禁用 {@code updateById()} 全欄位覆蓋（06 §2.4 INV-3）。
 *
 * <p>🔴 <b>任何寫 {@code status} 的 SQL 必須同句推進 {@code status_entered_at}</b>（65 §7.1.1.5，V11 ②）：
 * 它是 aging 指標的唯一錨，不寫就永遠停在建單當下的值——而那是<b>靜默</b>的，
 * 沒有 CHECK、沒有測試、沒有錯誤訊息會告訴你 aging 是假的。
 * 寫法一律 {@code status_entered_at = CASE WHEN status IS DISTINCT FROM <新值> THEN NOW() ELSE status_entered_at END}
 * （用 CASE 而非裸 {@code NOW()}：同值重寫不該重置「進入狀態的時間」）。
 * 目前的三個寫入者：{@code updateShipmentStatus}、{@code markLabelPrintSucceededByWebhook}、
 * {@code markTcatCallSucceeded}。
 */
public interface ShipmentMapper extends BaseMapper<ShipmentEntity> {

    /**
     * 標記訂單已確認收貨（純 MQ，無外部呼叫）
     */
    @Update("""
        UPDATE shipments
        SET receipt_confirmed = TRUE
        WHERE order_no = #{orderNo}
          AND receipt_confirmed = FALSE
        """)
    int markReceiptConfirmed(@Param("orderNo") String orderNo);

    /**
     * 物流貨態狀態更新（Webhook / queryAndSyncStatus 共用）
     * 併發保護：WHERE status = #{currentStatus}（樂觀鎖）
     */
    @Update("""
        UPDATE shipments
        SET status = #{newStatus},
            status_entered_at = CASE WHEN status IS DISTINCT FROM #{newStatus} THEN NOW() ELSE status_entered_at END,
            -- 🔴 provider 欄位用 COALESCE：**我方的狀態轉換不得抹掉 provider 說過的話**。
            --    confirmCvsShipment 走這一句時傳的是 (null, null, null)——它對 provider
            --    沒有新資訊，而原本的無條件覆寫會把三欄一起清空。
            --    本波剛開始保存的未知貨態 raw 值（persistUnknownProviderStatus）
            --    會被下一次「賣家確認出貨」整組清掉 ⇒ ADM 看到空欄，
            --    會以為 provider 從沒回報過。修一半比不修更容易造成誤信。
            --    ⚠️ 真的要清空時傳空字串，不要傳 null——兩者語意不同：
            --    null ＝「我沒有新資訊」，'' ＝「明確清空」。
            provider_status = COALESCE(#{providerStatus}, provider_status),
            provider_message = COALESCE(#{providerMessage}, provider_message),
            provider_status_time = COALESCE(#{providerStatusTime}, provider_status_time),
            updated_at = NOW()
        WHERE id = #{id}
          AND status = #{currentStatus}
        """)
    int updateShipmentStatus(
            @Param("id") Long id,
            @Param("currentStatus") String currentStatus,
            @Param("newStatus") String newStatus,
            @Param("providerStatus") String providerStatus,
            @Param("providerMessage") String providerMessage,
            @Param("providerStatusTime") LocalDateTime providerStatusTime
    );

    // ====================================================================
    // ProviderCallStatus — label_print_status 狀態機（§2.4.5.1）
    // ====================================================================

    /**
     * 列印 acquireRequested：transition (NONE | FAILED) → REQUESTED
     * <p>rows = 0 表示已被另一執行緒占用或已是 SUCCEEDED → ProviderCallExecutor 拋 ProviderCallBusyException
     */
    @Update("""
        UPDATE shipments
        SET label_print_status = '4102',
            label_print_requested_at = NOW(),
            updated_at = NOW()
        WHERE id = #{id}
          AND label_print_status IN ('4101', '4104')
        """)
    int acquireLabelPrintRequested(@Param("id") Long id);

    /**
     * Print Webhook 回填三段碼 + transition X → SUCCEEDED + status=920
     * <p>WHERE 條件涵蓋三種 transition 來源：
     * <ul>
     *   <li>4102 (REQUESTED)：常態 webhook 路徑</li>
     *   <li>4101 (NONE)：reconcile 補拉但 PayUni 端已有代碼 + 本地未發起列印</li>
     *   <li>4104 (FAILED)：賣家曾失敗但 PayUni 後續產生代碼，reconcile 主動撈到 → 直接補成 SUCCEEDED</li>
     * </ul>
     * <p>排除 4103 SUCCEEDED 終態（重複 webhook 走 idempotent skip 路徑）。
     */
    @Update("""
        UPDATE shipments
        SET partner_id = #{partnerId},
            odno = #{odno},
            validation_no = #{validationNo},
            label_print_status = '4103',
            processing_deadline = NULL,
            -- 🔴 終態（11 已取貨／41 已取消）不得被 Print 回填復活成 920。
            --    WHERE 只守 label_print_status，而這一句 SET 的是 status ——
            --    一張「取消後才收到 Print 回調」的單子會被推回「已取得代碼」，
            --    那正是 BLOCKER 3 要消滅的「較後階段被舊 action 推回較前階段」。
            --    ⚠️ 用 CASE 而不是把終態排除在 WHERE 外：三段碼仍要落庫（那是 provider 證據），
            --    只有狀態不准動。整列不寫等於連證據一起丟。
            status = CASE WHEN status IN ('11', '41') THEN status ELSE #{status} END,
            status_entered_at = CASE
                WHEN status IN ('11', '41') THEN status_entered_at
                WHEN status IS DISTINCT FROM #{status} THEN NOW()
                ELSE status_entered_at END,
            updated_at = NOW()
        WHERE id = #{id}
          AND label_print_status IN ('4102', '4101', '4104')
        """)
    int markLabelPrintSucceededByWebhook(
            @Param("id") Long id,
            @Param("partnerId") String partnerId,
            @Param("odno") String odno,
            @Param("validationNo") String validationNo,
            @Param("status") String status
    );

    /**
     * 宅配 PDF 取號成功：transition REQUESTED → SUCCEEDED + 寫入 PDF 欄位
     * <p>label_expire_at = NOW() + 24h（前端按鈕判斷用）
     */
    @Update("""
        UPDATE shipments
        SET label_object_key = #{labelObjectKey},
            label_asset_id = #{labelAssetId},
            label_expire_at = NOW() + INTERVAL '24 hours',
            label_print_status = '4103',
            updated_at = NOW()
        WHERE id = #{id}
          AND label_print_status = '4102'
        """)
    int markLabelPrintSucceededHome(
            @Param("id") Long id,
            @Param("labelObjectKey") String labelObjectKey,
            @Param("labelAssetId") Long labelAssetId
    );

    /**
     * 配送完成後清空 PDF 欄位（#2288）。
     * <p>
     * 必須與 {@code ShipmentLabelService.unlinkLabel} 成對執行。解除引用之後，
     * media-service 的 GC 會在 7 天後把 R2 上的 PDF 刪掉（那是既定政策），
     * 但欄位若留著就變成指向不存在物件的死指標——而 {@code buildDetailView}
     * 在配送完成後仍會回傳 labelKey，Gateway 照樣組出 URL，前端拿到 404。
     * <p>
     * 一併清掉 {@code label_expire_at}：它的語意是「PDF 還能下載到什麼時候」，
     * key 都沒了，留一個未來時間點會讓前端誤判按鈕還能按。
     * <p>
     * 不動 {@code label_print_status}——那是列印流程的狀態，不是檔案的狀態，
     * 清掉會讓「這張單曾經成功取號」的事實消失。
     */
    @Update("""
        UPDATE shipments
        SET label_object_key = NULL,
            label_asset_id = NULL,
            label_expire_at = NULL,
            updated_at = NOW()
        WHERE id = #{id}
        """)
    int clearLabelAfterDelivery(@Param("id") Long id);

    /**
     * 列印 markFailed：transition REQUESTED → FAILED + 落地 provider 失敗代碼
     * （V12 起寫 provider_error_*，不再污染 provider_status 這個貨態證據欄）
     */
    @Update("""
        UPDATE shipments
        SET label_print_status = '4104',
            provider_error_code = #{errorCode},
            provider_error_message = #{errorMessage},
            provider_error_at = NOW(),
            updated_at = NOW()
        WHERE id = #{id}
          AND label_print_status = '4102'
        """)
    int markLabelPrintFailed(
            @Param("id") Long id,
            @Param("errorCode") String errorCode,
            @Param("errorMessage") String errorMessage
    );

    // ====================================================================
    // ProviderCallStatus — tcat_call_status 狀態機（§2.4.5.2）
    // ====================================================================

    /**
     * 黑貓 acquireRequested：transition (NONE | FAILED) → REQUESTED + 寫入賣家備註
     */
    @Update("""
        UPDATE shipments
        SET tcat_call_status = '4202',
            tcat_call_requested_at = NOW(),
            is_contact = #{isContact},
            is_trolley = #{isTrolley},
            memo = #{memo},
            updated_at = NOW()
        WHERE id = #{id}
          AND tcat_call_status IN ('4201', '4204')
        """)
    int acquireTcatCallRequested(
            @Param("id") Long id,
            @Param("isContact") String isContact,
            @Param("isTrolley") String isTrolley,
            @Param("memo") String memo
    );

    /**
     * 黑貓 markSucceeded：transition REQUESTED → SUCCEEDED + status=210 + tcat_call_time
     */
    @Update("""
        UPDATE shipments
        SET tcat_call_status = '4203',
            tcat_call_time = #{tcatCallTime},
            processing_deadline = NULL,
            -- 🔴 終態（11 已取貨／41 已取消）不得被叫件成功回填復活成 210。
            --    WHERE 只守 tcat_call_status，而這一句 SET 的是 status。
            --    QA 在 dev 實測到的 11→210 倒退，Java 層的 require() 只收窄成 TOCTOU 窗口：
            --    require 用的是呼叫當下的 in-memory entity，而這一句在打完 PayUni（事務外、
            --    跨網路、數秒）之後才執行——webhook 恰好在那期間到達正是最可能被打中的時候。
            --    ⚠️ 姊妹 SQL markLabelPrintSucceededByWebhook 已有同一道守衛；
            --    同一份設計裡兩個相鄰決策方向相反，本身就是訊號。
            status = CASE WHEN status IN ('11', '41') THEN status ELSE #{status} END,
            status_entered_at = CASE
                WHEN status IN ('11', '41') THEN status_entered_at
                WHEN status IS DISTINCT FROM #{status} THEN NOW()
                ELSE status_entered_at END,
            updated_at = NOW()
        WHERE id = #{id}
          AND tcat_call_status = '4202'
        """)
    int markTcatCallSucceeded(
            @Param("id") Long id,
            @Param("tcatCallTime") LocalDateTime tcatCallTime,
            @Param("status") String status
    );

    /**
     * 黑貓 markFailed：transition REQUESTED → FAILED + 落地 provider 失敗代碼
     * （V12 起寫 provider_error_*，不再污染 provider_status 這個貨態證據欄）
     */
    @Update("""
        UPDATE shipments
        SET tcat_call_status = '4204',
            provider_error_code = #{errorCode},
            provider_error_message = #{errorMessage},
            provider_error_at = NOW(),
            updated_at = NOW()
        WHERE id = #{id}
          AND tcat_call_status = '4202'
        """)
    int markTcatCallFailed(
            @Param("id") Long id,
            @Param("errorCode") String errorCode,
            @Param("errorMessage") String errorMessage
    );

    // ====================================================================
    // 出貨照片 / Webhook 通用
    // ====================================================================

    /**
     * 出貨照片獨立更新
     */
    @Update("""
        UPDATE shipments
        SET shipment_photos = #{shipmentPhotos, typeHandler=io.github.will413028.hawkr.shipping.config.typehandler.ShipmentPhotoListTypeHandler},
            updated_at = NOW()
        WHERE id = #{id}
        """)
    int updateShipmentPhotos(
            @Param("id") Long id,
            @Param("shipmentPhotos") List<ShipmentEntity.ShipmentPhoto> shipmentPhotos
    );

    // ====================================================================
    // §5.19 Reconciliation 補收排程
    // ====================================================================

    /**
     * §5.19.3.A 物流貨態補收候選人
     */
    @Select("""
        SELECT *
        FROM shipments
        WHERE status NOT IN ('11', '41')
          AND shipping_method <> '3003'
          AND ship_trade_no IS NOT NULL
          AND created_at >= #{maxAgeCutoff}
          AND (last_reconciled_at IS NULL OR last_reconciled_at < #{backoffCutoff})
        ORDER BY last_reconciled_at NULLS FIRST, created_at
        LIMIT #{limit}
        """)
    List<ShipmentEntity> findReconcileCandidates(
            @Param("maxAgeCutoff") LocalDateTime maxAgeCutoff,
            @Param("backoffCutoff") LocalDateTime backoffCutoff,
            @Param("limit") int limit
    );

    /**
     * §5.19.3.A issue #2294 L2：貨態補收的 <b>lease claim</b>。
     *
     * <p>述詞與 {@link #findReconcileCandidates} 的租約條件同形（{@code last_reconciled_at} 既有欄，
     * 零 migration）；{@code SET last_reconciled_at = #{now}} 讓該列退出候選集合
     * {@code backoff-minutes}（預設 15 分）＝有效租約長度，到期自動回來。
     *
     * <p>⚠️ 與 {@link #touchLastReconciledAt} <b>是兩支</b>：後者的唯一寫入點在
     * {@code ShippingService.queryAndSyncStatus} 的 finally，而該方法同時被 §5.7 按需查詢 API 共用，
     * 把它改成 CAS 會讓 API 路徑的退避刷新靜默失效。此處刻意「另加一支」而非改既有那支。
     */
    @Update("""
        UPDATE shipments
        SET last_reconciled_at = #{now}
        WHERE id = #{id}
          AND (last_reconciled_at IS NULL OR last_reconciled_at < #{backoffCutoff})
        """)
    int claimForReconcile(@Param("id") Long id,
                          @Param("now") LocalDateTime now,
                          @Param("backoffCutoff") LocalDateTime backoffCutoff);

    /**
     * §5.19.3.B 列印狀態機補收候選人（REQUESTED 過久 / FAILED 待重試）
     *
     * <p>issue #2294 L3：加上 {@code label_reconcile_next_at} 租約述詞——租約未到期的列由別的 pod
     * 持有，本輪不撈。⚠️ 刻意<b>不用</b> {@code label_print_requested_at} 當租約：它是業務時戳
     * （「何時送印」），推它同時會移動 {@code requestedBefore} grace 與 {@code maxAgeCutoff}
     * 兩個判準的相對位置，該列的候選壽命會被無限延長。
     */
    @Select("""
        SELECT *
        FROM shipments
        WHERE label_print_status IN ('4102', '4104')
          AND ship_trade_no IS NOT NULL
          AND label_print_requested_at < #{requestedBefore}
          AND label_print_requested_at >= #{maxAgeCutoff}
          AND (label_reconcile_next_at IS NULL OR label_reconcile_next_at <= #{now})
        ORDER BY label_print_requested_at
        LIMIT #{limit}
        """)
    List<ShipmentEntity> findLabelReconcileCandidates(
            @Param("requestedBefore") LocalDateTime requestedBefore,
            @Param("maxAgeCutoff") LocalDateTime maxAgeCutoff,
            @Param("now") LocalDateTime now,
            @Param("limit") int limit
    );

    /**
     * §5.19.3.B issue #2294 L3：列印補收的 <b>lease claim</b>。
     *
     * <p>{@code SET label_reconcile_next_at = #{leaseUntil}}（= claim 當下 + 10 分鐘 = 一個 tick），
     * affected=1 才可以打 PayUni 查詢與後續 publish。租約 10 分鐘 ≪ 候選窗口 7 天
     * （{@code label.max-age-days}）⇒ 窗口內仍有約 1000 次重試機會，不會退化成 at-most-once。
     */
    @Update("""
        UPDATE shipments
        SET label_reconcile_next_at = #{leaseUntil}
        WHERE id = #{id}
          AND label_print_status IN (#{statusRequested}, #{statusFailed})
          AND (label_reconcile_next_at IS NULL OR label_reconcile_next_at <= #{now})
        """)
    int claimLabelReconcile(@Param("id") Long id,
                            @Param("statusRequested") String statusRequested,
                            @Param("statusFailed") String statusFailed,
                            @Param("now") LocalDateTime now,
                            @Param("leaseUntil") LocalDateTime leaseUntil);

    /**
     * §5.19.3.C 黑貓 REQUESTED 卡死候選人（ops 告警，不主動對帳）
     *
     * <p>issue #2294 L4：加上 {@code tcat_stuck_alerted_at} <b>冷卻租約</b>述詞。
     * ⚠️ 它不是「已告警旗標」——卡死狀態沒解除就該持續有聲音，所以是會過期的租約。
     */
    @Select("""
        SELECT *
        FROM shipments
        WHERE tcat_call_status = '4202'
          AND tcat_call_requested_at < #{requestedBefore}
          AND tcat_call_requested_at >= #{maxAgeCutoff}
          AND (tcat_stuck_alerted_at IS NULL OR tcat_stuck_alerted_at <= #{now})
        LIMIT #{limit}
        """)
    List<ShipmentEntity> findTcatStuckCandidates(
            @Param("requestedBefore") LocalDateTime requestedBefore,
            @Param("maxAgeCutoff") LocalDateTime maxAgeCutoff,
            @Param("now") LocalDateTime now,
            @Param("limit") int limit
    );

    /**
     * §5.19.3.C issue #2294 L4：黑貓卡死告警的 <b>冷卻租約 claim</b>。
     *
     * <p>🚨 <b>租約長度必須是 1 小時、不是 24 小時</b>：候選述詞的
     * {@code tcat_call_requested_at >= #{maxAgeCutoff}}（{@code max-age-hours} 預設 24）
     * 使候選窗口寬度<b>只有 24 小時</b>。首次告警在 {@code requested_at + 3min} 蓋章，
     * 24 小時租約到期是 {@code requested_at + 24h + 3min}，而該列在 {@code requested_at + 24h}
     * 就已掉出窗口 ⇒ <b>永遠只告警一次</b>，而 {@code publishAnomalyDetected} 在迴圈內無 try/catch，
     * 它一拋例外就是完全無聲。租約與候選窗口等寬 = 假 lease（行為上等同永久旗標）。
     */
    @Update("""
        UPDATE shipments
        SET tcat_stuck_alerted_at = #{leaseUntil}
        WHERE id = #{id}
          AND tcat_call_status = #{statusRequested}
          AND (tcat_stuck_alerted_at IS NULL OR tcat_stuck_alerted_at <= #{now})
        """)
    int claimTcatStuckAlert(@Param("id") Long id,
                            @Param("statusRequested") String statusRequested,
                            @Param("now") LocalDateTime now,
                            @Param("leaseUntil") LocalDateTime leaseUntil);

    /**
     * §5.19 觸碰 last_reconciled_at（不論狀態是否變更皆執行，避免 hot row）
     */
    @Update("""
        UPDATE shipments
        SET last_reconciled_at = #{now}
        WHERE id = #{id}
        """)
    int touchLastReconciledAt(@Param("id") Long id, @Param("now") LocalDateTime now);

    // ====================================================================
    // §5.20 賣家處理期限掃描
    // ====================================================================

    /**
     * §5.20 找出逾期且未處理的正向物流單。
     * 處理判定依新狀態機：
     * - 店到店：label_print_status &lt;&gt; '4103'（含 NONE/REQUESTED/FAILED 視為未完成）
     * - 宅配：tcat_call_status &lt;&gt; '4203'
     */
    @Select("""
        SELECT *
        FROM shipments
        WHERE processing_deadline IS NOT NULL
          AND processing_deadline < #{now}
          AND is_return = FALSE
          AND status <> '41'
          AND (
            (shipping_method = '3002' AND label_print_status <> '4103')
            OR
            (shipping_method = '3001' AND tcat_call_status <> '4203')
          )
        ORDER BY processing_deadline ASC
        LIMIT #{limit}
        """)
    List<ShipmentEntity> findExpiredUnprocessedShipments(
            @Param("now") LocalDateTime now,
            @Param("limit") int limit
    );

    /**
     * §5.20 issue #2294 L1：逾期未處理通知的 <b>lease claim</b>。
     *
     * <p>候選述詞本身就是 {@code processing_deadline IS NOT NULL AND processing_deadline < #{now}}，
     * 所以把 {@code processing_deadline} 推到 {@code leaseUntil}（= claim 當下 + 5 分鐘 = 一個 tick）
     * 就讓該列退出候選集合，租約到期後自動回來。
     *
     * <p>⚠️ <b>不可</b>改成「把 {@link #clearProcessingDeadline} 上移到 publish 之前」——那支是
     * {@code SET processing_deadline = NULL}（永久標記），上移後只要 publish 拋例外或 pod 崩潰，
     * {@code shipment.processing.timeout} 就<b>永久遺失</b>（AfterSales 的系統取消+退款永不觸發、
     * 且無任何告警）。{@code clearProcessingDeadline} 維持在 publish 之後，
     * 它的 {@code IS NOT NULL} 述詞對 leased 值仍然成立，該方法一行都不用改。
     */
    @Update("""
        UPDATE shipments
        SET processing_deadline = #{leaseUntil}
        WHERE id = #{id}
          AND processing_deadline IS NOT NULL
          AND processing_deadline < #{now}
        """)
    int leaseProcessingDeadline(@Param("id") Long id,
                                @Param("now") LocalDateTime now,
                                @Param("leaseUntil") LocalDateTime leaseUntil);

    /**
     * §5.20 清除已通知的 processing_deadline（防止重複掃描）
     */
    @Update("""
        UPDATE shipments
        SET processing_deadline = NULL,
            updated_at = NOW()
        WHERE id = #{id}
          AND processing_deadline IS NOT NULL
        """)
    int clearProcessingDeadline(@Param("id") Long id);

    // ====================================================================
    // PayUni 錯誤訊息落地（callTcat / 其他通用入口；新框架已取代大部分用途）
    // ====================================================================

    /**
     * PayUni 呼叫失敗落地 provider_status / provider_message。
     * 收斂規範 §4.1 SoT。已被 markLabelPrintFailed / markTcatCallFailed 取代大部分用途，
     * 保留供未走狀態機的舊呼叫點使用（例如 createShipment 失敗）。
     */
    @Update("""
        UPDATE shipments
        SET provider_status = #{providerStatus},
            provider_message = #{providerMessage},
            updated_at = NOW()
        WHERE id = #{id}
        """)
    int updateProviderStatus(@Param("id") Long id,
                             @Param("providerStatus") String providerStatus,
                             @Param("providerMessage") String providerMessage);

    // ====================================================================
    // §5.21 配送異常偵測：expected_delivery_date
    // ====================================================================

    /**
     * §5.21.8 寫入 expected_delivery_date（黑貓宅配建單成功 / 超商到店時呼叫）。
     * 僅在 NULL 時寫入（避免覆蓋已存在值）。
     */
    @Update("""
        UPDATE shipments
        SET expected_delivery_date = #{expectedDate},
            updated_at = NOW()
        WHERE id = #{id}
          AND expected_delivery_date IS NULL
        """)
    int setExpectedDeliveryDate(@Param("id") Long id,
                                @Param("expectedDate") LocalDate expectedDate);

    /**
     * 批次查詢：依 order_no + is_return 取每個 order 的「provider 最新回應」。
     * 收斂規範 §6.3 批次聚合（Order / AfterSales 聚合使用）。
     *
     * <p>🔴 <b>這裡刻意把貨態與失敗代碼併成一欄，而那不牴觸 V12 的拆分</b>：
     * 消費端（{@code AfterSalesGrpcService} 的售後對帳摘要）要的是「第三方最後說了什麼」，
     * 那在語意上本來就是兩者的聯集——它是<b>顯示欄</b>，不是異常判準。
     * 拆分要防的是「以有值判定異常」，那個判準在 {@code ShippingAdminQueryService} 上，
     * 錨的是 {@code provider_error_code} 而非本查詢。
     *
     * <p>順序是「貨態優先、失敗代碼兜底」：退貨建單失敗時貨態欄本來就是空的
     * （失敗代碼寫 provider_error_*），不做 COALESCE 的話這個顯示欄會從有值變成空白
     * ——那是 V12 會造成的一個真實資訊迴歸，在這裡收掉。
     */
    @Select({"<script>",
            "SELECT DISTINCT ON (order_no) order_no AS key,",
            "       COALESCE(NULLIF(provider_status, ''), provider_error_code) AS provider_status,",
            "       COALESCE(NULLIF(provider_message, ''), provider_error_message) AS provider_message",
            "FROM shipments",
            "WHERE is_return = #{isReturn}",
            "  AND order_no IN",
            "<foreach collection='orderNos' item='oid' open='(' separator=',' close=')'>#{oid}</foreach>",
            "ORDER BY order_no, created_at DESC",
            "</script>"})
    List<io.github.will413028.hawkr.shipping.dto.ProviderStatusRow> findProviderStatusByOrderNos(
            @Param("orderNos") List<String> orderNos,
            @Param("isReturn") boolean isReturn);
}
