package io.github.will413028.hawkr.common.mq;

import lombok.AllArgsConstructor;
import lombok.Data;
import lombok.NoArgsConstructor;
import lombok.experimental.SuperBuilder;

import java.time.Instant;
import java.util.HashMap;
import java.util.Map;
import java.util.UUID;

/**
 * Base class for all MQ events
 */
@Data
@SuperBuilder
@NoArgsConstructor
@AllArgsConstructor
public class BaseEvent {

    private String eventId;
    private String eventType;
    private String version;
    private Instant timestamp;
    private String source;
    private String correlationId;
    private Map<String, String> metadata;

    /**
     * Initialize common fields for a new event
     */
    public void initializeEvent(String eventType, String source) {
        this.eventId = UUID.randomUUID().toString();
        this.eventType = eventType;
        this.version = "1.0";
        this.timestamp = Instant.now();
        this.source = source;
        this.metadata = new HashMap<>();
    }

    /**
     * Create a new event with common fields initialized
     */
    public static BaseEvent create(String eventType, String source) {
        return BaseEvent.builder()
                .eventId(UUID.randomUUID().toString())
                .eventType(eventType)
                .version("1.0")
                .timestamp(Instant.now())
                .source(source)
                .metadata(new HashMap<>())
                .build();
    }
}
