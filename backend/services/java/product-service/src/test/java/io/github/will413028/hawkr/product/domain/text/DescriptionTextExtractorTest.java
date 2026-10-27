package io.github.will413028.hawkr.product.domain.text;

import io.github.will413028.hawkr.product.entity.ProductEntity;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * pure_description 純文字萃取測試（02 §3.6）。
 *
 * 聚焦 list block（issue #1257）萃取，並保留 text / table 既有行為的回歸基線。
 */
class DescriptionTextExtractorTest {

    private static ProductEntity.InlineRun run(String text) {
        ProductEntity.InlineRun r = new ProductEntity.InlineRun();
        r.setText(text);
        return r;
    }

    private static ProductEntity.DescriptionBlock block(String type) {
        ProductEntity.DescriptionBlock b = new ProductEntity.DescriptionBlock();
        b.setType(type);
        return b;
    }

    private static ProductEntity.ProductDescription desc(ProductEntity.DescriptionBlock... blocks) {
        ProductEntity.ProductDescription d = new ProductEntity.ProductDescription();
        d.setVersion("1.0");
        d.setBlocks(List.of(blocks));
        return d;
    }

    @Test
    @DisplayName("list block：依 item 順序串接每個 item 每個 run 的 text（含連結文字）")
    void extractList() {
        ProductEntity.DescriptionBlock list = block("list");
        list.setListJson("{\"ordered\":false,\"items\":["
                + "{\"runs\":[{\"text\":\"產地直送\"}]},"
                + "{\"runs\":[{\"text\":\"低溫配送，詳見\"},{\"text\":\"配送說明\",\"href\":\"https://example.com/s\"}]}"
                + "]}");

        assertEquals("產地直送 低溫配送，詳見 配送說明", DescriptionTextExtractor.extract(desc(list)));
    }

    @Test
    @DisplayName("list block：list_json 為空 / null 不納入、不拋例外")
    void extractListEmpty() {
        ProductEntity.DescriptionBlock empty = block("list");
        empty.setListJson("");
        assertEquals("", DescriptionTextExtractor.extract(desc(empty)));

        ProductEntity.DescriptionBlock nullJson = block("list");
        assertEquals("", DescriptionTextExtractor.extract(desc(nullJson)));
    }

    @Test
    @DisplayName("list block：list_json 格式錯誤容錯（後端 opaque，不阻斷）")
    void extractListMalformed() {
        ProductEntity.DescriptionBlock bad = block("list");
        bad.setListJson("{not-json");
        assertEquals("", DescriptionTextExtractor.extract(desc(bad)));
    }

    @Test
    @DisplayName("text / table / list / image 混合：依 blocks 順序串接、單一空格分隔")
    void extractMixed() {
        ProductEntity.DescriptionBlock text = block("text");
        text.setRuns(List.of(run("新鮮直送"), run("限量特惠")));

        ProductEntity.DescriptionBlock table = block("table");
        table.setTableJson("{\"withHeadings\":true,\"rows\":[[{\"runs\":[{\"text\":\"品名\"}]},{\"runs\":[{\"text\":\"產地\"}]}]]}");

        ProductEntity.DescriptionBlock list = block("list");
        list.setListJson("{\"ordered\":true,\"items\":[{\"runs\":[{\"text\":\"步驟一\"}]}]}");

        ProductEntity.DescriptionBlock image = block("image");
        image.setAlt("產地直送新鮮蔬菜");

        assertEquals("新鮮直送 限量特惠 品名 產地 步驟一 產地直送新鮮蔬菜",
                DescriptionTextExtractor.extract(desc(text, table, list, image)));
    }
}
