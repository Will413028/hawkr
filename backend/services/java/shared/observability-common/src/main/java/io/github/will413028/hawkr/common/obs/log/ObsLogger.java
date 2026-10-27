package io.github.will413028.hawkr.common.obs.log;

import io.github.will413028.hawkr.common.obs.context.ObsKeys;
import io.github.will413028.hawkr.common.obs.util.Preconditions;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;
import org.slf4j.MDC;

/**
 * Structured logger for observability with auto-detected className and methodName.
 *
 * <p>Event format: {@code <ClassName>.<methodName>.<result>}
 * - ClassName: auto-detected from forClass()
 * - methodName: auto-detected from call stack (empty if not in a method)
 * - result: user-provided (optional, can be empty)
 *
 * <p>Usage:
 * <pre>{@code
 * private static final ObsLogger log = ObsLogger.forClass(UserGrpcService.class);
 *
 * public void login() {
 *     // event = "UserGrpcService.login.success"
 *     log.info("success", "Login completed");
 *
 *     // event = "UserGrpcService.login" (no result)
 *     log.info("", "Processing login");
 *
 *     // With business context
 *     log.info("success", "Login completed", "user_id", "USR-12345");
 *
 *     // With exception
 *     log.error("failed", "Login failed", exception);
 * }
 * }</pre>
 *
 * <p>For Lombok users, add to lombok.config:
 * <pre>
 * lombok.log.custom.declaration = io.github.will413028.hawkr.common.obs.log.ObsLogger io.github.will413028.hawkr.common.obs.log.ObsLogger.forClass(TYPE)
 * </pre>
 * Then use: {@code @CustomLog}
 */
public final class ObsLogger {

    private final Logger logger;
    private final String className;

    private ObsLogger(Logger logger, String className) {
        this.logger = logger;
        this.className = className;
    }

    /**
     * Creates an ObsLogger for the specified class.
     * The simple class name and calling method name will be auto-detected for event names.
     *
     * @param clazz the class to create logger for
     * @return ObsLogger instance
     */
    public static ObsLogger forClass(Class<?> clazz) {
        return new ObsLogger(LoggerFactory.getLogger(clazz), clazz.getSimpleName());
    }

    /**
     * Creates an ObsLogger with the specified name (no auto-detection).
     *
     * @param name the logger name
     * @return ObsLogger instance
     */
    public static ObsLogger forName(String name) {
        return new ObsLogger(LoggerFactory.getLogger(name), null);
    }

    /**
     * Gets the calling method name from the stack trace.
     * Returns empty string if not determinable.
     */
    private String getCallerMethodName() {
        StackTraceElement[] stack = Thread.currentThread().getStackTrace();
        // stack[0] = getStackTrace
        // stack[1] = getCallerMethodName
        // stack[2] = buildEvent
        // stack[3] = info/warn/error
        // stack[4] = actual caller
        if (stack.length > 4) {
            String methodName = stack[4].getMethodName();
            // Filter out lambda and synthetic methods
            if (methodName != null && !methodName.startsWith("lambda$") && !methodName.contains("$")) {
                return methodName;
            }
        }
        return "";
    }

    /**
     * Builds the full event name: ClassName.methodName.result
     * - If className is null, returns just result (or empty)
     * - If methodName is empty, returns ClassName.result (or just ClassName)
     * - If result is empty, returns ClassName.methodName (or just ClassName)
     */
    private String buildEvent(String result) {
        String methodName = getCallerMethodName();

        StringBuilder sb = new StringBuilder();

        // Add className
        if (className != null && !className.isEmpty()) {
            sb.append(className);
        }

        // Add methodName
        if (!methodName.isEmpty()) {
            if (sb.length() > 0) sb.append(".");
            sb.append(methodName);
        }

        // Add result
        if (result != null && !result.isEmpty()) {
            if (sb.length() > 0) sb.append(".");
            sb.append(result);
        }

        return sb.length() > 0 ? sb.toString() : "unknown";
    }

    // ==================== INFO ====================

    /**
     * Logs at INFO level.
     *
     * @param result the result (e.g., "success", "failed"), can be empty
     * @param msg    the log message (required, can be empty)
     */
    public void info(String result, String msg) {
        Preconditions.requireNonNull(result, "result must not be null");
        Preconditions.requireNonNull(msg, "msg must not be null");

        try {
            MDC.put(ObsKeys.EVENT, buildEvent(result));
            logger.info(msg);
        } finally {
            MDC.remove(ObsKeys.EVENT);
        }
    }

    /**
     * Logs at INFO level with business context.
     *
     * @param result  the result (e.g., "success"), can be empty
     * @param msg     the log message (required, can be empty)
     * @param bzCol   the business context field name (e.g., "order_id", "user_id")
     * @param bzValue the business context field value
     */
    public void info(String result, String msg, String bzCol, String bzValue) {
        Preconditions.requireNonNull(result, "result must not be null");
        Preconditions.requireNonNull(msg, "msg must not be null");
        Preconditions.requireNotBlank(bzCol, "bzCol must not be blank");

        try {
            MDC.put(ObsKeys.EVENT, buildEvent(result));
            if (bzValue != null) {
                MDC.put(bzCol, bzValue);
            }
            logger.info(msg);
        } finally {
            MDC.remove(ObsKeys.EVENT);
            MDC.remove(bzCol);
        }
    }

    // ==================== WARN ====================

    /**
     * Logs at WARN level.
     *
     * @param result the result (e.g., "slow", "timeout"), can be empty
     * @param msg    the log message (required, can be empty)
     */
    public void warn(String result, String msg) {
        Preconditions.requireNonNull(result, "result must not be null");
        Preconditions.requireNonNull(msg, "msg must not be null");

        try {
            MDC.put(ObsKeys.EVENT, buildEvent(result));
            logger.warn(msg);
        } finally {
            MDC.remove(ObsKeys.EVENT);
        }
    }

    /**
     * Logs at WARN level with business context.
     *
     * @param result  the result (e.g., "slow"), can be empty
     * @param msg     the log message (required, can be empty)
     * @param bzCol   the business context field name
     * @param bzValue the business context field value
     */
    public void warn(String result, String msg, String bzCol, String bzValue) {
        Preconditions.requireNonNull(result, "result must not be null");
        Preconditions.requireNonNull(msg, "msg must not be null");
        Preconditions.requireNotBlank(bzCol, "bzCol must not be blank");

        try {
            MDC.put(ObsKeys.EVENT, buildEvent(result));
            if (bzValue != null) {
                MDC.put(bzCol, bzValue);
            }
            logger.warn(msg);
        } finally {
            MDC.remove(ObsKeys.EVENT);
            MDC.remove(bzCol);
        }
    }

    // ==================== ERROR ====================

    /**
     * Logs at ERROR level.
     *
     * @param result the result (e.g., "failed", "error"), can be empty
     * @param msg    the log message (required, can be empty)
     */
    public void error(String result, String msg) {
        Preconditions.requireNonNull(result, "result must not be null");
        Preconditions.requireNonNull(msg, "msg must not be null");

        try {
            MDC.put(ObsKeys.EVENT, buildEvent(result));
            logger.error(msg);
        } finally {
            MDC.remove(ObsKeys.EVENT);
        }
    }

    /**
     * Logs at ERROR level with business context.
     *
     * @param result  the result (e.g., "failed"), can be empty
     * @param msg     the log message (required, can be empty)
     * @param bzCol   the business context field name
     * @param bzValue the business context field value
     */
    public void error(String result, String msg, String bzCol, String bzValue) {
        Preconditions.requireNonNull(result, "result must not be null");
        Preconditions.requireNonNull(msg, "msg must not be null");
        Preconditions.requireNotBlank(bzCol, "bzCol must not be blank");

        try {
            MDC.put(ObsKeys.EVENT, buildEvent(result));
            if (bzValue != null) {
                MDC.put(bzCol, bzValue);
            }
            logger.error(msg);
        } finally {
            MDC.remove(ObsKeys.EVENT);
            MDC.remove(bzCol);
        }
    }

    /**
     * Logs at ERROR level with exception.
     *
     * @param result the result (e.g., "failed"), can be empty
     * @param msg    the log message (required, can be empty)
     * @param ex     the exception to log
     */
    public void error(String result, String msg, Throwable ex) {
        Preconditions.requireNonNull(result, "result must not be null");
        Preconditions.requireNonNull(msg, "msg must not be null");

        try {
            MDC.put(ObsKeys.EVENT, buildEvent(result));
            logger.error(msg, ex);
        } finally {
            MDC.remove(ObsKeys.EVENT);
        }
    }

    /**
     * Logs at ERROR level with business context and exception.
     *
     * @param result  the result (e.g., "failed"), can be empty
     * @param msg     the log message (required, can be empty)
     * @param bzCol   the business context field name
     * @param bzValue the business context field value
     * @param ex      the exception to log
     */
    public void error(String result, String msg, String bzCol, String bzValue, Throwable ex) {
        Preconditions.requireNonNull(result, "result must not be null");
        Preconditions.requireNonNull(msg, "msg must not be null");
        Preconditions.requireNotBlank(bzCol, "bzCol must not be blank");

        try {
            MDC.put(ObsKeys.EVENT, buildEvent(result));
            if (bzValue != null) {
                MDC.put(bzCol, bzValue);
            }
            logger.error(msg, ex);
        } finally {
            MDC.remove(ObsKeys.EVENT);
            MDC.remove(bzCol);
        }
    }
}
