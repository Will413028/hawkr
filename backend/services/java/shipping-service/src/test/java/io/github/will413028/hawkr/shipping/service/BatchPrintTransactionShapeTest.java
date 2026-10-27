package io.github.will413028.hawkr.shipping.service;

import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.LabelPrintStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingMethod;
import io.github.will413028.hawkr.payuni.shipping.ShippingClient;
import io.github.will413028.hawkr.shipping.client.BuyerGrpcClient;
import io.github.will413028.hawkr.shipping.client.MediaGrpcClient;
import io.github.will413028.hawkr.shipping.client.OrderGrpcClient;
import io.github.will413028.hawkr.shipping.client.SellerGrpcClient;
import io.github.will413028.hawkr.shipping.entity.ShipmentEntity;
import io.github.will413028.hawkr.shipping.entity.SellerShipmentSettingEntity;
import io.github.will413028.hawkr.shipping.mapper.ShipmentMapper;
import io.github.will413028.hawkr.shipping.mapper.ShipmentStatusLogMapper;
import io.github.will413028.hawkr.shipping.mq.ShipmentAnomalyEventPublisher;
import io.github.will413028.hawkr.shipping.mq.ShippingEventPublisher;
import io.github.will413028.hawkr.shipping.provider.create.ShipmentStrategy;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.TransactionDefinition;
import org.springframework.transaction.TransactionStatus;
import org.springframework.transaction.annotation.Transactional;
import org.springframework.transaction.support.SimpleTransactionStatus;

import java.util.Collections;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyLong;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.doAnswer;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.when;

/**
 * {@code batchPrintCvsLabel} 的<b>交易形狀</b>回歸（A1）。
 *
 * <p>修復前：整個方法標 {@code @Transactional}，於是 N 筆 {@code acquireLabelPrintRequested}
 * 的 row lock 一路持有到 PayUni HTTP 往返結束（《多副本排程搶佔規範》§一 ② 反對的形狀），
 * 而單筆路徑 {@code generateHomeLabelPdf} 早就用 {@code requiresNewTx} 把 claim 拆成獨立短交易
 * ——兩條路徑設計不一致。
 *
 * <p>本檔從兩面釘住修復：
 * <ol>
 *   <li>方法上不得再有 {@code @Transactional}（反射，語意層）</li>
 *   <li>呼叫 PayUni 的當下，claim 的交易必須<b>已經結束</b>（begin == commit，行為層）</li>
 * </ol>
 */
class BatchPrintTransactionShapeTest {

    /** 記錄每次 PayUni 呼叫當下的 (begin, commit) 快照，用來證明沒有交易還開著。 */
    private static final class ProbeTxManager implements PlatformTransactionManager {
        int beginCount = 0;
        int commitCount = 0;

        @Override
        public TransactionStatus getTransaction(TransactionDefinition definition) {
            beginCount++;
            return new SimpleTransactionStatus(true);
        }

        @Override
        public void commit(TransactionStatus status) {
            commitCount++;
        }

        @Override
        public void rollback(TransactionStatus status) {
            // 本測試不涉及 rollback
        }
    }

    private ShipmentMapper shipmentMapper;
    private ShipmentLabelService labelService;
    private OnboardingService onboardingService;
    private ProbeTxManager txManager;
    private ShippingService shippingService;

    @BeforeEach
    void setUp() {
        shipmentMapper = mock(ShipmentMapper.class);
        labelService = mock(ShipmentLabelService.class);
        onboardingService = mock(OnboardingService.class);
        txManager = new ProbeTxManager();

        shippingService = new ShippingService(
                shipmentMapper,
                mock(ShipmentStatusLogMapper.class),
                mock(OrderGrpcClient.class),
                mock(BuyerGrpcClient.class),
                mock(MediaGrpcClient.class),
                mock(SellerGrpcClient.class),
                mock(ShipmentStatusMapper.class),
                labelService,
                mock(ShippingEventPublisher.class),
                mock(ShipmentAnomalyEventPublisher.class),
                onboardingService,
                mock(HolidayService.class),
                mock(CvsPrintWaitingRegistry.class),
                mock(io.github.will413028.hawkr.common.providercall.ProviderCallExecutor.class),
                txManager,
                // 🔴 registry 必須拿到**同一個** labelService mock：批次列印已改走
                //    LabelCapability.requestLabelBatch，adapter 委派的就是它。
                //    傳另一個 mock 的話，下面的 doAnswer 永遠不會觸發，而測試會以
                //    「PayUni 從未被呼叫」的形式失敗——那看起來像交易形狀壞了，其實是接線錯了。
                io.github.will413028.hawkr.shipping.provider.TestFulfillmentProviders.realRegistry(
                        mock(ShippingClient.class), mock(ShipmentStatusMapper.class),
                        onboardingService, labelService),
                new io.github.will413028.hawkr.shipping.provider.action.AllowedActionResolver(
                        io.github.will413028.hawkr.shipping.provider.TestFulfillmentProviders.realRegistry(
                                mock(ShippingClient.class), mock(ShipmentStatusMapper.class),
                                onboardingService, labelService),
                        new ShipmentStatusMapper()),
                new io.micrometer.core.instrument.simple.SimpleMeterRegistry());
    }

    @Test
    @DisplayName("batchPrintCvsLabel 不得標 @Transactional（否則 N 列 row lock 會橫跨 PayUni HTTP）")
    void batchPrintIsNotWrappedInASingleTransaction() throws Exception {
        assertThat(ShippingService.class
                .getMethod("batchPrintCvsLabel", long.class, List.class, String.class)
                .getAnnotation(Transactional.class))
                .as("claim 走 requiresNewTx 短交易，方法本身不得再包一個長交易")
                .isNull();
    }

    @Test
    @DisplayName("PayUni 呼叫發生時，claim 的短交易必須已提交（沒有交易還開著）")
    void payUniCallHappensOutsideAnyOpenTransaction() {
        long storeId = 7001L;
        when(onboardingService.findActiveSetting(
                eq(storeId), eq(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value())))
                .thenReturn(new SellerShipmentSettingEntity());

        ShipmentEntity a = cvsShipment(1L, "O1", storeId);
        ShipmentEntity b = cvsShipment(2L, "O2", storeId);
        when(shipmentMapper.selectList(any())).thenReturn(List.of(a, b));
        when(shipmentMapper.acquireLabelPrintRequested(anyLong())).thenReturn(1);

        int[] snapshotAtPayUni = {-1, -1};
        doAnswer(inv -> {
            snapshotAtPayUni[0] = txManager.beginCount;
            snapshotAtPayUni[1] = txManager.commitCount;
            return null;
        }).when(labelService).triggerBatchCvsPrint(
                anyString(), anyString(), any(), any(), any(), anyString());

        shippingService.batchPrintCvsLabel(storeId, List.of("O1", "O2"), "2026-08-08");

        assertThat(snapshotAtPayUni[0])
                .as("兩筆 claim 各自開了一個短交易")
                .isEqualTo(2);
        assertThat(snapshotAtPayUni[1])
                .as("呼叫 PayUni 的當下所有 claim 交易都已提交 —— 沒有 row lock 橫跨網路往返")
                .isEqualTo(snapshotAtPayUni[0]);
    }

    private ShipmentEntity cvsShipment(long id, String orderNo, long storeId) {
        ShipmentEntity e = new ShipmentEntity();
        e.setId(id);
        e.setOrderNo(orderNo);
        e.setShipmentNo("S" + id);
        e.setStoreId(storeId);
        e.setShipTradeNo("T" + id);
        e.setMerId("HAWK" + storeId);
        e.setLabelPrintStatus(LabelPrintStatus.NONE.value());
        // 🔴 真實的 shipments.status 是 NOT NULL；fixture 原本沒設，
        //    而批次列印接上 canonical rule 之後，null status 會走 fail-closed 被跳過
        //    ——那是對的產線行為（null ＝ 髒資料），錯的是 fixture 少了一欄。
        e.setStatus(ShippingStatus.UNPROCESSED.value());
        // 配送方式是 adapter 分派的鍵：不設等於「查無 adapter」，而那是 fail-closed 的錯誤路徑
        e.setShippingMethod(ShippingMethod.SEVEN_ELEVEN_STORE_PICKUP.value());
        e.setGoodsType("1");
        e.setLgsType("B2C");
        e.setShipType("UNIMART");
        return e;
    }
}
