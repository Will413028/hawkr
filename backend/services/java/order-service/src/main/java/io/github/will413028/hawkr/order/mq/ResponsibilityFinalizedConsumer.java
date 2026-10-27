package io.github.will413028.hawkr.order.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ResponsibilityCause;
import io.github.will413028.hawkr.order.service.ResponsibilityFinalizedTxHandler;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Set;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

/**
 * {@code N16 fulfillment.responsibility.finalized} 的 order-service consumer（03 §5.7.3）。
 *
 * <p>🔴 <b>本 consumer 是 {@code DELIVERY_FAILED_NO_REFUND(54)} 的唯一執行者。</b>
 * order-service <b>不提供</b>人工改 {@code order.status} 的端點，也<b>不引入</b>補推排程或
 * 孤兒訂單掃描（03「無 reconcile / backstop」）⇒ {@code 35 → 54} 只能由本事件驅動。
 * 少了它，那個終態就是死碼，而訂單會無聲停在 {@code SHIPPING_ABNORMAL(35)}。
 *
 * <h2>分流（依 {@code responsibilityCause}）</h2>
 *
 * <table border="1">
 *   <caption>四個 resolved 成因的處置</caption>
 *   <tr><th>成因</th><th>本 handler 做什麼</th></tr>
 *   <tr><td>{@code BUYER_CAUSED}</td>
 *       <td>單一交易內 {@code 35 → 54} CAS ＋ 建 {@code grant_payment_tasks}</td></tr>
 *   <tr><td>{@code SELLER_CAUSED}／{@code PROVIDER_CAUSED}／{@code PLATFORM_CAUSED}</td>
 *       <td>🔴 <b>什麼都不做</b>——它們的 {@code refund_to_buyer = full}，走<b>既有</b>的
 *           全額退款 closure（AfterSales 主導 → {@code aftersales.refund.completed} FULL
 *           → {@code FULLY_REFUNDED(70)}，03 §5.7.1）。
 *           <b>不得</b>在這裡建第二套退款引擎，也不得把它們推進 {@code 54}。</td></tr>
 * </table>
 *
 * <h2>🔴 fail-loud：三種不合法 payload 一律進 DLQ，不得靜默吞掉</h2>
 *
 * <ul>
 *   <li><b>缺欄位</b>（{@code order_no} 或 {@code responsibility_cause} 為空）</li>
 *   <li><b>未知 {@code responsibilityCause}</b>——包含 {@code RESPONSIBILITY_UNRESOLVED}
 *       與不存在的 {@code SYSTEM_CAUSED}</li>
 *   <li><b>{@code financially_final != true}</b></li>
 * </ul>
 *
 * <p>吞掉的後果不是「少做一件事」而是「訂單無聲停在 {@code 35}」——而 {@code 35}
 * <b>沒有任何排程或人工端點</b>會把它推走。比照 03 §5.7.1 {@code refundOutcome} 的守門。
 *
 * <p>⚠️ <b>{@code financially_final} 這一欄看起來多餘，但它不是</b>：producer 的 routing key
 * 已經叫 {@code finalized} 了，理論上不會在未 final 時發。保留這個欄位並<b>斷言它</b>，
 * 是為了讓契約測試驗得到「不 final 就不動訂單」這個保證，而不是靠
 * 「這個 routing key 應該只在 final 時發」的紀律（36 §3.3.1 逐字）。
 *
 * <h2>冪等（L3）</h2>
 *
 * CAS 本身即冪等（{@code WHERE status='35'}）。🔴 但 {@code affected = 0} <b>不得</b>因此
 * 跳過既有終態的驗證——分流在 {@link ResponsibilityFinalizedTxHandler.Outcome}：
 * {@code ALREADY_FINAL} ⇒ ACK；{@code INCONSISTENT} ⇒ <b>進 DLQ</b>（必須有人看到）。
 *
 * <p>🔴 <b>絕對不得二次建立 payout task</b>（36 §3.3.1 逐字）。
 */
@Component
public class ResponsibilityFinalizedConsumer {

    private static final ObsLogger log = ObsLogger.forClass(ResponsibilityFinalizedConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();

    /**
     * 四個 resolved 成因。
     *
     * <p>⚠️ <b>從 CodeMeta 推導而不是硬寫四個字串</b>：{@code ResponsibilityCause} 日後若新增值，
     * 這裡會自動納入，而「新成因要怎麼分流」就必須在本 switch 明確面對
     * （落進 {@code else} 分支 ＝ 不進 54、走既有退款鏈），而不是靜默進 DLQ。
     * 🔴 {@code RESPONSIBILITY_UNRESOLVED} <b>不在</b> {@code ResponsibilityCause} 值域內
     * （它是 {@code FulfillmentExceptionCategory} 的一格）⇒ 它出現在本事件時必然 fail-loud。
     */
    private static final Set<String> KNOWN_CAUSES = Set.of(
            ResponsibilityCause.BUYER_CAUSED.value(),
            ResponsibilityCause.SELLER_CAUSED.value(),
            ResponsibilityCause.PROVIDER_CAUSED.value(),
            ResponsibilityCause.PLATFORM_CAUSED.value());

    private final ResponsibilityFinalizedTxHandler txHandler;

    public ResponsibilityFinalizedConsumer(ResponsibilityFinalizedTxHandler txHandler) {
        this.txHandler = txHandler;
    }

    @RabbitListener(queues = MqConstants.QUEUE_ORDER_FULFILLMENT_RESPONSIBILITY_FINALIZED,
            containerFactory = "manualAckContainerFactory")
    public void onResponsibilityFinalized(org.springframework.amqp.core.Message message,
                                          Channel channel,
                                          @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag)
            throws IOException {
        String body = new String(message.getBody(), StandardCharsets.UTF_8);
        try {
            JsonNode payload = MAPPER.readTree(body);

            // 🔴 wire 是 snake_case（producer 的 record 帶 @JsonNaming(SnakeCaseStrategy)）。
            //    這裡逐欄用 snake 讀取；欄名與型別的錨在 payment-service 的
            //    FulfillmentResponsibilityFinalizedPayload.java，由契約測試逐欄對拍。
            String orderNo = payload.path("order_no").asText("");
            String cause = payload.path("responsibility_cause").asText("");
            JsonNode finalFlag = payload.path("financially_final");
            String caseNo = payload.path("case_no").asText("");
            String resolutionId = payload.path("resolution_id").asText("");

            if (orderNo.isBlank() || cause.isBlank()) {
                throw new IllegalArgumentException(
                        "N16 缺必要欄位：order_no='" + orderNo + "' responsibility_cause='" + cause
                                + "'。🔴 order_no 是 35→54 CAS 的錨，為空時那道 CAS 永遠 rows=0 而無訊號");
            }
            // 🔴 不得用 asBoolean() 的預設值：missing 節點的 asBoolean(false) 與明確的 false
            //    無法分辨，而「欄位不見了」代表 producer 契約壞了，應該現形。
            if (!finalFlag.isBoolean() || !finalFlag.booleanValue()) {
                throw new IllegalArgumentException(
                        "N16 的 financially_final 不是 true（收到 " + finalFlag + "）——"
                                + "判定還可能翻案時推進 54 並建撥款任務是不可回收的錯誤方向");
            }
            if (!KNOWN_CAUSES.contains(cause)) {
                throw new IllegalArgumentException(
                        "N16 帶了未知的 responsibility_cause='" + cause + "'（合法值 " + KNOWN_CAUSES
                                + "）。🔴 RESPONSIBILITY_UNRESOLVED 是案件 category 不是成因；"
                                + "SYSTEM_CAUSED 不存在，平台／系統原因一律歸 PLATFORM_CAUSED");
            }

            if (!ResponsibilityCause.BUYER_CAUSED.value().equals(cause)) {
                // 🔴 非買家可歸責 ⇒ refund_to_buyer = full，走既有全額退款 closure。
                //    本 handler 不推進任何終態、不建撥款任務、也不發起退款
                //    ——那條鏈由 AfterSales 主導（03 §5.7.1），在這裡再造一條就是第二套退款引擎。
                log.info("ResponsibilityFinalizedConsumer.nonBuyerCaused",
                        "責任判定為非買家可歸責，不推進訂單終態（走既有全額退款 closure）",
                        "orderNo", orderNo);
                channel.basicAck(deliveryTag, false);
                return;
            }

            ResponsibilityFinalizedTxHandler.Outcome outcome =
                    txHandler.applyBuyerCausedFinality(orderNo);

            if (outcome == ResponsibilityFinalizedTxHandler.Outcome.INCONSISTENT) {
                // 🔴 事件與既有終態不一致 ⇒ **不得靜默 ACK**（03 §5.7.3 逐字）。
                //    進 DLQ 讓它在 order-service 自己的 DLX 上被看見；不建第二筆撥款任務。
                throw new IllegalStateException(
                        "N16 與訂單既有終態不一致，已拒絕建立第二筆撥款任務 orderNo=" + orderNo
                                + " caseNo=" + caseNo + " resolutionId=" + resolutionId);
            }

            log.info("ResponsibilityFinalizedConsumer.done",
                    "責任判定已套用 outcome=" + outcome, "orderNo", orderNo);
            channel.basicAck(deliveryTag, false);
        } catch (Exception e) {
            // 🔴 nack 不 requeue ⇒ 走 DLX 進 order.fulfillment.responsibility_finalized.dlq。
            //    這條路徑上的每一種例外都代表「事件與我方事實不一致」，重投不會讓它變好，
            //    而靜默 ACK 會讓訂單無聲停在 35。
            log.error("ResponsibilityFinalizedConsumer.error",
                    "處理責任判定 final 事件失敗，進 DLQ", e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag,
                    "ResponsibilityFinalizedConsumer", e);
        }
    }
}
