package io.github.will413028.hawkr.payment.logistics.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsCostFactJoinService;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCostFactRepository;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCostFactEntity;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.time.LocalDateTime;
import java.time.OffsetDateTime;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

/**
 * {@code logistics.cost.incurred} 的 consumer——durable join 的<b>成本側入口</b>（70 §3.1.4）。
 *
 * <h2>它做兩件事，順序不可顛倒</h2>
 * <ol>
 *   <li><b>先持久化成本事實</b>（{@code logistics_cost_facts}）——這是「先記錄，歸屬未定」
 *       （SD-15 逐字）的機械形式。</li>
 *   <li><b>再求值收斂述詞</b>（{@link LogisticsCostFactJoinService#converge}）——
 *       責任若已 financially final，這一次就會建立應收；否則 durably wait。</li>
 * </ol>
 * 🔴 <b>順序顛倒會讓「兩個事件同時抵達」變成漏建</b>：converge 讀不到還沒寫進去的事實，
 * 而 N16 那一側也讀不到 ⇒ 兩邊都以為對方沒到，應收永遠不建立，
 * <b>而且每一步都「照規則成功」、沒有任何錯誤訊號</b>。
 *
 * <h2>🔴 fail-loud 的邊界（§20）</h2>
 * 以下一律拋出 ⇒ 既有 retry／DLQ，<b>不得 log.warn ＋ ACK</b>：
 * <ul>
 *   <li>{@code provider_delivery_ref} 為空——它是 identity 的錨，沒有它這筆成本無處歸屬</li>
 *   <li>{@code estimated_cost} 不是正整數——0／負數代表 producer 契約壞了；
 *       「沒有成本」的正確表達是<b>不發這個事件</b>（見 {@code CostCapability} 的 empty 語意）</li>
 *   <li>{@code store_id}／{@code order_no} 缺漏——應收要向誰收、對應哪張單，兩者都不可推導</li>
 *   <li>收斂結果 {@code CONFLICT}——那代表正在向不該付的人收錢，或 identity 不變式壞了</li>
 * </ul>
 *
 * <h2>⚠️ 重投是正常的，不是錯誤</h2>
 * MQ 至少一次投遞。重投時 {@code insertIfAbsent} 回 {@code false}（兩道 UNIQUE 擋下），
 * converge 仍會照跑一次——它是冪等的，只讀當前事實。
 * 🔴 <b>重投必須 ACK</b>，否則同一則訊息會無限重試到 DLQ，而它其實什麼問題都沒有。
 */
@Component
public class LogisticsCostIncurredConsumer {

    private static final ObsLogger log = ObsLogger.forClass(LogisticsCostIncurredConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    private final LogisticsCostFactRepository costFacts;
    private final LogisticsCostFactJoinService join;

    public LogisticsCostIncurredConsumer(LogisticsCostFactRepository costFacts,
                                         LogisticsCostFactJoinService join) {
        this.costFacts = costFacts;
        this.join = join;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PAYMENT_LOGISTICS_COST_INCURRED,
            containerFactory = "manualAckContainerFactory")
    public void onCostIncurred(org.springframework.amqp.core.Message message,
                               Channel channel,
                               @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag)
            throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        try {
            JsonNode payload = MAPPER.readTree(body);

            // wire 是 snake_case（producer record 帶 @JsonNaming(SnakeCaseStrategy)）。
            // 欄名與型別的錨在 shipping-service 的 LogisticsCostIncurredPayload.java（70 §7.1）。
            String eventId = payload.path("event_id").asText("");
            String provider = payload.path("provider").asText("");
            String legRef = payload.path("provider_delivery_ref").asText("");
            String storeId = payload.path("store_id").asText("");
            String orderNo = payload.path("order_no").asText("");
            String shipmentId = payload.path("shipment_id").asText("");
            String estimatedCost = payload.path("estimated_cost").asText("");
            String occurredAt = payload.path("occurred_at").asText("");

            if (legRef.isBlank()) {
                throw new IllegalArgumentException(
                        "logistics.cost.incurred 缺 provider_delivery_ref——它是 identity 的錨"
                                + "（70 §3.1.2），沒有它這筆成本無處歸屬，"
                                + "而 receivable 的兩道 UNIQUE 也都錨在它身上");
            }
            if (provider.isBlank() || storeId.isBlank() || orderNo.isBlank()) {
                throw new IllegalArgumentException(
                        "logistics.cost.incurred 缺必要欄位：provider='" + provider
                                + "' store_id='" + storeId + "' order_no='" + orderNo
                                + "'——應收要向誰收、對應哪張單，兩者都不可推導");
            }

            long cost = parsePositiveAmount(estimatedCost);
            long store = parseLongOrThrow(storeId, "store_id");

            LogisticsCostFactEntity fact = new LogisticsCostFactEntity();
            fact.setId(TsidUtil.generate());
            fact.setProvider(provider);
            fact.setProviderDeliveryRef(legRef);
            fact.setStoreId(store);
            fact.setOrderNo(orderNo);
            // ⚠️ 允許缺席：取消可能發生在 shipment 建立之前（70 §7.1）。
            fact.setShipmentId(shipmentId.isBlank() ? null : parseLongOrThrow(shipmentId, "shipment_id"));
            fact.setEstimatedCost(cost);
            fact.setOccurredAt(parseOccurredAt(occurredAt));
            // 🔴 event_id 缺席時不得自己補一個 UUID：那會讓 L1 去重失效，
            //    而症狀是「重投產生第二筆成本事實」——雖然 delivery_ref 的 UNIQUE 仍擋得住，
            //    但那是第二道防線，不該讓第一道靜默失能。
            if (eventId.isBlank()) {
                throw new IllegalArgumentException(
                        "logistics.cost.incurred 缺 event_id——L1 去重的唯一依據（36 §7.4.1）");
            }
            fact.setEventId(eventId);

            // ① 先持久化事實（重投時回 false，那是正常的）
            boolean stored = costFacts.insertIfAbsent(fact);

            // ② 再求值收斂述詞。🔴 重投時**仍然要跑**：上一次可能在寫完事實之後、
            //    converge 之前就崩潰了，那時應收還沒建。converge 是冪等的，重跑安全。
            LogisticsCostFactJoinService.Outcome outcome = join.converge(legRef);

            if (outcome == LogisticsCostFactJoinService.Outcome.CONFLICT) {
                // 🔴 §20：financial inconsistency 不得 log.warn ＋ ACK 吞掉。
                throw new IllegalStateException(
                        "成本事實與既有物流應收／責任判定牴觸，已拒絕處置（詳見前一行 error log）"
                                + " ref=" + legRef + " provider=" + provider);
            }

            log.info("LogisticsCostIncurredConsumer.done",
                    "成本事實已處理 stored=" + stored + " outcome=" + outcome
                            + " ref=" + legRef + " cost=" + cost,
                    "orderNo", orderNo);
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            log.error("LogisticsCostIncurredConsumer.error",
                    "處理 logistics.cost.incurred 失敗: " + e.getMessage(), e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag,
                    "LogisticsCostIncurredConsumer", e);
        }
    }

    /**
     * 🔴 金額必須是正整數字串。
     *
     * <p>wire 上是 {@code String}（65 §6A.1.3 通則：id 與金額一律 string，
     * 因為 notification-service 把 payload 解進 {@code map[string]interface{}}，
     * {@code encoding/json} 對 {@code any} 用 float64 ⇒ 大數會被靜默截斷）。
     *
     * <p>⚠️ <b>0 不是合法值</b>：「沒有成本」的正確表達是 producer 不發這個事件
     * （{@code CostCapability} 回 empty），而不是發一個 0。
     * 收到 0 代表 producer 把「查不到」寫成了「零元」——那會建出一筆金額為 0 的應收。
     */
    private static long parsePositiveAmount(String raw) {
        long v = parseLongOrThrow(raw, "estimated_cost");
        if (v <= 0) {
            throw new IllegalArgumentException(
                    "logistics.cost.incurred 的 estimated_cost 必須為正，收到 " + v
                            + "——「沒有成本」的正確表達是不發這個事件，不是發 0");
        }
        return v;
    }

    private static long parseLongOrThrow(String raw, String field) {
        try {
            return Long.parseLong(raw.trim());
        } catch (RuntimeException e) {
            throw new IllegalArgumentException(
                    "logistics.cost.incurred 的 " + field + " 不是合法的十進位整數字串：'" + raw + "'");
        }
    }

    /**
     * ISO 8601 UTC 帶 {@code Z} → {@code LocalDateTime}（值語意 UTC）。
     *
     * <p>🔴 <b>不得用 {@code now()} 兜底</b>：它是 {@code charge_basis_frozen_at} 的來源，
     * 計費依據凍結在<b>事實發生</b>的那一刻。用收到事件的時間冒充，重投時會凍到完全不同的時點。
     */
    private static LocalDateTime parseOccurredAt(String raw) {
        if (raw == null || raw.isBlank()) {
            throw new IllegalArgumentException(
                    "logistics.cost.incurred 缺 occurred_at——它是 charge_basis_frozen_at 的來源，"
                            + "不得用收到事件的時間冒充");
        }
        try {
            return OffsetDateTime.parse(raw.trim()).toLocalDateTime();
        } catch (RuntimeException e) {
            throw new IllegalArgumentException(
                    "logistics.cost.incurred 的 occurred_at 不是 ISO 8601 UTC：'" + raw + "'");
        }
    }
}
