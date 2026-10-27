package io.github.will413028.hawkr.aftersales.scheduler;

import io.github.will413028.hawkr.common.grpc.PrincipalResolution;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient;
import io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.test.util.ReflectionTestUtils;

import java.time.Duration;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Map;
import java.util.Set;

import static org.junit.jupiter.api.Assertions.*;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * Plan C Phase 3 追加（Task 9b）：RETURN_STUCK shipment-anomaly 賣家通知必須把
 * store_id 反解回登入 account_id 才能送出，否則已重新上架（store_id 換發）的賣家會
 * 推播到錯誤的收件人（notification-service 的 extractUserID 直接讀 seller_account_id
 * 定址，不自行反解）。account_id != store_id 判別測試。
 *
 * <p>經公開入口 {@link ReturnStuckAlertScheduler#run()} 驅動（比對 private
 * {@code publishReturnStuck} 用 reflection 容易，見 task-9b-brief）。
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class ReturnStuckAlertSchedulerTest {

    @Mock private AfterSaleCaseMapper caseMapper;
    @Mock private ObsRabbitPublisher obsRabbitPublisher;
    @Mock private SellerGrpcClient sellerGrpcClient;
    @Mock private BuyerGrpcClient buyerGrpcClient;

    /**
     * R4（2026-08-08）：publish 改走 {@code publishConfirmed}（會傳遞失敗的變體），
     * Mockito 對 {@code boolean} 預設回 {@code false} ⇒ 不 stub 的話每一支都會走「發送失敗」
     * 分支並釋放租約。預設 stub 成成功，需要驗失敗路徑的測試自行覆寫。
     */
    @org.junit.jupiter.api.BeforeEach
    void publishSucceedsByDefault() {
        when(obsRabbitPublisher.publishConfirmed(
                anyString(), anyString(), any(byte[].class), anyString(), anyLong()))
                .thenReturn(true);
    }

    @Test
    void run_resolvesStoreIdToAccountIdInShipmentAnomalyPayload() {
        long storeId = 7001L;
        long resolvedAccountId = 42L; // 不同於 storeId：模擬已重新上架的賣家

        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(storeId);
        acase.setBuyerId(2001L);

        when(caseMapper.selectList(any())).thenReturn(List.of(acase));
        // issue #2294 L6：新增 lease claim 閘門，Mockito 對 int 預設回 0 ⇒ 必須 stub 成搶到
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(resolvedAccountId);

        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        scheduler.run();

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(1)).publishConfirmed(anyString(), anyString(), bodyCaptor.capture(), anyString(), anyLong());

        String json = new String(bodyCaptor.getValue());
        assertTrue(json.contains("\"seller_account_id\":" + resolvedAccountId),
                "shipment anomaly payload 的 seller_account_id 必須是反解後的 account_id(" + resolvedAccountId + ")，不是 store_id(" + storeId + ")：" + json);
        assertFalse(json.contains("\"seller_account_id\":" + storeId),
                "shipment anomaly payload 不應直接帶未反解的 store_id：" + json);
        // issue #2707 Phase A：新增 storeId 欄位必須是未反解的原始值，與 seller_account_id 分開帶。
        assertTrue(json.contains("\"store_id\":" + storeId),
                "shipment anomaly payload 必須帶原始 store_id(" + storeId + ")：" + json);
    }

    /**
     * 對稱案例（2026-08-05 identity-naming triage）：buyerAccountId 也必須反解成登入
     * principal account_id，而非 aftersale_cases 上持久化的 buyer_id(PK) 原始值——
     * 鏡像 ShipmentLabelConsumerTest 的
     * publishPayUniLgsNoCodeAnomaly_resolvesBuyerIdToAccountIdInShipmentAnomalyPayload。
     * 這個斷言原本完全不存在，是本次缺口能躲過 CI 的直接原因。
     *
     * <p>2026-08-05 真修後同時是「genuinely resolved」路徑的覆蓋：{@code buyerId} 存在於
     * {@code principals} 且**不在** {@code unresolvedIds} 中——{@link ReturnStuckAlertScheduler
     * #resolveNotifyBuyerAccountId} 必須把真實解析值原樣帶出，而不是誤判成 unresolved。
     */
    @Test
    void run_resolvesBuyerIdToAccountIdInShipmentAnomalyPayload() {
        long storeId = 7001L;
        long resolvedSellerAccountId = 42L;
        long buyerId = 2001L;
        long resolvedBuyerAccountId = 88L; // 刻意 != buyerId，也 != 上面任何一個數字

        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(storeId);
        acase.setBuyerId(buyerId);

        when(caseMapper.selectList(any())).thenReturn(List.of(acase));
        // issue #2294 L6：新增 lease claim 閘門，Mockito 對 int 預設回 0 ⇒ 必須 stub 成搶到
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(resolvedSellerAccountId);
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(new PrincipalResolution(
                        Map.of(buyerId, resolvedBuyerAccountId), Set.of()));

        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        scheduler.run();

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(1)).publishConfirmed(anyString(), anyString(), bodyCaptor.capture(), anyString(), anyLong());

        String json = new String(bodyCaptor.getValue());
        assertTrue(json.contains("\"buyer_account_id\":" + resolvedBuyerAccountId),
                "shipment anomaly payload 的 buyer_account_id 必須是反解後的 account_id(" + resolvedBuyerAccountId + ")，不是 buyer_id(" + buyerId + ")：" + json);
        assertFalse(json.contains("\"buyer_account_id\":" + buyerId),
                "shipment anomaly payload 不應直接帶未反解的 buyer_id：" + json);
        assertTrue(json.contains("\"seller_account_id\":" + resolvedSellerAccountId),
                "賣家側原有行為不得回歸：" + json);
    }

    /**
     * 降級案例：buyer-service 查詢失敗時 buyer_account_id 降級為 null（與
     * ShipmentLabelConsumer.resolveNotifyBuyerAccountId 同一哲學：本欄位在此 payload
     * 是渲染上下文、非路由必要欄位，缺失即不填，絕不回退成未反解的 buyer_id），
     * 且不得讓 publishReturnStuck 拋出中斷整批掃描。
     */
    @Test
    void run_buyerResolveFailureDegradesToNullNotRawBuyerId() {
        long storeId = 7001L;
        long buyerId = 2001L;

        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(storeId);
        acase.setBuyerId(buyerId);

        when(caseMapper.selectList(any())).thenReturn(List.of(acase));
        // issue #2294 L6：新增 lease claim 閘門，Mockito 對 int 預設回 0 ⇒ 必須 stub 成搶到
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(42L);
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenThrow(new RuntimeException("buyer-service unavailable"));

        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        scheduler.run();

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(1)).publishConfirmed(anyString(), anyString(), bodyCaptor.capture(), anyString(), anyLong());

        String json = new String(bodyCaptor.getValue());
        assertTrue(json.contains("\"buyer_account_id\":null"),
                "反解失敗應降級為 null：" + json);
        assertFalse(json.contains("\"buyer_account_id\":" + buyerId),
                "反解失敗不得回退成未反解的 buyer_id：" + json);
    }

    /**
     * unresolved 案例（2026-08-05 Wave 1 fixup round——取代原本 mock 不可能狀態的
     * {@code run_buyerResolveMissingKeyDegradesToNullNotRawBuyerId}）：舊測試 mock
     * {@code resolvePrincipals} 回傳「不含 buyerId 這個 key 的空 map」，但 buyer-service
     * 的 {@code GetPrincipalsByBuyerIDs} 用 {@code COALESCE(account_id, input.id)}，
     * 對每個查詢 id **一律**有 map entry——那個 mock 狀態在真實系統裡永遠不會發生，
     * 用它證明「換用 {@code Map#get} 就修好了」是自欺欺人的假綠燈（上一輪的
     * no-op 修復正是這樣被誤判為已修好）。
     *
     * <p>真實情況是：{@code principals} 一律有 entry（未解析時 echo 回原始 buyerId），
     * 真正的判別依據是新增的 {@code unresolved_ids}（{@link PrincipalResolution
     * #unresolvedIds()}）——{@code buyerId} 出現在其中才代表查無對應買家列。本測試
     * mock 這個真實可能發生的狀態：{@code principals} 裡有 buyerId 的 echo-back 值，
     * 但它同時出現在 {@code unresolvedIds}，驗證
     * {@link ReturnStuckAlertScheduler#resolveNotifyBuyerAccountId} 不會把這個
     * echo-back 值誤當成真實解析結果發布出去，而是正確降級為 null。
     */
    @Test
    void run_buyerUnresolvedIdDegradesToNullNotRawBuyerId() {
        long storeId = 7001L;
        long buyerId = 2001L;

        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(storeId);
        acase.setBuyerId(buyerId);

        when(caseMapper.selectList(any())).thenReturn(List.of(acase));
        // issue #2294 L6：新增 lease claim 閘門，Mockito 對 int 預設回 0 ⇒ 必須 stub 成搶到
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(storeId)).thenReturn(42L);
        // 查無對應買家列：principals 依 server 端 COALESCE 行為 echo 回原始 buyerId，
        // 但 buyerId 同時出現在 unresolvedIds，證明那個 echo-back 值不可信任。
        when(buyerGrpcClient.resolvePrincipals(List.of(buyerId)))
                .thenReturn(new PrincipalResolution(
                        Map.of(buyerId, buyerId), Set.of(buyerId)));

        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        scheduler.run();

        ArgumentCaptor<byte[]> bodyCaptor = ArgumentCaptor.forClass(byte[].class);
        verify(obsRabbitPublisher, times(1)).publishConfirmed(anyString(), anyString(), bodyCaptor.capture(), anyString(), anyLong());

        String json = new String(bodyCaptor.getValue());
        assertTrue(json.contains("\"buyer_account_id\":null"),
                "buyerId 出現在 unresolvedIds 時應降級為 null：" + json);
        assertFalse(json.contains("\"buyer_account_id\":" + buyerId),
                "unresolved 不得回退成 principals 裡未經驗證的 echo-back 值（getOrDefault 的原始 bug class）：" + json);
    }

    /**
     * issue #2294 L6（b）：lease claim 回 0（另一個 pod 已搶走且租約未到期）⇒ <b>不得 publish</b>。
     *
     * <p>沒有這一支，新加的閘門一律成立（Mockito int 預設 0 只會讓上面幾支紅，不會證明閘門有效），
     * 「兩個 pod 各發一次卡死告警」這個 root cause 就完全沒有測試覆蓋。
     */
    @Test
    void run_claimLost_doesNotPublish() {
        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(7001L);
        acase.setBuyerId(2001L);

        when(caseMapper.selectList(any())).thenReturn(List.of(acase));
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(0);

        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        scheduler.run();

        verify(obsRabbitPublisher, never()).publishConfirmed(anyString(), anyString(), any(byte[].class), anyString(), anyLong());
    }

    /**
     * issue #2294 <b>R4-P1-2</b>：租約基準必須在<b>迴圈內、claim 當下</b>取，
     * 不能像原實作那樣在 {@link ReturnStuckAlertScheduler#run()} 開頭（tick 起點）只算一次。
     *
     * <p>只算一次 ⇒ 全批共用同一個到期時間 ⇒ 第 k 筆的<b>有效</b>租約
     * = {@code returnStuckDays} − (前 k−1 筆的耗時)，不變式退化成
     * 「批次總耗時 &lt; 租約」——那條會隨 {@code batch-size}（預設 200，可由設定調大）
     * 一起破，是參數巧合不是結構保證。基準改在 claim 當下取之後，不變式降為
     * 「<b>單筆</b>最壞耗時 &lt; 租約」，與 batchSize 無關。
     *
     * <p><b>鑑別方式</b>：讓第一筆 claim 之後的工作（{@code resolveAccountIdForStore}）
     * 耗掉一段可觀測的時間，再比較兩筆送進 claim 的租約。
     * 基準在迴圈內 ⇒ 第二筆嚴格晚於第一筆；基準在迴圈外 ⇒ 兩者<b>完全相等</b>。
     */
    @Test
    @DisplayName("R4-P1-2：租約基準取 claim 當下——後一筆的租約嚴格晚於前一筆（迴圈外只算一次會轉紅）")
    void leaseBaselineIsTakenPerItemNotPerTick() {
        AfterSaleCaseEntity slow = new AfterSaleCaseEntity();
        slow.setCaseNo("A_SLOW");
        slow.setOrderNo("O100");
        slow.setStoreId(7001L);
        slow.setBuyerId(2001L);

        AfterSaleCaseEntity next = new AfterSaleCaseEntity();
        next.setCaseNo("A_NEXT");
        next.setOrderNo("O101");
        next.setStoreId(7002L);
        next.setBuyerId(2002L);

        when(caseMapper.selectList(any())).thenReturn(List.of(slow, next));
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(anyLong())).thenAnswer(inv -> {
            Thread.sleep(40L);
            return 42L;
        });

        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        // 租約長度取真實預設值（14 天），確保驗的是「基準」而非退化成 plusDays(0)
        ReflectionTestUtils.setField(scheduler, "returnStuckDays", 14);
        ReflectionTestUtils.setField(scheduler, "batchSize", 200);
        scheduler.run();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(caseMapper, times(2))
                .claimReturnStuckAlert(anyString(), anyString(), any(), leases.capture());
        long deltaMs = Duration
                .between(leases.getAllValues().get(0), leases.getAllValues().get(1)).toMillis();

        assertTrue(deltaMs >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 " + deltaMs + " ms）。"
                        + "兩者相等 ⇒ 租約基準在迴圈外只算一次 ⇒ 不變式退化成「批次總耗時 < 租約」，"
                        + "會隨 batch-size（預設 200）一起破。");
    }

    /**
     * issue #2294：{@code returnStuckDays} 的<b>下界</b>。
     *
     * <p>這一個設定值同時是①候選門檻 ②告警冷卻 ③<b>租約長度</b>——第三個角色是本 issue
     * 引入的，於是「搶佔保護」被綁在一個原本沒有守衛的可調參數上。
     * 設成 {@code 0} 時 {@code leaseUntil == 取值當下} ⇒ 租約長度歸零 ⇒ 另一個 pod
     * 在微秒後 claim 就會通過（{@code return_stuck_alerted_at <= now} 立刻成立）⇒
     * <b>搶佔整個失效且完全無聲</b>：SQL 在、CAS 在、程式碼一模一樣，只有行為回到修復前。
     *
     * <p>⚠️ 這一支是<b>上一支的鑑別力補完</b>：上一支（租約基準）在 {@code returnStuckDays=0}
     * 時<b>照樣綠</b>——它只比較兩筆之間的相對先後，對「租約長度是 0」完全沒有意見。
     */
    @Test
    @DisplayName("returnStuckDays <= 0 啟動即 fail-fast（租約長度歸零＝搶佔靜默失效）")
    void returnStuckDaysBelowLowerBoundFailsFastAtStartup() {
        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        for (int bad : new int[]{0, -1}) {
            ReflectionTestUtils.setField(scheduler, "returnStuckDays", bad);
            IllegalStateException ex = assertThrows(IllegalStateException.class,
                    scheduler::validateReturnStuckDays,
                    "returnStuckDays=" + bad + " 必須在啟動時就擋下");
            assertTrue(ex.getMessage().contains("return-stuck-days"),
                    "錯誤訊息要指名是哪個設定值，否則 operator 找不到；實際: " + ex.getMessage());
        }
    }

    /** 下界之上（含邊界 1 與預設 14）必須放行，否則守衛本身就是個新的可用性風險。 */
    @Test
    @DisplayName("returnStuckDays >= 1 放行（守衛不得誤擋合法值）")
    void returnStuckDaysAtOrAboveLowerBoundPasses() {
        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);

        for (int ok : new int[]{ReturnStuckAlertScheduler.MIN_RETURN_STUCK_DAYS, 14}) {
            ReflectionTestUtils.setField(scheduler, "returnStuckDays", ok);
            assertDoesNotThrow(scheduler::validateReturnStuckDays,
                    "returnStuckDays=" + ok + " 是合法值，不得被守衛擋下");
        }
    }

    /**
     * 守衛真正要防的行為：租約長度必須是<b>正的</b>未來時間。
     *
     * <p>直接驗行為而不只驗守衛存在——把守衛拿掉並設 {@code returnStuckDays=0} 時，
     * 這一支會抓到「{@code leaseUntil} 不晚於 claim 當下」。
     */
    @Test
    @DisplayName("預設配置下租約長度為正（0 天會讓 leaseUntil == now，搶佔失效）")
    void leaseLengthIsStrictlyPositive() {
        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo("A001");
        acase.setOrderNo("O100");
        acase.setStoreId(7001L);
        acase.setBuyerId(2001L);

        when(caseMapper.selectList(any())).thenReturn(List.of(acase));
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(anyLong())).thenReturn(42L);

        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        ReflectionTestUtils.setField(scheduler, "returnStuckDays", 14);
        ReflectionTestUtils.setField(scheduler, "batchSize", 200);
        scheduler.run();

        ArgumentCaptor<LocalDateTime> now = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(caseMapper).claimReturnStuckAlert(
                anyString(), anyString(), now.capture(), lease.capture());

        assertTrue(lease.getValue().isAfter(now.getValue()),
                "租約到期時間必須嚴格晚於 claim 判定用的 now，"
                        + "否則另一個 pod 立刻就能重新 claim ⇒ 搶佔等於不存在");
        assertTrue(Duration.between(now.getValue(), lease.getValue()).toDays() >= 1,
                "租約長度至少一天（單筆最壞耗時 = 一次 gRPC 解析 + 一次 MQ publish，遠小於此）");
    }

    // ==================== R4 追加：publish 失敗時的租約釋放 ====================

    private static AfterSaleCaseEntity stuckCase(String caseNo) {
        AfterSaleCaseEntity acase = new AfterSaleCaseEntity();
        acase.setCaseNo(caseNo);
        acase.setOrderNo("O100");
        acase.setStoreId(7001L);
        acase.setBuyerId(2001L);
        return acase;
    }

    private ReturnStuckAlertScheduler newScheduler() {
        ReturnStuckAlertScheduler scheduler = new ReturnStuckAlertScheduler(
                caseMapper, obsRabbitPublisher, sellerGrpcClient, buyerGrpcClient);
        ReflectionTestUtils.setField(scheduler, "returnStuckDays", 14);
        ReflectionTestUtils.setField(scheduler, "batchSize", 200);
        return scheduler;
    }

    /**
     * R4（2026-08-08）：<b>publish 沒送出去 ⇒ 租約必須還原</b>，否則這則卡死告警要等滿一整個
     * {@code returnStuckDays}（14 天）冷卻才會重來——而 14 天正是「卡死」的判定門檻，
     * 等於這則告警的 SLO 直接歸零。
     *
     * <p>這一支同時釘住兩件事，缺一都會轉紅：
     * <ol>
     *   <li>publish 走的是<b>會傳遞失敗</b>的 {@code publishConfirmed}。改回 fire-and-forget 的
     *       {@code publish} ⇒ 呼叫端永遠看到「成功」、永遠不會釋放租約 ⇒ 本 verify 失敗。
     *       （《多副本排程搶佔規範》§四 第 4 條：面對吞例外的 publisher，光加 catch 是零修復。）</li>
     *   <li>釋放的 CAS 條件是<b>剛剛 claim 進去的那個租約值</b>，還原目標是 claim <b>之前</b>的舊值。
     *       用別的值（例如 now）等於把別人的租約也一起清掉。</li>
     * </ol>
     */
    @Test
    @DisplayName("publishConfirmed 回 false ⇒ 用剛 claim 的租約值 CAS 還原，下一 tick 就能重試")
    void publishNotConfirmed_releasesLeaseWithClaimedValue() {
        LocalDateTime previous = LocalDateTime.of(2026, 1, 1, 0, 0);
        AfterSaleCaseEntity acase = stuckCase("A001");
        acase.setReturnStuckAlertedAt(previous);

        when(caseMapper.selectList(any())).thenReturn(List.of(acase));
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(anyLong())).thenReturn(42L);
        when(obsRabbitPublisher.publishConfirmed(
                anyString(), anyString(), any(byte[].class), anyString(), anyLong()))
                .thenReturn(false);

        newScheduler().run();

        ArgumentCaptor<LocalDateTime> claimedLease = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(caseMapper).claimReturnStuckAlert(
                anyString(), anyString(), any(), claimedLease.capture());

        ArgumentCaptor<LocalDateTime> expected = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> restoreTo = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(caseMapper).releaseReturnStuckAlertLease(
                eq("A001"), expected.capture(), restoreTo.capture());

        assertEquals(claimedLease.getValue(), expected.getValue(),
                "釋放的 CAS 條件必須是剛剛 claim 寫進去的那個租約值——這是唯一能證明"
                        + "「這個租約還是我的」的述詞；用別的值會清掉他人的租約");
        assertEquals(previous, restoreTo.getValue(),
                "還原目標必須是 claim 之前的舊值，讓該案件回到候選集合");
    }

    /** 對稱面：publish 成功就<b>不得</b>釋放租約，否則冷卻機制整個失效、每天都會重發。 */
    @Test
    @DisplayName("publishConfirmed 回 true ⇒ 不得釋放租約（冷卻要生效）")
    void publishConfirmed_keepsLease() {
        when(caseMapper.selectList(any())).thenReturn(List.of(stuckCase("A001")));
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(anyLong())).thenReturn(42L);

        newScheduler().run();

        verify(caseMapper, never()).releaseReturnStuckAlertLease(anyString(), any(), any());
    }

    /** publish 拋例外（連線層失敗）同樣要釋放，且不得中斷整批掃描。 */
    @Test
    @DisplayName("publish 拋例外 ⇒ 釋放租約，且後續案件照跑")
    void publishThrows_releasesLeaseAndContinuesBatch() {
        when(caseMapper.selectList(any()))
                .thenReturn(List.of(stuckCase("A_BAD"), stuckCase("A_OK")));
        when(caseMapper.claimReturnStuckAlert(anyString(), anyString(), any(), any())).thenReturn(1);
        when(sellerGrpcClient.resolveAccountIdForStore(anyLong())).thenReturn(42L);
        when(obsRabbitPublisher.publishConfirmed(
                anyString(), anyString(), any(byte[].class), anyString(), anyLong()))
                .thenThrow(new RuntimeException("broker down"))
                .thenReturn(true);

        newScheduler().run();

        verify(caseMapper).releaseReturnStuckAlertLease(eq("A_BAD"), any(), any());
        verify(caseMapper, never()).releaseReturnStuckAlertLease(eq("A_OK"), any(), any());
        verify(obsRabbitPublisher, times(2)).publishConfirmed(
                anyString(), anyString(), any(byte[].class), anyString(), anyLong());
    }
}
