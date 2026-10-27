package io.github.will413028.hawkr.product.mq;

import io.github.will413028.hawkr.common.mq.MqConstants;
import io.github.will413028.hawkr.common.mq.MqErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationGovVerifyState;
import io.github.will413028.hawkr.common.utils.code.ProductCertificationStatus;
import io.github.will413028.hawkr.product.entity.ProductCertificationApplicationEntity;
import io.github.will413028.hawkr.product.repository.ProductCertificationApplicationRepository;
import io.github.will413028.hawkr.product.service.ProductCertificationApplicationService;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.amqp.support.AmqpHeaders;
import org.springframework.messaging.handler.annotation.Header;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.Optional;

/**
 * 消費 {@code product.certification.verify.requested}，打政府開放資料 API 做<b>背景查驗補課</b>
 * （19 號設計書《農業認證驗證》§8.3 / §12，36 §3.6h.1，#2755）。
 *
 * <p><b>它存在的理由</b>：同步階段（商品建立／更新／匯入 commit／賣家單筆申請）一律
 * <b>只讀結構化快取、零外部呼叫</b>（§8.2）——快取未命中的申請就停在
 * {@code gov_verify_state = PENDING}。沒有這支 consumer，那些申請只能靠 ADM 在詳情頁
 * 手動按「重新查驗」補課，而 §15.3 migration 建出的 legacy 列與所有政府 API 逾時的案件全在這一類。
 *
 * <p><b>timeout 與斷路器</b>：走 {@code GovCertificationService.verifyDetailedForBackground}
 * （read-timeout 90 秒、<b>既不讀也不寫</b> circuit breaker key）。背景路徑本來就預期慢，
 * 不該被同步路徑的判斷短路；反向也成立——背景的慢不該回頭把同步 circuit 打開（§8.3）。
 *
 * <p><b>冪等</b>（36 §3.6h.1）：
 * <ol>
 *   <li>第一道是<b>狀態閘門</b>：申請只要離開 {@code SUBMITTED}（已核准／駁回／撤回／被取代），
 *       重送的訊息直接 ack 丟棄——不再打政府 API、不再核准、<b>不再發第二則通知</b>。</li>
 *   <li>第二道是 {@code version} CAS：回寫查驗結果與核准都帶「讀取當下的 version」，
 *       {@code rowsAffected = 0} ⇒ 期間已被人工裁決 ⇒ 放棄不重試（人工裁決優先）。</li>
 * </ol>
 * 兩道都不需要 dedup 表。
 *
 * <p>🔴 <b>不得</b>為了救「事件遺失／consumer 持續失敗」而新增掃 {@code gov_verify_state='PENDING'}
 * 的排程 job：那會同時踩《後端服務責任邊界規範》（禁 reconciliation job）與《多副本排程搶佔規範》。
 * 可靠性靠本 queue 的 retry／DLQ，人工補救靠 ADM 的「重新查驗」端點（§9.3）。
 */
@Component
public class CertificationVerifyConsumer {

    private static final ObsLogger log = ObsLogger.forClass(CertificationVerifyConsumer.class);
    private static final ObjectMapper MAPPER = new ObjectMapper();
    private static final String CONSUMER_NAME = "CertificationVerifyConsumer";

    private final ProductCertificationApplicationRepository applicationRepository;
    private final ProductCertificationApplicationService applicationService;

    public CertificationVerifyConsumer(ProductCertificationApplicationRepository applicationRepository,
                                       ProductCertificationApplicationService applicationService) {
        this.applicationRepository = applicationRepository;
        this.applicationService = applicationService;
    }

    @RabbitListener(queues = MqConstants.QUEUE_PRODUCT_CERTIFICATION_VERIFY,
                    containerFactory = "manualAckContainerFactory")
    public void onCertificationVerifyRequested(Message message, Channel channel,
                                               @Header(AmqpHeaders.DELIVERY_TAG) long deliveryTag)
            throws IOException {
        String applicationIdRaw = "unknown";
        try {
            JsonNode root = MAPPER.readTree(new String(message.getBody(), StandardCharsets.UTF_8));
            applicationIdRaw = root.path("application_id").asText("");
            if (applicationIdRaw.isEmpty()) {
                // 沒有 applicationId 的訊息重試幾次也不會變好 ⇒ 直接丟棄，不佔 DLQ
                log.warn("CertificationVerifyConsumer.invalid", "payload 缺 application_id，丟棄訊息");
                channel.basicAck(deliveryTag, false);
                return;
            }
            long applicationId = Long.parseLong(applicationIdRaw);

            Optional<ProductCertificationApplicationEntity> found =
                    applicationRepository.findById(applicationId);
            if (found.isEmpty()) {
                log.warn("CertificationVerifyConsumer.notFound", "認證申請不存在，丟棄訊息",
                        "application_id", applicationIdRaw);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 冪等第一道：離開 SUBMITTED 的申請一律 no-op（重複訊息／期間已被人工裁決）。
            // 這一格就是「重複訊息不得重複核准、不得重複發通知」的守門員。
            if (!ProductCertificationStatus.SUBMITTED.value().equals(found.get().getStatus())) {
                log.info("CertificationVerifyConsumer.skipNotSubmitted",
                        "申請已離開 SUBMITTED，略過背景查驗 status=" + found.get().getStatus(),
                        "application_id", applicationIdRaw);
                channel.basicAck(deliveryTag, false);
                return;
            }

            // 查驗 ＋ 以「讀取當下的 version」CAS 回寫三個查驗欄位（不改 status）
            ProductCertificationApplicationEntity verified =
                    applicationService.runBackgroundVerify(applicationId);

            boolean approved = false;
            if (isAutoApproveCandidate(verified)) {
                // 資格的最終判定在 approveBySystem 內（持 FOR UPDATE 重讀，另含
                // 「該 type 是否允許自動核准」）；這裡只是省下一次無謂的鎖。
                // approveBySystem 成功時會發 product.certification.approved 通知賣家。
                approved = applicationService.approveBySystem(applicationId);
            }

            log.info("CertificationVerifyConsumer.onCertificationVerifyRequested.success",
                    "背景查驗完成 govVerifyState=" + verified.getGovVerifyState()
                            + " autoApproved=" + approved,
                    "application_id", applicationIdRaw);
            channel.basicAck(deliveryTag, false);

        } catch (Exception e) {
            log.error("CertificationVerifyConsumer.onCertificationVerifyRequested.error",
                    "背景查驗失敗，application_id=" + applicationIdRaw, e);
            MqErrorHandler.handleFailure(message, channel, deliveryTag, CONSUMER_NAME, e);
        }
    }

    /**
     * 自動核准的快篩：查驗結論為<b>有效</b> ＋ {@code requires_manual_review = false}。
     *
     * <p>⚠️ {@code UNAVAILABLE}（我方／上游故障）<b>不是</b>查驗結論，必須讓例外或下一次重送去補
     * ——把故障當成「查無」會讓全站認證靜默變成假的（§8.4 防線 1）。這裡只認 {@code VERIFIED}。
     */
    private boolean isAutoApproveCandidate(ProductCertificationApplicationEntity app) {
        return ProductCertificationGovVerifyState.VERIFIED.value().equals(app.getGovVerifyState())
                && !Boolean.TRUE.equals(app.getRequiresManualReview());
    }
}
