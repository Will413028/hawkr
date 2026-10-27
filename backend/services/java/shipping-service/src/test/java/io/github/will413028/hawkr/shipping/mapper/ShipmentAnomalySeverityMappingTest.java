package io.github.will413028.hawkr.shipping.mapper;

import io.github.will413028.hawkr.common.utils.code.ShipmentAnomalyType;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.ArrayList;
import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * 🔴 severity 的 SQL 映射必須對 {@link ShipmentAnomalyType} <b>窮舉且互斥</b>。
 *
 * <h2>它守的是什麼</h2>
 * severity 決定賣家會不會收到即時推播。一個沒被列進任何一組的 anomaly type，
 * 會落到 {@code CASE_SEVERITY} 的 {@code ELSE} 分支——而那一格<b>看起來完全正常</b>：
 * Dashboard 有資料、查詢不報錯、測試全綠，只是那一類異常的嚴重度是錯的。
 *
 * <h2>🔴 這支測試是被「變異證明過需要」才寫的，不是預防性裝飾</h2>
 * 2026-09-12 獨立驗收時同時注入三個變異：
 * <ol>
 *   <li>從 {@code SEVERITY_HIGH_TYPES} 拿掉一個值</li>
 *   <li>{@code ELSE 'HIGH'} 改回 {@code ELSE 'LOW'}（＝把剛修好的 bug 原樣種回去）</li>
 *   <li>V14 的 {@code CREATE UNIQUE INDEX} 改成 {@code CREATE INDEX}</li>
 * </ol>
 * 三個同時存在，{@code mvn -pl shipping-service test} <b>375 支全綠</b>。
 * ⇒ 依 {@code 03-delegation-templates.md} §0.4 的三選一，這是<b>測試弱（根本沒有測試）</b>，
 * 不是「變異無鑑別力」。
 *
 * <h2>為什麼是這個形狀而不是重寫成 {@code @SelectProvider}</h2>
 * {@code @Select} 需要編譯期常數，所以 SQL 片段與 helper 無法整併成單一 SSOT。
 * 那個限制成立——但它論證的是「不做 {@code @SelectProvider} 重寫」，
 * <b>不是「不做 guard」</b>。錨（三個具名常數）本來就是為了 guard 而拆的。
 */
@DisplayName("ShipmentAnomaly severity 的 SQL 映射")
class ShipmentAnomalySeverityMappingTest {

    /** 把 {@code "'A', 'B'"} 這種 SQL in-list 字面值拆回值清單。 */
    private static List<String> parse(String inList) {
        List<String> out = new ArrayList<>();
        for (String raw : inList.split(",")) {
            String v = raw.trim();
            if (v.startsWith("'") && v.endsWith("'") && v.length() >= 2) {
                out.add(v.substring(1, v.length() - 1));
            } else if (!v.isEmpty()) {
                // 🔴 解析不出來就讓它爆，不要靜默跳過——
                //    靜默跳過會讓「常數格式被改壞」變成這支測試看不見的事。
                throw new IllegalStateException("無法解析 severity in-list 的項目：" + v);
            }
        }
        return out;
    }

    private static List<String> high() {
        return parse(ShipmentAnomalyQueryMapper.SEVERITY_HIGH_TYPES);
    }

    private static List<String> medium() {
        return parse(ShipmentAnomalyQueryMapper.SEVERITY_MEDIUM_TYPES);
    }

    private static List<String> low() {
        return parse(ShipmentAnomalyQueryMapper.SEVERITY_LOW_TYPES);
    }

    @Test
    @DisplayName("🔴 每一個 ShipmentAnomalyType 都必須恰好落在一組（窮舉 ＋ 互斥）")
    void everyTypeIsMappedExactlyOnce() {
        List<String> h = high();
        List<String> m = medium();
        List<String> l = low();
        List<String> missing = new ArrayList<>();
        List<String> duplicated = new ArrayList<>();

        for (ShipmentAnomalyType t : ShipmentAnomalyType.values()) {
            String v = t.value();
            int n = (h.contains(v) ? 1 : 0) + (m.contains(v) ? 1 : 0) + (l.contains(v) ? 1 : 0);
            if (n == 0) {
                missing.add(v);
            } else if (n > 1) {
                duplicated.add(v);
            }
        }

        assertThat(missing)
                .as("🔴 沒被列進任何一組的值會落到 CASE_SEVERITY 的 ELSE 分支——"
                        + "那一格看起來完全正常（有資料、不報錯、測試綠），只是嚴重度是錯的。"
                        + "新增 ShipmentAnomalyType 時要同步 SEVERITY_*_TYPES 三個常數之一。")
                .isEmpty();
        assertThat(duplicated)
                .as("同一個值出現在兩組 ⇒ CASE 的分支順序決定結果，而那個順序不是任何人刻意設計的")
                .isEmpty();
    }

    @Test
    @DisplayName("三組的聯集大小 = enum 值域大小（擋住「多列了一個不存在的值」）")
    void noPhantomValues() {
        List<String> all = new ArrayList<>();
        all.addAll(high());
        all.addAll(medium());
        all.addAll(low());

        List<String> known = new ArrayList<>();
        for (ShipmentAnomalyType t : ShipmentAnomalyType.values()) {
            known.add(t.value());
        }

        assertThat(known)
                .as("三組列到的值必須全部是真實存在的 enum 值——"
                        + "打錯字的值在 SQL 裡永遠不會命中，而且沒有任何東西會報錯")
                .containsAll(all);
        assertThat(all).hasSize(known.size());
    }

    @Test
    @DisplayName("🔴 ELSE 分支必須是 HIGH 不是 LOW（fail-loud 而不是 fail-silent）")
    void elseBranchFailsLoud() {
        // 值域理論上已窮舉，所以 ELSE 走不到。但「走不到」是這支測試保證的，
        // 而保證會被繞過（有人加了值卻沒跑測試就 merge）。
        // ⇒ ELSE 的值決定了那個情況下的後果：LOW 讓賣家收不到推播（靜默），
        //    HIGH 讓它吵（可見）。在分不出來的時候，吵比靜默安全。
        assertThat(ShipmentAnomalyQueryMapper.CASE_SEVERITY)
                .as("ELSE 用 LOW 收尾 ＝ 未知值靜默降級，正是本波要修掉的形狀")
                .contains("ELSE 'HIGH'")
                .doesNotContain("ELSE 'LOW'");
    }

    @Test
    @DisplayName("Uber 四值的嚴重度符合裁決原則（即時配送、需立即介入者為 HIGH）")
    void uberValuesFollowTheDecision() {
        List<String> h = high();
        List<String> l = low();
        // 裁決第 17 條：需要營運／Seller 立即介入、等到摘要才處理就失去處理價值的 ⇒ HIGH；
        // 純資訊型、不需人立即介入 ⇒ LOW。
        assertThat(h).contains("UBER_COURIER_UNASSIGNED", "UBER_PICKUP_STUCK", "UBER_CREATE_UNKNOWN_UNRESOLVED");
        assertThat(l).contains("UBER_POD_MISSING");
    }
}
