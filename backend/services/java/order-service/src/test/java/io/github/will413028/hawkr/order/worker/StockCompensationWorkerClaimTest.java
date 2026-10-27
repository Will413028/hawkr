package io.github.will413028.hawkr.order.worker;

import io.github.will413028.hawkr.order.constant.OrderConstants;
import io.github.will413028.hawkr.order.entity.StockTxRecordEntity;
import io.github.will413028.hawkr.order.grpc.client.ProductGrpcClient;
import io.github.will413028.hawkr.order.repository.StockTxRecordRepository;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;
import org.mockito.InOrder;

import java.time.LocalDateTime;
import java.util.List;

import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.ArgumentMatchers.any;
import static org.mockito.ArgumentMatchers.anyInt;
import static org.mockito.ArgumentMatchers.anyString;
import static org.mockito.ArgumentMatchers.eq;
import static org.mockito.Mockito.inOrder;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.verifyNoInteractions;
import static org.mockito.Mockito.when;

/**
 * issue #2294 H5：{@code StockCompensationWorker} 的 lease 閘門（兩個迴圈各一道）。
 *
 * <p>副作用是 {@code ProductGrpcClient.restoreStock}（歸還庫存）。沒有搶佔時
 * 兩個 pod 會各打一次；product 側雖有 {@code stock_tx_id} 冪等，但重複呼叫仍是
 * 多一次跨服務往返，且 {@code retry_count} 會被雙倍消耗。
 */
class StockCompensationWorkerClaimTest {

    private static StockTxRecordEntity record(String txId, String phase) {
        StockTxRecordEntity r = new StockTxRecordEntity();
        r.setStockTxId(txId);
        r.setOrderNo("ORD1");
        r.setPhase(phase);
        r.setRetryCount(0);
        r.setPayloadSnapshot("[{\"skuId\":1,\"quantity\":2}]");
        return r;
    }

    /**
     * ⚠️ 鑑別點：Mockito 對 {@code int} 預設回 <b>0</b>，
     * 「忘記 stub claim」與「claim 真的回 0」長得一樣。沒有本測試，
     * 把 claim 閘門整個拿掉不會有任何測試轉紅。
     */
    @Test
    @DisplayName("claim 回 0（另一個 pod 已認領）→ 兩個迴圈都不打 gRPC、不寫任何狀態")
    void claimLost_doesNothingInEitherLoop() {
        StockTxRecordRepository repo = mock(StockTxRecordRepository.class);
        ProductGrpcClient product = mock(ProductGrpcClient.class);
        when(repo.findPendingCompensation(anyInt()))
                .thenReturn(List.of(record("TX1", OrderConstants.STOCK_TX_PHASE_DEDUCT)));
        when(repo.findFailedRestores(anyInt()))
                .thenReturn(List.of(record("TX2", OrderConstants.STOCK_TX_PHASE_RESTORE)));
        when(repo.claimForCompensation(anyString(), anyString(), any())).thenReturn(0);

        new StockCompensationWorker(repo, product).compensate();

        verifyNoInteractions(product);
        verify(repo, never()).updateStatus(anyString(), anyString(), anyString());
        verify(repo, never()).updateFailed(anyString(), anyString(), anyString(), anyString(), anyInt());
        verify(repo, never()).markManualIntervention(anyString(), anyString());
        verify(repo, never()).insertRestoreIfAbsent(anyString(), anyString(), anyString(), anyString(), anyString());
    }

    @Test
    @DisplayName("claim 回 1 → 才打 restoreStock，且 claim 嚴格先於 gRPC（InOrder）")
    void claimWon_callsGrpcAfterClaim() {
        StockTxRecordRepository repo = mock(StockTxRecordRepository.class);
        ProductGrpcClient product = mock(ProductGrpcClient.class);
        when(repo.findPendingCompensation(anyInt()))
                .thenReturn(List.of(record("TX1", OrderConstants.STOCK_TX_PHASE_DEDUCT)));
        when(repo.findFailedRestores(anyInt())).thenReturn(List.of());
        when(repo.claimForCompensation(eq("TX1"), anyString(), any())).thenReturn(1);
        when(product.restoreStock(eq("TX1"), any()))
                .thenReturn(io.github.will413028.hawkr.product.grpc.StockOperationResponse.newBuilder()
                        .setStatus(io.github.will413028.hawkr.common.utils.code.StockOperationStatus.SUCCESS.value())
                        .build());

        new StockCompensationWorker(repo, product).compensate();

        InOrder order = inOrder(repo, product);
        order.verify(repo).claimForCompensation(eq("TX1"), eq(OrderConstants.STOCK_TX_PHASE_DEDUCT), any());
        order.verify(product).restoreStock(eq("TX1"), any());
    }

    @Test
    @DisplayName("claim 帶未來租約時間（lease），長度 = CLAIM_LEASE_SEC")
    void claimUsesFutureLease() {
        StockTxRecordRepository repo = mock(StockTxRecordRepository.class);
        ProductGrpcClient product = mock(ProductGrpcClient.class);
        when(repo.findPendingCompensation(anyInt()))
                .thenReturn(List.of(record("TX1", OrderConstants.STOCK_TX_PHASE_DEDUCT)));
        when(repo.findFailedRestores(anyInt())).thenReturn(List.of());
        when(repo.claimForCompensation(anyString(), anyString(), any())).thenReturn(0);

        LocalDateTime before = io.github.will413028.hawkr.common.utils.DateTimeUtils.nowUtc();
        new StockCompensationWorker(repo, product).compensate();

        ArgumentCaptor<LocalDateTime> leaseCaptor = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo).claimForCompensation(eq("TX1"), anyString(), leaseCaptor.capture());
        assertTrue(!leaseCaptor.getValue()
                        .isBefore(before.plusSeconds(StockCompensationWorker.CLAIM_LEASE_SEC)),
                "租約必須是 now + CLAIM_LEASE_SEC 的未來時間——寫 now 或過去即退化成永久旗標");
    }

    /**
     * issue #2294 <b>R4-P1-2</b>：租約基準必須在 {@code claim()} <b>內</b>取，
     * 不能像原實作那樣在<b>兩個迴圈之前</b>只算一次。
     *
     * <p>只算一次 ⇒ 全批共用同一個到期時間 ⇒ 第 k 筆的<b>有效</b>租約
     * = {@link StockCompensationWorker#CLAIM_LEASE_SEC} − (前 k−1 筆的耗時)。
     * 兩個迴圈合計 {@code 20 + 20 = 40} 筆、單筆最壞 10 秒
     * （{@code ProductGrpcClient.INTERNAL_READ_DEADLINE_SEC}）
     * ⇒ 批次上界 <b>400 秒 > 300 秒租約</b> ⇒ 跑到後段時租約已過期，
     * 另一個 pod 會在原持有者仍在打 gRPC 時合法搶走同一筆。
     *
     * <p><b>鑑別方式</b>：讓第一筆的 {@code restoreStock} 耗掉一段可觀測的時間，
     * 再比較兩筆的租約。基準在 {@code claim()} 內 ⇒ 第二筆嚴格晚於第一筆；
     * 基準在迴圈外 ⇒ 兩者<b>完全相等</b>。
     */
    @Test
    @DisplayName("R4-P1-2：租約基準取 claim 當下——後一筆的租約嚴格晚於前一筆（迴圈外只算一次會轉紅）")
    void leaseBaselineIsTakenPerItemNotPerTick() {
        StockTxRecordRepository repo = mock(StockTxRecordRepository.class);
        ProductGrpcClient product = mock(ProductGrpcClient.class);
        when(repo.findPendingCompensation(anyInt())).thenReturn(List.of(
                record("TX_SLOW", OrderConstants.STOCK_TX_PHASE_DEDUCT),
                record("TX_NEXT", OrderConstants.STOCK_TX_PHASE_DEDUCT)));
        when(repo.findFailedRestores(anyInt())).thenReturn(List.of());
        when(repo.claimForCompensation(anyString(), anyString(), any())).thenReturn(1);
        when(product.restoreStock(anyString(), any())).thenAnswer(inv -> {
            Thread.sleep(40L);
            return io.github.will413028.hawkr.product.grpc.StockOperationResponse.newBuilder()
                    .setStatus(io.github.will413028.hawkr.common.utils.code.StockOperationStatus.SUCCESS.value())
                    .build();
        });

        new StockCompensationWorker(repo, product).compensate();

        ArgumentCaptor<LocalDateTime> leases = ArgumentCaptor.forClass(LocalDateTime.class);
        verify(repo, org.mockito.Mockito.times(2))
                .claimForCompensation(anyString(), anyString(), leases.capture());
        long deltaMs = java.time.Duration
                .between(leases.getAllValues().get(0), leases.getAllValues().get(1)).toMillis();

        assertTrue(deltaMs >= 20L,
                "第二筆的租約必須明顯晚於第一筆（實測差 " + deltaMs + " ms）。"
                        + "兩者相等 ⇒ 租約基準在迴圈外只算一次 ⇒ 40 筆 × 10 秒 = 400 秒的批次上界"
                        + "已超過 300 秒租約，後段項目在 claim 當下租約就已過期。");
    }
}
