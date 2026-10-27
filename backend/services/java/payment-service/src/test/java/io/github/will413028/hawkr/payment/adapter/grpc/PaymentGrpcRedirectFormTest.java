package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.payment.application.service.PaymentPageService;
import io.github.will413028.hawkr.payment.application.service.PaymentRedirectFormStore;
import io.github.will413028.hawkr.payment.grpc.GetPaymentRedirectFormRequest;
import io.github.will413028.hawkr.payment.grpc.GetPaymentRedirectFormResponse;
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
 * 單元測試：GetPaymentRedirectForm gRPC handler（#1394）。
 * 驗證有效 id 回傳全欄位（含 isPlatform bool 映射），未知 / 過期 / 已用 id 回 NOT_FOUND。
 */
class PaymentGrpcRedirectFormTest {

    private PaymentGrpcService newService(PaymentPageService pageService) {
        // 建構子僅賦值不呼叫；只需要 pageService，其餘依賴傳 null
        return new PaymentGrpcService(
                null, null, pageService, null, null, null, null, null,
                null, null, null, null, null, null, null, null);
    }

    @Test
    @SuppressWarnings("unchecked")
    void validId_returnsAllFieldsWithIsPlatformMapped() {
        PaymentPageService pageService = mock(PaymentPageService.class);
        when(pageService.consumeRedirectForm("good-id")).thenReturn(Optional.of(
                new PaymentRedirectFormStore.RedirectForm(
                        "https://sandbox-api.payuni.com.tw/api/upp",
                        "HAWK123", "1.0", "ENC", "HASH", "1")));

        StreamObserver<GetPaymentRedirectFormResponse> observer = mock(StreamObserver.class);
        newService(pageService).getPaymentRedirectForm(
                GetPaymentRedirectFormRequest.newBuilder().setRedirectId("good-id").build(), observer);

        ArgumentCaptor<GetPaymentRedirectFormResponse> captor =
                ArgumentCaptor.forClass(GetPaymentRedirectFormResponse.class);
        verify(observer).onNext(captor.capture());
        verify(observer).onCompleted();
        verify(observer, never()).onError(org.mockito.ArgumentMatchers.any());

        GetPaymentRedirectFormResponse resp = captor.getValue();
        assertEquals("https://sandbox-api.payuni.com.tw/api/upp", resp.getActionUrl());
        assertEquals("HAWK123", resp.getMerId());
        assertEquals("1.0", resp.getVersion());
        assertEquals("ENC", resp.getEncryptInfo());
        assertEquals("HASH", resp.getHashInfo());
        assertTrue(resp.getIsPlatform(), "isPlatform=\"1\" 應映射為 true");
    }

    @Test
    @SuppressWarnings("unchecked")
    void unknownId_returnsNotFound() {
        PaymentPageService pageService = mock(PaymentPageService.class);
        when(pageService.consumeRedirectForm("missing")).thenReturn(Optional.empty());

        StreamObserver<GetPaymentRedirectFormResponse> observer = mock(StreamObserver.class);
        newService(pageService).getPaymentRedirectForm(
                GetPaymentRedirectFormRequest.newBuilder().setRedirectId("missing").build(), observer);

        ArgumentCaptor<Throwable> captor = ArgumentCaptor.forClass(Throwable.class);
        verify(observer).onError(captor.capture());
        verify(observer, never()).onNext(org.mockito.ArgumentMatchers.any());

        assertTrue(captor.getValue() instanceof StatusRuntimeException);
        assertEquals(Status.Code.NOT_FOUND,
                ((StatusRuntimeException) captor.getValue()).getStatus().getCode());
    }
}
