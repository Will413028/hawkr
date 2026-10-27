package io.github.will413028.hawkr.common.obs.mq;

import java.util.Collections;
import java.util.Map;

import org.aopalliance.intercept.MethodInterceptor;
import org.aopalliance.intercept.MethodInvocation;
import org.slf4j.MDC;
import org.springframework.amqp.core.Message;

import io.github.will413028.hawkr.common.obs.config.ObsProperties;
import io.github.will413028.hawkr.common.obs.context.ObsCarrier;
import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.context.ObsKeys;
import io.github.will413028.hawkr.common.obs.context.ObsScope;

import io.opentelemetry.api.trace.Span;
import io.opentelemetry.api.trace.SpanKind;
import io.opentelemetry.api.trace.StatusCode;
import io.opentelemetry.api.trace.Tracer;
import io.opentelemetry.context.Context;
import io.opentelemetry.context.Scope;
import io.opentelemetry.context.propagation.TextMapGetter;
import io.opentelemetry.context.propagation.TextMapPropagator;

/**
 * AOP advice for RabbitMQ listeners. Extracts observability headers from
 * incoming messages and manages context.
 */
public class ObsRabbitListenerAdvice implements MethodInterceptor {

    private final ObsContext obsContext;
    private final ObsProperties obsProperties;
    private final Tracer tracer;
    private final TextMapPropagator propagator;

    public ObsRabbitListenerAdvice(ObsContext obsContext,
            ObsProperties obsProperties,
            Tracer tracer,
            TextMapPropagator propagator) {
        this.obsContext = obsContext;
        this.obsProperties = obsProperties;
        this.tracer = tracer;
        this.propagator = propagator;
    }

    @Override
    public Object invoke(MethodInvocation invocation) throws Throwable {
        Message message = findMessageArgument(invocation.getArguments());
        if (message == null) {
            return invocation.proceed();
        }

        Map<String, Object> headers = message.getMessageProperties().getHeaders();

        String correlation = getHeaderAsString(headers, ObsKeys.HEADER_CORRELATION_ID);
        String request = getHeaderAsString(headers, ObsKeys.HEADER_REQUEST_ID);
        String event = getHeaderAsString(headers, ObsKeys.HEADER_EVENT);
        String traceparent = getHeaderAsString(headers, ObsKeys.HEADER_TRACEPARENT);

        Context parentCtx = propagator.extract(
                Context.current(),
                headers,
                new HeadersGetter()
        );

        String spanName = "mq.consume/" + (event != null && !event.isBlank() ? event : "unknown");
        Span span = tracer.spanBuilder(spanName)
                .setParent(parentCtx)
                .setSpanKind(SpanKind.CONSUMER)
                .startSpan();

        Scope otelScope = span.makeCurrent();
        ObsScope obsScope = obsContext.open(new ObsCarrier(correlation, request, traceparent));

        ObsScope eventScope = null;
        if (event != null && !event.isBlank()) {
            eventScope = obsContext.withEvent(event);
        }

        // Only put trace_id/span_id if SpanContext is valid (Hard Rule: no all-zero output)
        if (span.getSpanContext().isValid()) {
            MDC.put(ObsKeys.TRACE_ID, span.getSpanContext().getTraceId());
            MDC.put(ObsKeys.SPAN_ID, span.getSpanContext().getSpanId());
        }
        MDC.put(ObsKeys.SERVICE, obsProperties.getService());

        try {
            return invocation.proceed();
        } catch (Throwable ex) {
            span.recordException(ex);
            span.setStatus(StatusCode.ERROR, ex.getMessage());
            throw ex;
        } finally {
            span.end();
            if (eventScope != null) {
                eventScope.close();
            }
            obsScope.close();
            otelScope.close();
            MDC.remove(ObsKeys.TRACE_ID);
            MDC.remove(ObsKeys.SPAN_ID);
        }
    }

    private Message findMessageArgument(Object[] args) {
        if (args == null) {
            return null;
        }
        for (Object arg : args) {
            if (arg instanceof Message) {
                return (Message) arg;
            }
        }
        return null;
    }

    private String getHeaderAsString(Map<String, Object> headers, String key) {
        Object value = headers.get(key);
        return value != null ? value.toString() : null;
    }

    private static class HeadersGetter implements TextMapGetter<Map<String, Object>> {

        @Override
        public Iterable<String> keys(Map<String, Object> carrier) {
            return carrier != null ? carrier.keySet() : Collections.emptySet();
        }

        @Override
        public String get(Map<String, Object> carrier, String key) {
            if (carrier == null) {
                return null;
            }
            Object value = carrier.get(key);
            return value != null ? value.toString() : null;
        }
    }
}
