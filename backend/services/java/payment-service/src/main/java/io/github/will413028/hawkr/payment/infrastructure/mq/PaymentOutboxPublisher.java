package io.github.will413028.hawkr.payment.infrastructure.mq;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.obs.mq.ObsRabbitPublisher;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.OutboxEventEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.OutboxEventMapper;
import org.springframework.beans.factory.annotation.Value;
import org.springframework.stereotype.Component;

import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;

/**
 * §2A.6 Outbox 事件發布器（payment-service）。
 * <p>
 * 將 outbox_events 表中 PENDING / FAILED 紀錄派送到 RabbitMQ exchange。
 * 由 {@link io.github.will413028.hawkr.payment.scheduler.OutboxEventPublisherScheduler}
 * 排程觸發。失敗時走指數退避重試。
 */
@Component
public class PaymentOutboxPublisher {

    private static final ObsLogger log = ObsLogger.forClass(PaymentOutboxPublisher.class);

    private final ObsRabbitPublisher rabbitPublisher;
    private final OutboxEventMapper outboxMapper;

    /**
     * §5.10.2 publisher-confirm 等待 broker ack 的逾時（毫秒）。
     * <p>
     * L1' afterCommit 即時投遞在原請求執行緒同步等待此逾時 → 取值不宜過長，寧可
     * 逾時 markFailed 交排程器兜底，也不阻塞付款 API 太久（見 §5.10.1）。
     */
    @Value("${payment.outbox.confirm-timeout-ms:3000}")
    private long confirmTimeoutMs;

    /**
     * issue #2294 H2：claim 租約長度（秒）。
     *
     * <p><b>上界</b>（《多副本排程搶佔規範》§三第 1–3 步）：候選述詞
     * （{@link OutboxEventMapper#findPendingEvents}）只有 {@code status IN (PENDING, FAILED)}
     * ＋ {@code next_retry_at <= NOW()}，<b>沒有任何時間上界</b> ⇒ 候選窗口寬度 <b>∞</b>
     * ⇒ 任何租約長度都合格；cron 週期 5 秒遠小於窗口，第 4 步的極端情況不成立。
     *
     * <p><b>下界</b>（同規範第 5 步：租約必須 &gt; 單筆工作的最壞耗時）：claim 之後唯一的長工作是
     * {@code publishConfirmed}，其上界由 {@code payment.outbox.confirm-timeout-ms}
     * （預設 <b>3 秒</b>）硬性封頂。取 <b>60 秒</b> = 20 倍餘裕，即使把 confirm-timeout 調到
     * 十幾秒仍成立。
     */
    static final long CLAIM_LEASE_SEC = 60L;

    public PaymentOutboxPublisher(ObsRabbitPublisher rabbitPublisher, OutboxEventMapper outboxMapper) {
        this.rabbitPublisher = rabbitPublisher;
        this.outboxMapper = outboxMapper;
    }

    /**
     * 發布單筆 outbox 事件（§5.10.2 publisher-confirm）。
     * <p>
     * 由兩條路徑共用：(a) L1' afterCommit 即時投遞、(b) OutboxEventPublisherScheduler
     * 每 5s 兜底。以 {@link ObsRabbitPublisher#publishConfirmed} 等待 broker ack，
     * 確認收下才 {@code markSENT}；nack / timeout / unroutable → {@code markFailed}
     * 走指數退避重投。
     * <p>
     * 🔴 <b>exchange 一律取自該列持久化的 {@code exchange} 欄</b>（36 §3.3.1 explicit persisted
     * exchange）：不 hardcode、不從 {@code routing_key} 推導、不從 {@code event_type} 推導。
     * 該欄為 null／空白時 <b>fail-closed</b>——不投遞、標 FAILED、走既有退避與 DEAD lifecycle，
     * <b>不得</b> fallback 回 {@code payment.events}：那會把「事件發錯 exchange」變成一次完全
     * 靜默的成功（訊息被接受、沒有 consumer、沒有錯誤）。
     *
     * @return true=broker 已確認收下（已標 SENT）；false=未確認（已標 FAILED）或未搶到本筆
     */
    public boolean publish(OutboxEventEntity event) {
        // issue #2294 H2：搶佔閘門必須在副作用（MQ 投遞）之前，且必須放在**兩條進入點的共同收斂點**。
        // 兩條進入點 = OutboxEventPublisherScheduler 每 5s 兜底 + PaymentOutboxWriter 的
        // afterCommit 即時投遞；閘門放 Scheduler 會漏掉 afterCommit 那條。
        // 閘門為 lease（推 next_retry_at）不是永久旗標——租約到期該列自動回到候選集合。
        LocalDateTime now = DateTimeUtils.nowUtc();
        int claimed = outboxMapper.claimForDispatch(event.getId(), now.plusSeconds(CLAIM_LEASE_SEC), now);
        if (claimed == 0) {
            log.info("PaymentOutboxPublisher.publish.notClaimed",
                    "本筆已被另一個 pod 認領或狀態已變，skip id=" + event.getId()
                            + " routing_key=" + event.getRoutingKey());
            return false;
        }

        // BL7 fail-closed 守衛：claim 之後、publish 之前驗證持久化的 exchange。
        // 位置理由：放 claim 之前 → 沒有獨占權就去標 FAILED，會和其他 pod 互相覆蓋 retry_count；
        //          放 publish 之後 → 已經發出去了，守不住任何東西。
        // writer/factory 已 fail-fast，這裡是 defense-in-depth：擋 corrupt／legacy／手寫 SQL 造出的壞列。
        String exchange = event.getExchange();
        if (exchange == null || exchange.isBlank()) {
            log.error("PaymentOutboxPublisher.publish.invalidExchange",
                    "outbox 列的 exchange 缺失或空白，拒絕投遞（不 fallback 回 payment.events）"
                            + " id=" + event.getId()
                            + " event_type=" + event.getEventType()
                            + " routing_key=" + event.getRoutingKey());
            // 不呼叫 Rabbit。直接走既有 guarded FAILED path：沿用退避序列，
            // 達 max-retries 時由 scheduler 標 DEAD（不是留著 60 秒租約空等）。
            markFailed(event, "persisted exchange 缺失或空白（fail-closed，未投遞）");
            return false;
        }

        byte[] payloadBytes = event.getPayload().getBytes(StandardCharsets.UTF_8);

        boolean acked;
        try {
            acked = rabbitPublisher.publishConfirmed(
                    exchange,
                    event.getRoutingKey(),
                    payloadBytes,
                    event.getEventType(),
                    confirmTimeoutMs
            );
        } catch (Exception e) {
            log.error("PaymentOutboxPublisher.publish.failed",
                    "事件發布失敗 id=" + event.getId() + " routing_key=" + event.getRoutingKey(), e);
            markFailed(event, e.getMessage());
            return false;
        }

        if (acked) {
            // issue #2294 H2：具名守衛 UPDATE，取代 updateById 全欄覆寫
            outboxMapper.markSent(event.getId(), DateTimeUtils.nowUtc());

            log.info("PaymentOutboxPublisher.publish.success",
                    "事件已確認投遞 id=" + event.getId() + " routing_key=" + event.getRoutingKey());
            return true;
        }

        // nack / timeout / unroutable：broker 未確認收下，交排程器兜底重投
        log.warn("PaymentOutboxPublisher.publish.unconfirmed",
                "broker 未確認投遞（nack/timeout/unroutable） id=" + event.getId()
                        + " routing_key=" + event.getRoutingKey());
        markFailed(event, "publisher-confirm 未 ack（nack/timeout/unroutable）");
        return false;
    }

    /**
     * 標記為 FAILED 並設定指數退避（5s, 15s, 45s, 135s, 405s）。
     *
     * <p>issue #2294 H2：具名守衛 UPDATE（{@code status <> SENT}），取代 {@code updateById}
     * 全欄覆寫——後者會把「另一個 pod 已標 SENT」的結果回寫成 FAILED，造成重複投遞一輪。
     */
    private void markFailed(OutboxEventEntity event, String error) {
        int retryCount = (event.getRetryCount() == null ? 0 : event.getRetryCount()) + 1;
        long delaySec = (long) (5 * Math.pow(3, retryCount - 1));
        outboxMapper.markFailedGuarded(event.getId(), retryCount, error,
                DateTimeUtils.nowUtc().plusSeconds(delaySec));
    }
}
