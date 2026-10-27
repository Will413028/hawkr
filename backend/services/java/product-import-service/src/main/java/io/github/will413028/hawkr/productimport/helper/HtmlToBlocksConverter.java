package io.github.will413028.hawkr.productimport.helper;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.node.ArrayNode;
import com.fasterxml.jackson.databind.node.ObjectNode;
import org.jsoup.Jsoup;
import org.jsoup.nodes.Document;
import org.jsoup.nodes.Element;
import org.jsoup.nodes.Node;
import org.jsoup.nodes.TextNode;
import org.jsoup.safety.Safelist;
import org.springframework.stereotype.Component;

import java.nio.charset.StandardCharsets;
import java.util.ArrayList;
import java.util.List;
import java.util.regex.Matcher;
import java.util.regex.Pattern;

/**
 * HTML → ProductDescription.blocks 轉換器（40 號設計書 §12 / §12.2 / §12.3 / §12.3a）。
 *
 * <p>inline-run 模型（issue #1255 表格 / #1256 行內連結）：
 * <ul>
 *   <li>text block：內容為 {@code runs[]}，遇 inline 樣式 / 連結切換在同一 block 內切 run（保段落完整）</li>
 *   <li>inline：{@code b/strong→bold}、{@code i/em→italic}、{@code u/ins→underline}、
 *       {@code span/font color→color(#RRGGBB)}、{@code a href(http/https)→href}</li>
 *   <li>標題：{@code h1/h2→fontSize:26}、{@code h3~h6→fontSize:20}、{@code fontWeight:700}（段落級）</li>
 *   <li>表格：{@code <table>} → {@code table} block（{@code tableJson}，cell 內為 runs）；上限 欄10/列50/cell200/20KB</li>
 *   <li>清單：{@code <ul>/<ol>} → {@code list} block（{@code listJson}，item 內為 runs；ordered 依 ol/ul）；
 *       巢狀清單攤平為單層（warning list_nested_flattened）；上限 項目50/單項200/20KB（warning list_truncated）</li>
 *   <li>圖片：{@code img/picture} 取 src，剝除已知 CDN suffix</li>
 *   <li>媒體丟棄：{@code iframe/video/audio/embed}；HTML 先過 jsoup Safelist 防 XSS</li>
 * </ul>
 *
 * <p>輸出 {@link Result} 含 blocks JsonNode + 抽出 image URLs + warnings。
 */
@Component
public class HtmlToBlocksConverter {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    // §12.2 標題 → 離散字級 px 字串（h1/h2→26px 大標；h3~h6→20px 小標；對齊 web BlockTextStyle）
    private static String headingFontSize(String tag) {
        return switch (tag) {
            case "h1", "h2" -> "26px";
            case "h3", "h4", "h5", "h6" -> "20px";
            default -> null;
        };
    }

    // 註：<li> 不在此列 — 由所屬 <ul>/<ol> 讀成 list block 項目（handleList），非 text block 邊界（40 §12.2a / §12.3b）
    private static final java.util.Set<String> BLOCK_LEVEL_TAGS = java.util.Set.of(
            "p", "div", "blockquote", "pre",
            "section", "article", "aside", "header", "footer", "main"
    );

    private static final java.util.Set<String> INLINE_TAGS = java.util.Set.of(
            "strong", "b", "em", "i", "u", "ins", "span", "font", "small", "mark", "a"
    );

    private static final java.util.Set<String> MEDIA_DROP_TAGS = java.util.Set.of(
            "iframe", "video", "audio", "embed", "object"
    );

    // §12.4 已知 CDN 後綴剝除 patterns
    private static final List<String> CDN_SUFFIX_PATTERNS = List.of(
            "@resize_w\\d+(_nl)?(\\.webp)?",
            "\\?x-oss-process=[^&]*",
            "\\?w=\\d+&h=\\d+"
    );

    private static final Pattern COLOR_HEX = Pattern.compile("color\\s*:\\s*(#[0-9a-fA-F]{6})");

    // 表格上限（對齊 02 §3.6 / 40 §12.3a）
    private static final int MAX_TABLE_COLS = 10;
    private static final int MAX_TABLE_ROWS = 50;
    private static final int MAX_CELL_TEXT_LEN = 200;
    private static final int MAX_TABLE_JSON_BYTES = 20 * 1024;

    // 清單上限（對齊 02 §3.6 / 40 §12.3b）
    private static final int MAX_LIST_ITEMS = 50;
    private static final int MAX_LIST_JSON_BYTES = 20 * 1024;

    public Result convert(String input) {
        if (input == null || input.isBlank()) {
            return emptyResult();
        }
        String trimmed = input.trim();
        if (!looksLikeHtml(trimmed)) {
            return textOnly(trimmed);
        }
        String safe = Jsoup.clean(trimmed, Safelist.basicWithImages()
                .addTags("h1", "h2", "h3", "h4", "h5", "h6", "table", "thead", "tbody", "tr", "th", "td", "pre", "code"));
        Document doc = Jsoup.parseBodyFragment(safe);
        WalkState state = new WalkState();
        for (Node child : doc.body().childNodes()) {
            walk(child, InlineStyle.NONE, state);
        }
        state.flushRuns(null);
        return buildResult(state.blocks, state.images, state.warnings);
    }

    /** 區塊結構走訪：block-level 邊界 flush runs 成 text block；inline 交給 appendInline 累積 runs。 */
    private void walk(Node node, InlineStyle style, WalkState state) {
        if (node instanceof TextNode tn) {
            appendInline(tn, style, state.runsBuf, state);
            return;
        }
        if (!(node instanceof Element el)) return;
        String tag = el.tagName().toLowerCase();

        if ("img".equals(tag)) {
            state.flushRuns(null);
            handleImage(el, state);
            return;
        }
        if ("picture".equals(tag)) {
            state.flushRuns(null);
            for (Element img : el.select("img")) handleImage(img, state);
            return;
        }
        if ("table".equals(tag)) {
            state.flushRuns(null);
            handleTable(el, state);
            return;
        }
        if ("ul".equals(tag) || "ol".equals(tag)) {
            state.flushRuns(null);
            handleList(el, "ol".equals(tag), state);
            return;
        }
        if (MEDIA_DROP_TAGS.contains(tag)) {
            state.flushRuns(null);
            state.addWarningOnce("media_dropped", "已忽略影片 / 音訊 / 嵌入元素", "tag=" + tag);
            return;
        }
        if (headingFontSize(tag) != null) {
            state.flushRuns(null);
            List<RunAccum> runs = new ArrayList<>();
            for (Node c : el.childNodes()) appendInline(c, style, runs, state);
            String styleJson = "{\"fontSize\":\"" + headingFontSize(tag) + "\",\"fontWeight\":700}";
            state.emitTextBlock(runs, styleJson);
            return;
        }
        if (BLOCK_LEVEL_TAGS.contains(tag)) {
            state.flushRuns(null);
            if ("blockquote".equals(tag)) {
                state.addWarningOnce("unsupported_tag_degraded", "已將 blockquote 降級為純文字段落",
                        "tag=blockquote, degraded=text");
            }
            for (Node c : el.childNodes()) walk(c, style, state);
            state.flushRuns(null);
            return;
        }
        if ("br".equals(tag)) {
            state.brCount++;
            if (state.brCount >= 2) {
                state.flushRuns(null);
                state.brCount = 0;
            } else {
                state.emitRun(state.runsBuf, " ", style);
            }
            return;
        }
        state.brCount = 0;

        if (INLINE_TAGS.contains(tag)) {
            appendInline(el, style, state.runsBuf, state);
            return;
        }
        // 未明列 tag → 視為透明容器，遞迴其子節點
        for (Node c : el.childNodes()) walk(c, style, state);
    }

    /** inline 走訪：只產出 runs（供段落 runsBuf / 標題 / 表格 cell），不 emit block。 */
    private void appendInline(Node node, InlineStyle style, List<RunAccum> target, WalkState state) {
        if (node instanceof TextNode tn) {
            String t = tn.text();
            if (!t.isEmpty()) state.emitRun(target, t + " ", style);
            return;
        }
        if (!(node instanceof Element el)) return;
        String tag = el.tagName().toLowerCase();

        switch (tag) {
            case "strong", "b" -> recurseInline(el, style.withBold(), target, state);
            case "em", "i" -> recurseInline(el, style.withItalic(), target, state);
            case "u", "ins" -> recurseInline(el, style.withUnderline(), target, state);
            case "span", "font" -> {
                InlineStyle next = style;
                String styleAttr = el.attr("style");
                String colorAttr = el.attr("color");
                Matcher m = styleAttr != null ? COLOR_HEX.matcher(styleAttr) : null;
                if (m != null && m.find()) {
                    next = style.withColor(m.group(1).toLowerCase());
                } else if (colorAttr != null && colorAttr.matches("#[0-9a-fA-F]{6}")) {
                    next = style.withColor(colorAttr.toLowerCase());
                } else if (styleAttr != null && styleAttr.toLowerCase().contains("color")) {
                    state.addWarningOnce("color_format_invalid", "顏色格式不支援，已忽略", "tag=" + tag);
                }
                if (styleAttr != null && styleAttr.toLowerCase().contains("font-size")) {
                    state.addWarningOnce("style_unsupported_flutter", "段內字級未套用（僅標題支援字級）",
                            "tag=" + tag + ", dropped=font-size");
                }
                recurseInline(el, next, target, state);
            }
            case "a" -> {
                String href = el.attr("href");
                InlineStyle next = style;
                if (href != null && (href.toLowerCase().startsWith("http://") || href.toLowerCase().startsWith("https://"))) {
                    next = style.withHref(href.trim());
                } else if (href != null && !href.isBlank()) {
                    state.addWarningOnce("hyperlink_scheme_rejected", "連結網址格式不支援，僅保留文字",
                            "href=" + href);
                }
                recurseInline(el, next, target, state);
            }
            case "br" -> state.emitRun(target, " ", style);
            case "img", "picture", "iframe", "video", "audio", "embed", "object" -> { /* inline 內忽略媒體 */ }
            case "table" -> {
                // 巢狀表格（cell 內）→ 展平為純文字 run
                state.addWarningOnce("table_nested_flattened", "巢狀表格已轉為純文字", "");
                String t = el.text();
                if (!t.isBlank()) state.emitRun(target, t + " ", style);
            }
            default -> recurseInline(el, style, target, state);
        }
    }

    private void recurseInline(Element el, InlineStyle style, List<RunAccum> target, WalkState state) {
        for (Node c : el.childNodes()) appendInline(c, style, target, state);
    }

    /** <table> → table block（tableJson；cell 內含 runs）。 */
    private void handleTable(Element table, WalkState state) {
        List<Element> allRows = new ArrayList<>();
        for (Element row : table.select("tr")) {
            if (row.closest("table") == table) allRows.add(row); // 排除巢狀表格的 tr
        }
        if (allRows.isEmpty()) return;

        boolean truncated = false;
        boolean mergedSplit = false;
        boolean withHeadings = false;
        ArrayNode rowsNode = MAPPER.createArrayNode();

        int rowCount = 0;
        for (Element row : allRows) {
            if (rowCount >= MAX_TABLE_ROWS) { truncated = true; break; }
            List<Element> cells = new ArrayList<>();
            for (Element child : row.children()) {
                String ct = child.tagName().toLowerCase();
                if ("th".equals(ct) || "td".equals(ct)) cells.add(child);
            }
            if (cells.isEmpty()) continue;
            if (rowCount == 0) {
                withHeadings = cells.stream().anyMatch(c -> "th".equals(c.tagName().toLowerCase()));
            }
            ArrayNode rowNode = MAPPER.createArrayNode();
            int colCount = 0;
            for (Element cell : cells) {
                if (colCount >= MAX_TABLE_COLS) { truncated = true; break; }
                if (cell.hasAttr("colspan") || cell.hasAttr("rowspan")) mergedSplit = true;
                List<RunAccum> cellRuns = new ArrayList<>();
                for (Node c : cell.childNodes()) appendInline(c, InlineStyle.NONE, cellRuns, state);
                ObjectNode cellNode = MAPPER.createObjectNode();
                cellNode.set("runs", buildRunsArray(cellRuns, true));
                rowNode.add(cellNode);
                colCount++;
            }
            rowsNode.add(rowNode);
            rowCount++;
        }
        if (rowsNode.isEmpty()) return;

        ObjectNode tableObj = MAPPER.createObjectNode();
        tableObj.put("withHeadings", withHeadings);
        tableObj.set("rows", rowsNode);

        // 20KB 上限：超過從末列砍
        String tableJson = writeJson(tableObj);
        while (tableJson.getBytes(StandardCharsets.UTF_8).length > MAX_TABLE_JSON_BYTES && rowsNode.size() > 1) {
            rowsNode.remove(rowsNode.size() - 1);
            truncated = true;
            tableJson = writeJson(tableObj);
        }

        if (truncated) state.addWarningOnce("table_truncated", "表格超出上限（欄 10 / 列 50 / 20KB），已截斷", "");
        if (mergedSplit) state.addWarningOnce("table_merged_cell_split", "表格合併儲存格已拆開", "");

        ObjectNode b = MAPPER.createObjectNode();
        b.put("type", "table");
        b.put("id", "blk-" + state.blockIdx);
        b.put("tableJson", tableJson);
        state.blocks.add(b);
        state.blockIdx++;
    }

    /** <ul>/<ol> → list block（listJson；item 內含 runs）。巢狀清單攤平為單層（40 §12.3b）。 */
    private void handleList(Element listEl, boolean ordered, WalkState state) {
        // select("li") 依 document order 回傳清單樹下所有 <li>（含巢狀）→ 天然攤平
        List<Element> lis = listEl.select("li");
        if (lis.isEmpty()) return;

        // 巢狀偵測：清單樹內是否還有子 <ul>/<ol>
        boolean nested = !listEl.select("ul, ol").isEmpty();

        boolean truncated = false;
        ArrayNode itemsNode = MAPPER.createArrayNode();
        for (Element li : lis) {
            if (itemsNode.size() >= MAX_LIST_ITEMS) { truncated = true; break; }
            List<RunAccum> liRuns = new ArrayList<>();
            appendListItemRuns(li, liRuns, state);
            ArrayNode runsArr = buildRunsArray(liRuns, true); // 單項 200 字靜默截斷
            if (runsArr.isEmpty()) continue; // 僅含子清單 / 空白的 <li> 不產生空 item
            ObjectNode itemNode = MAPPER.createObjectNode();
            itemNode.set("runs", runsArr);
            itemsNode.add(itemNode);
        }
        if (itemsNode.isEmpty()) return; // 無有效項目 → 不產出 block

        ObjectNode listObj = MAPPER.createObjectNode();
        listObj.put("ordered", ordered);
        listObj.set("items", itemsNode);

        // 20KB 上限：超過從末項砍
        String listJson = writeJson(listObj);
        while (listJson.getBytes(StandardCharsets.UTF_8).length > MAX_LIST_JSON_BYTES && itemsNode.size() > 1) {
            itemsNode.remove(itemsNode.size() - 1);
            truncated = true;
            listJson = writeJson(listObj);
        }

        if (truncated) state.addWarningOnce("list_truncated", "清單超出上限（項目 50 / 20KB），已截斷", "");
        if (nested) state.addWarningOnce("list_nested_flattened", "巢狀清單已攤平為單層", "");

        ObjectNode b = MAPPER.createObjectNode();
        b.put("type", "list");
        b.put("id", "blk-" + state.blockIdx);
        b.put("listJson", listJson);
        state.blocks.add(b);
        state.blockIdx++;
    }

    /** 單一 <li> 的 inline runs：走 appendInline，但跳過子 <ul>/<ol>/<li>（巢狀項目已由 select("li") 各自承接）。 */
    private void appendListItemRuns(Element li, List<RunAccum> target, WalkState state) {
        for (Node c : li.childNodes()) {
            if (c instanceof Element ce) {
                String t = ce.tagName().toLowerCase();
                if ("ul".equals(t) || "ol".equals(t) || "li".equals(t)) continue;
            }
            appendInline(c, InlineStyle.NONE, target, state);
        }
    }

    /** RunAccum list → runs ArrayNode（trim 外緣、丟空 run、單 cell 截斷 200 字）。 */
    private ArrayNode buildRunsArray(List<RunAccum> runs, boolean cellLimit) {
        List<ObjectNode> out = new ArrayList<>();
        int totalLen = 0;
        for (int i = 0; i < runs.size(); i++) {
            RunAccum r = runs.get(i);
            String text = r.text.toString().replaceAll("\\s+", " ");
            if (i == 0) text = text.replaceAll("^\\s+", "");
            if (i == runs.size() - 1) text = text.replaceAll("\\s+$", "");
            if (text.isEmpty()) continue;
            if (cellLimit && totalLen + text.length() > MAX_CELL_TEXT_LEN) {
                text = text.substring(0, Math.max(0, MAX_CELL_TEXT_LEN - totalLen));
            }
            totalLen += text.length();
            out.add(r.style.toRunNode(text));
            if (cellLimit && totalLen >= MAX_CELL_TEXT_LEN) break;
        }
        ArrayNode arr = MAPPER.createArrayNode();
        out.forEach(arr::add);
        return arr;
    }

    private void handleImage(Element imgEl, WalkState state) {
        String src = imgEl.absUrl("src");
        if (src.isEmpty()) src = imgEl.attr("src");
        if (src.isBlank()) return;
        String stripped = stripCdnSuffix(src);
        ObjectNode b = MAPPER.createObjectNode();
        b.put("type", "image");
        b.put("id", "blk-" + state.blockIdx);
        b.put("stagedToken", "");
        b.put("assetId", "");
        b.put("objectKey", "");
        b.put("url", stripped);
        b.put("width", "");
        b.put("height", "");
        state.blocks.add(b);
        state.images.add(new ExtractedImage(stripped, state.blockIdx));
        state.blockIdx++;
    }

    private static String stripCdnSuffix(String url) {
        String out = url;
        for (String pattern : CDN_SUFFIX_PATTERNS) out = out.replaceAll(pattern, "");
        return out;
    }

    private static String writeJson(JsonNode node) {
        try {
            return MAPPER.writeValueAsString(node);
        } catch (Exception e) {
            return "{}";
        }
    }

    private Result textOnly(String text) {
        ObjectNode b = MAPPER.createObjectNode();
        b.put("type", "text");
        b.put("id", "blk-0");
        ArrayNode runs = MAPPER.createArrayNode();
        ObjectNode run = MAPPER.createObjectNode();
        run.put("text", text);
        runs.add(run);
        b.set("runs", runs);
        return buildResult(List.of(b), List.of(), List.of());
    }

    private Result emptyResult() {
        return buildResult(List.of(), List.of(), List.of());
    }

    private Result buildResult(List<ObjectNode> blocks, List<ExtractedImage> images,
                               List<ConversionWarning> warnings) {
        ObjectNode root = MAPPER.createObjectNode();
        root.put("version", "1.0");
        ArrayNode arr = root.putArray("blocks");
        blocks.forEach(arr::add);
        return new Result(root, images, warnings);
    }

    private boolean looksLikeHtml(String s) {
        return s.contains("<") && s.contains(">");
    }

    /** 行內樣式（不可變，遞迴傳遞）。 */
    private record InlineStyle(boolean bold, boolean italic, boolean underline, String color, String href) {
        static final InlineStyle NONE = new InlineStyle(false, false, false, null, null);

        InlineStyle withBold() { return new InlineStyle(true, italic, underline, color, href); }
        InlineStyle withItalic() { return new InlineStyle(bold, true, underline, color, href); }
        InlineStyle withUnderline() { return new InlineStyle(bold, italic, true, color, href); }
        InlineStyle withColor(String c) { return new InlineStyle(bold, italic, underline, c, href); }
        InlineStyle withHref(String h) { return new InlineStyle(bold, italic, underline, color, h); }

        ObjectNode toRunNode(String text) {
            ObjectNode r = MAPPER.createObjectNode();
            r.put("text", text);
            if (bold) r.put("bold", true);
            if (italic) r.put("italic", true);
            if (underline) r.put("underline", true);
            if (color != null) r.put("color", color);
            if (href != null) r.put("href", href);
            return r;
        }
    }

    /** 累積中的 run（同樣式相鄰合併）。 */
    private static final class RunAccum {
        final StringBuilder text;
        final InlineStyle style;
        RunAccum(StringBuilder text, InlineStyle style) { this.text = text; this.style = style; }
    }

    /** walk 過程的累積狀態。 */
    private final class WalkState {
        final List<ObjectNode> blocks = new ArrayList<>();
        final List<ExtractedImage> images = new ArrayList<>();
        final List<ConversionWarning> warnings = new ArrayList<>();
        final List<RunAccum> runsBuf = new ArrayList<>();
        final java.util.Set<String> emittedWarningKeys = new java.util.HashSet<>();
        int blockIdx = 0;
        int brCount = 0;

        /** 相鄰同樣式 run 合併（canonical form）。 */
        void emitRun(List<RunAccum> target, String text, InlineStyle style) {
            if (text == null || text.isEmpty()) return;
            if (!target.isEmpty() && target.get(target.size() - 1).style.equals(style)) {
                target.get(target.size() - 1).text.append(text);
            } else {
                target.add(new RunAccum(new StringBuilder(text), style));
            }
        }

        /** flush runsBuf 成 text block。 */
        void flushRuns(String styleJson) {
            if (runsBuf.isEmpty()) return;
            List<RunAccum> copy = new ArrayList<>(runsBuf);
            runsBuf.clear();
            emitTextBlock(copy, styleJson);
        }

        void emitTextBlock(List<RunAccum> runs, String styleJson) {
            ArrayNode arr = buildRunsArray(runs, false);
            if (arr.isEmpty()) return;
            ObjectNode b = MAPPER.createObjectNode();
            b.put("type", "text");
            b.put("id", "blk-" + blockIdx);
            b.set("runs", arr);
            if (styleJson != null) b.put("styleJson", styleJson);
            blocks.add(b);
            blockIdx++;
        }

        void addWarningOnce(String warningEnum, String message, String detail) {
            String key = warningEnum + "|" + detail;
            if (emittedWarningKeys.add(key)) {
                warnings.add(new ConversionWarning(warningEnum, message, detail));
            }
        }
    }

    public record Result(JsonNode descriptionJson, List<ExtractedImage> images,
                         List<ConversionWarning> warnings) {}

    public record ExtractedImage(String url, int blockIndex) {}

    public record ConversionWarning(String warningEnum, String message, String detail) {}
}
