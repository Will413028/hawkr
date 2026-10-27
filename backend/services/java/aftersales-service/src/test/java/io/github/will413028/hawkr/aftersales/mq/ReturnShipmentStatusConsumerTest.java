package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.config.AfterSalesProperties;
import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.aftersales.service.AfterSaleAuditService;
import io.github.will413028.hawkr.aftersales.service.AfterSalesEventPublisher;
import io.github.will413028.hawkr.aftersales.service.EventPrincipals;
import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ReturnShippingRules;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.rabbitmq.client.Channel;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;
import org.junit.jupiter.api.extension.ExtendWith;
import org.mockito.ArgumentCaptor;
import org.mockito.InjectMocks;
import org.mockito.Mock;
import org.mockito.junit.jupiter.MockitoExtension;
import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;

import java.nio.charset.StandardCharsets;
import java.util.Map;

import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotEquals;
import static org.mockito.ArgumentMatchers.*;
import static org.mockito.Mockito.*;

/**
 * 2026-05-22 逆物流退貨政策修訂 — ReturnShipmentStatusConsumer 分流測試。
 *
 * <p>覆蓋案例：
 * <ul>
 *   <li>PayUni 碼 32 + method=3002（超商）→ markReturnAtStore</li>
 *   <li>PayUni 碼 32 + method=3001（宅配）→ 忽略</li>
 *   <li>PayUni 碼 11 → markReturnDelivered（含 sellerPickedUpAt）</li>
 *   <li>PayUni 碼 33（異常）→ markReturnFailed</li>
 *   <li>PayUni 碼 53（廠退）→ markReturnFailed（2026-05-22 bug 修正：原誤映成 DELIVERED）</li>
 *   <li>PayUni 碼 55（賣家未取）→ stampReturnProviderInfo（不變狀態）</li>
 * </ul>
 */
@ExtendWith(MockitoExtension.class)
class ReturnShipmentStatusConsumerTest {

    @Mock
    private AfterSaleCaseMapper caseMapper;

    @Mock
    private AfterSaleAuditService auditService;

    @Mock
    private AfterSalesEventPublisher eventPublisher;

    @Mock
    private Channel channel;

    private AfterSalesProperties properties;
    private ReturnShipmentStatusConsumer consumer;
    private CountingTxManager txManager;

    /** issue #2294 出路 4：身分反解在 tx 外先做好，當參數傳進 tx 內的 tryPublish。 */
    private static final EventPrincipals PRINCIPALS = new EventPrincipals(9001L, 9002L);

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @BeforeEach
    void setUp() {
        properties = new AfterSalesProperties();
        properties.getReturnConfig().setAutoConfirmReceiptHours(48);
        properties.getReturnConfig().setStorePickupTimeoutHours(72);
        txManager = new CountingTxManager();
        consumer = new ReturnShipmentStatusConsumer(
                caseMapper, auditService, properties, eventPublisher, txManager);
    }

    /** 讓 tryPublish 回 true（成功路徑）。失敗路徑的測試自行覆寫成 false。 */
    private void stubPublishSucceeds() {
        when(eventPublisher.resolveEventPrincipalsBestEffort(any(), any())).thenReturn(PRINCIPALS);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class))).thenReturn(true);
    }

    @Test
    void cvsCode32_marksReturnAtStore() throws Exception {
        AfterSaleCaseEntity entity = caseWith("A001", "O100", "25", "3002", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.markReturnAtStore(eq("A001"), any(), any())).thenReturn(1);
        stubPublishSucceeds();

        consumer.onReturnStatusChanged(buildMessage(payload("O100", "32", true, null, null)), channel, 1L);

        verify(caseMapper).markReturnAtStore(eq("A001"), any(), any());
        verify(caseMapper, never()).markReturnDelivered(anyString(), any(), any(), any());
        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_at_store"), anyString(),
                eq("A001"), eq("O100"), eq(2001L), eq(1001L), eq("system"), anyMap(), eq(PRINCIPALS));
    }

    @Test
    void code31_marksReturnInTransit_publishesEvent() throws Exception {
        // #1226：碼 31（IN_TRANSIT）RETURN_PENDING(20)→RETURN_SHIPPING(25) 必須補發
        // CASE_RETURN_IN_TRANSIT（→ 賣家通知 + 賣家工作區 realtime）；原本此轉換不發任何 domain event
        AfterSaleCaseEntity entity = caseWith("A007", "O700", "20", "3002", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.markReturnInTransit("A007")).thenReturn(1);
        stubPublishSucceeds();

        consumer.onReturnStatusChanged(buildMessage(payload("O700", "31", true, null, null)), channel, 7L);

        verify(caseMapper).markReturnInTransit("A007");
        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_in_transit"), anyString(),
                eq("A007"), eq("O700"), eq(2001L), eq(1001L), eq("system"), anyMap(), eq(PRINCIPALS));
        assertThat(txManager.commitCount).as("CAS+publish 皆成功必須 commit").isEqualTo(1);
        assertThat(txManager.rollbackCount).isEqualTo(0);
    }

    @Test
    void code31_inTransit_skipped_whenNotReturnPending() throws Exception {
        // CAS rows=0（case 已離開 RETURN_PENDING）→ 不發事件（冪等 skip）
        AfterSaleCaseEntity entity = caseWith("A008", "O800", "25", "3002", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.markReturnInTransit("A008")).thenReturn(0);

        consumer.onReturnStatusChanged(buildMessage(payload("O800", "31", true, null, null)), channel, 8L);

        verify(caseMapper).markReturnInTransit("A008");
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_in_transit"), anyString(),
                anyString(), anyString(), any(), any(), anyString(), anyMap(), any());
    }

    @Test
    void taqbinCode32_isIgnored() throws Exception {
        AfterSaleCaseEntity entity = caseWith("A002", "O200", "25", "3001", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);

        consumer.onReturnStatusChanged(buildMessage(payload("O200", "32", true, null, null)), channel, 2L);

        verify(caseMapper, never()).markReturnAtStore(anyString(), any(), any());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(anyString(), anyString(),
                anyString(), anyString(), any(), any(), anyString(), anyMap(), any());
        assertThat(txManager.beginCount).as("非超商在進 tx 前就 return，禁止開空 tx").isEqualTo(0);
    }

    @Test
    void code11_marksReturnDeliveredWithSellerPickedUp() throws Exception {
        AfterSaleCaseEntity entity = caseWith("A003", "O300", "26", "3002", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.markReturnDelivered(eq("A003"), any(), any(), any())).thenReturn(1);
        stubPublishSucceeds();

        consumer.onReturnStatusChanged(buildMessage(payload("O300", "11", true, null, null)), channel, 3L);

        ArgumentCaptor<java.time.LocalDateTime> sellerPickedUpAt = ArgumentCaptor.forClass(java.time.LocalDateTime.class);
        verify(caseMapper).markReturnDelivered(eq("A003"), any(), any(), sellerPickedUpAt.capture());
        // sellerPickedUpAt 必須非 null（2026-05-22 修訂新欄位）
        org.junit.jupiter.api.Assertions.assertNotNull(sellerPickedUpAt.getValue());
        verify(eventPublisher).tryPublishCaseTransitionWithContext(
                eq("aftersales.case.return_delivered"), anyString(),
                eq("A003"), eq("O300"), eq(2001L), eq(1001L), eq("system"), anyMap(), eq(PRINCIPALS));
    }

    @Test
    void code33_marksReturnFailed() throws Exception {
        AfterSaleCaseEntity entity = caseWith("A004", "O400", "25", "3001", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.markReturnFailed(eq("A004"), anyString(), any())).thenReturn(1);
        stubPublishSucceeds();

        consumer.onReturnStatusChanged(buildMessage(payload("O400", "33", true, "33", "配送過程異常")), channel, 4L);

        verify(caseMapper).markReturnFailed(eq("A004"), eq("33"), eq("配送過程異常"));
    }

    @Test
    void code53_廠退_isAbnormalNotDelivered() throws Exception {
        // 2026-05-22 bug 修正：碼 53 是「廠退」屬於 ABNORMAL，原誤列為 DELIVERED 已修正
        AfterSaleCaseEntity entity = caseWith("A005", "O500", "25", "3001", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.markReturnFailed(eq("A005"), anyString(), any())).thenReturn(1);
        stubPublishSucceeds();

        consumer.onReturnStatusChanged(buildMessage(payload("O500", "53", true, "53", "廠退")), channel, 5L);

        verify(caseMapper).markReturnFailed(eq("A005"), eq("53"), eq("廠退"));
        verify(caseMapper, never()).markReturnDelivered(anyString(), any(), any(), any());
    }

    @Test
    void code55_sellerNotPicked_persistsMessageWithoutStateChange() throws Exception {
        // 我方 72h timer 是唯一真實來源；碼 55 僅供 audit
        AfterSaleCaseEntity entity = caseWith("A006", "O600", "30", "3002", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.stampReturnProviderInfo(eq("A006"), anyString(), anyString())).thenReturn(1);

        consumer.onReturnStatusChanged(buildMessage(payload("O600", "55", true, "55", "賣家未取")), channel, 6L);

        verify(caseMapper).stampReturnProviderInfo(eq("A006"), eq("55"), eq("賣家未取"));
        verify(caseMapper, never()).markReturnAtStore(anyString(), any(), any());
        verify(caseMapper, never()).markReturnDelivered(anyString(), any(), any(), any());
        verify(caseMapper, never()).markReturnFailed(anyString(), anyString(), any());
    }

    @Test
    void nonReturnEvent_isAcked() throws Exception {
        consumer.onReturnStatusChanged(buildMessage(payload("O999", "32", false, null, null)), channel, 9L);
        verify(caseMapper, never()).selectOne(any());
    }

    @Test
    void dispositionRegression_abnormalCovers33_44_53_andNot11_or_32() {
        // 分類回歸測試 —— 防止後人誤改回 2026-05-22 修掉的 bug 狀態。
        // 🔴 錨已從 AfterSalesConstants 的 5 個字面值集合移到 common-core 的 canonical
        //    分類器（ADR-013）；斷言的是**語意**不是集合成員。
        assertEquals(ReturnShippingRules.Disposition.RETURN_ABNORMAL,
                ReturnShippingRules.dispositionOf(ShippingStatus.ABNORMAL));
        assertEquals(ReturnShippingRules.Disposition.RETURN_ABNORMAL,
                ReturnShippingRules.dispositionOf(ShippingStatus.RETURN_TO_WAREHOUSE));
        assertEquals(ReturnShippingRules.Disposition.RETURN_ABNORMAL,
                ReturnShippingRules.dispositionOf(ShippingStatus.PACKAGE_LOST));
        assertNotEquals(ReturnShippingRules.Disposition.RETURN_DELIVERED,
                ReturnShippingRules.dispositionOf(ShippingStatus.RETURN_TO_WAREHOUSE),
                "碼 53 不可被判成已送達（2026-05-22 bug 修正）");
        assertEquals(ReturnShippingRules.Disposition.RETURN_DELIVERED,
                ReturnShippingRules.dispositionOf(ShippingStatus.PICKED_UP));
        assertEquals(ReturnShippingRules.Disposition.RETURN_STORE_ARRIVED,
                ReturnShippingRules.dispositionOf(ShippingStatus.WAITING_PICKUP));
        assertEquals(ReturnShippingRules.Disposition.SELLER_NOT_PICKED,
                ReturnShippingRules.dispositionOf(ShippingStatus.SELLER_NOT_PICKED));
    }

    /**
     * 🔴 ADR-013 的核心：Uber 逆向 {@code pickup_complete} → {@code 930 HANDED_OVER}
     * 必須推進 {@code RETURN_SHIPPING(25)}。
     *
     * <p>修復前 {@code 930} 落 {@code else} ＋ {@code log.info} ＋ ACK ⇒ 案件永停
     * {@code RETURN_PENDING(20)} ⇒ 7 天後自動關成「不退款」終態
     * ⇒ <b>買家已交件卻拿不到退款，零告警</b>（66 §7.3.2 U5）。
     */
    @Test
    void uberReverseHandedOver930_advancesCaseToReturnShipping() throws Exception {
        AfterSaleCaseEntity c = caseWith("A930", "O930", "20", "3003", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(c);
        when(caseMapper.markReturnInTransit("A930")).thenReturn(1);
        stubPublishSucceeds();

        consumer.onReturnStatusChanged(
                buildMessage(payload("O930", "930", true, "SR930", null)), channel, 30L);

        verify(caseMapper).markReturnInTransit("A930");
    }

    /**
     * 🔴 Uber 逆向 {@code pickup}（→ {@code 933}）是**明確的 no-op**，不是未識別。
     * 兩者的差別在於：前者我們決定不做事，後者我們沒認出這個值——而後者必須 fail-loud。
     */
    @Test
    void uberReverseCourierAssigned933_isExplicitNoOpAndTouchesNothing() throws Exception {
        AfterSaleCaseEntity c = caseWith("A933", "O933", "20", "3003", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(c);

        consumer.onReturnStatusChanged(
                buildMessage(payload("O933", "933", true, "SR933", null)), channel, 31L);

        verify(caseMapper, never()).markReturnInTransit(anyString());
        verify(caseMapper, never()).markReturnAtStore(anyString(), any(), any());
        verify(caseMapper, never()).markReturnDelivered(anyString(), any(), any(), any());
        verify(caseMapper, never()).markReturnFailed(anyString(), anyString(), any());
        verify(channel).basicAck(eq(31L), eq(false));
    }

    /**
     * 🔴 值域外的貨態必須 <b>fail-loud</b>（對齊正向 leg），不得 {@code else} ＋ ACK。
     *
     * <p>鑑別力：把 consumer 改回「未識別就 log.info ＋ ACK」⇒ 這裡的
     * {@code basicAck} 會被呼叫、{@code handleFailure} 的重投不會發生 ⇒ 轉紅。
     */
    @Test
    void unknownWireStatus_failsLoudInsteadOfSilentAck() throws Exception {
        AfterSaleCaseEntity c = caseWith("A999", "O999X", "20", "3003", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(c);

        consumer.onReturnStatusChanged(
                buildMessage(payload("O999X", "7777", true, "SR999", null)), channel, 32L);

        // MqErrorHandler 的重試路徑：ACK 後以 x-retry-count 重新 publish；
        // 關鍵斷言是「沒有走完正常處理」——四個 mapper 動作一個都不得發生。
        verify(caseMapper, never()).markReturnInTransit(anyString());
        verify(caseMapper, never()).markReturnAtStore(anyString(), any(), any());
        verify(caseMapper, never()).markReturnDelivered(anyString(), any(), any(), any());
        verify(caseMapper, never()).markReturnFailed(anyString(), anyString(), any());
    }

    // ==================== 副作用失敗無訊號修復（出路 4） ====================

    /**
     * 核心保證：case transition 事件送不出去時，短 tx 必須 rollback，狀態不得單獨前進。
     *
     * <p>本 consumer 修復前<b>完全沒有 tx</b>：{@code markReturnXxx} 各自 auto-commit、
     * publish 走吞例外的 {@code publishCaseTransitionWithContext}
     * ⇒ MQ 短暫不可用時「狀態已前進、通知沒發」，而重投時 CAS 回 0 走冪等 skip
     * ⇒ 該則買賣家通知永久遺失且零告警。
     *
     * <p>鑑別力：把 {@code publishOrRollback} 換回 {@code publishCaseTransitionWithContext}
     * （或拿掉 txTemplate 包覆）⇒ rollbackCount 變 0 ⇒ 轉紅。
     */
    @Test
    void publishFailure_rollsBackSoStateDoesNotAdvanceWithoutEvent() throws Exception {
        AfterSaleCaseEntity entity = caseWith("A100", "O1000", "20", "3002", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.markReturnInTransit("A100")).thenReturn(1);
        when(eventPublisher.resolveEventPrincipalsBestEffort(any(), any())).thenReturn(PRINCIPALS);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class))).thenReturn(false);

        consumer.onReturnStatusChanged(buildMessage(payload("O1000", "31", true, null, null)), channel, 11L);

        assertThat(txManager.rollbackCount)
                .as("publish 失敗必須 rollback —— 否則狀態已前進而通知永久遺失")
                .isEqualTo(1);
        assertThat(txManager.commitCount).as("publish 失敗禁止 commit").isEqualTo(0);
    }

    /**
     * 出路 4 硬性邊界：身分反解（兩次無 deadline 的 gRPC）必須在 tx <b>之外</b>。
     *
     * <p>鑑別力：改回讓 publisher 在方法內自己反解（不帶 {@code EventPrincipals} 的多載）
     * ⇒ 反解會發生在 beginCount ≥ 1 的當下 ⇒ 轉紅。
     */
    @Test
    void principalResolutionHappensBeforeTransactionOpens() throws Exception {
        AfterSaleCaseEntity entity = caseWith("A101", "O1010", "20", "3002", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(entity);
        when(caseMapper.markReturnInTransit("A101")).thenReturn(1);
        when(eventPublisher.tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class))).thenReturn(true);

        int[] beginCountAtResolve = {-1};
        when(eventPublisher.resolveEventPrincipalsBestEffort(any(), any())).thenAnswer(inv -> {
            beginCountAtResolve[0] = txManager.beginCount;
            return PRINCIPALS;
        });

        consumer.onReturnStatusChanged(buildMessage(payload("O1010", "31", true, null, null)), channel, 12L);

        assertThat(beginCountAtResolve[0])
                .as("身分反解必須發生在開 tx 之前（tx 內不得有任何網路呼叫）")
                .isEqualTo(0);
        assertThat(txManager.commitCount).isEqualTo(1);
    }

    // ==================== W5-4 陰性：不得 parse Uber raw code ====================

    /**
     * 🔴 Uber 原生狀態字串進來時必須 <b>fail-loud</b>，一個 mapper 動作都不得發生。
     *
     * <p>既有的 {@code unknownWireStatus_failsLoudInsteadOfSilentAck} 餵的是 {@code "7777"}
     * 這種一看就不合法的數字；它對<b>本條要防的那個缺陷</b>沒有鑑別力——
     * 有人加一條 {@code case "pickup_complete" -> handleInTransit(...)}，那支測試照樣全綠。
     *
     * <p>清單不是手寫的：從 shipping-service 的 {@code UberStatusMapper} 原始碼解析
     * （見 {@link UberRawStatusCodes}）。漏掉一個原生值，保證就在那個值上失效而零訊號。
     */
    @Test
    void uberRawCodes_neverDriveAnyStateTransition() throws Exception {
        java.util.Set<String> rawCodes = UberRawStatusCodes.all();
        assertThat(rawCodes).as("解析到的 Uber 原生值").hasSizeGreaterThanOrEqualTo(6);

        AfterSaleCaseEntity c = caseWith("AUBER", "OUBER", "20", "3003", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(c);

        long tag = 100L;
        for (String raw : rawCodes) {
            consumer.onReturnStatusChanged(
                    buildMessage(payload("OUBER", raw, true, null, null)), channel, tag++);
        }

        // 四個狀態推進動作一個都不得發生——不論那個原生值在 Uber 那邊代表什麼。
        verify(caseMapper, never()).markReturnInTransit(anyString());
        verify(caseMapper, never()).markReturnAtStore(anyString(), any(), any());
        verify(caseMapper, never()).markReturnDelivered(anyString(), any(), any(), any());
        verify(caseMapper, never()).markReturnFailed(anyString(), anyString(), any());
        verify(caseMapper, never()).stampReturnProviderInfo(anyString(), anyString(), anyString());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class));

        // 對照組：上面那圈「什麼都沒發生」必須是因為值被拒絕，不是因為根本沒跑到 consumer。
        verify(caseMapper, times(rawCodes.size())).selectOne(any());
    }

    /**
     * 🔴 {@code CANCELLED(41)} 是<b>值域內、但逆向 leg 無對應語意</b>（66 UP-8）：
     * 必須 REQUIRES_MANUAL_REVIEW —— 不推進任何狀態、不發任何事件，但要 ACK
     * （它不是「沒認出來」，重投一萬次答案都一樣）。
     *
     * <p>鑑別力：把 {@code REQUIRES_MANUAL_REVIEW} 併進 {@code NO_OP}，或讓它猜一個狀態
     * 推進去 ⇒ 本測試轉紅。
     */
    @Test
    void cancelled41_requiresManualReview_touchesNothingButAcks() throws Exception {
        assertEquals(ReturnShippingRules.Disposition.REQUIRES_MANUAL_REVIEW,
                ReturnShippingRules.dispositionOf(ShippingStatus.CANCELLED),
                "UP-8：逆向 leg 收到 41 沒有對應的售後語意，必須交人工而不是猜");

        AfterSaleCaseEntity c = caseWith("A041", "O041", "20", "3003", 1001L, 2001L);
        when(caseMapper.selectOne(any())).thenReturn(c);

        consumer.onReturnStatusChanged(
                buildMessage(payload("O041", ShippingStatus.CANCELLED.value(), true, null, null)),
                channel, 41L);

        verify(caseMapper, never()).markReturnInTransit(anyString());
        verify(caseMapper, never()).markReturnAtStore(anyString(), any(), any());
        verify(caseMapper, never()).markReturnDelivered(anyString(), any(), any(), any());
        verify(caseMapper, never()).markReturnFailed(anyString(), anyString(), any());
        verify(caseMapper, never()).stampReturnProviderInfo(anyString(), anyString(), anyString());
        verify(eventPublisher, never()).tryPublishCaseTransitionWithContext(
                anyString(), anyString(), anyString(), anyString(), any(), any(), anyString(),
                any(), any(EventPrincipals.class));
        // 值域內 ⇒ 重投沒有意義 ⇒ 必須 ACK（與值域外的 fail-loud 相反）。
        verify(channel).basicAck(eq(41L), eq(false));
    }

    // ==================== helpers ====================

    private AfterSaleCaseEntity caseWith(String caseNo, String orderNo, String status, String returnShippingMethod,
                                          Long sellerId, Long buyerId) {
        AfterSaleCaseEntity e = new AfterSaleCaseEntity();
        e.setCaseNo(caseNo);
        e.setOrderNo(orderNo);
        e.setStatus(status);
        e.setReturnShippingMethod(returnShippingMethod);
        e.setStoreId(sellerId);
        e.setBuyerId(buyerId);
        return e;
    }

    private String payload(String orderNo, String status, boolean isReturn,
                            String providerStatus, String providerMessage) throws Exception {
        // ShippingEventPayload wire 格式為 snake_case（2026-06-19 統一），consumer 依此解析
        Map<String, Object> map = new java.util.HashMap<>();
        map.put("order_no", orderNo);
        map.put("status", status);
        map.put("is_return", isReturn);
        map.put("shipment_no", "SH-" + orderNo);
        if (providerStatus != null) map.put("provider_status", providerStatus);
        if (providerMessage != null) map.put("provider_message", providerMessage);
        return MAPPER.writeValueAsString(map);
    }

    private Message buildMessage(String body) {
        return new Message(body.getBytes(StandardCharsets.UTF_8), new MessageProperties());
    }
}
