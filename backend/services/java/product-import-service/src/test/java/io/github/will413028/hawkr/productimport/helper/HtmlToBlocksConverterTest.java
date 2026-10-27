package io.github.will413028.hawkr.productimport.helper;

import com.fasterxml.jackson.databind.JsonNode;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNotNull;
import static org.junit.jupiter.api.Assertions.assertTrue;

/**
 * #1255（匯入 table）/ #1256（匯入行內連結）/ #1258（匯入條列 li）驗收：
 * 驗證來源 HTML 的 &lt;table&gt; 轉為 table block、&lt;a href&gt; 與局部粗體轉為 inline run、
 * &lt;ul&gt;/&lt;ol&gt; 轉為 list block（項目內為 runs、巢狀攤平、超上限截斷），
 * 不再展平 / 丟 href。含純文字回歸（無 HTML 的既有資料不受影響）。
 */
class HtmlToBlocksConverterTest {

    private final HtmlToBlocksConverter conv = new HtmlToBlocksConverter();

    private static JsonNode firstBlockOfType(JsonNode blocks, String type) {
        for (JsonNode b : blocks) {
            if (type.equals(b.path("type").asText())) return b;
        }
        return null;
    }

    private static boolean hasWarning(HtmlToBlocksConverter.Result r, String warningEnum) {
        return r.warnings().stream().anyMatch(w -> warningEnum.equals(w.warningEnum()));
    }

    @Test
    void convertsTableAndInlineLinkAndLocalBold() {
        String html =
                "<p>新鮮直送 <b>限量特惠</b>，詳見 "
                        + "<a href=\"https://www.example.com\">官方農場網站</a>。</p>"
                        + "<table><thead><tr><th>品名</th><th>產地</th></tr></thead>"
                        + "<tbody><tr><td>六龜芒果</td><td>高雄</td></tr></tbody></table>";

        HtmlToBlocksConverter.Result result = conv.convert(html);
        JsonNode blocks = result.descriptionJson().get("blocks");
        assertNotNull(blocks, "descriptionJson 應含 blocks");

        JsonNode tableBlock = null;
        JsonNode textBlock = null;
        for (JsonNode b : blocks) {
            String type = b.path("type").asText();
            if ("table".equals(type)) tableBlock = b;
            if ("text".equals(type) && textBlock == null) textBlock = b;
        }

        // #1255：table 未被展平
        assertNotNull(tableBlock, "應輸出 table block，而非展平成 text");
        String tableJson = tableBlock.path("tableJson").asText();
        assertTrue(tableJson.contains("withHeadings"), "tableJson 應標記 withHeadings");
        assertTrue(tableJson.contains("品名"), "tableJson 應含標題列 cell");
        assertTrue(tableJson.contains("六龜芒果"), "tableJson 應含資料列 cell");

        // #1256：行內連結 href 保留 + 局部粗體
        assertNotNull(textBlock, "應有 text block 承載段落 runs");
        boolean hasHref = false;
        boolean hasBold = false;
        for (JsonNode r : textBlock.path("runs")) {
            if (r.path("href").asText("").startsWith("https://")) hasHref = true;
            if (r.path("bold").asBoolean(false)) hasBold = true;
        }
        assertTrue(hasHref, "&lt;a href&gt; 應轉為帶 href 的 inline run，而非丟 href");
        assertTrue(hasBold, "&lt;b&gt; 應轉為局部粗體 inline run");
    }

    @Test
    void plainTextRegression_noHtmlStillProducesTextBlock() {
        HtmlToBlocksConverter.Result result = conv.convert("純文字說明，無任何 HTML 標籤");
        JsonNode blocks = result.descriptionJson().get("blocks");
        assertNotNull(blocks);
        assertTrue(blocks.size() >= 1, "純文字應至少一個 block");
        assertEquals("text", blocks.get(0).path("type").asText(), "純文字應為 text block");
    }

    // ── #1258 清單轉換 ──────────────────────────────────────────────

    @Test
    void convertsUnorderedList_toListBlockNotFlattenedText() {
        String html = "<p>特色</p><ul><li>產地直送</li><li>低溫配送</li></ul>";
        HtmlToBlocksConverter.Result result = conv.convert(html);
        JsonNode blocks = result.descriptionJson().get("blocks");

        JsonNode listBlock = firstBlockOfType(blocks, "list");
        assertNotNull(listBlock, "應輸出 list block，而非把 <li> 展平成多個 text block");
        String listJson = listBlock.path("listJson").asText();
        assertTrue(listJson.contains("\"ordered\":false"), "<ul> 應為 ordered:false");
        assertTrue(listJson.contains("產地直送"), "listJson 應含第 1 項");
        assertTrue(listJson.contains("低溫配送"), "listJson 應含第 2 項");

        // <li> 不再各自成 text block（僅 <p>特色</p> 一個 text block）
        int textBlocks = 0;
        for (JsonNode b : blocks) if ("text".equals(b.path("type").asText())) textBlocks++;
        assertEquals(1, textBlocks, "<li> 不應展平成 text block");
    }

    @Test
    void convertsOrderedList_setsOrderedTrue() {
        HtmlToBlocksConverter.Result result = conv.convert("<ol><li>第一步</li><li>第二步</li></ol>");
        JsonNode listBlock = firstBlockOfType(result.descriptionJson().get("blocks"), "list");
        assertNotNull(listBlock, "應輸出 list block");
        assertTrue(listBlock.path("listJson").asText().contains("\"ordered\":true"), "<ol> 應為 ordered:true");
    }

    @Test
    void listItemPreservesInlineLinkAndBold() {
        String html = "<ul><li>低溫配送，詳見 <a href=\"https://example.com/shipping\">配送說明</a></li>"
                + "<li><b>限量</b>供應</li></ul>";
        HtmlToBlocksConverter.Result result = conv.convert(html);
        String listJson = firstBlockOfType(result.descriptionJson().get("blocks"), "list").path("listJson").asText();
        assertTrue(listJson.contains("https://example.com/shipping"), "項目內 <a href> 應保留");
        assertTrue(listJson.contains("\"bold\":true"), "項目內 <b> 應轉為粗體 run");
    }

    @Test
    void nestedList_flattenedToSingleLevelWithWarning() {
        String html = "<ul><li>父項目<ul><li>子項目 A</li><li>子項目 B</li></ul></li><li>另一項</li></ul>";
        HtmlToBlocksConverter.Result result = conv.convert(html);
        JsonNode listBlock = firstBlockOfType(result.descriptionJson().get("blocks"), "list");
        assertNotNull(listBlock, "應輸出單一 list block");
        String listJson = listBlock.path("listJson").asText();
        // 攤平為 4 個 item：父項目 / 子項目 A / 子項目 B / 另一項
        assertTrue(listJson.contains("父項目") && listJson.contains("子項目 A")
                && listJson.contains("子項目 B") && listJson.contains("另一項"), "巢狀應攤平為單層各項目");
        assertTrue(hasWarning(result, "list_nested_flattened"), "巢狀應發 list_nested_flattened warning");

        // 父項目 item 不應吞入子清單文字（父 li 自身文字獨立成 item）
        JsonNode items = null;
        try { items = new com.fasterxml.jackson.databind.ObjectMapper().readTree(listJson).path("items"); }
        catch (Exception e) { throw new RuntimeException(e); }
        assertEquals(4, items.size(), "應攤平為 4 個 item");
        String firstItemText = items.get(0).path("runs").get(0).path("text").asText();
        assertEquals("父項目", firstItemText.trim(), "父項目 item 不應含子清單文字");
    }

    @Test
    void listOverMaxItems_truncatedWithWarning() {
        StringBuilder sb = new StringBuilder("<ul>");
        for (int i = 1; i <= 60; i++) sb.append("<li>項目").append(i).append("</li>");
        sb.append("</ul>");
        HtmlToBlocksConverter.Result result = conv.convert(sb.toString());
        String listJson = firstBlockOfType(result.descriptionJson().get("blocks"), "list").path("listJson").asText();
        JsonNode items;
        try { items = new com.fasterxml.jackson.databind.ObjectMapper().readTree(listJson).path("items"); }
        catch (Exception e) { throw new RuntimeException(e); }
        assertEquals(50, items.size(), "超過 50 項應截斷至 50");
        assertTrue(hasWarning(result, "list_truncated"), "超上限應發 list_truncated warning");
    }

    @Test
    void noListRegression_tableAndTextUnaffected() {
        // 無清單的來源（table + inline link）不因新增 li 解析而破壞（對齊 #1255/#1256）
        String html = "<p>說明 <a href=\"https://x.com\">連結</a></p>"
                + "<table><tr><td>A</td><td>B</td></tr></table>";
        HtmlToBlocksConverter.Result result = conv.convert(html);
        JsonNode blocks = result.descriptionJson().get("blocks");
        assertNotNull(firstBlockOfType(blocks, "table"), "table block 應不受影響");
        assertNotNull(firstBlockOfType(blocks, "text"), "text block 應不受影響");
        assertEquals(null, firstBlockOfType(blocks, "list"), "無清單來源不應產生 list block");
    }
}
