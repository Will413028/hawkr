package io.github.will413028.hawkr.aftersales.mapper;

import com.baomidou.mybatisplus.core.mapper.BaseMapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.entity.projection.SellerCaseRef;
import org.apache.ibatis.annotations.Mapper;
import org.apache.ibatis.annotations.Param;
import org.apache.ibatis.annotations.Select;
import org.apache.ibatis.annotations.Update;

import java.util.List;

@Mapper
public interface AfterSaleCaseMapper extends BaseMapper<AfterSaleCaseEntity> {

    /**
     * 7-11 退貨出貨便代碼冪等回填（§5.3.6）。
     * {@code WHERE partner_id IS NULL} 防止重複寫入與併發覆寫。
     * 限 7-11 退貨且處於 RETURN_PENDING / RETURN_SHIPPING 的活躍案件。
     *
     * @return 更新列數；0 = 已回填過或案件狀態已變更，consumer 應 ACK skip
     */
    @Update("""
            UPDATE aftersale_cases
               SET partner_id = #{partnerId},
                   odno = #{odno},
                   validation_no = #{validationNo}
             WHERE order_no = #{orderNo}
               AND status IN ('20', '25')
               AND return_shipping_method = '3002'
               AND partner_id IS NULL
            """)
    int updateLabelInfo(
            @Param("orderNo") String orderNo,
            @Param("partnerId") String partnerId,
            @Param("odno") String odno,
            @Param("validationNo") String validationNo
    );

    /**
     * 重試 Worker 掃描：尚未回填 + 逾時 5 分鐘 + 預算未用完 + 租約可搶（§5.3.7）。
     *
     * <p><b>issue #2294 L7</b>：刻意<b>移除</b>了 {@code FOR UPDATE SKIP LOCKED}。
     * 它從來沒有保護到任何東西——{@code ReturnLabelRetryWorker.autoRetry()} 沒有
     * {@code @Transactional}，自動提交下 SELECT 一回傳鎖就釋放，兩個 pod 照樣各拿一份
     *（規範《多副本排程搶佔規範》§一 ②）。搶佔改由 {@link #claimLabelRetry} 的 lease CAS 負責。
     *
     * <p>述詞新增 {@code (label_retry_at IS NULL OR label_retry_at <= NOW())}：
     * 這是租約條件，讓「claim 之後崩潰」的案件在租約到期後自動回到候選集合。
     */
    @org.apache.ibatis.annotations.Select("""
            SELECT * FROM aftersale_cases
             WHERE status IN ('20', '25')
               AND return_shipping_method = '3002'
               AND partner_id IS NULL
               AND label_retry_count = 0
               AND (label_retry_at IS NULL OR label_retry_at <= NOW())
               AND ship_trade_no IS NOT NULL
               AND created_at < NOW() - INTERVAL '5 minutes'
             LIMIT #{limit}
            """)
    List<AfterSaleCaseEntity> selectRetryCandidates(@Param("limit") int limit);

    /**
     * issue #2294 L7：自動重試的 <b>lease claim</b>（搶佔閘門，在 gRPC 副作用<b>之前</b>）。
     *
     * <p>把 {@code label_retry_at} 推到 {@code leaseUntil}：affected=1 表示本 pod 搶到，
     * affected=0 表示另一個 pod 已搶走且租約未到期（或預算已用完）。
     *
     * <p>⚠️ <b>這裡刻意不動 {@code label_retry_count}</b>。舊實作用 count 的 0→1 CAS 當閘門，
     * 而候選述詞是 {@code label_retry_count = 0} ⇒ CAS 成功即<b>永久</b>離開候選集合、
     * 沒有任何時間欄能讓它回來 ＝ 規範 §二 第 3 步的永久旗標；副作用（shipping gRPC）在其後
     * 且例外被 catch 吞掉 ⇒ 不必等 pod 崩潰，<b>一次 gRPC 失敗就永久用掉唯一一次自動重試</b>。
     * count 現在只在 gRPC 確認成功之後由 {@link #markLabelRetried} 寫。
     *
     * <p>⚠️ 述詞含非 PK 條件（規則 ⑦）；租約基準由呼叫端在 claim 當下取（§三 第 6 點）。
     */
    @Update("""
            UPDATE aftersale_cases
               SET label_retry_at = #{leaseUntil}
             WHERE case_no = #{caseNo}
               AND label_retry_count = 0
               AND (label_retry_at IS NULL OR label_retry_at <= #{now})
            """)
    int claimLabelRetry(@Param("caseNo") String caseNo,
                        @Param("now") java.time.LocalDateTime now,
                        @Param("leaseUntil") java.time.LocalDateTime leaseUntil);

    /**
     * issue #2294 L7：{@link #claimLabelRetry} 的<b>釋放</b>——shipping gRPC 失敗時把租約設成
     * <b>一個已過去的時間</b>，讓下一個 tick（2 分）就能重來，而不是等滿一個租約（6 分）。
     *
     * <p>CAS 條件是「{@code label_retry_at} 仍等於我剛寫進去的那個值」，不能只用 case_no：
     * 否則會清掉期間別人重新 claim 的租約（規則 ⑦ 的同一個道理）。
     *
     * <p>🚨 <b>釋放成「過去時間」而不是「還原成 claim 之前的舊值」，這是本方法的關鍵。</b>
     * 曾經的寫法是還原舊值，而對一個「從未成功觸發過」的案件那個舊值<b>就是 {@code NULL}</b>
     * ⇒ {@link #selectExhaustedCandidates} 賴以判定耗盡的
     * {@code count = 0 AND label_retry_at IS NOT NULL} 只在<b>租約正被持有的那一小段時間</b>成立，
     * 而失敗路徑的租約在一次 gRPC 往返（秒級）內就被還原、{@code checkExhausted} 每 10 分鐘才取樣
     * ⇒ <b>取樣幾乎永遠命中不了</b>：shipping 長時間打不通時案件每 2 分鐘靜靜重試、
     * {@code LABEL_RETRY_EXHAUSTED} 告警與「請聯繫客服」的 ops 動線永遠不會被觸發
     * —— 正是述詞放寬想避免的結果。「加速恢復」與「保留偵測證據」兩個目標互相抵銷。
     *
     * <p><b>三態語意（設成過去時間之後，本欄的三個狀態才互斥且完整）</b>：
     * <ul>
     *   <li>{@code NULL} ＝ <b>從未嘗試過</b>（新案件，或 ops 透過 {@link #resetLabelRetryCount} 重置）</li>
     *   <li><b>過去</b>時間 ＝ 嘗試過但<b>目前無人持有</b>（可立即搶佔，且已留下「試過」的證據）</li>
     *   <li><b>未來</b>時間 ＝ 正在嘗試中（租約有效，不可搶佔）</li>
     * </ul>
     * 兩個消費述詞都同時滿足：{@code label_retry_at <= NOW()} ⇒ 立刻可重新搶佔；
     * {@code label_retry_at IS NOT NULL} ⇒ 耗盡掃描看得到。
     *
     * <p>⚠️ <b>這不是搶佔正確性的一部分，是失敗恢復的加速</b>：規範 §二 第 5 步明說
     * 「catch 裡有回退不算回頭路」——進程消失時沒有任何 catch 會跑。租約本身仍是唯一的正確性保證。
     * 而「進程消失」的情形租約自然到期，同樣落在「過去時間」這一態，兩條路徑語意一致。
     *
     * @param expectedLeaseUntil claim 當時寫進去的租約值；不相符即放棄釋放（回 0）
     * @param expiredAt          釋放後要寫入的<b>過去</b>時間；呼叫端傳 claim 當下取的 {@code now}
     *                           （必定早於本次釋放時刻），與 {@link #claimLabelRetry} 的
     *                           {@code #{now}} 維持<b>同一個時鐘基準</b>，不混用 DB {@code NOW()}
     */
    @Update("""
            UPDATE aftersale_cases
               SET label_retry_at = #{expiredAt}
             WHERE case_no = #{caseNo}
               AND label_retry_at = #{expectedLeaseUntil}
            """)
    int releaseLabelRetryLease(@Param("caseNo") String caseNo,
                               @Param("expectedLeaseUntil") java.time.LocalDateTime expectedLeaseUntil,
                               @Param("expiredAt") java.time.LocalDateTime expiredAt);

    /**
     * CAS 更新 retry count：從 0 → 1，確保只觸發一次自動重試。
     *
     * <p><b>issue #2294 L7</b>：本方法已從「搶佔閘門」降為「<b>已確認觸發</b>的事後記錄」——
     * 只有在 shipping {@code RetryReturnPrintLabel} 回報成功之後才呼叫。
     * 搶佔改由 {@link #claimLabelRetry} 負責。
     */
    @Update("""
            UPDATE aftersale_cases
               SET label_retry_count = 1
             WHERE case_no = #{caseNo}
               AND label_retry_count = 0
            """)
    int markLabelRetried(@Param("caseNo") String caseNo);

    /**
     * 異常掃描：總計超過 10 分鐘仍未回填且預算尚未標記耗盡，待 ops 介入（§5.3.7）。
     *
     * <p><b>issue #2294 L7</b>：述詞從 {@code label_retry_count = 1} 放寬為
     * 「{@code count = 1} <b>或</b>（{@code count = 0} 且 {@code label_retry_at IS NOT NULL}）」。
     * 理由：修復後 count 只在 gRPC 確認成功後才寫，所以「shipping 持續打不通」的案件會一直
     * 停在 <b>count = 0</b>；不放寬的話它每 2 分鐘靜靜重試到天荒地老、而
     * {@code LABEL_RETRY_EXHAUSTED} 告警<b>永遠不會發</b>——那是拿修復換掉一個既有的偵測能力。
     *
     * <p>⚠️ 第二個分支<b>必須帶 {@code label_retry_at IS NOT NULL}</b>，不能只寫
     * {@code count IN (0,1)}：{@code label_retry_at IS NULL} 的意思是「這個案件<b>一次都還沒被
     * 搶佔過</b>」（例如服務停機超過 10 分鐘期間堆積的案件）。把那種案件直接判成耗盡，等於
     * 讓它一次自動重試都沒做就丟給 ops——比修復前更差。有了這個條件，只有「試過但沒能確認觸發」
     * 才算耗盡，與修復前的語意一致。
     *
     * <p>🚨 <b>這條述詞成立的前提是「{@code label_retry_at} 一旦被 claim 過就不再回到 {@code NULL}」</b>
     *（唯一的例外是 ops 的 {@link #resetLabelRetryCount}，那本來就是「重新給一次機會」）。
     * 該不變式由 {@link #releaseLabelRetryLease} 保證——它釋放成<b>過去時間</b>而非還原成舊值。
     * 若有人把釋放改回「還原成 claim 之前的值」，本述詞的第二個分支會退化成
     * <b>只在租約被持有的那幾秒內成立</b>，而取樣間隔是 10 分鐘 ⇒ 本告警實質失效。
     * 覆蓋這條的是 {@code AfterSaleLabelRetryLeaseIT.exhausted_stillDetectedAfterLeaseRelease}。
     */
    @org.apache.ibatis.annotations.Select("""
            SELECT * FROM aftersale_cases
             WHERE status IN ('20', '25')
               AND return_shipping_method = '3002'
               AND partner_id IS NULL
               AND (label_retry_count = 1
                    OR (label_retry_count = 0 AND label_retry_at IS NOT NULL))
               AND created_at < NOW() - INTERVAL '10 minutes'
             LIMIT #{limit}
            """)
    List<AfterSaleCaseEntity> selectExhaustedCandidates(@Param("limit") int limit);

    /**
     * ops 人工重試：重置 retry count 讓 Worker 在必要時可再次介入。
     *
     * <p><b>issue #2294 L7</b>：同時清掉 {@code label_retry_at} 租約。
     * 只歸零 count 而留著一個未到期的租約，會讓 ops 的救濟延遲到租約到期才生效——
     * 規範 §二 第 6 步「救濟路徑收不收得到這個狀態」講的就是這種落差。
     *
     * <p>⚠️ 在 {@link #releaseLabelRetryLease} 的三態語意下，清成 {@code NULL} ＝ 把案件送回
     * 「<b>從未嘗試過</b>」。這正是 ops 救濟要的語意：重新給一次完整的自動重試機會，且在它
     * 真的被重新 claim 之前<b>不會</b>再被 {@link #selectExhaustedCandidates} 判成耗盡
     *（否則 ops 一按重試就立刻又被標記耗盡，救濟等於沒有發生）。
     * 這是全系統<b>唯一</b>可以讓本欄回到 {@code NULL} 的路徑。
     */
    @Update("""
            UPDATE aftersale_cases
               SET label_retry_count = 0,
                   label_retry_at = NULL
             WHERE case_no = #{caseNo}
            """)
    int resetLabelRetryCount(@Param("caseNo") String caseNo);

    /**
     * 將自動重試已耗盡（仍未回填）的 case 標記為 2，
     * 讓 {@link ReturnLabelRetryWorker} 不再每輪重複記 {@code LABEL_RETRY_EXHAUSTED} audit log，
     * 同時也把它移出 {@link #selectRetryCandidates}（該述詞要求 count = 0）。
     * ops 透過 {@link #resetLabelRetryCount} 會把它重新歸零。
     *
     * <p><b>issue #2294 L7</b>：述詞與 {@link #selectExhaustedCandidates} <b>逐字同步</b>放寬——
     * 否則「count 停在 0 的持續失敗案件」被撈到卻標不動，每 10 分鐘重複記一次 audit log；
     * 而少了 {@code label_retry_at IS NOT NULL} 又會把「從未被搶佔過」的案件也標成耗盡。
     * 兩支述詞不一致的任何一個方向都會壞，所以它們必須一起改。
     */
    @Update("""
            UPDATE aftersale_cases
               SET label_retry_count = 2
             WHERE case_no = #{caseNo}
               AND (label_retry_count = 1
                    OR (label_retry_count = 0 AND label_retry_at IS NOT NULL))
            """)
    int markLabelRetryExhausted(@Param("caseNo") String caseNo);

    // ==================== §10 申訴與平台裁決 ====================

    /**
     * 買家提交申訴：樂觀鎖 CAS，條件必須 status IN (REJECTED(60), AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33))
     * + appeal_state=NONE（'0'）。2026-06-15 status 集合擴 33（待寄回逾時自動關案後可申訴）。
     * 同步寫入 appeal_reason / appeal_description（§10.6.6 Admin 詳情 API 直讀）。
     * 影響列數 = 1 成功；= 0 為狀態變更或他人已申訴（§10.8.1）。
     */
    @Update("""
            UPDATE aftersale_cases
               SET appeal_state = '10',
                   appeal_submitted_at = #{now},
                   appeal_reason = #{appealReason},
                   appeal_description = #{appealDescription}
             WHERE case_no = #{caseNo}
               AND status IN ('60', '33')
               AND appeal_state = '0'
            """)
    int markAppealed(@Param("caseNo") String caseNo,
                     @Param("now") java.time.LocalDateTime now,
                     @Param("appealReason") String appealReason,
                     @Param("appealDescription") String appealDescription);

    /**
     * backend#3262 P0：申訴資格判定的<b>持鎖讀</b>（{@code FOR UPDATE}）。
     *
     * <p><b>為什麼需要它</b>：{@code submitAppeal} 的資格檢查（A1/A2/A3）通過之後，交易還要跑完
     * media gRPC、evidence insert、{@link #markAppealed}、event insert、MQ publish 才 commit。
     * 這段期間該筆申訴對其他連線<b>不可見</b>（MVCC），而撥款閘門
     * {@code AfterSalePayoutHoldService} 讀到的是舊值 ⇒ 可能在「申訴其實已取得資格、只是還沒
     * commit」的當下放行撥款。原本靠 {@code payoutHoldGraceMinutes}（15 分）掩蓋這個窗口，
     * 但那段交易<b>沒有可證明的上界</b>（media gRPC 無 deadline、全 repo 無 statement_timeout、
     * JVM STW 與 pod 排程本質不可界）⇒ 用時間長度證明資金安全不成立。
     *
     * <p><b>本方法建立的是順序保證而非時間保證</b>：資格判定一律在本鎖之後進行，鎖持有到
     * commit / rollback 為止 ⇒ 對任何一筆通過 A3 的申訴，撥款閘門在它落地之前都看得到
     * 「這一列被鎖住」（{@link #tryLockCaseForPayoutRelease} 回 null）⇒ fail-closed 不放行。
     * 鎖釋放後只有兩種結果：申訴已 commit（閘門讀得到 appeal_state='10' ⇒ 無時間界 hold），
     * 或已 rollback（deadline 已過 ⇒ 買家重試會被 A3 擋下 ⇒ 申訴永遠不可能成立）。
     * 兩者都安全，且<b>不依賴任何耗時假設</b>。
     *
     * @return 該筆 case（已持有 row lock）；不存在時 null
     */
    @org.apache.ibatis.annotations.Select("""
            SELECT * FROM aftersale_cases
             WHERE case_no = #{caseNo}
             FOR UPDATE
            """)
    AfterSaleCaseEntity selectByCaseNoForUpdate(@Param("caseNo") String caseNo);

    /**
     * backend#3262 P0：撥款閘門的<b>在途申訴探測</b>（{@code FOR UPDATE SKIP LOCKED}）。
     *
     * <p>撥款閘門只有在「申訴窗（含 grace）已過、準備放行」時才會呼叫本方法。
     * 回 null ＝ 這一列正被別的交易持鎖 ＝ 可能有一筆已通過 A3、尚未 commit 的申訴
     * （見 {@link #selectByCaseNoForUpdate}）⇒ 閘門 fail-closed 不放行，推遲後重問。
     *
     * <p><b>必須是 {@code SKIP LOCKED} 而不是等待或 {@code NOWAIT}</b>：本查詢跑在
     * {@code GrantPaymentWorker} 的租約窗口內（經 5 秒 gRPC deadline）。等待會把持鎖者的耗時
     * 灌進租約下界的算式（《多副本排程搶佔規範》§三 第 5 步）；{@code SKIP LOCKED} 立即返回，
     * 租約不等式不受影響。{@code NOWAIT} 會丟 SQLSTATE 55P03，在 PostgreSQL 上還得處理
     * 「交易內任一句失敗即 aborted」，沒有必要。
     *
     * <p>⚠️ 本方法必須在<b>非交易</b>情境呼叫（autocommit），鎖在語句結束即釋放；
     * 放進交易會讓探測自己變成長時間持鎖者。{@code AfterSalePayoutHoldService} 非
     * {@code @Transactional}，{@code AfterSalesGrpcService} 亦無類別層 {@code @Transactional}。
     *
     * <p>🔴 <b>必須回傳整列而不只是「鎖得到嗎」</b>：閘門的第一次讀（{@code selectList}）是非鎖定
     * 快照，可能<b>比探測時點舊</b>。若只檢查可鎖性，會漏掉這個交錯——
     * ① 閘門讀到 {@code appeal_state='0'}（申訴交易當時還持鎖未 commit）
     * ② 申訴交易 commit、鎖釋放
     * ③ 探測「鎖得到」⇒ 放行撥款，而申訴<b>已經成立</b>。
     * 呼叫端必須拿本方法回傳的<b>新鮮那一列</b>重跑一次資格判定，不得沿用第一次讀到的值。
     *
     * @return 該筆 case 的<b>最新</b>狀態（成功取得鎖 ＝ 當下無人持鎖）；
     *         被別的交易持鎖或該列不存在時 null
     */
    @org.apache.ibatis.annotations.Select("""
            SELECT * FROM aftersale_cases
             WHERE case_no = #{caseNo}
             FOR UPDATE SKIP LOCKED
            """)
    AfterSaleCaseEntity tryLockCaseForPayoutRelease(@Param("caseNo") String caseNo);

    /**
     * Admin 裁決：樂觀鎖 CAS，條件必須 appeal_state=APPEALED（'10'）。
     * targetStatus = '40'（REFUND_PROCESSING）或 '50'（CLOSED）。
     */
    @Update("""
            UPDATE aftersale_cases
               SET appeal_state = '30',
                   appeal_resolved_at = #{now},
                   status = #{targetStatus}
             WHERE case_no = #{caseNo}
               AND appeal_state = '10'
            """)
    int markAppealResolved(@Param("caseNo") String caseNo,
                           @Param("targetStatus") String targetStatus,
                           @Param("now") java.time.LocalDateTime now);

    /**
     * 賣家 REJECT 時更新 last_rejected_at（由 AfterSaleResolutionService 在 reject 分支呼叫）。
     */
    @Update("""
            UPDATE aftersale_cases
               SET last_rejected_at = #{now}
             WHERE case_no = #{caseNo}
            """)
    int stampLastRejectedAt(@Param("caseNo") String caseNo,
                            @Param("now") java.time.LocalDateTime now);

    /**
     * 依 caseNo 取賣家 store_id（scalar projection，回 {@code Long}）。
     * <p>
     * 供退款完成（{@code publishRefundCompleted}）補發賣家工作區 realtime 用——退款任務
     * （{@code RefundTaskEntity}）不帶 storeId，由 caller 以 caseNo 反查。
     * 回 scalar 非 Entity，避免 partial-projection 反模式（docs/開發規範/Java 微服務開發規範.md §9.2）。
     *
     * @return 賣家 store_id；查無 case 回 {@code null}
     */
    @Select("SELECT store_id FROM aftersale_cases WHERE case_no = #{caseNo}")
    Long findStoreIdByCaseNo(@Param("caseNo") String caseNo);

    /**
     * 依 caseNo 取買家 buyer_id（scalar projection，回 {@code Long}）。
     * <p>
     * 供退款完成（{@code publishRefundCompleted}）補上 notify_buyer_account_id 用
     * （issue #1626）——退款任務（{@code RefundTaskEntity}）不帶 buyerId，由 caller 以 caseNo 反查。
     * 回 scalar 非 Entity，避免 partial-projection 反模式（docs/開發規範/Java 微服務開發規範.md §9.2）。
     *
     * @return 買家 buyer_id；查無 case 回 {@code null}
     */
    @Select("SELECT buyer_id FROM aftersale_cases WHERE case_no = #{caseNo}")
    Long findBuyerIdByCaseNo(@Param("caseNo") String caseNo);

    /**
     * Admin 待裁決列表查詢。appealStateValue 為 AfterSaleAppealState Value（'10' / '20' / '30'）；
     * 傳 null 代表不限（'all'，即全部 appeal_state &lt;&gt; '0' 的案件）。
     * 值域收斂在 {@code AfterSaleAppealService#resolveAppealStateFilter}。
     */
    @org.apache.ibatis.annotations.Select("""
            <script>
            SELECT * FROM aftersale_cases
             WHERE appeal_state &lt;&gt; '0'
               <if test="appealStateValue != null">
                 AND appeal_state = #{appealStateValue}
               </if>
             ORDER BY appeal_submitted_at DESC
             LIMIT #{limit} OFFSET #{offset}
            </script>
            """)
    List<AfterSaleCaseEntity> selectAppealsForAdmin(
            @Param("appealStateValue") String appealStateValue,
            @Param("limit") int limit,
            @Param("offset") int offset);

    @org.apache.ibatis.annotations.Select("""
            <script>
            SELECT COUNT(*) FROM aftersale_cases
             WHERE appeal_state &lt;&gt; '0'
               <if test="appealStateValue != null">
                 AND appeal_state = #{appealStateValue}
               </if>
            </script>
            """)
    long countAppealsForAdmin(@Param("appealStateValue") String appealStateValue);

    // ==================== §5.10 賣家處理逾時自動升級 ====================

    /**
     * §5.10 CaseProcessTimeoutWorker：掃描賣家處理逾時且尚未升級的案件。
     * 條件：status IN (PENDING) + appeal_state = NONE + process_deadline 已過 + 尚未升級。
     * 使用 {@code FOR UPDATE SKIP LOCKED} 支援多實例並行掃描不重覆。
     */
    @org.apache.ibatis.annotations.Select("""
            SELECT * FROM aftersale_cases
             WHERE status = '0'
               AND appeal_state = '0'
               AND auto_escalated = false
               AND process_deadline IS NOT NULL
               AND process_deadline < NOW()
             LIMIT #{limit}
             FOR UPDATE SKIP LOCKED
            """)
    List<AfterSaleCaseEntity> selectProcessTimeoutCandidates(@Param("limit") int limit);

    /**
     * §5.10 樂觀鎖 CAS：一次更新升級欄位。
     * <ul>
     *   <li>SET：{@code appeal_state=APPEALED, auto_escalated=true, escalated_at=now,
     *       appeal_submitted_at=now}</li>
     *   <li>{@code appeal_submitted_at} 同步寫入：admin 仲裁列表用此欄位 ORDER BY DESC，
     *       若不寫會被 NULLS LAST 擠到最後一頁，admin 看不到</li>
     *   <li>WHERE 雙重防護：{@code auto_escalated=false} 防 worker 重跑 / 多實例 race；
     *       {@code process_deadline IS NOT NULL AND process_deadline < NOW()} 防 SELECT
     *       後賣家剛好 resolve 清空 deadline 的極端 race</li>
     * </ul>
     *
     * @return 1 = 成功；0 = 已被他人處理或狀態變更
     */
    @Update("""
            UPDATE aftersale_cases
               SET appeal_state = '10',
                   auto_escalated = true,
                   escalated_at = #{now},
                   appeal_submitted_at = #{now}
             WHERE case_no = #{caseNo}
               AND auto_escalated = false
               AND status = '0'
               AND appeal_state = '0'
               AND process_deadline IS NOT NULL
               AND process_deadline < NOW()
            """)
    int markAutoEscalated(@Param("caseNo") String caseNo,
                          @Param("now") java.time.LocalDateTime now);

    /**
     * §5.10 AUTO_REJECT 樂觀鎖 CAS：一次更新拒絕欄位。
     * <ul>
     *   <li>SET：{@code status=REJECTED(60), last_rejected_at=now, process_deadline=NULL}</li>
     *   <li>WHERE 雙重防護：{@code status = '0'} 防賣家剛剛 resolve（status 已變）；
     *       {@code process_deadline IS NOT NULL AND process_deadline < NOW()} 防 SELECT
     *       後賣家剛好 resolve 清空 deadline 的極端 race；
     *       透過 status 變更（PENDING → REJECTED）天然防多實例 race</li>
     * </ul>
     *
     * @return 1 = 成功；0 = 已被他人處理或狀態變更
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '60',
                   last_rejected_at = #{now},
                   process_deadline = NULL
             WHERE case_no = #{caseNo}
               AND status = '0'
               AND process_deadline IS NOT NULL
               AND process_deadline < NOW()
            """)
    int markAutoRejected(@Param("caseNo") String caseNo,
                         @Param("now") java.time.LocalDateTime now);

    /**
     * §2.1 INV-3 對齊：退貨運送中 transition。
     * 由 ReturnShipmentStatusConsumer 收到 PayUni 物流配送中事件時觸發。
     * WHERE status='20' 樂觀鎖：只有 RETURN_PENDING 狀態才轉為 RETURN_SHIPPING；
     * 防 webhook 重投或與其他 worker race。
     *
     * @return 1 = transition 成功；0 = 已是 25/30 或其他終態，consumer 應 ACK skip
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '25'
             WHERE case_no = #{caseNo}
               AND status = '20'
            """)
    int markReturnInTransit(@Param("caseNo") String caseNo);

    /**
     * §2.1 / §5.3.4 INV-3 對齊：退貨送達 transition
     * （RETURN_PENDING 20 / RETURN_SHIPPING 25 / RETURN_AT_STORE 26 → RETURN_DELIVERED 27）。
     * 由 ReturnShipmentStatusConsumer 收到 PayUni 碼 11 (PICKED_UP) 時觸發。
     * <p>
     * WHERE status IN ('20','25','26')，涵蓋（退貨跳號 downward-closure，見 09 設計書）：
     * <ul>
     *   <li>RETURN_PENDING（20）：退貨跳號或送達事件早於在途/到店事件到達時，自 RETURN_PENDING 直達 27</li>
     *   <li>宅配（3001）：25 → 27（包裹送至賣家地址簽收）</li>
     *   <li>超商（3002）：26 → 27（賣家從超商實際取貨）</li>
     * </ul>
     * 同時 stamp return_delivered_at + auto_confirm_at（= now + AUTO_CONFIRM_RECEIPT_HOURS）+ seller_picked_up_at。
     *
     * @return 1 = transition 成功；0 = 已 27/28/30/40 或其他狀態，consumer 應 ACK skip
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '27',
                   return_delivered_at = #{deliveredAt},
                   auto_confirm_at = #{autoConfirmAt},
                   seller_picked_up_at = #{sellerPickedUpAt}
             WHERE case_no = #{caseNo}
               AND status IN ('20', '25', '26')
            """)
    int markReturnDelivered(@Param("caseNo") String caseNo,
                            @Param("deliveredAt") java.time.LocalDateTime deliveredAt,
                            @Param("autoConfirmAt") java.time.LocalDateTime autoConfirmAt,
                            @Param("sellerPickedUpAt") java.time.LocalDateTime sellerPickedUpAt);

    /**
     * §2.1 / §5.3.4 INV-3：超商退貨到店待取 transition
     * （RETURN_PENDING 20 / RETURN_SHIPPING 25 → RETURN_AT_STORE 26）。
     * <p>
     * 由 ReturnShipmentStatusConsumer 收到 PayUni 碼 32 (WAITING_PICKUP) 且
     * return_shipping_method='3002' 時觸發。WHERE status IN ('20','25')：退貨跳號或到店事件早於
     * 在途事件到達時，可自 RETURN_PENDING(20) 直達 26（downward-closure，見 09 設計書）。
     * 同時 stamp store_arrived_at + store_pickup_deadline_at（= now + STORE_PICKUP_TIMEOUT_HOURS）。
     *
     * @return 1 = transition 成功；0 = 已 26/27/28+ 或其他狀態，consumer 應 ACK skip
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '26',
                   store_arrived_at = #{storeArrivedAt},
                   store_pickup_deadline_at = #{storePickupDeadlineAt}
             WHERE case_no = #{caseNo}
               AND status IN ('20', '25')
            """)
    int markReturnAtStore(@Param("caseNo") String caseNo,
                          @Param("storeArrivedAt") java.time.LocalDateTime storeArrivedAt,
                          @Param("storePickupDeadlineAt") java.time.LocalDateTime storePickupDeadlineAt);

    /**
     * §2.1 / §5.3.11 INV-3 對齊：退貨物流失敗 transition（20/25/26/27 → RETURN_FAILED 28）。
     * 由 ReturnShipmentStatusConsumer 收到 PayUni 碼 33/53/44 時觸發。
     * 同時寫入 return_provider_status + return_provider_message（PayUni 原文，回傳前端）。
     * <p>
     * 2026-05-22 修訂：WHERE 擴增包含 26（超商到店待取也可能突然異常）。
     *
     * @return 1 = transition 成功；0 = 已 28/30/40+ 或其他狀態，consumer 應 ACK skip
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '28',
                   return_provider_status = #{providerStatus},
                   return_provider_message = #{providerMessage}
             WHERE case_no = #{caseNo}
               AND status IN ('20', '25', '26', '27')
            """)
    int markReturnFailed(@Param("caseNo") String caseNo,
                         @Param("providerStatus") String providerStatus,
                         @Param("providerMessage") String providerMessage);

    /**
     * §5.3.15 PayUni 碼 55（賣家未取）訊息持久化。
     * <p>
     * 2026-05-22 新增。**不變狀態**，僅將 PayUni 原文寫入 provider 欄位供 Admin / Seller UI 顯示。
     * 本平台 72h timer 是「賣家未取」自動完成的唯一真實來源；PayUni 碼 55 因保留期較長，
     * 通常在我方 case 已 status=30 之後才到達，屬於後到的補充資訊。
     *
     * @return 1 = stamp 成功（case 存在）；0 = case 不存在
     */
    @Update("""
            UPDATE aftersale_cases
               SET return_provider_status = #{providerStatus},
                   return_provider_message = #{providerMessage}
             WHERE case_no = #{caseNo}
            """)
    int stampReturnProviderInfo(@Param("caseNo") String caseNo,
                                @Param("providerStatus") String providerStatus,
                                @Param("providerMessage") String providerMessage);

    /**
     * §2.1 INV-3 / INV-4 對齊：退款成功後關閉 case（REFUND_PROCESSING 40 → CLOSED 50）+ stamp closed_at。
     * 由 RefundResultConsumer.closeParentAndPublish 觸發；webhook 重投或多 path race 時冪等 skip。
     * WHERE status IN ('40') INV-4 收緊：CLOSED 唯一合法前置為 REFUND_PROCESSING；
     * coupon compensation 路徑（PENDING → CLOSED）走 closeByCouponCompensation 不共用本 mapper。
     *
     * @return 1 = 關閉成功；0 = 已關閉或前置不合法，consumer 應 ACK skip
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '50',
                   closed_at = #{closedAt}
             WHERE case_no = #{caseNo}
               AND status IN ('40')
            """)
    int closeIfNotClosed(@Param("caseNo") String caseNo,
                         @Param("closedAt") java.time.LocalDateTime closedAt);

    // ==================== §2.1 INV-3 / INV-4：賣家決議 / 買家動作 / 退貨確認 transition ====================

    /**
     * §2.1 / §5.2.1 RETURN_REFUND：PENDING(0) → RETURN_PENDING(20)。
     * 同時寫入退貨物流欄位、初始化 label_retry_count、清空 process_deadline。
     *
     * @return 1 = 成功；0 = 已被 §5.10 worker 升級或他路徑變更，caller 應拋業務例外
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '20',
                   return_shipment_no = #{returnShipmentNo},
                   return_shipping_method = #{returnShippingMethod},
                   ship_trade_no = #{shipTradeNo},
                   label_retry_count = 0,
                   process_deadline = NULL,
                   ship_back_deadline_at = #{shipBackDeadlineAt}
             WHERE case_no = #{caseNo}
               AND status = '0'
            """)
    int markReturnPending(@Param("caseNo") String caseNo,
                          @Param("returnShipmentNo") String returnShipmentNo,
                          @Param("returnShippingMethod") String returnShippingMethod,
                          @Param("shipTradeNo") String shipTradeNo,
                          @Param("shipBackDeadlineAt") java.time.LocalDateTime shipBackDeadlineAt);

    /**
     * §2.1 / §5.2.2 PARTIAL_REFUND / §5.2.3 FULL_REFUND_NO_RETURN：
     * PENDING(0) → REFUND_PROCESSING(40)，清空 process_deadline。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '40',
                   process_deadline = NULL
             WHERE case_no = #{caseNo}
               AND status = '0'
            """)
    int markRefundProcessingFromResolution(@Param("caseNo") String caseNo);

    /**
     * §2.1 / §5.2.4 COUPON_COMPENSATION：PENDING(0) → CLOSED(50)。
     * 與 closeIfNotClosed（退款路徑，前置 status=40）並存：兩者前置集合互斥，無法共用。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '50',
                   closed_at = #{closedAt},
                   process_deadline = NULL
             WHERE case_no = #{caseNo}
               AND status = '0'
            """)
    int closeByCouponCompensation(@Param("caseNo") String caseNo,
                                  @Param("closedAt") java.time.LocalDateTime closedAt);

    /**
     * §2.1 / §5.2.5 REJECT：PENDING(0) → REJECTED(60)。
     * 同時 stamp last_rejected_at（供申訴窗口起算）+ 清空 process_deadline。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '60',
                   last_rejected_at = #{lastRejectedAt},
                   process_deadline = NULL
             WHERE case_no = #{caseNo}
               AND status = '0'
            """)
    int markRejectedByResolution(@Param("caseNo") String caseNo,
                                 @Param("lastRejectedAt") java.time.LocalDateTime lastRejectedAt);

    /**
     * §2.1 / §5.3.7 confirmReceipt：RETURN_DELIVERED(27) → RETURN_RECEIVED(30)。
     * 賣家手動確認與 AutoConfirmReceiptScheduler 自動確認共用本 mapper。
     * 兩條路徑同樂觀鎖 WHERE status='27'，先到者贏，後到者 affected=0 → skip。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '30'
             WHERE case_no = #{caseNo}
               AND status = '27'
            """)
    int markReturnReceived(@Param("caseNo") String caseNo);

    /**
     * §2.1 / §5.3.12 Admin 強制確認收貨：任意進行中狀態 (20/25/26/27/28) → RETURN_RECEIVED(30)。
     * 僅 ForceConfirmReceiptForAdmin RPC 走本 mapper；audit log action=FORCE_CONFIRM_RECEIPT。
     * <p>
     * 2026-05-22 修訂：WHERE 擴增 26（超商到店待取也可被 Admin 強制完成）。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '30'
             WHERE case_no = #{caseNo}
               AND status IN ('20', '25', '26', '27', '28')
            """)
    int markReturnReceivedByAdmin(@Param("caseNo") String caseNo);

    /**
     * §5.3.10.1 AutoConfirmReceiptScheduler 掃描：找 status='27' 且 auto_confirm_at 已到期的 case。
     */
    @Select("""
            SELECT case_no FROM aftersale_cases
             WHERE status = '27'
               AND auto_confirm_at IS NOT NULL
               AND auto_confirm_at <= #{cutoff}
             ORDER BY auto_confirm_at ASC
             LIMIT #{limit}
            """)
    java.util.List<String> findAutoConfirmExpired(@Param("cutoff") java.time.LocalDateTime cutoff,
                                                  @Param("limit") int limit);

    // ==================== 2026-05-22 逆物流退貨政策修訂：scheduler 掃描 + auto-complete CAS ====================

    /**
     * §5.3.10.2 StorePickupTimeoutScheduler 掃描：找 status='26' 且 store_pickup_deadline_at 已到期的 case。
     */
    @Select("""
            SELECT case_no FROM aftersale_cases
             WHERE status = '26'
               AND store_pickup_deadline_at IS NOT NULL
               AND store_pickup_deadline_at <= #{cutoff}
             ORDER BY store_pickup_deadline_at ASC
             LIMIT #{limit}
            """)
    java.util.List<String> findStorePickupExpired(@Param("cutoff") java.time.LocalDateTime cutoff,
                                                  @Param("limit") int limit);

    /**
     * §5.3.10.3 ReturnTwentyFourHourWarnScheduler 掃描：超商待取剩 ≤24h 仍未預警的 case。
     * 條件：status='26' AND warned_store_pickup_24h_at IS NULL AND store_pickup_deadline_at 在 [now, now+24h]。
     */
    @Select("""
            SELECT case_no FROM aftersale_cases
             WHERE status = '26'
               AND store_pickup_deadline_at IS NOT NULL
               AND warned_store_pickup_24h_at IS NULL
               AND store_pickup_deadline_at > #{now}
               AND store_pickup_deadline_at <= #{cutoff}
             ORDER BY store_pickup_deadline_at ASC
             LIMIT #{limit}
            """)
    java.util.List<String> findStorePickup24hWarnCandidates(@Param("now") java.time.LocalDateTime now,
                                                            @Param("cutoff") java.time.LocalDateTime cutoff,
                                                            @Param("limit") int limit);

    /**
     * §5.3.10.3 ReturnTwentyFourHourWarnScheduler 掃描：確認收貨剩 ≤24h 仍未預警的 case。
     */
    @Select("""
            SELECT case_no FROM aftersale_cases
             WHERE status = '27'
               AND auto_confirm_at IS NOT NULL
               AND warned_confirm_receipt_24h_at IS NULL
               AND auto_confirm_at > #{now}
               AND auto_confirm_at <= #{cutoff}
             ORDER BY auto_confirm_at ASC
             LIMIT #{limit}
            """)
    java.util.List<String> findConfirmReceipt24hWarnCandidates(@Param("now") java.time.LocalDateTime now,
                                                               @Param("cutoff") java.time.LocalDateTime cutoff,
                                                               @Param("limit") int limit);

    /**
     * §5.3.10.3 ReturnTwentyFourHourWarnScheduler 掃描：待寄回剩 ≤24h 仍未預警的 case（2026-06-15 新增）。
     * 條件：status='20' AND warned_ship_back_24h_at IS NULL AND ship_back_deadline_at 在 (now, now+24h]。
     */
    @Select("""
            SELECT case_no FROM aftersale_cases
             WHERE status = '20'
               AND ship_back_deadline_at IS NOT NULL
               AND warned_ship_back_24h_at IS NULL
               AND ship_back_deadline_at > #{now}
               AND ship_back_deadline_at <= #{cutoff}
             ORDER BY ship_back_deadline_at ASC
             LIMIT #{limit}
            """)
    java.util.List<String> findShipBack24hWarnCandidates(@Param("now") java.time.LocalDateTime now,
                                                         @Param("cutoff") java.time.LocalDateTime cutoff,
                                                         @Param("limit") int limit);

    /**
     * §5.3.10.3 stamp warned_store_pickup_24h_at（<b>事後記錄，非冪等閘門</b>）。
     *
     * <p>issue #2294 A1 例外 B：呼叫順序是 <b>publish → 成功後才 stamp</b>，所以 affected 值
     * <b>與「有沒有發 FCM」無關</b>——本方法被呼叫時 FCM 已經發出去了。affected=0 只代表
     * 「這一列已經被記錄過或狀態已改變」，不代表「本次沒發」。
     * 跨 pod 去重由 notification 端 {@code notifications_inbox_dedup_idx} 承擔。
     *
     * <p>SQL 一個字沒改（例外 B 不改任何 SQL）；候選述詞仍讀 {@code IS NULL}，作用是效率優化。
     */
    @Update("""
            UPDATE aftersale_cases
               SET warned_store_pickup_24h_at = #{now}
             WHERE case_no = #{caseNo}
               AND status = '26'
               AND warned_store_pickup_24h_at IS NULL
            """)
    int stampStorePickup24hWarned(@Param("caseNo") String caseNo,
                                  @Param("now") java.time.LocalDateTime now);

    /**
     * §5.3.10.3 stamp warned_confirm_receipt_24h_at（<b>事後記錄，非冪等閘門</b>；
     * 語意與 {@link #stampStorePickup24hWarned} 相同，見 issue #2294 A2）。
     */
    @Update("""
            UPDATE aftersale_cases
               SET warned_confirm_receipt_24h_at = #{now}
             WHERE case_no = #{caseNo}
               AND status = '27'
               AND warned_confirm_receipt_24h_at IS NULL
            """)
    int stampConfirmReceipt24hWarned(@Param("caseNo") String caseNo,
                                     @Param("now") java.time.LocalDateTime now);

    /**
     * §5.3.10.3 stamp warned_ship_back_24h_at（待寄回剩 24h D-1 預警的<b>事後記錄，非冪等閘門</b>）。
     * 2026-06-15 新增；WHERE status='20' AND warned_ship_back_24h_at IS NULL。
     * 語意與 {@link #stampStorePickup24hWarned} 相同，見 issue #2294 A3。
     */
    @Update("""
            UPDATE aftersale_cases
               SET warned_ship_back_24h_at = #{now}
             WHERE case_no = #{caseNo}
               AND status = '20'
               AND warned_ship_back_24h_at IS NULL
            """)
    int stampShipBack24hWarned(@Param("caseNo") String caseNo,
                               @Param("now") java.time.LocalDateTime now);

    /**
     * §5.21 issue #2294 L6：RETURN_STUCK 卡死告警的 <b>lease claim</b>。
     *
     * <p>把 {@code return_stuck_alerted_at} 推到 {@code leaseUntil}（= now + returnStuckDays）：
     * affected=1 表示本 pod 搶到這一輪的告警權；affected=0 表示另一個 pod 已搶走且租約未到期。
     * 租約到期後該列會被候選述詞重新撈回並再次告警——<b>這是刻意的</b>，卡死沒解除就該一直吵。
     *
     * <p>⚠️ 這支是 lease 不是「已告警旗標」：不得改成 {@code IS NULL} 的永久 CAS，
     * 否則卡死案件只會被告警一次（規範《多副本排程搶佔規範》§二 第 4 步）。
     * <p>⚠️ 刻意不寫 {@code updated_at}：那是本排程的候選判定欄，寫它會讓案件立刻退出候選集合。
     */
    @Update("""
            UPDATE aftersale_cases
               SET return_stuck_alerted_at = #{leaseUntil}
             WHERE case_no = #{caseNo}
               AND status = #{status}
               AND (return_stuck_alerted_at IS NULL OR return_stuck_alerted_at <= #{now})
            """)
    int claimReturnStuckAlert(@Param("caseNo") String caseNo,
                              @Param("status") String status,
                              @Param("now") java.time.LocalDateTime now,
                              @Param("leaseUntil") java.time.LocalDateTime leaseUntil);

    /**
     * §5.21 R4 追加（2026-08-08）：{@link #claimReturnStuckAlert} 的<b>釋放</b>——
     * publish 沒有成功時把租約還原成 claim 之前的值，讓該案件在<b>下一個 tick</b>（每日）
     * 就能重新告警，而不是等滿一整個 {@code returnStuckDays}（14 天）的冷卻。
     *
     * <p><b>為什麼 CAS 的條件是「租約仍等於我寫進去的那個值」</b>：這是唯一能證明
     * 「這個租約還是我的」的述詞。若期間有人（另一個 pod 在租約到期後、或人工）重新 claim，
     * 值就不同了，本次還原自動變成 0 列——<b>不會把別人的租約清掉</b>。
     * 也因此 {@code caseNo} 之外一定要帶上 {@code expectedLeaseUntil}，不能只用 PK
     * （《多副本排程搶佔規範》§一 規則 ⑦ 的同一個道理）。
     *
     * <p>⚠️ <b>這不是搶佔正確性的一部分，是失敗恢復的加速</b>：規範 §二 第 5 步明說
     * 「catch 裡有回退不算回頭路」——進程消失時沒有任何 catch 會跑。租約本身
     * （14 天後自動回到候選集合）仍然是唯一的正確性保證，本方法只把「MQ 短暫不可用」
     * 這條**進程還活著**的路徑從 14 天縮到 1 天。
     *
     * @param expectedLeaseUntil claim 當時寫進去的租約值；不相符即放棄還原（回 0）
     * @param restoreTo          還原目標＝claim 之前的舊值（可為 {@code null}）
     */
    @Update("""
            UPDATE aftersale_cases
               SET return_stuck_alerted_at = #{restoreTo,jdbcType=TIMESTAMP}
             WHERE case_no = #{caseNo}
               AND return_stuck_alerted_at = #{expectedLeaseUntil}
            """)
    int releaseReturnStuckAlertLease(@Param("caseNo") String caseNo,
                                     @Param("expectedLeaseUntil") java.time.LocalDateTime expectedLeaseUntil,
                                     @Param("restoreTo") java.time.LocalDateTime restoreTo);

    /**
     * §5.3.8 ReturnTimeoutWorker 自動關案 transition（RETURN_PENDING 20 → AUTO_COMPLETED_RETURN_PENDING_TIMEOUT 33）。
     * <p>
     * 2026-06-15 新增；終態，不啟動退款。CAS 防競態：買家壓哨寄出 20→25 搶先 → affected=0 skip。
     * 同時 stamp closed_at 表示案件正式關閉。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '33',
                   closed_at = #{now}
             WHERE case_no = #{caseNo}
               AND status = '20'
               AND ship_back_deadline_at IS NOT NULL
               AND ship_back_deadline_at <= NOW()
            """)
    int markReturnPendingTimeoutAutoClosed(@Param("caseNo") String caseNo,
                                           @Param("now") java.time.LocalDateTime now);

    /**
     * §10.8.2 33+APPEALED + Admin 裁決 FULL/PARTIAL_REFUND：33 → REFUND_PROCESSING(40)。
     * 2026-06-15 新增；與 §10 既有 REJECTED(60) → 40 同 pattern。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '40'
             WHERE case_no = #{caseNo}
               AND status = '33'
               AND appeal_state = '10'
            """)
    int markReturnPendingTimeoutByAppealToRefund(@Param("caseNo") String caseNo);

    /**
     * §10.8.2 33+APPEALED + Admin 裁決 REJECT_APPEAL：33 → CLOSED(50)。
     * 2026-06-15 新增；與 §10 既有 REJECTED(60) → 50 同 pattern。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '50',
                   closed_at = #{now}
             WHERE case_no = #{caseNo}
               AND status = '33'
               AND appeal_state = '10'
            """)
    int markReturnPendingTimeoutByAppealToClosed(@Param("caseNo") String caseNo,
                                                 @Param("now") java.time.LocalDateTime now);

    /**
     * §5.3.10.2 StorePickupTimeoutScheduler 自動完成 transition
     * （RETURN_AT_STORE 26 → AUTO_COMPLETED_STORE_PICKUP_TIMEOUT 31）。
     * <p>
     * status 直接表達原因（B 派設計，2026-05-22 修訂取代舊 status=30 + auto_complete_reason metadata）。
     * 只 stamp auto_complete_at_stamp 作為實際完成時間。
     * <p>WHERE status='26'：賣家剛從超商取貨觸發 26→27 時，本 mapper affected=0 → skip。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '31',
                   auto_complete_at_stamp = #{now}
             WHERE case_no = #{caseNo}
               AND status = '26'
            """)
    int markAutoCompletedFromAtStore(@Param("caseNo") String caseNo,
                                     @Param("now") java.time.LocalDateTime now);

    /**
     * §5.3.10.1 AutoConfirmReceiptScheduler 自動完成 transition
     * （RETURN_DELIVERED 27 → AUTO_COMPLETED_CONFIRM_RECEIPT_TIMEOUT 32）。
     * <p>
     * status 直接表達原因（B 派設計，2026-05-22 修訂取代舊 status=30 + auto_complete_reason metadata）。
     * 只 stamp auto_complete_at_stamp 作為實際完成時間。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '32',
                   auto_complete_at_stamp = #{now}
             WHERE case_no = #{caseNo}
               AND status = '27'
            """)
    int markAutoCompletedFromDelivered(@Param("caseNo") String caseNo,
                                       @Param("now") java.time.LocalDateTime now);

    /**
     * §2.1 / §5.3.3 收貨完成後續：30/31/32 → REFUND_PROCESSING(40)。
     * 同事務內接續 markReturnReceived（手動 30）/ markAutoCompletedFromAtStore（31）/
     * markAutoCompletedFromDelivered（32），覆蓋三種「已收退貨」終態。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '40'
             WHERE case_no = #{caseNo}
               AND status IN ('30', '31', '32')
            """)
    int markRefundProcessingFromReceived(@Param("caseNo") String caseNo);

    /**
     * §2.1 / §5.1.3 supplementEvidence：補充描述（不轉 status）。
     * WHERE status = '0' 防 §5.10 worker 已升級後寫者覆蓋 stale description。
     */
    @Update("""
            UPDATE aftersale_cases
               SET description = #{newDescription}
             WHERE case_no = #{caseNo}
               AND status = '0'
            """)
    int appendDescription(@Param("caseNo") String caseNo,
                          @Param("newDescription") String newDescription);

    /**
     * §2.1 / §5.1.4 cancelCase（買家取消）：PENDING(0) → CANCELLED(70)。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '70',
                   process_deadline = NULL
             WHERE case_no = #{caseNo}
               AND status = '0'
            """)
    int markCancelledByBuyer(@Param("caseNo") String caseNo);

    /**
     * §2.1.1 / §8.2.1（#1225）確認收貨自動取消：A 桶 PENDING(0) → CANCELLED_BY_CONFIRM_RECEIPT(71)。
     * <p>
     * 前置集與 {@link #markCancelledByBuyer} 相同（CAS WHERE status = '0'），僅終態 status 不同（71 區分撥款歸屬）。
     * 由 order-service confirmReceipt 同步觸發；affected=0 表示已離開 0/65（被賣家同意 / worker auto-reject / 已終結），
     * 由 service 重讀分類為 BLOCKED 或 NO_ACTIVE_CASE（§8.2.1 競態矩陣）。
     */
    @Update("""
            UPDATE aftersale_cases
               SET status = '71',
                   process_deadline = NULL
             WHERE case_no = #{caseNo}
               AND status = '0'
            """)
    int markCancelledByConfirmReceipt(@Param("caseNo") String caseNo);

    // ==================== Order Service 反查（§4.4.1 訂單倒數恢復） ====================

    /**
     * 訂單活躍 case 計數：用於 Order Service 自動完成倒數恢復前反查（03 §5.8 / 09 §4.4.1）。
     *
     * 活躍 = case status 未終態（NOT IN AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33)/CLOSED(50)/REJECTED(60)/CANCELLED(70)）
     *       OR （status=REJECTED(60) AND appeal_state=APPEALED(10)）
     *       OR （status=AUTO_COMPLETED_RETURN_PENDING_TIMEOUT(33) AND appeal_state=APPEALED(10)）2026-06-15 新增
     *
     * 33 含申訴的活躍判斷：33 預設終態（撥款給賣家），但若買家對 33 提申訴 → Admin 裁決前 case 仍活躍 → order-service 維持倒數暫停不撥款。
     */
    @org.apache.ibatis.annotations.Select("""
            SELECT COUNT(*) FROM aftersale_cases
             WHERE order_no = #{orderNo}
               AND (status NOT IN ('33','50','60','70','71')
                    OR (status = '60' AND appeal_state = '10')
                    OR (status = '33' AND appeal_state = '10'))
            """)
    int countActiveCasesByOrder(@Param("orderNo") String orderNo);

    /**
     * 賣家所有售後 case 的分類 projection（賣家工作台 Dashboard count 用，09 §4.4）。
     * <p>
     * 只取 order_no / status / appeal_state 三欄，交由 {@code AfterSaleCaseStatusRules}
     * 在記憶體做 canonical 分類（active / terminal-override），canonical SSOT 集中於 helper
     * 而非散落 SQL（docs/開發規範/Java 微服務開發規範.md §6 / §15）。
     */
    @Select("""
            SELECT order_no, status, appeal_state
            FROM aftersale_cases
            WHERE store_id = #{storeId}
            """)
    List<SellerCaseRef> selectStoreCaseRefs(@Param("storeId") long storeId);
}
