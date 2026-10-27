package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import com.google.protobuf.Any;
import com.google.rpc.ErrorInfo;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.protobuf.StatusProto;

/** Maps the shared billing machine symbol to the standard gRPC rich error contract. */
final class MembershipBillingGrpcError {

    private static final String DOMAIN = "hawkr.membership.billing";

    private MembershipBillingGrpcError() {
    }

    static StatusRuntimeException toStatusRuntimeException(BillingValidationException exception) {
        Status grpcStatus = statusFor(exception);
        ErrorInfo details = ErrorInfo.newBuilder()
                .setReason(exception.errorCode().value())
                .setDomain(DOMAIN)
                .putMetadata("error_code", exception.errorCode().code())
                .build();
        com.google.rpc.Status status = com.google.rpc.Status.newBuilder()
                .setCode(grpcStatus.getCode().value())
                .setMessage(exception.getMessage())
                .addDetails(Any.pack(details))
                .build();
        return StatusProto.toStatusRuntimeException(status);
    }

    private static Status statusFor(BillingValidationException exception) {
        return switch (exception.errorCode()) {
            case BILLING_SUBJECT_INVALID, BILLING_OFFER_INVALID, BILLING_QUOTE_INVALID,
                    BILLING_AMOUNT_MISMATCH -> Status.INVALID_ARGUMENT;
            case BILLING_COMMAND_CONFLICT, BILLING_SUBSCRIPTION_CONFLICT -> Status.ALREADY_EXISTS;
            case PROVIDER_CAPABILITY_UNSUPPORTED -> Status.UNIMPLEMENTED;
            case PROVIDER_REJECTED -> Status.FAILED_PRECONDITION;
            case PROVIDER_RESULT_CONFLICT -> Status.ABORTED;
            // Status.INTERNAL 而非 UNAVAILABLE（#3330）：BILLING_UNAVAILABLE 的語意是
            // 「PayUni 呼叫 transport 失敗、結果未知，請用同一把 request_id 重試」——
            // 那是**下游的下游**倒了，payment-service 自己是健康的。
            //
            // gateway 的 circuit breaker 以 target 為 key、一個服務的所有路由共用一顆，
            // 把 UNAVAILABLE 當 transport 失敗計入 ⇒ PayUni 一出事就會推開
            // gateway→payment-service 的 breaker，而 payment 在結帳 hot path 上。
            //
            // 不違反本服務契約：billing.proto 明寫「Clients must branch on
            // ErrorInfo.reason, never on the human-readable status message」⇒ gRPC code
            // 只是粗粒度 bucket，reason=BILLING_UNAVAILABLE 才是契約，而它不變。
            //
            // 為什麼不是別的：FAILED_PRECONDITION 已被 PROVIDER_REJECTED 佔用且語意相反
            //（明確拒絕 vs 結果未知）；ABORTED 在 envelope 映 409，會把「未知結果」
            // 講成「衝突」。
            case BILLING_UNAVAILABLE -> Status.INTERNAL;
        };
    }
}
