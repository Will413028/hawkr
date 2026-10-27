package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetUsage;
import io.github.will413028.hawkr.product.grpc.ProductDescription;
import io.github.will413028.hawkr.product.grpc.ProductDraft;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.Test;

import java.util.List;

import static org.assertj.core.api.Assertions.assertThat;

class ProductDraftBuilderTest {

    private static final ObjectMapper MAPPER = new ObjectMapper();

    @Test
    void descriptionJsonIsSerializedAsProductDescriptionProtoBytes() throws Exception {
        ProductImportItemEntity item = new ProductImportItemEntity();
        item.setName("草莓");
        item.setCategoryId(123L);
        item.setOrigin("台灣");
        item.setTargetStatus("ACTIVE");
        item.setDescriptionJson(readJson("""
                {
                  "version": "1",
                  "blocks": [
                    {"type":"text","id":"blk-0","content":"甜度高","styleJson":"{}"},
                    {"type":"image","id":"blk-1","url":"https://cdn.example.test/a.jpg","width":"","height":""}
                  ]
                }
                """));

        ProductImportItemAssetEntity descriptionAsset = new ProductImportItemAssetEntity();
        descriptionAsset.setUsage(ImportAssetUsage.DESCRIPTION.value());
        descriptionAsset.setStatus(ImportAssetLocalStatus.UPLOADED.value());
        descriptionAsset.setBlockIndex(1);
        descriptionAsset.setMediaImportAssetId("import-asset-1");

        ProductDraft draft = ProductDraftBuilder.fromDbCurrent(
                item, List.of(), List.of(descriptionAsset), "ACTIVE");

        ProductDescription description = ProductDescription.parseFrom(draft.getDescriptionJson());
        assertThat(description.getVersion()).isEqualTo("1");
        assertThat(description.getBlocksCount()).isEqualTo(2);
        assertThat(description.getBlocks(0).getType()).isEqualTo("text");
        assertThat(description.getBlocks(0).getContent()).isEqualTo("甜度高");
        assertThat(description.getBlocks(1).getType()).isEqualTo("image");
        assertThat(description.getBlocks(1).getUrl()).isEqualTo("https://cdn.example.test/a.jpg");
        assertThat(description.getBlocks(1).getImportAssetId()).isEqualTo("import-asset-1");
    }

    @Test
    void inlineRunTableAndListBlocksCarryThroughToProto() throws Exception {
        // #1255 table / #1256 text runs / #1258 list：draft proto 必須帶 runs / tableJson / listJson，
        // 否則 commit 後描述變空 / 被 ProductPublishValidator 拒。
        ProductImportItemEntity item = new ProductImportItemEntity();
        item.setName("條列商品");
        item.setCategoryId(123L);
        item.setTargetStatus("ACTIVE");
        item.setDescriptionJson(readJson("""
                {
                  "version": "1",
                  "blocks": [
                    {"type":"text","id":"blk-0","runs":[{"text":"新鮮 "},{"text":"直送","bold":true}]},
                    {"type":"list","id":"blk-1","listJson":"{\\"ordered\\":false,\\"items\\":[{\\"runs\\":[{\\"text\\":\\"產地直送\\"}]},{\\"runs\\":[{\\"text\\":\\"配送\\",\\"href\\":\\"https://x.com\\"}]}]}"},
                    {"type":"table","id":"blk-2","tableJson":"{\\"withHeadings\\":true,\\"rows\\":[[{\\"runs\\":[{\\"text\\":\\"品名\\"}]}]]}"}
                  ]
                }
                """));

        ProductDraft draft = ProductDraftBuilder.fromDbCurrent(item, List.of(), List.of(), "ACTIVE");
        ProductDescription description = ProductDescription.parseFrom(draft.getDescriptionJson());

        assertThat(description.getBlocksCount()).isEqualTo(3);
        // text block runs（保留粗體）
        assertThat(description.getBlocks(0).getRunsCount()).isEqualTo(2);
        assertThat(description.getBlocks(0).getRuns(1).getText()).isEqualTo("直送");
        assertThat(description.getBlocks(0).getRuns(1).getBold()).isTrue();
        // list block listJson 非空且帶 items
        assertThat(description.getBlocks(1).getListJson()).contains("ordered", "產地直送", "https://x.com");
        // table block tableJson 非空
        assertThat(description.getBlocks(2).getTableJson()).contains("withHeadings", "品名");
    }

    private static JsonNode readJson(String json) throws Exception {
        return MAPPER.readTree(json);
    }
}
