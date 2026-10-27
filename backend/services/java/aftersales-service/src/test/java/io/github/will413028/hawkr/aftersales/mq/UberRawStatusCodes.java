package io.github.will413028.hawkr.aftersales.mq;

import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.LinkedHashSet;
import java.util.Set;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * Uber 原生狀態字串的<b>唯一來源</b>：shipping-service 的 {@code UberStatusMapper}。
 *
 * <h2>為什麼不手寫這份清單</h2>
 * 手寫 fixture ＝自己跟自己對，必然全綠：漏掉一個原生值，
 * 「aftersales 不得 parse Uber raw code」這條保證就在那個值上完全失效，
 * 而測試不會有任何訊號。
 * ⇒ 清單直接從 <b>擁有它的那份原始碼</b> 解析出來；
 * {@code UberStatusMapper} 搬家或改寫映射表時本類別 fail-loud，不是靜默少掃幾個值。
 *
 * <h2>🔴 觀測窗（這個來源看不到什麼）</h2>
 * 只解析 {@code FORWARD} 映射表的 key 與 {@code RETURN_LEG_EXPLICIT_NO_OP} 集合，
 * 外加顯式列出的容忍值常數。Uber 日後新增一個我方尚未映射的原生值時，
 * 它<b>不會</b>出現在這份清單裡——那是 shipping-service 的未知值處理路徑要管的事，
 * 不是本清單能涵蓋的。
 */
final class UberRawStatusCodes {

    /** shipping-service 的映射表所在——搬家就 fail-loud。 */
    private static final String MAPPER_PATH =
            "services/java/shipping-service/src/main/java/io/github/will413028/hawkr/shipping/provider/uber/UberStatusMapper.java";

    private UberRawStatusCodes() {
    }

    static Path repoRoot() {
        Path p = Path.of("").toAbsolutePath();
        // backend 根目錄以 go.work 為錨：monorepo 裡 .git 在上一層，不是 backend/。
        while (p != null && !Files.exists(p.resolve("go.work"))) {
            p = p.getParent();
        }
        if (p == null) {
            throw new AssertionError("找不到 backend 根目錄（往上沒有 go.work）");
        }
        return p;
    }

    /**
     * 從 {@code UberStatusMapper} 原始碼解析出全部 Uber 原生狀態字串。
     *
     * @throws AssertionError 檔案不存在、或解析不到任何值（＝抽取器失效，不是「清單空了」）
     */
    static Set<String> all() {
        Path src = repoRoot().resolve(MAPPER_PATH);
        if (!Files.isRegularFile(src)) {
            throw new AssertionError("找不到 Uber 原生值的來源檔：" + src
                    + " —— UberStatusMapper 搬家了，本測試的錨要跟著改");
        }
        String code;
        try {
            code = Files.readString(src);
        } catch (IOException e) {
            throw new AssertionError("讀不到 " + src, e);
        }

        Set<String> out = new LinkedHashSet<>();
        // ① FORWARD 表：`"pending", ShippingStatus.UNPROCESSED,` 這一類的 key
        out.addAll(literalsBetween(code,
                "private static final Map<String, ShippingStatus> FORWARD = Map.of(", ");"));
        // ② 逆向 leg 的「明確不處理」集合
        out.addAll(literalsBetween(code,
                "RETURN_LEG_EXPLICIT_NO_OP = Set.of(", ");"));
        // ③ 容忍值常數
        Matcher tolerated = Pattern.compile(
                "TOLERATED_SHOPPING_COMPLETED\\s*=\\s*\"([a-z_]+)\"").matcher(code);
        if (tolerated.find()) {
            out.add(tolerated.group(1));
        }

        if (out.size() < 6) {
            throw new AssertionError("只從 " + MAPPER_PATH + " 解析到 " + out.size()
                    + " 個原生值（" + out + "）—— 抽取器失效，不是「Uber 只有這幾個狀態」");
        }
        return out;
    }

    /** 取 {@code from}…{@code to} 之間的片段，撈出所有 {@code "..."} 字面值。 */
    private static Set<String> literalsBetween(String code, String from, String to) {
        int start = code.indexOf(from);
        if (start < 0) {
            throw new AssertionError("UberStatusMapper 裡找不到錨 " + from + " —— 結構已改，本抽取器要跟著改");
        }
        int end = code.indexOf(to, start);
        if (end < 0) {
            throw new AssertionError("錨 " + from + " 之後找不到結尾 " + to);
        }
        Set<String> out = new LinkedHashSet<>();
        Matcher m = Pattern.compile("\"([a-z_]+)\"").matcher(code.substring(start, end));
        while (m.find()) {
            out.add(m.group(1));
        }
        return out;
    }
}
