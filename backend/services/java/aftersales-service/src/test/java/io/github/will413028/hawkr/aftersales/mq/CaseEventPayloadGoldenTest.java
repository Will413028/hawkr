package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.aftersales.mq.payload.AppealSubmittedPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.CaseAutoEscalatedPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.CaseAutoRejectedPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.CaseTransitionPayload;
import io.github.will413028.hawkr.aftersales.mq.payload.PlatformDecisionMadePayload;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.params.ParameterizedTest;
import org.junit.jupiter.params.provider.Arguments;
import org.junit.jupiter.params.provider.MethodSource;

import java.io.IOException;
import java.util.Map;
import java.util.Set;
import java.util.stream.Stream;

import static org.junit.jupiter.api.Assertions.assertTrue;

class CaseEventPayloadGoldenTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    static Stream<Arguments> goldenCases() {
        return Stream.of(
                Arguments.of("aftersales.case.applied.buyer",
                        keysOf(new CaseTransitionPayload(
                                "CASE1", "ORD1", "9001", "42", "7001", "5001",
                                "buyer", "2026-08-05T00:00:00", null))),
                Arguments.of("aftersales.case.applied.seller",
                        keysOf(new CaseTransitionPayload(
                                "CASE1", "ORD1", "9001", "42", "7001", "5001",
                                "buyer", "2026-08-05T00:00:00", null))),
                Arguments.of("aftersales.case.auto_rejected.buyer",
                        keysOf(new CaseAutoRejectedPayload(
                                "CASE1", "ORD1", "9001", "42", "7001",
                                "48", "system", "PROCESS_TIMEOUT", "2026-08-05T00:00:00"))),
                Arguments.of("aftersales.case.auto_escalated.buyer",
                        keysOf(new CaseAutoEscalatedPayload(
                                "CASE1", "ORD1", "9001", "42", "7001", "5001",
                                "48", "system", "PROCESS_TIMEOUT", "2026-08-05T00:00:00"))),
                Arguments.of("aftersales.case.auto_escalated.seller",
                        keysOf(new CaseAutoEscalatedPayload(
                                "CASE1", "ORD1", "9001", "42", "7001", "5001",
                                "48", "system", "PROCESS_TIMEOUT", "2026-08-05T00:00:00"))),
                Arguments.of("aftersales.appeal.submitted.buyer",
                        keysOf(new AppealSubmittedPayload(
                                "CASE1", "ORD1", "9001", "42", "7001", "5001",
                                "reason", "2026-08-05T00:00:00"))),
                Arguments.of("aftersales.appeal.submitted.seller",
                        keysOf(new AppealSubmittedPayload(
                                "CASE1", "ORD1", "9001", "42", "7001", "5001",
                                "reason", "2026-08-05T00:00:00"))),
                Arguments.of("aftersales.platform_decision.made.buyer",
                        keysOf(new PlatformDecisionMadePayload(
                                "CASE1", "ORD1", "9001", "42", "7001", "5001",
                                "REFUND", "1000", "2026-08-05T00:00:00"))),
                Arguments.of("aftersales.platform_decision.made.seller",
                        keysOf(new PlatformDecisionMadePayload(
                                "CASE1", "ORD1", "9001", "42", "7001", "5001",
                                "REFUND", "1000", "2026-08-05T00:00:00"))));
    }

    @ParameterizedTest(name = "{0}")
    @MethodSource("goldenCases")
    void serializedKeys_containAllGoldenKeys(String eventType, Set<String> actualKeys) throws IOException {
        Set<String> goldenKeys = GoldenPayloadKeys.emittedKeys(eventType);
        assertTrue(actualKeys.containsAll(goldenKeys),
                eventType + " 的 golden 宣告了 payload 根本不會序列化出來的 key（publisher↔golden 漂移）"
                        + ", golden=" + goldenKeys + ", actual=" + actualKeys);
    }

    @SuppressWarnings("unchecked")
    private static Set<String> keysOf(Object payload) {
        try {
            byte[] json = MAPPER.writeValueAsBytes(payload);
            return ((Map<String, Object>) MAPPER.readValue(json, Map.class)).keySet();
        } catch (IOException e) {
            throw new IllegalStateException("payload 序列化失敗: " + payload.getClass().getName(), e);
        }
    }
}
