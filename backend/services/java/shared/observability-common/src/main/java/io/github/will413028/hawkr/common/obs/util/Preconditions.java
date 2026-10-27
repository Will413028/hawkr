package io.github.will413028.hawkr.common.obs.util;

/**
 * Utility class for argument validation.
 */
public final class Preconditions {

    private Preconditions() {
    }

    /**
     * Checks that the specified string is not null or blank.
     *
     * @param value   the value to check
     * @param message the exception message if check fails
     * @throws IllegalArgumentException if value is null or blank
     */
    public static void requireNotBlank(String value, String message) {
        if (value == null || value.isBlank()) {
            throw new IllegalArgumentException(message);
        }
    }

    /**
     * Checks that the specified object is not null.
     *
     * @param value   the value to check
     * @param message the exception message if check fails
     * @param <T>     the type of the value
     * @return the value if not null
     * @throws IllegalArgumentException if value is null
     */
    public static <T> T requireNonNull(T value, String message) {
        if (value == null) {
            throw new IllegalArgumentException(message);
        }
        return value;
    }
}
