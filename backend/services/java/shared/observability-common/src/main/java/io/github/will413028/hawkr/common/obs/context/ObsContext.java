package io.github.will413028.hawkr.common.obs.context;

import io.github.will413028.hawkr.common.obs.util.IdGenerator;
import io.github.will413028.hawkr.common.obs.util.Preconditions;
import org.slf4j.MDC;

/**
 * Manages observability context for the current thread.
 * Handles correlation_id, request_id, and event with support for nested scopes.
 * All values are synchronized to MDC for structured logging.
 */
public final class ObsContext {

    private static final ThreadLocal<ContextState> THREAD_LOCAL = new ThreadLocal<>();

    /**
     * Opens a new observability scope with data from the carrier.
     * If correlationId is missing, generates a new UUID v4.
     * Supports nested scopes - closing restores the previous state.
     *
     * @param carrier the carrier containing observability data
     * @return a scope that must be closed to restore the previous state
     */
    public ObsScope open(ObsCarrier carrier) {
        ContextState prevState = THREAD_LOCAL.get();

        String nextCorrelation = carrier.correlationId();
        if (nextCorrelation == null || nextCorrelation.isBlank()) {
            nextCorrelation = IdGenerator.uuidV4Lower();
        }

        String nextRequest = carrier.requestId();
        if (nextRequest != null && nextRequest.isBlank()) {
            nextRequest = null;
        }

        String nextEvent = (prevState != null) ? prevState.event() : null;

        ContextState nextState = new ContextState(nextCorrelation, nextRequest, nextEvent);
        THREAD_LOCAL.set(nextState);

        syncMdc(nextState);

        return new DefaultObsScope(prevState);
    }

    /**
     * Opens a scope with a specific event name.
     * Does not generate correlation_id - that is only done by open().
     *
     * @param event the event name (must not be blank)
     * @return a scope that must be closed to restore the previous state
     */
    public ObsScope withEvent(String event) {
        Preconditions.requireNotBlank(event, "event must not be blank");

        ContextState prevState = THREAD_LOCAL.get();
        ContextState nextState;

        if (prevState == null) {
            nextState = new ContextState(null, null, event);
        } else {
            nextState = new ContextState(prevState.correlationId(), prevState.requestId(), event);
        }

        THREAD_LOCAL.set(nextState);
        MDC.put(ObsKeys.EVENT, event);

        return new DefaultObsScope(prevState);
    }

    /**
     * Gets the current correlation_id or null if not set.
     *
     * @return correlation_id or null
     */
    public String getCorrelationIdOrNull() {
        ContextState state = THREAD_LOCAL.get();
        return (state != null) ? state.correlationId() : null;
    }

    /**
     * Gets the current request_id or null if not set.
     *
     * @return request_id or null
     */
    public String getRequestIdOrNull() {
        ContextState state = THREAD_LOCAL.get();
        return (state != null) ? state.requestId() : null;
    }

    /**
     * Gets the current event or null if not set.
     *
     * @return event or null
     */
    public String getEventOrNull() {
        ContextState state = THREAD_LOCAL.get();
        return (state != null) ? state.event() : null;
    }

    /**
     * Internal method to force set correlation_id when it's missing in outbound interceptors.
     * This updates both the ThreadLocal state and MDC without creating a new scope.
     *
     * @param correlationId the correlation_id to set
     */
    public void forceSetCorrelationId(String correlationId) {
        ContextState prevState = THREAD_LOCAL.get();
        ContextState nextState;

        if (prevState == null) {
            nextState = new ContextState(correlationId, null, null);
        } else {
            nextState = new ContextState(correlationId, prevState.requestId(), prevState.event());
        }

        THREAD_LOCAL.set(nextState);
        MDC.put(ObsKeys.CORRELATION_ID, correlationId);
    }

    private void syncMdc(ContextState state) {
        if (state.correlationId() != null) {
            MDC.put(ObsKeys.CORRELATION_ID, state.correlationId());
        } else {
            MDC.remove(ObsKeys.CORRELATION_ID);
        }

        if (state.requestId() != null) {
            MDC.put(ObsKeys.REQUEST_ID, state.requestId());
        } else {
            MDC.remove(ObsKeys.REQUEST_ID);
        }

        if (state.event() != null) {
            MDC.put(ObsKeys.EVENT, state.event());
        } else {
            MDC.remove(ObsKeys.EVENT);
        }
    }

    private record ContextState(
            String correlationId,
            String requestId,
            String event
    ) {
    }

    private final class DefaultObsScope implements ObsScope {

        private final ContextState prevState;

        DefaultObsScope(ContextState prevState) {
            this.prevState = prevState;
        }

        @Override
        public void close() {
            THREAD_LOCAL.set(prevState);

            if (prevState != null) {
                syncMdc(prevState);
            } else {
                MDC.remove(ObsKeys.CORRELATION_ID);
                MDC.remove(ObsKeys.REQUEST_ID);
                MDC.remove(ObsKeys.EVENT);
            }
        }
    }
}
