package io.github.will413028.hawkr.aftersales.mq;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.File;
import java.io.IOException;
import java.util.LinkedHashSet;
import java.util.Set;

/**
 * 測試工具：讀 repo 根的 {@code tests/contract/golden/event_payload_keys.json}，
 * 取出指定 event_type 的 {@code emitted_keys}。
 *
 * <p>issue #1626：golden guardrail 的重點是「golden 檔本身」要對著真實 payload
 * serializer 自我驗證——所以期望值必須從檔案讀進來，不能在測試裡手抄成
 * {@code Set.of(...)} 常數（手抄等於把 golden 複製一份，golden 寫錯時測試照樣綠，
 * 正是 #1626 潛伏一年沒被抓到的破口）。
 *
 * <p>檔案定位：從 JVM 工作目錄（surefire 預設 = module basedir，
 * {@code services/java/aftersales-service}）往上走，找第一個含
 * {@code tests/contract/golden/event_payload_keys.json} 的目錄。不寫死相對層數，
 * 以免日後 module 搬家或 IDE 用不同工作目錄時默默失效。
 */
final class GoldenPayloadKeys {

    private static final String GOLDEN_RELATIVE_PATH = "tests/contract/golden/event_payload_keys.json";

    private GoldenPayloadKeys() {
    }

    /**
     * @param eventType golden 檔的頂層 key，例如 {@code aftersales.refund.completed.seller}
     * @return 該 event_type 宣告的 emitted_keys（保序）
     */
    static Set<String> emittedKeys(String eventType) throws IOException {
        File golden = goldenFile();
        JsonNode root = new ObjectMapper().readTree(golden);
        JsonNode entry = root.get(eventType);
        if (entry == null) {
            throw new IllegalStateException(
                    "golden 檔找不到 event_type=" + eventType + ", file=" + golden.getAbsolutePath());
        }
        JsonNode emitted = entry.get("emitted_keys");
        // 空陣列會讓 containsAll 恆真（vacuous pass），必須 fail-loud 而非默默放行
        if (emitted == null || !emitted.isArray() || emitted.isEmpty()) {
            throw new IllegalStateException("golden 檔的 " + eventType
                    + ".emitted_keys 缺失或為空, file=" + golden.getAbsolutePath());
        }
        Set<String> keys = new LinkedHashSet<>();
        emitted.forEach(node -> keys.add(node.asText()));
        return keys;
    }

    private static File goldenFile() {
        File dir = new File("").getAbsoluteFile();
        while (dir != null) {
            File candidate = new File(dir, GOLDEN_RELATIVE_PATH);
            if (candidate.isFile()) {
                return candidate;
            }
            dir = dir.getParentFile();
        }
        throw new IllegalStateException("往上找不到 " + GOLDEN_RELATIVE_PATH
                + "，工作目錄=" + new File("").getAbsolutePath());
    }
}
