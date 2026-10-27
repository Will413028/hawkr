package io.github.will413028.hawkr.common.obs.grpc;

import io.github.will413028.hawkr.common.obs.context.ObsContext;
import io.github.will413028.hawkr.common.obs.context.ObsKeys;
import io.github.will413028.hawkr.common.obs.util.IdGenerator;
import io.grpc.CallOptions;
import io.grpc.Channel;
import io.grpc.ClientCall;
import io.grpc.ClientInterceptor;
import io.grpc.ForwardingClientCall;
import io.grpc.Metadata;
import io.grpc.MethodDescriptor;
import io.opentelemetry.context.Context;
import io.opentelemetry.context.propagation.TextMapPropagator;
import io.opentelemetry.context.propagation.TextMapSetter;

/**
 * gRPC client interceptor for observability.
 * Injects traceparent, correlation_id, and request_id into outgoing metadata.
 */
public class ObsGrpcClientInterceptor implements ClientInterceptor {

    private static final Metadata.Key<String> KEY_CORRELATION_ID =
            Metadata.Key.of(ObsKeys.HEADER_CORRELATION_ID, Metadata.ASCII_STRING_MARSHALLER);
    private static final Metadata.Key<String> KEY_REQUEST_ID =
            Metadata.Key.of(ObsKeys.HEADER_REQUEST_ID, Metadata.ASCII_STRING_MARSHALLER);

    private final ObsContext obsContext;
    private final TextMapPropagator propagator;

    public ObsGrpcClientInterceptor(ObsContext obsContext, TextMapPropagator propagator) {
        this.obsContext = obsContext;
        this.propagator = propagator;
    }

    @Override
    public <ReqT, RespT> ClientCall<ReqT, RespT> interceptCall(
            MethodDescriptor<ReqT, RespT> method,
            CallOptions callOptions,
            Channel next) {

        return new ForwardingClientCall.SimpleForwardingClientCall<>(
                next.newCall(method, callOptions)) {

            @Override
            public void start(Listener<RespT> responseListener, Metadata headers) {
                String correlation = obsContext.getCorrelationIdOrNull();
                if (correlation == null) {
                    correlation = IdGenerator.uuidV4Lower();
                    obsContext.forceSetCorrelationId(correlation);
                }

                String request = obsContext.getRequestIdOrNull();

                propagator.inject(Context.current(), headers, new MetadataSetter());

                headers.put(KEY_CORRELATION_ID, correlation);
                if (request != null) {
                    headers.put(KEY_REQUEST_ID, request);
                }

                super.start(responseListener, headers);
            }
        };
    }

    private static class MetadataSetter implements TextMapSetter<Metadata> {

        @Override
        public void set(Metadata carrier, String key, String value) {
            Metadata.Key<String> metadataKey = Metadata.Key.of(key, Metadata.ASCII_STRING_MARSHALLER);
            carrier.put(metadataKey, value);
        }
    }
}
