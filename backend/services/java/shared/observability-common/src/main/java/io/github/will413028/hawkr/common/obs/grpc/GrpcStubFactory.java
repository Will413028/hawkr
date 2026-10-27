package io.github.will413028.hawkr.common.obs.grpc;

import io.grpc.Channel;

import java.util.function.Function;

/**
 * Factory for creating gRPC stubs with proper observability configuration.
 * This is the only allowed way to create gRPC stubs in the application.
 */
public final class GrpcStubFactory {

    private final GrpcChannelRegistry registry;

    public GrpcStubFactory(GrpcChannelRegistry registry) {
        this.registry = registry;
    }

    /**
     * Creates a blocking stub for the specified target.
     *
     * @param targetName  the target name as configured in application.yml
     * @param stubFactory the stub factory function (e.g., MediaGrpc::newBlockingStub)
     * @param <T>         the stub type
     * @return the configured blocking stub
     * @throws IllegalArgumentException if target is not configured
     */
    public <T> T blocking(String targetName, Function<Channel, T> stubFactory) {
        Channel channel = registry.getOrCreate(targetName);
        return stubFactory.apply(channel);
    }
}
