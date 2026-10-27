package io.github.will413028.hawkr.shipping.mq;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;
import org.springframework.amqp.rabbit.annotation.RabbitListener;
import org.springframework.beans.factory.config.BeanDefinition;
import org.springframework.context.annotation.ClassPathScanningCandidateComponentProvider;
import org.springframework.core.type.filter.AnnotationTypeFilter;
import org.springframework.stereotype.Component;

import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Set;
import java.util.TreeSet;

import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * 🔴 <b>本 service 每一條 {@code @RabbitListener} 的 queue，都必須登記在
 * {@code infra/mq/topology/topology.yaml}。</b>
 *
 * <h2>為什麼需要這一支</h2>
 * dev/prod 已切 passive declare（{@code SPRING_RABBITMQ_DYNAMIC=false}）：broker 上沒有的 queue
 * <b>不會讓服務啟動失敗</b>，那筆 subscription 直接靜默失效——不進 DLQ（queue 根本不存在），
 * 也沒有錯誤訊息（10 檔 §10.7）。
 *
 * <p>repo 既有的機制是 {@code scripts/mq-consumer-registration-check.py}，但它<b>只掃 Go</b>
 * （該檔自述「Go 端認得的 queue 名」，三個抽取器全是 Go 語法形狀）。
 * 10 檔 §10.7 逐字：「⚠️ 登記 gate 只掃 Go；<b>Java 漏登記無任何 CI 攔得住</b>，review 必人工查」。
 * 本測試就是 shipping-service 這一塊的那道 gate。
 *
 * <h2>觀測窗與它擋不住的東西（誠實標明邊界）</h2>
 * <ul>
 *   <li>✅ 擋得住：本 service 新增 {@code @RabbitListener} 而忘了加 topology 條目</li>
 *   <li>❌ 擋不住：<b>其他 Java service</b> 的漏登記（各自需要一份；本測試的 scan 範圍是
 *       {@code io.github.will413028.hawkr.shipping}）</li>
 *   <li>❌ 擋不住：queue 名以<b>執行期</b>字串組出來的 listener
 *       （{@code @RabbitListener} 的 {@code queues} 是編譯期常數才讀得到）</li>
 *   <li>❌ 擋不住：binding 的 routing key 對不對——只驗 queue 名存在</li>
 * </ul>
 */
class RabbitListenerTopologyRegistrationTest {

    private static final String SCAN_PACKAGE = "io.github.will413028.hawkr.shipping";

    /** repo root：surefire 的 cwd 是 module 目錄 {@code services/java/shipping-service}。 */
    private static final Path TOPOLOGY =
            Path.of("..", "..", "..", "infra", "mq", "topology", "topology.yaml").normalize();

    @Test
    @DisplayName("每一條 @RabbitListener 的 queue 都登記在 topology.yaml")
    void everyListenerQueueIsRegistered() throws IOException {
        Set<String> declaredQueues = topologyQueueNames();
        // 🔴 對照組：先證明「讀 topology」這件事是有效的。
        //    少了這一條，檔案讀不到／路徑錯／解析失敗都會讓本測試變成恆真。
        assertTrue(declaredQueues.contains("shipping.payuni.webhook"),
                "對照組失敗：topology.yaml 讀不到既有的 shipping.payuni.webhook，"
                        + "本測試的結論全部不可信。解析到的 queue 數=" + declaredQueues.size()
                        + " 路徑=" + TOPOLOGY.toAbsolutePath());

        Set<String> listenerQueues = listenerQueueNames();
        // 🔴 對照組二：先證明「掃 listener」這件事是有效的。
        assertTrue(listenerQueues.contains("shipping.payuni.webhook"),
                "對照組失敗：掃不到既有的 ShippingWebhookConsumer，掃到的=" + listenerQueues);

        Set<String> missing = new TreeSet<>(listenerQueues);
        missing.removeAll(declaredQueues);
        assertTrue(missing.isEmpty(),
                "🔴 下列 queue 有 @RabbitListener 但沒有登記進 infra/mq/topology/topology.yaml："
                        + missing + "。passive declare 下這不會讓服務啟動失敗，"
                        + "而是讓該筆訂閱靜默失效（10 檔 §10.7）。");
    }

    @Test
    @DisplayName("Uber 回調 queue 已登記（本波新增，單獨釘住）")
    void uberWebhookQueueIsRegistered() throws IOException {
        assertTrue(topologyQueueNames().contains("shipping.uber.webhook"),
                "shipping.uber.webhook 未登記 ⇒ UberWebhookConsumer 上線即靜默失效");
        assertTrue(listenerQueueNames().contains("shipping.uber.webhook"),
                "UberWebhookConsumer 沒有掛在 shipping.uber.webhook 上");
    }

    /**
     * 解析 topology.yaml 的 {@code queues:} 區段。
     *
     * <p>刻意不引入 YAML 解析器：本測試要的只是「有沒有這個 name」，而
     * {@code exchanges:} 區段用的是同一個 {@code - name:} 形狀 ⇒ 必須分段，不能全檔 grep。
     */
    private static Set<String> topologyQueueNames() throws IOException {
        List<String> lines = Files.readAllLines(TOPOLOGY, StandardCharsets.UTF_8);
        Set<String> names = new LinkedHashSet<>();
        boolean inQueues = false;
        for (String line : lines) {
            if (line.startsWith("queues:")) {
                inQueues = true;
                continue;
            }
            // 🔴 **註解不是區段結束**（2026-09-17 修）。原本的判準是
            //    「非空白開頭、非 `-` 開頭 ⇒ 離開 queues:」，而頂格的 `#` 註解三個條件全中
            //    ⇒ 解析在第一條頂格註解就停下來，後面的 queue 一個都讀不到。
            //
            //    ⚠️ 這個 bug 的可怕之處在於它**兩個方向都會錯**：
            //      * 要找的 queue 排在截斷點之後 ⇒ 誤報「未登記」（假紅）
            //      * 要找的 queue 排在截斷點之前 ⇒ 照樣通過，而**後面那些根本沒被檢查**（假綠）
            //    本檔的對照組（找得到既有的 shipping.payuni.webhook 嗎）是唯一擋住它的東西
            //    ——它在 Batch A 新增 topology 註解的當下就讓整支測試宣告「結論不可信」。
            //    🔴 那正是對照組存在的理由：**擋不住錯誤，但擋得住「錯誤被當成結論」**。
            if (line.startsWith("#")) {
                continue;
            }
            // 下一個頂層區段開始（非空白開頭、非 "- " 開頭）⇒ 離開 queues:
            if (inQueues && !line.isEmpty() && !line.startsWith(" ") && !line.startsWith("-")) {
                inQueues = false;
            }
            if (inQueues && line.startsWith("- name: ")) {
                names.add(line.substring("- name: ".length()).trim());
            }
        }
        return names;
    }

    private static Set<String> listenerQueueNames() {
        ClassPathScanningCandidateComponentProvider scanner =
                new ClassPathScanningCandidateComponentProvider(false);
        scanner.addIncludeFilter(new AnnotationTypeFilter(Component.class));
        Set<String> queues = new TreeSet<>();
        for (BeanDefinition bd : scanner.findCandidateComponents(SCAN_PACKAGE)) {
            Class<?> clazz;
            try {
                clazz = Class.forName(bd.getBeanClassName());
            } catch (ClassNotFoundException | NoClassDefFoundError e) {
                continue;
            }
            for (var method : clazz.getDeclaredMethods()) {
                RabbitListener rl = method.getAnnotation(RabbitListener.class);
                if (rl != null) {
                    queues.addAll(List.of(rl.queues()));
                }
            }
        }
        return queues;
    }
}
