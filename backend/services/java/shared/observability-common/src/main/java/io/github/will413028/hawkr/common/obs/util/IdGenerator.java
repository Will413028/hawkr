package io.github.will413028.hawkr.common.obs.util;

import java.util.UUID;

/**
 * Utility class for generating unique identifiers.
 */
public final class IdGenerator {

    private IdGenerator() {
    }

    /**
     * Generates a UUID v4 in lowercase format.
     *
     * @return lowercase UUID v4 string
     */
    public static String uuidV4Lower() {
        return UUID.randomUUID().toString().toLowerCase();
    }
}
