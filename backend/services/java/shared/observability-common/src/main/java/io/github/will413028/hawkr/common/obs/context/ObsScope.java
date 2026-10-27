package io.github.will413028.hawkr.common.obs.context;

/**
 * Represents a scoped context that must be closed to restore the previous state.
 * Implements AutoCloseable for try-with-resources pattern.
 */
public interface ObsScope extends AutoCloseable {

    /**
     * Closes this scope and restores the previous context state.
     * Also restores MDC to match the restored state.
     */
    @Override
    void close();
}
