package io.github.will413028.hawkr.aftersales.scheduler;

import com.baomidou.mybatisplus.core.conditions.query.LambdaQueryWrapper;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.payload.ShipmentAnomalyDetectedPayload;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AfterSaleCaseStatus;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalySeverity;
import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import io.github.will413028.hawkr.common.utils.code.helper.ShipmentAnomalyRules;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.boot.autoconfigure.condition.ConditionalOnProperty;
import org.springframework.scheduling.annotation.Scheduled;
import org.springframework.stereotype.Component;

import java.time.LocalDateTime;
import java.util.List;
import java.util.UUID;

/**
 * §5.21 配送異常賣家通知 — RETURN_STUCK 偵測 scheduler。
 *
 * <p>掃描 {@code aftersale_cases} 中 status=25（退貨運送中）超過設定門檻天數仍未進展的案件，
 * 對每筆發 {@code shipment.anomaly.detected} event（type=RETURN_STUCK）給 notification-service。
 *
 * <p>每天執行一次（cron 預設 03:00 UTC = 11:00 台灣），降低噪音。
 *
 * <p>跨服務 publish：直接寫至 {@code hawkr.shipping} exchange（與 shipping-service 共用 routing key），
 * notification-service 端的 consumer 不區分 producer 來源。
 *
 * <p>實作備註：aftersale_cases 無 {@code status_changed_at} 欄位，採 {@code updated_at} 作近似
 * 「最後狀態變動時間」；任何更新都會推進 updated_at，可作為「停滯時間」代理指標。
 *
 * <p><b>搶佔語意（issue #2294 L6）</b>：原本<b>完全沒有搶佔</b>，prod {@code replicas=2} 下同一則
 * 卡死告警會被兩個 pod 各發一次。修法＝<b>lease on {@code aftersale_cases.return_stuck_alerted_at}</b>，
 * 租約長度 = {@code returnStuckDays}（＝告警冷卻），claim 成功才 publish。
 * <p>⚠️ <b>不得走例外 B（只發一次）</b>：卡死告警在狀態沒解除時再發一次對收件人<b>仍有新資訊</b>
 * （沒人處理就該一直吵），與 24h 預警方向相反（09 §8.5）。所以這裡必須是會過期的租約，
 * 不是 {@code IS NULL} 的永久旗標。
 *
 * 設計書：06 §5.21.2 / 36 §3.3 / 09 §8.5
 */
@Component
@ConditionalOnProperty(name = "hawkr.aftersales.return-stuck-alert.enabled",
        havingValue = "true", matchIfMissing = true)
public class ReturnStuckAlertScheduler {

    private static final ObsLogger log = ObsLogger.forClass(ReturnStuckAlertScheduler.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final AfterSaleCaseMapper caseMapper;
    private final ObsRabbitPublisher obsRabbitPublisher;
    private final io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient sellerGrpcClient;
    private final io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient buyerGrpcClient;

    @Value("${hawkr.shipping.anomaly.return-stuck-days:14}")
    private int returnStuckDays;

    @Value("${hawkr.aftersales.return-stuck-alert.batch-size:200}")
    private int batchSize;

    /**
     * issue #2294：{@code returnStuckDays} 的<b>下界檢查</b>（啟動即 fail-fast）。
     *
     * <p><b>為什麼需要</b>：這一個設定值同時扮演三個角色——① 候選門檻
     * （{@code updated_at < now − N 天}）② 告警冷卻 ③ <b>租約長度</b>。第三個角色是本 issue
     * 引入的，而它把「搶佔保護」綁在一個<b>沒有守衛的可調參數</b>上：設成 {@code 0} 時
     * {@code leaseUntil == 取值當下} ⇒ 租約長度歸零 ⇒ 另一個 pod 在微秒後 claim 就會通過
     * （{@code return_stuck_alerted_at <= now} 立刻成立）⇒ <b>搶佔整個失效，而且完全無聲</b>：
     * SQL 還在、CAS 還在、程式碼看起來一模一樣，只有行為回到修復前。
     * 負值更糟（{@code leaseUntil} 落在過去），且會讓候選門檻變成「未來」。
     *
     * <p><b>為什麼是 fail-fast 而不是夾住（clamp）</b>：夾住等於<b>默默忽略操作者的意圖</b>
     * ——把 0 夾成 14 之後，operator 以為自己把門檻調成「立刻告警」，實際上是 14 天，
     * 而唯一的線索是一行 WARN。這個值有預設值（14），唯一會踩到 ≤0 的路徑是「有人明確設了它」，
     * 也就是部署期的組態錯誤，該在部署當下就吵。
     * <p>要停用本排程請用既有的開關 {@code hawkr.aftersales.return-stuck-alert.enabled=false}，
     * 不要拿 {@code return-stuck-days=0} 當停用手段。
     */
    @jakarta.annotation.PostConstruct
    void validateReturnStuckDays() {
        if (returnStuckDays < MIN_RETURN_STUCK_DAYS) {
            throw new IllegalStateException(
                    "hawkr.shipping.anomaly.return-stuck-days 必須 >= " + MIN_RETURN_STUCK_DAYS
                            + "（目前 " + returnStuckDays + "）。此值同時是候選門檻、告警冷卻與"
                            + "issue #2294 的搶佔租約長度；<= 0 會讓租約長度歸零，"
                            + "多副本搶佔靜默失效（SQL 與 CAS 都還在，只有行為回到修復前）。"
                            + "要停用本排程請用 hawkr.aftersales.return-stuck-alert.enabled=false。");
        }
    }

    /** 租約長度的下界（天）：必須 &gt; 單筆最壞耗時（一次 gRPC 解析 ＋ 一次 MQ publish），1 天已極寬鬆。 */
    static final int MIN_RETURN_STUCK_DAYS = 1;

    /**
     * publisher-confirm 的等待上限（毫秒）。
     *
     * <p>單筆最壞耗時 = 2 次帶 deadline 的 gRPC 反解 ＋ 本值；batch 上限 200 筆 ⇒ 最壞單 tick
     * 佔用仍遠小於租約下界（1 天），§三 第 5 步的下界檢查成立。
     * 取 5 秒與 payment outbox（{@code payment.outbox.confirm-timeout-ms}）同量級。
     */
    static final long PUBLISH_CONFIRM_TIMEOUT_MS = 5_000L;

    public ReturnStuckAlertScheduler(AfterSaleCaseMapper caseMapper,
                                     ObsRabbitPublisher obsRabbitPublisher,
                                     io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient sellerGrpcClient,
                                     io.github.will413028.hawkr.aftersales.grpc.client.BuyerGrpcClient buyerGrpcClient) {
        this.caseMapper = caseMapper;
        this.obsRabbitPublisher = obsRabbitPublisher;
        this.sellerGrpcClient = sellerGrpcClient;
        this.buyerGrpcClient = buyerGrpcClient;
    }

    /**
     * 每天 03:00 UTC 執行（= 11:00 台灣時間）
     */
    @Scheduled(cron = "${hawkr.aftersales.return-stuck-alert.cron:0 0 3 * * ?}")
    public void run() {
        // ⚠️ R4-P1-2（2026-08-08）：這個 now 只用於「候選述詞」與「租約未到期判定」。
        // 租約的**基準**不在這裡取（見下方迴圈內），否則整批共用同一個到期時間。
        LocalDateTime now = DateTimeUtils.nowUtc();
        LocalDateTime cutoff = now.minusDays(returnStuckDays);
        List<AfterSaleCaseEntity> candidates = caseMapper.selectList(
                new LambdaQueryWrapper<AfterSaleCaseEntity>()
                        .eq(AfterSaleCaseEntity::getStatus, AfterSaleCaseStatus.RETURN_SHIPPING.value())
                        .lt(AfterSaleCaseEntity::getUpdatedAt, cutoff)
                        // 租約未到期的列由別的 pod 持有（或仍在冷卻中），本輪不撈
                        .and(lease -> lease
                                .isNull(AfterSaleCaseEntity::getReturnStuckAlertedAt)
                                .or()
                                .le(AfterSaleCaseEntity::getReturnStuckAlertedAt, now))
                        .last("LIMIT " + batchSize)
        );

        if (candidates.isEmpty()) {
            log.info("ReturnStuckAlertScheduler.empty",
                    "本輪無 RETURN_STUCK 候選 cutoff=" + cutoff);
            return;
        }

        int published = 0;
        for (AfterSaleCaseEntity acase : candidates) {
            // 宣告在 try 之外：catch 分支要用它做租約釋放的 CAS 條件（見 releaseLease）。
            LocalDateTime leaseUntil = null;
            try {
                // ⚠️ R4-P1-2（2026-08-08）：租約基準取 **claim 當下**而非 tick 起點的 now。
                // 取 tick 起點 ⇒ 全批共用同一個到期時間 ⇒ 第 k 筆的**有效**租約
                // = returnStuckDays − (前 k−1 筆的耗時)，不變式退化成「批次總耗時 < 租約」，
                // 會隨 batchSize（預設 200，可由設定調大）一起破。改在這裡取之後，
                // 不變式降為「**單筆**最壞耗時 < 租約」——與 batchSize 無關，是結構保證而非參數巧合。
                // 述詞用的 now/cutoff 維持 tick 起點：那個方向讓候選集合與可搶佔集合更小，是保守的。
                // issue #2294 L6：租約長度 = returnStuckDays（與告警冷卻同值）
                leaseUntil = DateTimeUtils.nowUtc().plusDays(returnStuckDays);
                // issue #2294 L6：lease claim —— 搶到才 publish（0 = 另一個 pod 已搶走）
                int claimed = caseMapper.claimReturnStuckAlert(
                        acase.getCaseNo(), AfterSaleCaseStatus.RETURN_SHIPPING.value(), now, leaseUntil);
                if (claimed == 0) {
                    log.info("ReturnStuckAlertScheduler.claim_skip",
                            "租約由他人持有或狀態已變更，skip caseNo=" + acase.getCaseNo());
                    continue;
                }
                if (publishReturnStuck(acase)) {
                    published++;
                } else {
                    releaseLease(acase, leaseUntil, "broker 未 ack／unroutable／confirm timeout");
                }
            } catch (Exception e) {
                log.error("ReturnStuckAlertScheduler.publish.failed",
                        "發 RETURN_STUCK event 失敗 caseNo=" + acase.getCaseNo()
                                + " err=" + e.getMessage());
                releaseLease(acase, leaseUntil, e.getClass().getSimpleName() + ": " + e.getMessage());
            }
        }

        log.info("ReturnStuckAlertScheduler.done",
                "RETURN_STUCK 偵測完成 candidates=" + candidates.size() + " published=" + published);
    }

    /**
     * R4 追加（2026-08-08）：publish 沒成功時把租約還原成 claim 之前的值。
     *
     * <p><b>要解決的問題</b>：租約長度 = {@code returnStuckDays}（14 天）＝告警冷卻，這個值是
     * 照「成功送出之後多久該再吵一次」訂的。但它同時也是**失敗之後多久才會重來**——
     * 一次 MQ 不可用就讓這則卡死告警靜默 14 天，而卡死告警的整個存在理由就是
     * 「在 14 天內把這件事講出來」⇒ SLO 直接歸零。還原之後，重來的間隔變成
     * <b>一個 tick（每日 03:00）</b>。
     *
     * <p><b>不會搶到別人的租約</b>：CAS 條件是「{@code return_stuck_alerted_at} 仍等於我剛寫進去
     * 的那個值」。若期間有人重新 claim（值不同），本次還原回 0 列、不做任何事。
     *
     * <p><b>已知殘留（誠實標註）</b>：{@code publishConfirmed} 回 {@code false} 的成因包含
     * confirm timeout，那種情況下訊息**可能其實已經送達**；還原之後隔天會再發一次，
     * 由消費端當作重複投遞吸收（本 payload 每次帶新的 {@code UUID}，所以是「同一件事的第二則通知」
     * 而不是可被 event id 去重的重放）。取捨是刻意的：**多吵一次** ≪ **靜默 14 天**。
     * 進程直接消失（OOM／驅逐）時本方法不會被執行——那是 14 天租約仍然存在的理由，
     * 也是規範 §二 第 5 步「catch 裡有回退不算回頭路」講的那件事。
     */
    private void releaseLease(AfterSaleCaseEntity acase, LocalDateTime leaseUntil, String reason) {
        if (leaseUntil == null) {
            // claim 之前就失敗（例如租約基準都還沒取到）⇒ 沒有租約要還。
            return;
        }
        try {
            int released = caseMapper.releaseReturnStuckAlertLease(
                    acase.getCaseNo(), leaseUntil, acase.getReturnStuckAlertedAt());
            log.warn("ReturnStuckAlertScheduler.lease_released",
                    "publish 未成功，租約已還原（released=" + released + "，0 = 期間已被他人重新 claim）"
                            + " caseNo=" + acase.getCaseNo() + " reason=" + reason);
        } catch (Exception e) {
            // 還原失敗不是新的破口：不還原就等於維持修復前的行為（等滿一個冷卻週期），
            // 正確性仍由租約本身保證。
            log.error("ReturnStuckAlertScheduler.lease_release_failed",
                    "租約還原失敗，本案件要等滿 " + returnStuckDays + " 天冷卻才會重試 caseNo="
                            + acase.getCaseNo() + " err=" + e.getMessage());
        }
    }

    /**
     * @return {@code true} 只在 broker ack 且訊息可路由時；{@code false} 代表沒送出去
     *         （nack／unroutable／confirm timeout）。
     *
     * <p><b>為什麼從 {@code publish} 換成 {@code publishConfirmed}</b>：搶佔閘門（claim）在
     * publish 之前，所以「publish 到底有沒有成功」決定了要不要把租約還回去。而
     * {@link ObsRabbitPublisher#publish} 是 fire-and-forget——連線層失敗會拋，但
     * <b>broker nack 與 unroutable 完全不會有任何訊號</b>，呼叫端永遠看到「成功」。
     * 在那個前提下光加 catch 是零修復（《多副本排程搶佔規範》§四 例外 B 第 4 條講的是同一件事：
     * 面對吞例外的 publisher，換順序／加 catch 都不構成修復）。
     * {@code publishConfirmed} 等 publisher-confirm（全服務已由 observability-common 開啟
     * {@code publisher-confirm-type=correlated} ＋ {@code publisher-returns} ＋ {@code mandatory}），
     * 才是真的失敗訊號。
     */
    private boolean publishReturnStuck(AfterSaleCaseEntity acase) throws Exception {
        ShipmentAnomalyType type = ShipmentAnomalyType.RETURN_STUCK;
        ShipmentAnomalySeverity severity = ShipmentAnomalyRules.severityOf(type);
        // Plan C Phase 3 追加：sellerAccountId 對外是已反解的登入 account_id，非 store_id
        // 代理鍵，見 ShipmentLabelConsumer 同款註解。
        Long resolvedSellerAccountId = sellerGrpcClient.resolveAccountIdForStore(acase.getStoreId());
        // 2026-08-05 identity-naming triage：對稱處理買家側——buyerAccountId 契約上同樣是
        // 「已反解的登入 account_id」（見 ShipmentAnomalyDetectedPayload javadoc「買家 accountId」），
        // 不是 aftersale_cases 的 buyer_id 代理鍵。本呼叫點是同一批 remediation（Plan C Phase 3
        // Task 9b）唯一漏掉的一處：ShipmentLabelConsumer.publishPayUniLgsNoCodeAnomaly 與
        // shipping-service 的 ShipmentAnomalyEventPublisher（同一 payload 的另一 producer）
        // 早就都有這一步。
        Long buyerAccountId = resolveNotifyBuyerAccountId(acase.getBuyerId());
        ShipmentAnomalyDetectedPayload payload = new ShipmentAnomalyDetectedPayload(
                UUID.randomUUID().toString(),
                acase.getOrderNo(),
                "",                                       // 退貨流程無對應 shipments.id（退貨運單在 aftersale_case 上）
                resolvedSellerAccountId,
                buyerAccountId,
                type.value(),
                type.label(),
                severity.value(),
                DateTimeUtils.formatIso8601(DateTimeUtils.nowUtc()),
                acase.getStoreId()
        );
        byte[] bytes = MAPPER.writeValueAsBytes(payload);
        return obsRabbitPublisher.publishConfirmed(
                MqConstants.EXCHANGE_SHIPPING,
                MqConstants.ROUTING_KEY_SHIPMENT_ANOMALY_DETECTED,
                bytes,
                MqConstants.EVENT_TYPE_SHIPMENT_ANOMALY_DETECTED,
                PUBLISH_CONFIRM_TIMEOUT_MS
        );
    }

    /**
     * 反解 buyer_id（PK，{@code aftersale_cases.buyerId}）→ 登入 principal account_id，
     * 與上方 sellerAccountId 反解（{@link io.github.will413028.hawkr.aftersales.grpc.client.SellerGrpcClient#resolveAccountIdForStore}）
     * 同一「查詢失敗降級回 null」哲學，刻意**不**與
     * {@code AfterSalesEventPublisherImpl.resolveNotifyBuyerAccountId} 抽共用：後者失敗回 0L
     * 是因為那些 payload 的買家欄位是 notification 的**路由**欄位（fail-closed，0 = 查無此人、
     * 不會誤投），本欄位則只是渲染上下文（缺失即不填），兩者的降級語意不同，硬抽會逼一邊改行為。
     *
     * <p><b>2026-08-05 真修（本方法先前的「改用 {@code Map#get}」修法被證明是 no-op，這次是
     * 對根因的修復）</b>：buyer-service {@code GetPrincipalsByBuyerIDs} 的 SQL 是
     * {@code SELECT input.id, COALESCE(b.account_id, input.id) ... LEFT JOIN}——**每個查詢
     * id 一律有 map entry**，COALESCE 保證絕不缺 key，soft-deleted / 未知 buyer_id 一樣拿到
     * 一個看似合法的值（echo 回原始 buyer_id）。上一輪把
     * {@code Map#getOrDefault(buyerId, buyerId)} 換成 {@code Map#get(buyerId)}
     * 想靠「缺 key → null」做降級判斷，但 map **從來不會缺 key**，所以那次修改是 no-op：
     * 查無對應買家列時仍會把未反解的 raw buyer_id 當成「解析結果」發布出去，正是本
     * campaign 要根除的 identity-confusion bug class 本身。
     *
     * <p><b>真正的修法是 contract 層——{@code buyer.proto} 的
     * {@code ResolvePrincipalsResponse} 新增 {@code unresolved_ids}</b>（純附加欄位，
     * 既有 principals map 語意不變）：buyer-service 現在會誠實回報「這個 id 沒有對應的
     * live buyers 列」，而不是只回一個無法分辨真假的 echo 值。本方法據此判斷：
     * {@code buyerId} 出現在 {@link io.github.will413028.hawkr.common.grpc.PrincipalResolution#unresolvedIds()}
     * 中 → 真正查無此人，回傳 {@code null}（可證明的「不知道」）；否則
     * {@code principals().get(buyerId)} 保證存在**且**保證是真實解析值，直接回傳。
     *
     * <p><b>已知仍在舊 echo-back 反模式、同一個 bug class 的其餘 producer（campaign-level
     * follow-up 範圍，本次改動不觸碰）</b>：不要相信任何寫死的 enumerated 清單——上一版
     * javadoc 在這裡宣稱「三個 producer 本應對稱」，而本次修這段時手動列出的清單同樣當場
     * 被下面這條 grep 證明是不完整的（漏了 media-service {@code ImportAssetService}
     * 兩處、cart-service {@code CartService}、coupon-service {@code CouponGrpcService}
     * 兩處等）——手動維護的清單本身就會腐爛，不要再嘗試維護一份，每次都重新跑：
     * <p>Java（已於 2026-08-05 實測跑過，此指令本身抓得到）：
     * {@code grep -rn -A2 "resolvePrincipals(" services/ --include='*.java' | grep -B2 getOrDefault}
     * <p>Go 沒有 {@code getOrDefault} 這種單一 token 的 idiom（是 map-lookup-miss 手寫
     * if 分支），上面這條抓不到，要另外人工看：先
     * {@code grep -rn "ResolvePrincipal(\|resolvePrincipals(" services/*}{@code /internal --include='*.go'}
     * 列出候選呼叫點，再逐一讀呼叫端 map-miss 分支是否 echo 回原始 id
     * （已知至少 {@code im-service internal/client/buyer/client.go:68}、
     * {@code social-service internal/client/buyer/client.go:98} 屬此類，但這兩個是
     * grep 當下順手看到的例子，不是窮舉——用來提醒別漏了 Go 端，不是可信清單）。**單純
     * {@code getOrDefault}→{@code get} 這種 call-site 層級的修法已被證明是 no-op——真正的
     * 修復是 contract 層新增的 {@code unresolved_ids} 欄位，每個 producer 都要改成消費它
     * （如同本方法），而不能重演「以為換個 map 存取方式就修好了」的錯誤。
     *
     * <p><b>Version skew 風險（部署順序）</b>：{@code unresolved_ids} 是否真的被填值，取決於
     * buyer-service 是否已部署到含本欄位的版本——proto 是 wire-compatible 的附加欄位，若
     * buyer-service 還在跑舊 image，gRPC 回應就是預設值（空 repeated field），
     * {@code resolution.unresolvedIds()} 會永遠是空集合，本方法會直接落到
     * {@code principals().get(buyerId)} 分支，對「查無此人」的 buyer_id 一樣拿到 echo
     * 回來的原始值——與修復前完全相同的 fail-open。**部署本修復時 buyer-service 必須先於
     * aftersales-service 上線**（目前兩者同在 {@code deploy-k8s.yml} 的 changed_services，
     * 單一 run 內完成，此風險窗口小但非零）。下方加了一個啟發式 WARN log：當
     * {@code unresolvedIds} 整批為空、但某個 buyerId 的解析值又剛好等於自己時，記一筆警告。
     * <p><b>這個 heuristic 對舊帳號會誤發，不是決定性訊號</b>：2026-07-18 buyer rekey
     * 上線前建立的帳號，{@code buyer_id} 預設值本來就等於 {@code account_id}（rekey 後才由
     * {@code EnsureProfile} 改配發獨立 TSID）——dev 實測 {@code buyers} 表 140 筆中 51 筆
     * （36%，2026-08-05）屬此類，這批帳號的 buyerId 解析值「剛好等於自己」是真實、正確的
     * 解析結果，不是 echo-back，但一樣會觸發這條 WARN。因此看到這條警告時，先查該
     * {@code buyer_id} 的 {@code buyers.created_at} 是否早於 2026-07-18——是的話大機率是
     * 誤報，只有搭配「該 buyer_id 建立於 rekey 之後」才有鑑別力；不能單看警告本身就認定是
     * version skew，仍要配合部署順序核對。
     */
    private Long resolveNotifyBuyerAccountId(Long buyerId) {
        if (buyerId == null) {
            return null;
        }
        try {
            io.github.will413028.hawkr.common.grpc.PrincipalResolution resolution =
                    buyerGrpcClient.resolvePrincipals(java.util.List.of(buyerId));
            if (resolution.unresolvedIds().contains(buyerId)) {
                return null;
            }
            Long resolved = resolution.principals().get(buyerId);
            if (resolution.unresolvedIds().isEmpty() && java.util.Objects.equals(resolved, buyerId)) {
                log.warn("ReturnStuckAlertScheduler.resolveNotifyBuyerAccountId.possibleVersionSkew",
                        "unresolvedIds 為空且反解值等於原始 buyerId，可能是 buyer-service"
                                + " 尚未部署到支援 unresolved_ids 的版本（version skew）——無法區分"
                                + "『真的解析成功』與『echo-back 被誤判成功』，buyerId=" + buyerId);
            }
            return resolved;
        } catch (Exception e) {
            log.warn("ReturnStuckAlertScheduler.resolveNotifyBuyerAccountId.error",
                    "buyer_id 反解 account_id 失敗，降級回 null buyerId=" + buyerId + "，err=" + e.getMessage());
            return null;
        }
    }
}
