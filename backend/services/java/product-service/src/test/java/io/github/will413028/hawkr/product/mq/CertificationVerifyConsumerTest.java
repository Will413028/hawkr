package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.utils.code.ProductCertificationGovVerifyState;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationStatus;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.repository.ProductCertificationApplicationRepository;
import io.github.will413028.hawkr.product.service.ProductCertificationApplicationService;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.mockito.junit.jupiter.MockitoSettings;
import org.mockito.quality.Strictness;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;
import java.util.Optional;

import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyBoolean;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.times;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * {@link CertificationVerifyConsumer} 的行為契約（19 §8.3 / §12，36 §3.6h.1，#2755）。
 *
 * <p>斷言集中在三件「零訊號失效」上：
 * <ol>
 *   <li><b>冪等</b>：重複訊息不得重複核准、不得重複發通知，也不得重複打 90 秒的政府 API。</li>
 *   <li><b>自動核准條件</b>：{@code VERIFIED} ＋ {@code requiresManualReview = false} 才核准；
 *       {@code UNAVAILABLE}（我方／上游故障）<b>不是</b>查驗結論，一律不得當成「查無」放行或駁回。</li>
 *   <li><b>失敗處置</b>：走 {@code MqErrorHandler}（retry → DLQ），不得靜默 ack 吞掉。</li>
 * </ol>
 */
@ExtendWith(MockitoExtension.class)
@MockitoSettings(strictness = Strictness.LENIENT)
class CertificationVerifyConsumerTest {

    private static final long APPLICATION_ID = 843900000000000001L;
    private static final long DELIVERY_TAG = 42L;
    private static final String QUEUE = "product.certification.verify";

    @Mock ProductCertificationApplicationRepository applicationRepository;
    @Mock ProductCertificationApplicationService applicationService;
    @Mock Channel channel;

    CertificationVerifyConsumer consumer;

    @BeforeEach
    void setUp() {
        consumer = new CertificationVerifyConsumer(applicationRepository, applicationService);
    }

    @Test
    @DisplayName("SUBMITTED ＋ 查驗有效 ＋ 免人工 → 回寫後自動核准並 ack")
    void verifiedAndAutoApprovable_approves() throws Exception {
        givenApplication(ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationGovVerifyState.PENDING.value(), false);
        when(applicationService.runBackgroundVerify(APPLICATION_ID)).thenReturn(
                application(ProductCertificationStatus.SUBMITTED.value(),
                        ProductCertificationGovVerifyState.VERIFIED.value(), false));
        when(applicationService.approveBySystem(APPLICATION_ID)).thenReturn(true);

        consumer.onCertificationVerifyRequested(message(payload(APPLICATION_ID)), channel, DELIVERY_TAG);

        verify(applicationService).runBackgroundVerify(APPLICATION_ID);
        verify(applicationService).approveBySystem(APPLICATION_ID);
        verify(channel).basicAck(DELIVERY_TAG, false);
    }

    /**
     * 🔴 冪等的守門員：第一則訊息已把申請核准掉，重送的第二則看到 {@code status != SUBMITTED}
     * ⇒ <b>不再打政府 API、不再核准、不再發第二則通知</b>。
     * 少了這一格，重送會多推一則「已核准」給賣家，而且沒有任何資料會壞掉 ⇒ 沒有 gate 抓得到。
     */
    @Test
    @DisplayName("重複訊息（申請已核准）→ 不查驗、不核准、直接 ack")
    void duplicateMessageAfterApproval_isNoOp() throws Exception {
        givenApplication(ProductCertificationStatus.APPROVED.value(),
                ProductCertificationGovVerifyState.VERIFIED.value(), false);

        consumer.onCertificationVerifyRequested(message(payload(APPLICATION_ID)), channel, DELIVERY_TAG);

        verify(applicationService, never()).runBackgroundVerify(anyLong());
        verify(applicationService, never()).approveBySystem(anyLong());
        verify(channel).basicAck(DELIVERY_TAG, false);
    }

    /** 同一則訊息投兩次：第二次時申請已離開 SUBMITTED ⇒ 全鏈路只核准一次、只通知一次。 */
    @Test
    @DisplayName("同一則訊息連投兩次 → 只核准一次")
    void sameMessageTwice_approvesOnce() throws Exception {
        ProductCertificationApplicationEntity submitted = application(
                ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationGovVerifyState.PENDING.value(), false);
        ProductCertificationApplicationEntity approved = application(
                ProductCertificationStatus.APPROVED.value(),
                ProductCertificationGovVerifyState.VERIFIED.value(), false);
        when(applicationRepository.findById(APPLICATION_ID))
                .thenReturn(Optional.of(submitted))
                .thenReturn(Optional.of(approved));
        when(applicationService.runBackgroundVerify(APPLICATION_ID)).thenReturn(
                application(ProductCertificationStatus.SUBMITTED.value(),
                        ProductCertificationGovVerifyState.VERIFIED.value(), false));
        when(applicationService.approveBySystem(APPLICATION_ID)).thenReturn(true);

        Message msg = message(payload(APPLICATION_ID));
        consumer.onCertificationVerifyRequested(msg, channel, DELIVERY_TAG);
        consumer.onCertificationVerifyRequested(msg, channel, DELIVERY_TAG);

        verify(applicationService, times(1)).approveBySystem(APPLICATION_ID);
        verify(applicationService, times(1)).runBackgroundVerify(APPLICATION_ID);
        verify(channel, times(2)).basicAck(DELIVERY_TAG, false);
    }

    /**
     * §6.3 Step 3：曾被駁回／撤銷的號碼重送 ⇒ 查驗照跑（ADM 重審需要政府資料），
     * 但<b>永不自動核准</b>——不得讓查驗剛好通過就繞過 ADM 的裁決。
     */
    @Test
    @DisplayName("requiresManualReview=true ＋ 查驗有效 → 只回寫，不自動核准")
    void manualReviewRequired_doesNotApprove() throws Exception {
        givenApplication(ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationGovVerifyState.PENDING.value(), true);
        when(applicationService.runBackgroundVerify(APPLICATION_ID)).thenReturn(
                application(ProductCertificationStatus.SUBMITTED.value(),
                        ProductCertificationGovVerifyState.VERIFIED.value(), true));

        consumer.onCertificationVerifyRequested(message(payload(APPLICATION_ID)), channel, DELIVERY_TAG);

        verify(applicationService).runBackgroundVerify(APPLICATION_ID);
        verify(applicationService, never()).approveBySystem(anyLong());
        verify(channel).basicAck(DELIVERY_TAG, false);
    }

    /** {@code UNAVAILABLE} 是我方／上游故障，不是查驗結論——當成「查無」會讓全站認證靜默變假。 */
    @Test
    @DisplayName("查驗結果 UNAVAILABLE → 不自動核准")
    void govUnavailable_doesNotApprove() throws Exception {
        givenApplication(ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationGovVerifyState.PENDING.value(), false);
        when(applicationService.runBackgroundVerify(APPLICATION_ID)).thenReturn(
                application(ProductCertificationStatus.SUBMITTED.value(),
                        ProductCertificationGovVerifyState.UNAVAILABLE.value(), false));

        consumer.onCertificationVerifyRequested(message(payload(APPLICATION_ID)), channel, DELIVERY_TAG);

        verify(applicationService, never()).approveBySystem(anyLong());
        verify(channel).basicAck(DELIVERY_TAG, false);
    }

    @Test
    @DisplayName("查驗結果 NOT_FOUND → 不自動核准（留給 ADM 裁決）")
    void govNotFound_doesNotApprove() throws Exception {
        givenApplication(ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationGovVerifyState.PENDING.value(), false);
        when(applicationService.runBackgroundVerify(APPLICATION_ID)).thenReturn(
                application(ProductCertificationStatus.SUBMITTED.value(),
                        ProductCertificationGovVerifyState.NOT_FOUND.value(), false));

        consumer.onCertificationVerifyRequested(message(payload(APPLICATION_ID)), channel, DELIVERY_TAG);

        verify(applicationService, never()).approveBySystem(anyLong());
        verify(channel).basicAck(DELIVERY_TAG, false);
    }

    @Test
    @DisplayName("申請不存在 → 丟棄（ack），不查驗")
    void applicationMissing_acksAndSkips() throws Exception {
        when(applicationRepository.findById(APPLICATION_ID)).thenReturn(Optional.empty());

        consumer.onCertificationVerifyRequested(message(payload(APPLICATION_ID)), channel, DELIVERY_TAG);

        verify(applicationService, never()).runBackgroundVerify(anyLong());
        verify(channel).basicAck(DELIVERY_TAG, false);
    }

    /** 缺 applicationId 的訊息重試幾次也不會變好 ⇒ 直接丟棄，不佔 DLQ。 */
    @Test
    @DisplayName("payload 缺 application_id → 丟棄（ack），不進 retry")
    void missingApplicationId_acksAndSkips() throws Exception {
        consumer.onCertificationVerifyRequested(message("{\"event_type\":\"X\"}"), channel, DELIVERY_TAG);

        verify(applicationService, never()).runBackgroundVerify(anyLong());
        verify(channel).basicAck(DELIVERY_TAG, false);
        verify(channel, never()).basicPublish(any(), any(), any(), any());
    }

    /**
     * 政府 API 拋例外 ⇒ 交給 {@code MqErrorHandler}：retry 次數未達上限時
     * 「ack 原訊息 ＋ 帶 {@code x-retry-count} 重投同一 queue」。
     * 🔴 這一格擋的是「catch 完就靜默 ack」——那會讓查驗失敗的申請永遠停在 PENDING 而無人知曉。
     */
    @Test
    @DisplayName("查驗拋例外 → 走 MqErrorHandler 重投，不靜默吞掉")
    void verifyThrows_goesToRetry() throws Exception {
        givenApplication(ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationGovVerifyState.PENDING.value(), false);
        when(applicationService.runBackgroundVerify(APPLICATION_ID))
                .thenThrow(new IllegalStateException("gov api down"));

        consumer.onCertificationVerifyRequested(message(payload(APPLICATION_ID)), channel, DELIVERY_TAG);

        verify(channel).basicAck(DELIVERY_TAG, false);
        verify(channel).basicPublish(eq(""), eq(QUEUE), any(), any());
    }

    /** 已達重試上限 ⇒ 直接 reject 進 DLQ，不再重投（訊息不會無限打轉）。 */
    @Test
    @DisplayName("重試已達上限時查驗再失敗 → reject 進 DLQ")
    void verifyThrowsAtMaxRetry_goesToDlq() throws Exception {
        givenApplication(ProductCertificationStatus.SUBMITTED.value(),
                ProductCertificationGovVerifyState.PENDING.value(), false);
        when(applicationService.runBackgroundVerify(APPLICATION_ID))
                .thenThrow(new IllegalStateException("gov api down"));

        Message msg = message(payload(APPLICATION_ID));
        msg.getMessageProperties().setHeader("x-retry-count", 3);

        consumer.onCertificationVerifyRequested(msg, channel, DELIVERY_TAG);

        verify(channel).basicReject(DELIVERY_TAG, false);
        verify(channel, never()).basicPublish(any(), any(), any(), any());
        verify(channel, never()).basicAck(anyLong(), anyBoolean());
    }

    // ==================== helper ====================

    private void givenApplication(String status, String govState, boolean manualReview) {
        when(applicationRepository.findById(APPLICATION_ID))
                .thenReturn(Optional.of(application(status, govState, manualReview)));
    }

    private ProductCertificationApplicationEntity application(String status, String govState,
                                                              boolean manualReview) {
        ProductCertificationApplicationEntity app = new ProductCertificationApplicationEntity();
        app.setId(APPLICATION_ID);
        app.setProductId(843856384063471755L);
        app.setStoreId(2685914682L);
        app.setCertificationType(3701);
        app.setCertificateNo("1-009-300022");
        app.setStatus(status);
        app.setGovVerifyState(govState);
        app.setRequiresManualReview(manualReview);
        app.setVersion(0L);
        return app;
    }

    /** wire 是 snake_case（publisher 端帶 {@code @JsonNaming(SnakeCaseStrategy)}）。 */
    private String payload(long applicationId) {
        return "{\"event_id\":\"3f2b1c00-0000-4000-8000-000000000000\","
                + "\"event_type\":\"PRODUCT_CERTIFICATION_VERIFY_REQUESTED\","
                + "\"application_id\":\"" + applicationId + "\","
                + "\"product_id\":\"843856384063471755\",\"store_id\":\"2685914682\","
                + "\"certification_type\":\"3701\",\"certificate_no\":\"1-009-300022\","
                + "\"requires_manual_review\":false,\"occurred_at\":\"2026-08-26T00:00:00Z\"}";
    }

    private Message message(String body) {
        MessageProperties props = new MessageProperties();
        props.setConsumerQueue(QUEUE);
        props.setContentType(MessageProperties.CONTENT_TYPE_JSON);
        return new Message(body.getBytes(StandardCharsets.UTF_8), props);
    }
}
