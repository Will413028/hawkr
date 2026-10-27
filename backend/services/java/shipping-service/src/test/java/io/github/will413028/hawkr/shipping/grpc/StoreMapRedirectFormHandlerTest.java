package io.github.will413028.hawkr.shipping.grpc;

import io.github.will413028.hawkr.shipping.service.StoreMapRedirectFormStore;
import io.github.will413028.hawkr.shipping.service.StoreMapService;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import org.junit.jupiter.api.Test;
import org.mockito.ArgumentCaptor;

import java.util.Optional;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertTrue;
import static org.mockito.Mockito.mock;
import static org.mockito.Mockito.never;
import static org.mockito.Mockito.verify;
import static org.mockito.Mockito.when;

/**
 * 單元測試：GetStoreMapRedirectForm gRPC handler（#1394）。
 * 驗證有效 id 回傳全欄位，未知 / 過期 / 已用 id 回 NOT_FOUND。
 */
class StoreMapRedirectFormHandlerTest {

    private ShippingGrpcServiceImpl newService(StoreMapService storeMapService) {
        // 建構子僅賦值不呼叫；只需要 storeMapService，其餘依賴傳 null
        // ⚠️ 位置參數 ⇒ 建構子多一個參數這裡就編譯失敗。那是**好事**：
        //    它強迫加參數的人看一眼這支測試，而不是讓它靜默地傳錯位置。
        return new ShippingGrpcServiceImpl(
                null, null, null, storeMapService, null, null, null, null, null, null, null, null, null,
                null, null);
    }

    @Test
    @SuppressWarnings("unchecked")
    void validId_returnsAllFields() {
        StoreMapService storeMapService = mock(StoreMapService.class);
        when(storeMapService.consumeRedirectForm("good-id")).thenReturn(Optional.of(
                new StoreMapRedirectFormStore.RedirectForm(
                        "https://sandbox-api.payuni.com.tw/api/storeMap",
                        "HAWK456", "1.0", "ENC", "HASH")));

        StreamObserver<GetStoreMapRedirectFormResponse> observer = mock(StreamObserver.class);
        newService(storeMapService).getStoreMapRedirectForm(
                GetStoreMapRedirectFormRequest.newBuilder().setRedirectId("good-id").build(), observer);

        ArgumentCaptor<GetStoreMapRedirectFormResponse> captor =
                ArgumentCaptor.forClass(GetStoreMapRedirectFormResponse.class);
        verify(observer).onNext(captor.capture());
        verify(observer).onCompleted();
        verify(observer, never()).onError(org.mockito.ArgumentMatchers.any());

        GetStoreMapRedirectFormResponse resp = captor.getValue();
        assertEquals("https://sandbox-api.payuni.com.tw/api/storeMap", resp.getFormAction());
        assertEquals("HAWK456", resp.getMerId());
        assertEquals("1.0", resp.getVersion());
        assertEquals("ENC", resp.getEncryptInfo());
        assertEquals("HASH", resp.getHashInfo());
    }

    @Test
    @SuppressWarnings("unchecked")
    void unknownId_returnsNotFound() {
        StoreMapService storeMapService = mock(StoreMapService.class);
        when(storeMapService.consumeRedirectForm("missing")).thenReturn(Optional.empty());

        StreamObserver<GetStoreMapRedirectFormResponse> observer = mock(StreamObserver.class);
        newService(storeMapService).getStoreMapRedirectForm(
                GetStoreMapRedirectFormRequest.newBuilder().setRedirectId("missing").build(), observer);

        ArgumentCaptor<Throwable> captor = ArgumentCaptor.forClass(Throwable.class);
        verify(observer).onError(captor.capture());
        verify(observer, never()).onNext(org.mockito.ArgumentMatchers.any());

        assertTrue(captor.getValue() instanceof StatusRuntimeException);
        assertEquals(Status.Code.NOT_FOUND,
                ((StatusRuntimeException) captor.getValue()).getStatus().getCode());
    }
}
