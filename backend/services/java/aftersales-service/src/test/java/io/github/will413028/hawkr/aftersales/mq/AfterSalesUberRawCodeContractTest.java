package io.github.will413028.hawkr.aftersales.mq;

import io.github.will413028.hawkr.common.utils.code.ShippingStatus;
import io.github.will413028.hawkr.common.utils.code.helper.ReturnShippingRules;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.stream.Stream;

import static org.assertj.core.api.Assertions.assertThat;
import static org.junit.jupiter.api.Assertions.assertThrows;

/**
 * 🔴 W5-4 的<b>陰性</b>保證：aftersales 只依 normalized {@link ShippingStatus} 與
 * {@link ReturnShippingRules} 判定，<b>不得 parse Uber raw code</b>。
 *
 * <p>既有的 {@code ReturnShipmentStatusConsumerTest} 驗的是「正規化之後分得對」，
 * 那是<b>陽性</b>側；它對「有人偷偷加了一條 {@code case "pickup_complete"}」完全沒有鑑別力。
 * 本類別補的就是那一半。
 *
 * <h2>🔴 觀測窗（本 gate 擋不住什麼，照 §3.10 誠實標明）</h2>
 * <ul>
 *   <li>只掃 {@code aftersales-service/src/main/java} 的 <b>字串字面值</b>，
 *       且比對方式是<b>完全相等</b>。{@code raw.startsWith("pickup_")} 這種
 *       部分比對、或從設定／常數組出來的字串，本 gate 看不到。</li>
 *   <li>不掃測試碼（測試裡出現 {@code pickup_complete} 是在描述被禁的東西，不是在用它）。</li>
 *   <li>不掃別的 module——shipping-service 本來就該認得這些值，那是它的職責。</li>
 * </ul>
 * 一道誠實標明邊界的 gate 是資產；一道被當成牆的 gate 是負債。
 */
class AfterSalesUberRawCodeContractTest {

    /** aftersales 的 production 原始碼根目錄。 */
    private static final String MAIN_SRC = "services/java/aftersales-service/src/main/java";

    /**
     * ① Uber 原生值<b>不在</b> normalized 值域裡——所以它根本到不了 aftersales。
     *
     * <p>這條是整個 ADR-013 的前提：Uber 的原生值在 shipping-service 就被轉成
     * {@code 930}／{@code 933}，逆向 leg 收到的永遠是數字碼。前提若被打破
     * （某天有人把原生值加進 {@code ShippingStatus}），本測試立刻轉紅，
     * 因為那時「不得 parse raw code」這句話的意思就變了。
     */
    @Test
    @DisplayName("① Uber 原生值不在 ShippingStatus 值域內 ⇒ fromValue 必須拋例外")
    void uberRawCodesAreOutsideTheNormalizedDomain() {
        Set<String> raw = UberRawStatusCodes.all();
        assertThat(raw)
                .as("從 UberStatusMapper 解析到的原生值")
                .contains("pickup_complete", "dropoff", "delivered", "canceled", "returned");

        for (String code : raw) {
            assertThrows(IllegalArgumentException.class,
                    () -> ShippingStatus.fromValue(code),
                    () -> "Uber 原生值 " + code + " 竟然是合法的 normalized ShippingStatus —— "
                            + "ADR-013 的前提被打破了");
        }

        // 對照組：同一支方法對一個**確知存在**的值必須回得來。
        // 少了它，上面那圈「全部都拋例外」也可能只是因為 fromValue 壞了。
        assertThat(ShippingStatus.fromValue(ShippingStatus.HANDED_OVER.value()))
                .as("對照組：fromValue 對合法值必須回得來，否則上面的否定結論沒有意義")
                .isEqualTo(ShippingStatus.HANDED_OVER);
    }

    /**
     * ② aftersales production 原始碼裡不得出現任何 Uber 原生值的字串字面值。
     *
     * <p>鑑別力來自兩件事：掃描器<b>真的看到了檔案</b>（下面的對照組），
     * 而且<b>真的剝掉了註解</b>——{@code ReturnShipmentStatusConsumer} 的註解裡就寫著
     * 「Uber 逆向 canceled」，剝不乾淨的話這條會直接誤報。
     */
    @Test
    @DisplayName("② aftersales production 原始碼零 Uber 原生值字面值")
    void aftersalesMainSourcesContainNoUberRawCodeLiteral() throws IOException {
        Set<String> raw = UberRawStatusCodes.all();
        Path root = UberRawStatusCodes.repoRoot().resolve(MAIN_SRC);
        assertThat(Files.isDirectory(root)).as("掃描根目錄 %s 必須存在", root).isTrue();

        Map<String, List<String>> offenders = new LinkedHashMap<>();
        int scannedFiles = 0;
        int totalLiterals = 0;
        boolean sawKnownLiteral = false;

        List<Path> files;
        try (Stream<Path> s = Files.walk(root)) {
            files = s.filter(Files::isRegularFile)
                    .filter(p -> p.toString().endsWith(".java"))
                    .sorted()
                    .toList();
        }
        for (Path f : files) {
            scannedFiles++;
            List<String> literals = stringLiterals(Files.readString(f));
            totalLiterals += literals.size();
            if (literals.contains("is_return")) {
                // ReturnShipmentStatusConsumer 解析 payload 用的 key，確知存在。
                sawKnownLiteral = true;
            }
            for (String lit : literals) {
                if (raw.contains(lit)) {
                    offenders.computeIfAbsent(root.relativize(f).toString(), k -> new ArrayList<>()).add(lit);
                }
            }
        }

        // 🔴 對照組三連：掃描範圍是空的時候，上面的「零命中」毫無意義。
        assertThat(scannedFiles).as("掃到的 .java 檔數").isGreaterThan(50);
        assertThat(totalLiterals).as("抽到的字串字面值總數").isGreaterThan(100);
        assertThat(sawKnownLiteral)
                .as("抽取器必須抽得到一個確知存在的字面值 \"is_return\"（ReturnShipmentStatusConsumer）"
                        + " —— 抽不到代表抽取器壞了，而不是「沒有 Uber 原生值」")
                .isTrue();

        assertThat(offenders)
                .as("aftersales 不得比對 Uber 原生狀態字串；一律走 normalized ShippingStatus "
                        + "＋ ReturnShippingRules（ADR-013）。命中：%s", offenders)
                .isEmpty();
    }

    /**
     * ③ 逆向 leg 的分類必須全部來自 {@link ReturnShippingRules}，而它是 total function：
     * 每一個 {@link ShippingStatus} 都有處置，沒有 default 落空。
     *
     * <p>這是「不得 parse raw code」的正面配套：既然不准自己判，就必須有人把每一個值都判掉。
     */
    @Test
    @DisplayName("③ ReturnShippingRules 對每一個 ShippingStatus 都給得出處置")
    void returnShippingRulesIsTotalOverTheNormalizedDomain() {
        ShippingStatus[] all = ShippingStatus.values();
        assertThat(all.length).as("ShippingStatus 值域大小").isGreaterThan(20);
        for (ShippingStatus s : all) {
            assertThat(ReturnShippingRules.dispositionOf(s))
                    .as("ShippingStatus.%s 沒有對應的逆向處置", s)
                    .isNotNull();
        }
    }

    // ==================== 字串字面值抽取（狀態機，不用正則） ====================

    /**
     * 抽出一份 Java 原始碼裡的所有字串字面值。
     *
     * <p>🔴 <b>刻意用字元狀態機而不是正則</b>：正則版的 {@code stripComments} 有一個已知的
     * 壞法——{@code //} 註解裡出現 {@code /*} 會開出一個假的區塊註解，把後面大半個檔案吃掉，
     * 而結果是「零命中」＝看起來完全正常。狀態機把 code／行註解／區塊註解／字串／字元／
     * text block 五種脈絡分開，不會有那種失明。
     */
    private static List<String> stringLiterals(String src) {
        List<String> out = new ArrayList<>();
        StringBuilder cur = new StringBuilder();
        int i = 0;
        int n = src.length();
        // 0=code 1=line comment 2=block comment 3=string 4=char 5=text block
        int state = 0;
        while (i < n) {
            char c = src.charAt(i);
            char next = i + 1 < n ? src.charAt(i + 1) : '\0';
            switch (state) {
                case 0 -> {
                    if (c == '/' && next == '/') {
                        state = 1;
                        i += 2;
                        continue;
                    }
                    if (c == '/' && next == '*') {
                        state = 2;
                        i += 2;
                        continue;
                    }
                    if (c == '"' && next == '"' && i + 2 < n && src.charAt(i + 2) == '"') {
                        state = 5;
                        i += 3;
                        continue;
                    }
                    if (c == '"') {
                        state = 3;
                        cur.setLength(0);
                        i++;
                        continue;
                    }
                    if (c == '\'') {
                        state = 4;
                        i++;
                        continue;
                    }
                }
                case 1 -> {
                    if (c == '\n') {
                        state = 0;
                    }
                }
                case 2 -> {
                    if (c == '*' && next == '/') {
                        state = 0;
                        i += 2;
                        continue;
                    }
                }
                case 3 -> {
                    if (c == '\\') {
                        i += 2;
                        continue;
                    }
                    if (c == '"') {
                        out.add(cur.toString());
                        state = 0;
                        i++;
                        continue;
                    }
                    cur.append(c);
                }
                case 4 -> {
                    if (c == '\\') {
                        i += 2;
                        continue;
                    }
                    if (c == '\'') {
                        state = 0;
                    }
                }
                case 5 -> {
                    // text block 整段當一個字面值捨棄：它裝的是多行文字（SQL／JSON），
                    // 不會是「一個 Uber 原生狀態碼」那種單值比對。
                    if (c == '"' && next == '"' && i + 2 < n && src.charAt(i + 2) == '"') {
                        state = 0;
                        i += 3;
                        continue;
                    }
                }
                default -> throw new IllegalStateException("unreachable state " + state);
            }
            i++;
        }
        return out;
    }
}
