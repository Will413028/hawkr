package io.github.will413028.hawkr.aftersales.service;

import io.github.will413028.hawkr.aftersales.entity.AfterSaleCaseEntity;
import io.github.will413028.hawkr.aftersales.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.aftersales.mapper.AfterSaleCaseMapper;
import io.github.will413028.hawkr.shipping.grpc.RetryReturnPrintLabelResponse;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.Duration;
import java.time.LocalDateTime;

import static org.junit.jupiter.api.Assertions.assertFalse;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 <b>L7</b>：{@code AfterSaleLabelRetryService.retryAutomatic} 的搶佔語意。
 *
 * <p><b>修復前的缺陷</b>：閘門是 {@code label_retry_count} 的 0→1 CAS
 * （{@code AfterSaleCaseMapper.markLabelRetried}），而候選述詞是 {@code label_retry_count = 0}
 * ⇒ CAS 成功即<b>永久</b>離開候選集合、沒有任何時間欄能讓它回來
 * ＝《多副本排程搶佔規範》§二 第 3 步定義的永久旗標。
 * 副作用（shipping {@code RetryReturnPrintLabel}）排在其後、且在 {@code afterCommit} 回呼裡，
 * 例外被 catch 吞掉而 CAS 所在的 tx 早已 commit
 * ⇒ <b>不必等 pod 崩潰，一次 gRPC 失敗就永久用掉這個案件唯一一次自動重試</b>。
 *
 * <p>⚠️ 這裡的每一支都可以用「把修法還原」鑑別；沒有它們，把 claim 換回 count CAS
 * 不會有任何測試轉紅。
 */
class AfterSaleLabelRetryServiceClaimTest {

    private static final String CASE_NO = "AS_LABEL_1";
    private static final String SHIP_TRADE_NO = "SHIP_1";

    private static AfterSaleCaseEntity caseEntity() {
        AfterSaleCaseEntity e = new AfterSaleCaseEntity();
        e.setCaseNo(CASE_NO);
        e.setShipTradeNo(SHIP_TRADE_NO);
        e.setLabelRetryCount((short) 0);
        return e;
    }

    private static RetryReturnPrintLabelResponse resp(int code) {
        return RetryReturnPrintLabelResponse.newBuilder().setCode(code).build();
    }

    /**
     * 規則 ①：搶佔在副作用之前，claim 回 0 ⇒ 一個副作用都不做。
     *
     * <p>⚠️ 鑑別點：Mockito 對 {@code int} 預設回 <b>0</b>，「忘記 stub claim」與
     * 「claim 真的回 0」長得一樣——所以另一支 {@link #claimWon_callsShippingThenConsumesBudget}
     * 必須明確 stub 回 1，兩支合起來才有鑑別力。
     */
    @Test
    @DisplayName("claim 回 0（另一個 pod 已持有租約）→ 不呼叫 shipping、不消耗預算")
    void claimLost_doesNotCallShipping() {
        AfterSaleCaseMapper mapper = mock(AfterSaleCaseMapper.class);
        ShippingGrpcClient shipping = mock(ShippingGrpcClient.class);
        AfterSaleAuditService audit = mock(AfterSaleAuditService.class);
        when(mapper.claimLabelRetry(eq(CASE_NO), any(), any())).thenReturn(0);

        boolean ok = new AfterSaleLabelRetryService(mapper, shipping, audit).retryAutomatic(caseEntity());

        assertFalse(ok, "claim 失敗必須回 false");
        verifyNoInteractions(shipping);
        verify(mapper, never()).markLabelRetried(anyString());
        verifyNoInteractions(audit);
    }

    /**
     * 規則 ①＋預算語意：claim 先於 shipping，shipping 成功之後才消耗預算。
     */
    @Test
    @DisplayName("claim 回 1 → claim → shipping → markLabelRetried，順序嚴格（InOrder）")
    void claimWon_callsShippingThenConsumesBudget() {
        AfterSaleCaseMapper mapper = mock(AfterSaleCaseMapper.class);
        ShippingGrpcClient shipping = mock(ShippingGrpcClient.class);
        AfterSaleAuditService audit = mock(AfterSaleAuditService.class);
        when(mapper.claimLabelRetry(eq(CASE_NO), any(), any())).thenReturn(1);
        when(shipping.retryReturnPrintLabel(SHIP_TRADE_NO)).thenReturn(resp(0));

        boolean ok = new AfterSaleLabelRetryService(mapper, shipping, audit).retryAutomatic(caseEntity());

        assertTrue(ok);
        InOrder order = inOrder(mapper, shipping);
        order.verify(mapper).claimLabelRetry(eq(CASE_NO), any(), any());
        order.verify(shipping).retryReturnPrintLabel(SHIP_TRADE_NO);
        order.verify(mapper).markLabelRetried(CASE_NO);
    }

    /**
     * <b>本 issue 的核心</b>：shipping 拋例外時<b>不得</b>消耗預算。
     *
     * <p>修復前這裡是「先 CAS 0→1、再在 afterCommit 裡呼 shipping、例外被 catch 吞掉」
     * ⇒ 預算已消耗、列印沒觸發、案件永久離開候選集合。
     */
    @Test
    @DisplayName("shipping 拋例外 → 不消耗 label_retry_count（唯一一次自動重試沒被浪費掉）")
    void shippingThrows_doesNotConsumeBudget() {
        AfterSaleCaseMapper mapper = mock(AfterSaleCaseMapper.class);
        ShippingGrpcClient shipping = mock(ShippingGrpcClient.class);
        AfterSaleAuditService audit = mock(AfterSaleAuditService.class);
        when(mapper.claimLabelRetry(eq(CASE_NO), any(), any())).thenReturn(1);
        when(shipping.retryReturnPrintLabel(SHIP_TRADE_NO))
                .thenThrow(new RuntimeException("UNAVAILABLE: shipping down"));

        boolean ok = new AfterSaleLabelRetryService(mapper, shipping, audit).retryAutomatic(caseEntity());

        assertFalse(ok);
        verify(mapper, never()).markLabelRetried(anyString());
        verifyNoInteractions(audit);

        // 租約立刻釋放 ⇒ 下一個 tick（2 分）就重試，而不是等滿租約
        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> expiredAt = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper).releaseLabelRetryLease(eq(CASE_NO), lease.capture(), expiredAt.capture());

        // 🚨 釋放值不得是 null：那會把耗盡掃描述詞 (count=0 AND label_retry_at IS NOT NULL)
        // 賴以判定的證據抹掉，使 LABEL_RETRY_EXHAUSTED 只在租約被持有的那幾秒可見，
        // 而 checkExhausted 每 10 分鐘才取樣 ⇒ 告警實質失效。
        assertNotNull(expiredAt.getValue(),
                "釋放必須寫入一個過去時間，不得還原成 NULL——NULL 的語意是「從未嘗試過」，"
                        + "會讓耗盡告警在 shipping 持續打不通時永遠不發");
        assertTrue(expiredAt.getValue().isBefore(lease.getValue()),
                "釋放值必須早於租約到期時間（＝已過期），否則案件不會回到候選集合");
    }

    /**
     * gRPC 沒拋例外、但 shipping 回非成功碼，同樣不算觸發成功。
     *
     * <p>只判「有沒有拋例外」會漏掉這條——{@code retryReturnPrintLabel} 是 blocking stub，
     * 業務失敗是在回應的 {@code code} 上正常返回的。
     */
    @Test
    @DisplayName("shipping 回非 0 業務碼 → 不消耗預算、釋放租約")
    void shippingNonSuccessCode_doesNotConsumeBudget() {
        AfterSaleCaseMapper mapper = mock(AfterSaleCaseMapper.class);
        ShippingGrpcClient shipping = mock(ShippingGrpcClient.class);
        AfterSaleAuditService audit = mock(AfterSaleAuditService.class);
        when(mapper.claimLabelRetry(eq(CASE_NO), any(), any())).thenReturn(1);
        when(shipping.retryReturnPrintLabel(SHIP_TRADE_NO)).thenReturn(resp(5001));

        boolean ok = new AfterSaleLabelRetryService(mapper, shipping, audit).retryAutomatic(caseEntity());

        assertFalse(ok);
        verify(mapper, never()).markLabelRetried(anyString());
        verify(mapper).releaseLabelRetryLease(eq(CASE_NO), any(), any());
    }

    /**
     * 規則 ③：claim 寫進去的是<b>未來時間</b>（lease），不是永久旗標。
     *
     * <p>並釘住租約長度的<b>上界</b>：{@code ReturnLabelRetryWorker.checkExhausted()} 每 10 分鐘
     * 會把仍未回填的案件標成耗盡並告警，租約若 ≥ 10 分，案件在租約到期前就被判耗盡
     * ⇒ 租約永遠沒機會讓它回來，退化成 at-most-once。
     */
    @Test
    @DisplayName("claim 寫的是未來時間，且租約長度明顯小於 checkExhausted 的 10 分鐘")
    void claimWritesFutureLeaseBoundedByExhaustedScan() {
        AfterSaleCaseMapper mapper = mock(AfterSaleCaseMapper.class);
        ShippingGrpcClient shipping = mock(ShippingGrpcClient.class);
        AfterSaleAuditService audit = mock(AfterSaleAuditService.class);
        when(mapper.claimLabelRetry(eq(CASE_NO), any(), any())).thenReturn(1);
        when(shipping.retryReturnPrintLabel(SHIP_TRADE_NO)).thenReturn(resp(0));

        new AfterSaleLabelRetryService(mapper, shipping, audit).retryAutomatic(caseEntity());

        ArgumentCaptor<LocalDateTime> now = ArgumentCaptor.forClass(LocalDateTime.class);
        ArgumentCaptor<LocalDateTime> lease = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(mapper).claimLabelRetry(eq(CASE_NO), now.capture(), lease.capture());

        long leaseMinutes = Duration.between(now.getValue(), lease.getValue()).toMinutes();
        assertTrue(leaseMinutes > 0,
                "租約必須是正的未來時間（0 / 負 / NULL = 永久旗標，pod 崩潰即永久卡死），實測 "
                        + leaseMinutes + " 分");
        assertTrue(leaseMinutes < 10,
                "租約 " + leaseMinutes + " 分必須明顯小於 checkExhausted 的 10 分鐘掃描窗口，"
                        + "否則案件在租約到期前就被標記耗盡，租約等同永久旗標");
    }
}
