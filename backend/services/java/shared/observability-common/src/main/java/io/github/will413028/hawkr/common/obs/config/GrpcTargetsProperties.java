package io.github.will413028.hawkr.common.obs.config;

import java.util.HashMap;
import java.util.Map;

/**
 * Configuration properties for gRPC targets.
 */
public class GrpcTargetsProperties {

    private Map<String, GrpcTarget> targets = new HashMap<>();

    public Map<String, GrpcTarget> getTargets() {
        return targets;
    }

    public void setTargets(Map<String, GrpcTarget> targets) {
        this.targets = targets;
    }
}
