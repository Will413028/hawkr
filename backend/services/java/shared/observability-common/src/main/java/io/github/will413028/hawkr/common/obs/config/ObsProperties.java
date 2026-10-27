package io.github.will413028.hawkr.common.obs.config;

import org.springframework.boot.context.properties.ConfigurationProperties;

import java.util.HashMap;
import java.util.Map;

/**
 * Configuration properties for observability library.
 * Bound to prefix "hawkr.obs" in application.yml.
 */
@ConfigurationProperties(prefix = "hawkr.obs")
public class ObsProperties {

    /**
     * The canonical service name (required).
     * Example: product-service, media-service
     */
    private String service;

    /**
     * @deprecated No longer used by the library.
     */
    @Deprecated
    private String env;

    /**
     * gRPC configuration.
     */
    private Grpc grpc = new Grpc();

    /**
     * Access log configuration.
     */
    private AccessLog accessLog = new AccessLog();

    public String getService() {
        return service;
    }

    public void setService(String service) {
        this.service = service;
    }

    /**
     * @deprecated No longer used by the library.
     */
    @Deprecated
    public String getEnv() {
        return env;
    }

    /**
     * @deprecated No longer used by the library.
     */
    @Deprecated
    public void setEnv(String env) {
        this.env = env;
    }

    public Grpc getGrpc() {
        return grpc;
    }

    public void setGrpc(Grpc grpc) {
        this.grpc = grpc;
    }

    public AccessLog getAccessLog() {
        return accessLog;
    }

    public void setAccessLog(AccessLog accessLog) {
        this.accessLog = accessLog;
    }

    /**
     * gRPC specific configuration.
     */
    public static class Grpc {

        /**
         * Map of target name to GrpcTarget configuration.
         * Only required if this service acts as a gRPC client.
         */
        private Map<String, GrpcTarget> targets = new HashMap<>();

        public Map<String, GrpcTarget> getTargets() {
            return targets;
        }

        public void setTargets(Map<String, GrpcTarget> targets) {
            this.targets = targets;
        }
    }

    /**
     * Access log configuration for gRPC server.
     * Controls when and how access logs are emitted.
     */
    public static class AccessLog {

        /**
         * Sample rate for OK requests (0.0 to 1.0).
         * - 0.0 = never log OK requests (default)
         * - 0.01 = 1% of OK requests
         * - 1.0 = all OK requests (not recommended for production)
         * ERROR and SLOW requests are always logged regardless of this setting.
         */
        private double okSampleRate = 0.0;

        /**
         * Threshold in milliseconds for slow request detection.
         * Requests exceeding this duration will be logged as WARN.
         * Default: 1000ms (1 second)
         */
        private long slowThresholdMs = 1000;

        public double getOkSampleRate() {
            return okSampleRate;
        }

        public void setOkSampleRate(double okSampleRate) {
            this.okSampleRate = okSampleRate;
        }

        public long getSlowThresholdMs() {
            return slowThresholdMs;
        }

        public void setSlowThresholdMs(long slowThresholdMs) {
            this.slowThresholdMs = slowThresholdMs;
        }
    }
}
