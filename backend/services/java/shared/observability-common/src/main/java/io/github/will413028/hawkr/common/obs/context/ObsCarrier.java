package io.github.will413028.hawkr.common.obs.context;

/**
 * Carrier for observability data extracted from inbound transports.
 * The traceparent field is passed through for data transfer only;
 * actual parsing is handled by OpenTelemetry propagator/interceptor.
 */
public record ObsCarrier(
        String correlationId,
        String requestId,
        String traceparent
) {
}
