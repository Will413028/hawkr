package io.github.will413028.hawkr.common.obs.grpc;

import io.github.will413028.hawkr.common.obs.config.ObsProperties;
import io.github.will413028.hawkr.common.obs.context.ObsCarrier;
import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.context.ObsKeys;
import io.github.will413028.hawkr.common.obs.context.ObsScope;
import io.grpc.ForwardingServerCall;
import io.grpc.ForwardingServerCallListener;
import io.grpc.Metadata;
import io.grpc.ServerCall;
import io.grpc.ServerCallHandler;
import io.grpc.ServerInterceptor;
import io.grpc.Status;
import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.SpanKind;
import io.opentelemetry.api.trace.StatusCode;
import io.opentelemetry.api.trace.Tracer;
import io.opentelemetry.context.Context;
import io.opentelemetry.context.Scope;
import io.opentelemetry.context.propagation.TextMapGetter;
import io.opentelemetry.context.propagation.TextMapPropagator;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;

import java.util.concurrent.ThreadLocalRandom;

/**
 * Guard-type gRPC server interceptor for observability.
 *
 * Design principles (Hard Rules):
 * - Only observes, never modifies responses
 * - Never swallows exceptions
 * - Never calls responseObserver.onNext/onCompleted/onError
 *
 * Access Log Rules:
 * - ERROR (Status != OK): Always log at ERROR level
 * - SLOW (OK but duration > threshold): Always log at WARN level
 * - OK (normal): Sample-based logging at INFO level
 */
public class ObsGrpcServerInterceptor implements ServerInterceptor {

    private static final Logger log = LoggerFactory.getLogger(ObsGrpcServerInterceptor.class);

    private static final Metadata.Key<String> KEY_TRACEPARENT =
            Metadata.Key.of(ObsKeys.HEADER_TRACEPARENT, Metadata.ASCII_STRING_MARSHALLER);
    private static final Metadata.Key<String> KEY_CORRELATION_ID =
            Metadata.Key.of(ObsKeys.HEADER_CORRELATION_ID, Metadata.ASCII_STRING_MARSHALLER);
    private static final Metadata.Key<String> KEY_REQUEST_ID =
            Metadata.Key.of(ObsKeys.HEADER_REQUEST_ID, Metadata.ASCII_STRING_MARSHALLER);

    // Duplicate MDC keys to remove (set by other frameworks like micrometer-tracing)
    // Hard Rule: Only snake_case allowed, remove camelCase duplicates
    private static final String MDC_TRACE_ID_CAMEL = "traceId";
    private static final String MDC_SPAN_ID_CAMEL = "spanId";

    private final ObsContext obsContext;
    private final ObsProperties obsProperties;
    private final Tracer tracer;
    private final TextMapPropagator propagator;

    public ObsGrpcServerInterceptor(ObsContext obsContext,
                                    ObsProperties obsProperties,
                                    Tracer tracer,
                                    TextMapPropagator propagator) {
        this.obsContext = obsContext;
        this.obsProperties = obsProperties;
        this.tracer = tracer;
        this.propagator = propagator;
    }

    @Override
    public <ReqT, RespT> ServerCall.Listener<ReqT> interceptCall(
            ServerCall<ReqT, RespT> call,
            Metadata headers,
            ServerCallHandler<ReqT, RespT> next) {

        String fullMethodName = call.getMethodDescriptor().getFullMethodName();
        String[] parts = fullMethodName.split("/");
        String grpcServiceFull = parts.length > 0 ? parts[0] : fullMethodName;
        String grpcMethod = parts.length > 1 ? parts[1] : fullMethodName;

        // Extract simple service name (e.g., "hawkr.user.UserService" -> "UserService")
        final String grpcService = extractSimpleName(grpcServiceFull);

        // 1. Extract inbound metadata
        String correlation = headers.get(KEY_CORRELATION_ID);
        String request = headers.get(KEY_REQUEST_ID);
        String traceparent = headers.get(KEY_TRACEPARENT);

        // 2. Extract parent context from traceparent
        Context parentOtelContext = propagator.extract(
                Context.current(),
                headers,
                new MetadataGetter()
        );

        // 3. Start server span
        Span span = tracer.spanBuilder("grpc.server/" + fullMethodName)
                .setParent(parentOtelContext)
                .setSpanKind(SpanKind.SERVER)
                .startSpan();

        // 4. Make span current
        Scope otelScope = span.makeCurrent();

        // 5. Open ObsContext scope (generates correlation if missing)
        ObsScope obsScope = obsContext.open(new ObsCarrier(correlation, request, traceparent));

        // 6. Write MDC (Hard Rule: only snake_case)
        // Remove camelCase duplicates that may be set by other frameworks (micrometer-tracing)
        MDC.remove(MDC_TRACE_ID_CAMEL);
        MDC.remove(MDC_SPAN_ID_CAMEL);

        // Always put trace_id/span_id (empty string if invalid)
        if (span.getSpanContext().isValid()) {
            MDC.put(ObsKeys.TRACE_ID, span.getSpanContext().getTraceId());
            MDC.put(ObsKeys.SPAN_ID, span.getSpanContext().getSpanId());
        } else {
            MDC.put(ObsKeys.TRACE_ID, "");
            MDC.put(ObsKeys.SPAN_ID, "");
        }

        MDC.put(ObsKeys.SERVICE, obsProperties.getService());

        // 7. Record start time
        long startTimeNanos = System.nanoTime();

        // 8. Emit start log (always)
        emitStartLog(grpcService, grpcMethod);

        // 9. Create context holder for exception tracking
        ExceptionHolder exceptionHolder = new ExceptionHolder();

        // 10. Wrap ServerCall to intercept close()
        ServerCall<ReqT, RespT> wrappedCall = new ForwardingServerCall.SimpleForwardingServerCall<>(call) {
            @Override
            public void close(Status status, Metadata trailers) {
                try {
                    // Calculate duration
                    long durationMs = (System.nanoTime() - startTimeNanos) / 1_000_000;

                    // Set span status
                    if (status.isOk()) {
                        span.setStatus(StatusCode.OK);
                    } else {
                        span.setStatus(StatusCode.ERROR, status.getDescription());
                        if (status.getCause() != null) {
                            span.recordException(status.getCause());
                        }
                    }

                    // gRPC RPC 語意屬性，供 OTel Collector spanmetricsconnector 提升成
                    // metric dimension（rpc_method / rpc_response_status_code）。
                    // 命名與值格式對齊 Go 側官方 otelgrpc@v0.67.0（internal/parse.go、
                    // stats_handler.go canonicalString()）：fullMethodName 已無開頭斜線，
                    // status.getCode().name() 原生即為 UPPER_SNAKE_CASE，兩邊天生一致，
                    // 不需要正規化。詳見 docs/superpowers/specs/2026-08-20-grpc-
                    // observability-spanmetrics-consolidation-design.md §3.4。
                    span.setAttribute("rpc.method", fullMethodName);
                    span.setAttribute("rpc.response.status_code", status.getCode().name());

                    // Record exception from listener if any
                    if (exceptionHolder.exception != null) {
                        span.recordException(exceptionHolder.exception);
                        span.setStatus(StatusCode.ERROR, exceptionHolder.exception.getMessage());
                    }

                    // Access log based on rules (uses event format)
                    emitAccessLog(status, durationMs, grpcService, grpcMethod, exceptionHolder.exception);

                } finally {
                    // End span
                    span.end();

                    // Close scopes
                    obsScope.close();
                    otelScope.close();

                    // Cleanup MDC (all keys we may have set)
                    MDC.remove(ObsKeys.TRACE_ID);
                    MDC.remove(ObsKeys.SPAN_ID);
                    MDC.remove(ObsKeys.EVENT);
                    // Also remove camelCase duplicates (Hard Rule: no camelCase in output)
                    MDC.remove(MDC_TRACE_ID_CAMEL);
                    MDC.remove(MDC_SPAN_ID_CAMEL);

                    // Delegate to original call
                    super.close(status, trailers);
                }
            }
        };

        // 11. Start call with wrapped ServerCall
        ServerCall.Listener<ReqT> listener;
        try {
            listener = next.startCall(wrappedCall, headers);
        } catch (Throwable t) {
            // Exception during handler setup - record and rethrow
            exceptionHolder.exception = t;
            span.recordException(t);
            span.setStatus(StatusCode.ERROR, t.getMessage());
            throw t;
        }

        // 12. Wrap listener to catch exceptions (Guard pattern - never swallow)
        return new ForwardingServerCallListener.SimpleForwardingServerCallListener<>(listener) {

            @Override
            public void onMessage(ReqT message) {
                try {
                    super.onMessage(message);
                } catch (Throwable t) {
                    handleListenerException(t);
                    throw t; // Never swallow
                }
            }

            @Override
            public void onHalfClose() {
                try {
                    super.onHalfClose();
                } catch (Throwable t) {
                    handleListenerException(t);
                    throw t; // Never swallow
                }
            }

            @Override
            public void onCancel() {
                try {
                    super.onCancel();
                } catch (Throwable t) {
                    handleListenerException(t);
                    throw t; // Never swallow
                }
            }

            @Override
            public void onComplete() {
                try {
                    super.onComplete();
                } catch (Throwable t) {
                    handleListenerException(t);
                    throw t; // Never swallow
                }
            }

            @Override
            public void onReady() {
                try {
                    super.onReady();
                } catch (Throwable t) {
                    handleListenerException(t);
                    throw t; // Never swallow
                }
            }

            private void handleListenerException(Throwable t) {
                exceptionHolder.exception = t;
                span.recordException(t);
                span.setStatus(StatusCode.ERROR, t.getMessage());
            }
        };
    }

    /**
     * Emit start log when request arrives.
     *
     * <p>Event format: {@code <grpcService>.<grpcMethod>.start}
     * <p>Always logged at INFO level.
     */
    private void emitStartLog(String grpcService, String grpcMethod) {
        String event = grpcService + "." + grpcMethod + ".start";
        MDC.put(ObsKeys.EVENT, event);
        log.info("");
    }

    /**
     * Emit access log based on status and duration.
     *
     * <p>Event format: {@code <grpcService>.<grpcMethod>.<end|error>}
     *
     * <p>Rules (Hard):
     * <ul>
     *   <li>Status != OK: ERROR log (always), msg = exception message</li>
     *   <li>Status == OK && duration > slowThreshold: WARN log (always), msg = empty</li>
     *   <li>Status == OK && normal: INFO log (sampled), msg = empty</li>
     * </ul>
     */
    private void emitAccessLog(Status status, long durationMs, String grpcService, String grpcMethod, Throwable ex) {
        ObsProperties.AccessLog config = obsProperties.getAccessLog();

        if (!status.isOk()) {
            // ERROR: Always log with exception details
            String event = grpcService + "." + grpcMethod + ".error";
            MDC.put(ObsKeys.EVENT, event);
            if (ex != null) {
                log.error(ex.getMessage(), ex);
            } else if (status.getCause() != null) {
                log.error(status.getCause().getMessage(), status.getCause());
            } else {
                log.error(status.getDescription() != null ? status.getDescription() : "");
            }
        } else if (durationMs > config.getSlowThresholdMs()) {
            // SLOW: Always log with empty msg
            String event = grpcService + "." + grpcMethod + ".end";
            MDC.put(ObsKeys.EVENT, event);
            log.warn("");
        } else {
            // OK: Sample-based logging with empty msg
            if (config.getOkSampleRate() > 0 && ThreadLocalRandom.current().nextDouble() < config.getOkSampleRate()) {
                String event = grpcService + "." + grpcMethod + ".end";
                MDC.put(ObsKeys.EVENT, event);
                log.info("");
            }
        }
    }

    /**
     * Extract simple name from fully qualified service name.
     * e.g., "hawkr.user.UserService" -> "UserService"
     */
    private static String extractSimpleName(String fullName) {
        int lastDot = fullName.lastIndexOf('.');
        if (lastDot >= 0 && lastDot < fullName.length() - 1) {
            return fullName.substring(lastDot + 1);
        }
        return fullName;
    }

    /**
     * Holder for exception caught in listener methods.
     */
    private static class ExceptionHolder {
        volatile Throwable exception;
    }

    private static class MetadataGetter implements TextMapGetter<Metadata> {

        @Override
        public Iterable<String> keys(Metadata carrier) {
            return carrier.keys();
        }

        @Override
        public String get(Metadata carrier, String key) {
            Metadata.Key<String> metadataKey = Metadata.Key.of(key, Metadata.ASCII_STRING_MARSHALLER);
            return carrier.get(metadataKey);
        }
    }
}
