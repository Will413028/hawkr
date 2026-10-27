package io.github.will413028.hawkr.payment.logistics.mq;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.Mockito.atLeastOnce;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

import io.github.will413028.hawkr.payment.logistics.application.LogisticsCostFactJoinService;
import io.github.will413028.hawkr.payment.logistics.repository.LogisticsCostFactRepository;
import com.rabbitmq.client.Channel;
import java.nio.charset.StandardCharsets;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

/**
 * 🔴 <b>損壞的 financial finality 在 consumer 層的處置</b>。
 *
 * <h2>本檔要證明的一件事</h2>
 * 「{@code converge} 回 {@code CONFLICT}」只是<b>半個</b>保證——真正要成立的是
 * <b>那則訊息會被重新投遞而不是當成處理完成</b>。service 單元測試看不到這一層：
 * 它只斷言回傳值，而回傳值對了、consumer 照樣 {@code basicAck} 的話，
 * 這筆損壞仍然無聲消失。
 *
 * <h2>被修掉的缺陷（本檔的存在理由）</h2>
 * 「案件已 {@code RESOLVED} 但 {@code resolution_code} 非法」曾經回
 * {@code AWAITING_RESPONSIBILITY} ⇒ consumer 走<b>成功路徑</b>（ack 且不重發）。
 * 那是一個<b>偽裝成 fail-closed 的 fail-open</b>：註解寫著 fail-closed、
 * 也有 {@code log.error}，但 ACK 之後沒有任何東西會回來看那一行 log。
 *
 * <h2>⚠️ 「有沒有 ack」在這裡零鑑別力</h2>
 * {@code MqErrorHandler} 在 {@code retryCount < 3} 時<b>也會 basicAck</b>
 * （然後重新發布並遞增 {@code x-retry-count}——因為 {@code basicNack(requeue=true)}
 * 搭配 {@code x-death} 計數不生效）。⇒ 成功與失敗路徑<b>都會 ack</b>。
 * 🔴 分得出兩者的是 <b>有沒有重新投遞</b>。
 *
 * <h2>⚠️ 兩個 consumer 都要驗</h2>
 * {@code converge} 有<b>兩個</b> production 呼叫端（成本側與責任側）。
 * 只驗一個，另一個把 {@code CONFLICT} 當正常 no-op 時不會有任何東西出聲。
 */
@DisplayName("損壞的 financial finality：consumer 不得正常 ACK")
class CorruptFinalityConsumerBehaviourTest {

    private static final long TAG = 42L;

    /**
     * ⚠️ consumer 會呼叫 {@code TsidUtil.generate()} 建成本事實的 id。
     *
     * <p>🔴 <b>沒有這一段時，失敗會偽裝成斷言失敗</b>：實際拋的是
     * 「TsidUtil 尚未初始化」，被 {@code MqErrorHandler} 當成消費失敗吃掉 ⇒ 走重投路徑
     * ⇒ 對照組的「不得重新投遞」轉紅——而真正的原因與被測邏輯<b>毫無關係</b>。
     *
     * <p>⚠️ 用 try/ignore 而不是 {@code isInitialized()} 判斷，是沿用本模組既有慣例：
     * surefire {@code reuseForks} 下，同一個 JVM 可能已被別的測試初始化過。
     */
    @org.junit.jupiter.api.BeforeAll
    static void initTsid() {
        try {
            io.github.will413028.hawkr.common.utils.TsidUtil.init(901, 0);
        } catch (Exception ignored) {
            // 已初始化則忽略
        }
    }

    private static Message message(String body) {
        MessageProperties props = new MessageProperties();
        props.setConsumerQueue("payment.logistics.cost_incurred");
        return new Message(body.getBytes(StandardCharsets.UTF_8), props);
    }

    @Nested
    @DisplayName("成本側（logistics.cost.incurred）")
    class CostSide {

        private static final String VALID_BODY = """
                {"event_id":"evt-1","event_type":"LOGISTICS_COST_INCURRED","event_version":"v1",
                 "occurred_at":"2026-09-17T12:00:00Z",
                 "provider":"3003","provider_delivery_ref":"del_corrupt",
                 "store_id":"2685914682","order_no":"O-1","shipment_id":null,
                 "estimated_cost":"115"}
                """;

        @Test
        @DisplayName("🔴 CONFLICT ⇒ 不得 basicAck（走 MqErrorHandler ⇒ retry×3 → DLQ）")
        void conflictIsNotAcked() throws Exception {
            LogisticsCostFactJoinService join = mock(LogisticsCostFactJoinService.class);
            LogisticsCostFactRepository facts = mock(LogisticsCostFactRepository.class);
            when(facts.insertIfAbsent(org.mockito.ArgumentMatchers.any())).thenReturn(true);
            when(join.converge("del_corrupt"))
                    .thenReturn(LogisticsCostFactJoinService.Outcome.CONFLICT);

            Channel channel = mock(Channel.class);
            new LogisticsCostIncurredConsumer(facts, join)
                    .onCostIncurred(message(VALID_BODY), channel, TAG);

            // 🔴 核心斷言：訊息**被重新投遞**了 ⇒ 它沒有走成功路徑。
            //
            // ⚠️ **不能用「沒有 basicAck」當斷言**——我一開始就是這樣寫，而它是錯的：
            //    MqErrorHandler 在 retryCount < 3 時**確實會 basicAck 原訊息**，
            //    然後把它重新發布並遞增 x-retry-count（那是它刻意的設計，
            //    因為 basicNack(requeue=true) 搭配 x-death 計數不生效）。
            //    ⇒ 「有沒有 ack」在這裡**零鑑別力**，成功與失敗路徑都會 ack。
            //
            // 🔴 分得出兩者的是**有沒有重新投遞**：
            //    成功 ⇒ basicAck 且不重發；失敗 ⇒ basicAck ＋ basicPublish（或 basicReject）。
            verify(channel, atLeastOnce())
                    .basicPublish(anyString(), anyString(), any(), any());
        }

        @Test
        @DisplayName("對照：正常 outcome ⇒ basicAck（證明上一支不是「什麼都不 ack」）")
        void normalOutcomeIsAcked() throws Exception {
            LogisticsCostFactJoinService join = mock(LogisticsCostFactJoinService.class);
            LogisticsCostFactRepository facts = mock(LogisticsCostFactRepository.class);
            when(facts.insertIfAbsent(org.mockito.ArgumentMatchers.any())).thenReturn(true);
            when(join.converge("del_corrupt"))
                    .thenReturn(LogisticsCostFactJoinService.Outcome.AWAITING_RESPONSIBILITY);

            Channel channel = mock(Channel.class);
            new LogisticsCostIncurredConsumer(facts, join)
                    .onCostIncurred(message(VALID_BODY), channel, TAG);

            // 🔴 少了這一支，把 consumer 寫成「一律走失敗路徑」也會讓上一支通過，
            //    而那會讓每一則正常訊息都被反覆重投到 DLQ。
            verify(channel).basicAck(TAG, false);
            verify(channel, never()).basicPublish(anyString(), anyString(), any(), any());
        }
    }

    @Nested
    @DisplayName("責任側（N16 fulfillment.responsibility.finalized）")
    class ResponsibilitySide {

        private static final String VALID_BODY = """
                {"event_id":"evt-2","responsibility_cause":"BUYER_CAUSED",
                 "financially_final":true,"provider_delivery_ref":"del_corrupt",
                 "case_no":"C-1","order_no":"O-1"}
                """;

        @Test
        @DisplayName("🔴 CONFLICT ⇒ 不得 basicAck")
        void conflictIsNotAcked() throws Exception {
            LogisticsCostFactJoinService join = mock(LogisticsCostFactJoinService.class);
            when(join.converge("del_corrupt"))
                    .thenReturn(LogisticsCostFactJoinService.Outcome.CONFLICT);

            Channel channel = mock(Channel.class);
            new ResponsibilityFinalizedConsumer(join)
                    .onResponsibilityFinalized(message(VALID_BODY), channel, TAG);

            // 見成本側同名測試的說明：分得出成功／失敗的是「有沒有重新投遞」。
            verify(channel, atLeastOnce())
                    .basicPublish(anyString(), anyString(), any(), any());
        }

        @Test
        @DisplayName("對照：正常 outcome ⇒ basicAck 且**不**重新投遞")
        void normalOutcomeIsAcked() throws Exception {
            LogisticsCostFactJoinService join = mock(LogisticsCostFactJoinService.class);
            when(join.converge("del_corrupt"))
                    .thenReturn(LogisticsCostFactJoinService.Outcome.AWAITING_COST_FACT);

            Channel channel = mock(Channel.class);
            new ResponsibilityFinalizedConsumer(join)
                    .onResponsibilityFinalized(message(VALID_BODY), channel, TAG);

            verify(channel).basicAck(TAG, false);
            verify(channel, never()).basicPublish(anyString(), anyString(), any(), any());
        }
    }

    // ⚠️ 「CONFLICT 之後會發生什麼」由 common-core 的 MqErrorHandler 自己的測試守，
    //    本檔不重複驗。為了記錄而記錄：實作是
    //      retryCount < 3  ⇒ recordOutcome(queue,"retry")、ACK 原訊息並重發（遞增 x-retry-count）
    //      retryCount >= 3 ⇒ recordOutcome(queue,"dlq")、basicReject(requeue=false) ⇒ 進 DLX
    //    兩條路徑都有 metric ⇒ 損壞的 finality 在 Grafana 上看得見，
    //    而不是只留一行沒有人會回來看的 log。
}
