package io.github.will413028.hawkr.common.obs.mq;

import java.util.Map;
import java.util.concurrent.ExecutionException;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.TimeoutException;

import org.springframework.amqp.core.Message;
import org.springframework.amqp.core.MessageProperties;
import org.springframework.amqp.rabbit.connection.CorrelationData;
import org.springframework.amqp.rabbit.core.RabbitTemplate;

import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.context.ObsKeys;
import io.github.will413028.hawkr.common.obs.util.IdGenerator;
import io.github.will413028.hawkr.common.obs.util.Preconditions;

import io.opentelemetry.context.Context;
import io.opentelemetry.context.propagation.TextMapPropagator;
import io.opentelemetry.context.propagation.TextMapSetter;

/**
 * RabbitMQ publisher with automatic observability header injection. Injects
 * traceparent, correlation_id, request_id, and event into message headers.
 */
public final class ObsRabbitPublisher {

    private final RabbitTemplate rabbitTemplate;
    private final ObsContext obsContext;
    private final TextMapPropagator propagator;

    public ObsRabbitPublisher(RabbitTemplate rabbitTemplate,
            ObsContext obsContext,
            TextMapPropagator propagator) {
        this.rabbitTemplate = rabbitTemplate;
        this.obsContext = obsContext;
        this.propagator = propagator;
    }

    /**
     * Publishes a message with observability headers.
     *
     * @param exchange the exchange name
     * @param routingKey the routing key
     * @param body the message body
     * @param event the event name (required, must not be blank)
     * @throws IllegalArgumentException if event is blank
     */
    public void publish(String exchange, String routingKey, byte[] body, String event) {
        Preconditions.requireNotBlank(event, "event must not be blank");

        String correlation = obsContext.getCorrelationIdOrNull();
        if (correlation == null) {
            correlation = IdGenerator.uuidV4Lower();
            obsContext.forceSetCorrelationId(correlation);
        }

        String request = obsContext.getRequestIdOrNull();

        final String finalCorrelation = correlation;
        final String finalRequest = request;

        rabbitTemplate.convertAndSend(exchange, routingKey, body, message -> {
            MessageProperties props = message.getMessageProperties();
            Map<String, Object> headers = props.getHeaders();

            headers.put(ObsKeys.HEADER_EVENT, event);
            headers.put(ObsKeys.HEADER_CORRELATION_ID, finalCorrelation);

            if (finalRequest != null) {
                headers.put(ObsKeys.HEADER_REQUEST_ID, finalRequest);
            }

            propagator.inject(Context.current(), headers, new HeadersSetter());

            return message;
        });
    }

    /**
     * Publishes a pre-built Message with observability headers.
     *
     * @param exchange the exchange name
     * @param routingKey the routing key
     * @param message the message
     * @param event the event name (required, must not be blank)
     * @throws IllegalArgumentException if event is blank
     */
    public void publish(String exchange, String routingKey, Message message, String event) {
        Preconditions.requireNotBlank(event, "event must not be blank");

        String correlation = obsContext.getCorrelationIdOrNull();
        if (correlation == null) {
            correlation = IdGenerator.uuidV4Lower();
            obsContext.forceSetCorrelationId(correlation);
        }

        String request = obsContext.getRequestIdOrNull();

        Map<String, Object> headers = message.getMessageProperties().getHeaders();
        headers.put(ObsKeys.HEADER_EVENT, event);
        headers.put(ObsKeys.HEADER_CORRELATION_ID, correlation);

        if (request != null) {
            headers.put(ObsKeys.HEADER_REQUEST_ID, request);
        }

        propagator.inject(Context.current(), headers, new HeadersSetter());

        rabbitTemplate.send(exchange, routingKey, message);
    }

    /**
     * Publishes a message and blocks until the broker confirms receipt
     * (publisher-confirm). Injects the same observability headers as
     * {@link #publish(String, String, byte[], String)}. Requires the broker
     * connection factory to have {@code publisher-confirm-type=correlated};
     * with {@code publisher-returns=true} + {@code template.mandatory=true}, an
     * unroutable message is reported as a failure via the returned message.
     *
     * <p>Does not throw on delivery failure so the caller (e.g. an outbox
     * publisher) can decide whether to retry: returns {@code false} on nack,
     * timeout, or unroutable return; {@code true} only when the broker acked
     * and the message was routed.
     *
     * @param exchange the exchange name
     * @param routingKey the routing key
     * @param body the message body
     * @param event the event name (required, must not be blank)
     * @param timeoutMs how long to wait for the broker confirm, in milliseconds
     * @return {@code true} if acked and routed; {@code false} on nack / timeout / unroutable
     * @throws IllegalArgumentException if event is blank
     */
    public boolean publishConfirmed(String exchange, String routingKey, byte[] body,
            String event, long timeoutMs) {
        Preconditions.requireNotBlank(event, "event must not be blank");

        String correlation = obsContext.getCorrelationIdOrNull();
        if (correlation == null) {
            correlation = IdGenerator.uuidV4Lower();
            obsContext.forceSetCorrelationId(correlation);
        }

        String request = obsContext.getRequestIdOrNull();

        final String finalCorrelation = correlation;
        final String finalRequest = request;

        CorrelationData correlationData = new CorrelationData(IdGenerator.uuidV4Lower());

        rabbitTemplate.convertAndSend(exchange, routingKey, body, message -> {
            MessageProperties props = message.getMessageProperties();
            Map<String, Object> headers = props.getHeaders();

            headers.put(ObsKeys.HEADER_EVENT, event);
            headers.put(ObsKeys.HEADER_CORRELATION_ID, finalCorrelation);

            if (finalRequest != null) {
                headers.put(ObsKeys.HEADER_REQUEST_ID, finalRequest);
            }

            propagator.inject(Context.current(), headers, new HeadersSetter());

            return message;
        }, correlationData);

        try {
            CorrelationData.Confirm confirm =
                    correlationData.getFuture().get(timeoutMs, TimeUnit.MILLISECONDS);
            if (confirm == null || !confirm.isAck()) {
                return false;
            }
            // mandatory + publisher-returns: unroutable message populates the
            // returned message on the correlation data before the future acks.
            return correlationData.getReturned() == null;
        } catch (InterruptedException ex) {
            Thread.currentThread().interrupt();
            return false;
        } catch (ExecutionException | TimeoutException ex) {
            return false;
        }
    }

    private static class HeadersSetter implements TextMapSetter<Map<String, Object>> {

        @Override
        public void set(Map<String, Object> carrier, String key, String value) {
            carrier.put(key, value);
        }
    }
}
