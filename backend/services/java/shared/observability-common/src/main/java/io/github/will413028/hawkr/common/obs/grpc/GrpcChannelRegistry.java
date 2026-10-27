package io.github.will413028.hawkr.common.obs.grpc;

import io.github.will413028.hawkr.common.obs.config.GrpcTarget;
import io.github.will413028.hawkr.common.obs.config.ObsProperties;
import io.grpc.ClientInterceptor;
import io.grpc.ManagedChannel;
import io.grpc.ManagedChannelBuilder;
import jakarta.annotation.PreDestroy;

import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.TimeUnit;

/**
 * Registry for managing gRPC channels.
 * Channels are cached and reused per target name.
 * Ensures all channels are properly configured with client interceptors.
 *
 * 攔截器堆疊（出站順序）：
 * 1. ObsGrpcClientInterceptor - 注入 traceparent、correlation_id
 * 2. GrpcAuthClientInterceptor - 標記內部呼叫 + 傳播 auth context
 */
public final class GrpcChannelRegistry {

    private final Map<String, ManagedChannel> channels = new ConcurrentHashMap<>();
    private final ObsProperties obsProperties;
    private final List<ClientInterceptor> interceptors;

    public GrpcChannelRegistry(ObsProperties obsProperties, ObsGrpcClientInterceptor obsInterceptor,
                               ClientInterceptor... additionalInterceptors) {
        this.obsProperties = obsProperties;
        this.interceptors = new ArrayList<>();
        this.interceptors.add(obsInterceptor);
        for (ClientInterceptor interceptor : additionalInterceptors) {
            if (interceptor != null) {
                this.interceptors.add(interceptor);
            }
        }
    }

    /**
     * Gets or creates a managed channel for the specified target.
     *
     * @param targetName the target name as configured in application.yml
     * @return the managed channel
     * @throws IllegalArgumentException if target is not configured
     */
    public ManagedChannel getOrCreate(String targetName) {
        GrpcTarget config = obsProperties.getGrpc().getTargets().get(targetName);
        if (config == null) {
            throw new IllegalArgumentException(
                    String.format("gRPC target '%s' not configured. " +
                                    "Service: %s. " +
                                    "Please add hawkr.obs.grpc.targets.%s in application.yml",
                            targetName,
                            obsProperties.getService(),
                            targetName));
        }

        return channels.computeIfAbsent(targetName, name -> buildChannel(config));
    }

    private ManagedChannel buildChannel(GrpcTarget config) {
        ManagedChannelBuilder<?> builder = ManagedChannelBuilder
                .forAddress(config.getHost(), config.getPort())
                .intercept(interceptors);

        if (config.isPlaintext()) {
            builder.usePlaintext();
        }

        return builder.build();
    }

    /**
     * Shuts down all managed channels.
     * Called automatically on Spring context close.
     */
    @PreDestroy
    public void shutdownAll() {
        channels.values().forEach(channel -> {
            channel.shutdown();
            try {
                if (!channel.awaitTermination(5, TimeUnit.SECONDS)) {
                    channel.shutdownNow();
                }
            } catch (InterruptedException e) {
                channel.shutdownNow();
                Thread.currentThread().interrupt();
            }
        });
        channels.clear();
    }
}
