package io.github.will413028.hawkr.common.obs.context;

/**
 * Defines all fixed key names for MDC and transport headers.
 */
public final class ObsKeys {

    private ObsKeys() {
    }

    // MDC keys (fixed strings)
    public static final String TRACE_ID = "trace_id";
    public static final String SPAN_ID = "span_id";
    public static final String CORRELATION_ID = "correlation_id";
    public static final String REQUEST_ID = "request_id";
    public static final String EVENT = "event";
    public static final String SERVICE = "service";

    // Transport keys for gRPC metadata and RabbitMQ headers (all lowercase)
    public static final String HEADER_TRACEPARENT = "traceparent";
    public static final String HEADER_CORRELATION_ID = "x-correlation-id";
    public static final String HEADER_REQUEST_ID = "x-request-id";
    public static final String HEADER_EVENT = "event";
}
