package io.github.will413028.hawkr.productimport.helper;

import io.github.will413028.hawkr.common.utils.code.ImportAssetLocalStatus;
import io.github.will413028.hawkr.common.utils.code.ImportAssetUsage;
import io.github.will413028.hawkr.product.grpc.Certification;
import io.github.will413028.hawkr.product.grpc.DescriptionBlock;
import io.github.will413028.hawkr.product.grpc.GalleryItem;
import io.github.will413028.hawkr.product.grpc.InlineRun;
import io.github.will413028.hawkr.product.grpc.ProductDescription;
import io.github.will413028.hawkr.product.grpc.ProductDraft;
import io.github.will413028.hawkr.product.grpc.ProductSkuImport;
import io.github.will413028.hawkr.product.grpc.ShippingMethod;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemAssetEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemEntity;
import io.github.will413028.hawkr.productimport.entity.ProductImportItemSkuEntity;
import com.fasterxml.jackson.databind.JsonNode;

import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.function.Function;
import java.util.stream.Collectors;

/**
 * 40 §4.3a「Draft snapshot 構建來源」三時機點共用 helper：
 *
 * <ul>
 *   <li>{@link #fromCsvParseSnapshot} — §5.2 g：CSV parse worker 解析完 in-memory ProductDraft
 *       （含已截斷至 9 張的 gallery、assets 仍為 PENDING/URL 樣態）</li>
 *   <li>{@link #fromUpdateSnapshot} — §5.5 2.7：套用本次 UpdateImportItem input 後的 in-memory snapshot</li>
 *   <li>{@link #fromDbCurrent} — §5.7 4.5：commit final guard，由 DB 當前 row 構建（item + sku + asset）</li>
 * </ul>
 *
 * <p>本 helper 為純函式（沒有 DI / DB 依賴）；轉換規則：
 * <ul>
 *   <li>{@code gallery} 來源：CSV_PARSE 用 in-memory URL；IMPORT_UPDATE / IMPORT_COMMIT 用 DB asset 的
 *       {@code mediaImportAssetId}（若已 bind）+ {@code sourceUrl} fallback</li>
 *   <li>{@code skus} 一律由 DB / in-memory entity 轉換為 {@link ProductSkuImport}</li>
 *   <li>{@code description_json} 來自 {@code item.descriptionJson}（JsonNode）→
 *       ProductDescription proto bytes</li>
 * </ul>
 */
public final class ProductDraftBuilder {

    private ProductDraftBuilder() {}

    /** §5.7 4.5：commit final guard — DB 當前狀態。caller 傳已 SELECT 完的 item / skus / assets。 */
    public static ProductDraft fromDbCurrent(ProductImportItemEntity item,
                                              List<ProductImportItemSkuEntity> skus,
                                              List<ProductImportItemAssetEntity> assets,
                                              String targetStatusFallback) {
        ProductDraft.Builder b = baseBuilder(item, targetStatusFallback, assets);
        // gallery：優先 media_import_asset_id（已 UPLOADED），fallback source_url；按 sort_order 排序
        assets.stream()
                .filter(a -> ImportAssetUsage.GALLERY.value().equals(a.getUsage()))
                .filter(a -> !ImportAssetLocalStatus.ORPHAN.value().equals(a.getStatus()))
                .sorted(Comparator.comparing(a -> a.getSortOrder() == null ? 0 : a.getSortOrder()))
                .forEach(a -> b.addGallery(toGalleryItem(a)));
        for (ProductImportItemSkuEntity s : skus) b.addSkus(toProductSku(s));
        return b.build();
    }

    /**
     * §5.5 2.7：套用 update 後的 in-memory snapshot。caller 已 mutate item / skus 並可能含本次
     * BindStagedAsset 後拿到的 import_asset_id（透過 assets 引數注入）。
     */
    public static ProductDraft fromUpdateSnapshot(ProductImportItemEntity item,
                                                   List<ProductImportItemSkuEntity> skus,
                                                   List<ProductImportItemAssetEntity> assets,
                                                   String targetStatusFallback) {
        return fromDbCurrent(item, skus, assets, targetStatusFallback);
    }

    /**
     * §5.2 g：CSV parse — in-memory draft，asset 仍為 PENDING/URL 樣態。
     * CSV_PARSE context 下 validator 會放寬 ASSET_NOT_READY 檢核，故 staged_token 為空可接受。
     */
    public static ProductDraft fromCsvParseSnapshot(ProductImportItemEntity item,
                                                     List<ProductImportItemSkuEntity> skus,
                                                     List<String> galleryUrls,
                                                     String targetStatusFallback) {
        ProductDraft.Builder b = baseBuilder(item, targetStatusFallback, List.of());
        if (galleryUrls != null) {
            int order = 0;
            for (String url : galleryUrls) {
                if (url == null || url.isBlank()) continue;
                // CSV_PARSE context：尚未綁定 asset_id，僅放 URL 作為佔位
                b.addGallery(GalleryItem.newBuilder()
                        .setUrl(url)
                        .build());
                order++;
                if (order >= 9) break; // §4.3a MAX_GALLERY_IMAGES
            }
        }
        for (ProductImportItemSkuEntity s : skus) b.addSkus(toProductSku(s));
        return b.build();
    }

    private static ProductDraft.Builder baseBuilder(ProductImportItemEntity item, String targetStatusFallback,
                                                    List<ProductImportItemAssetEntity> assets) {
        ProductDraft.Builder b = ProductDraft.newBuilder()
                .setName(orEmpty(item.getName()))
                .setCategoryId(item.getCategoryId() == null ? "" : String.valueOf(item.getCategoryId()))
                .setOrigin(orEmpty(item.getOrigin()))
                .setTargetStatus(pickTargetStatus(item.getTargetStatus(), targetStatusFallback));
        JsonNode desc = item.getDescriptionJson();
        if (desc != null) {
            b.setDescriptionJson(toProductDescription(desc, assets).toByteString());
        }
        if (item.getHashtags() != null) {
            for (String h : item.getHashtags()) {
                if (h != null && !h.isBlank()) b.addHashtags(h);
            }
        }
        if (item.getCertifications() != null) {
            for (Map<String, Object> c : item.getCertifications()) {
                String name = c.get("name") == null ? "" : String.valueOf(c.get("name"));
                String certNo = c.get("certNo") == null ? "" : String.valueOf(c.get("certNo"));
                if (name.isBlank()) continue;
                b.addCertifications(Certification.newBuilder()
                        .setName(name).setCertNo(certNo).build());
            }
        }
        return b;
    }

    private static ProductDescription toProductDescription(JsonNode root,
                                                           List<ProductImportItemAssetEntity> assets) {
        ProductDescription.Builder desc = ProductDescription.newBuilder()
                .setVersion(root.path("version").asText("1"));
        Map<Integer, ProductImportItemAssetEntity> descriptionAssetsByBlock =
                assets == null ? Map.of() : assets.stream()
                        .filter(a -> ImportAssetUsage.DESCRIPTION.value().equals(a.getUsage()))
                        .filter(a -> !ImportAssetLocalStatus.ORPHAN.value().equals(a.getStatus()))
                        .filter(a -> a.getBlockIndex() != null)
                        .collect(Collectors.toMap(
                                ProductImportItemAssetEntity::getBlockIndex,
                                Function.identity(),
                                (a, b) -> a
                        ));
        JsonNode blocks = root.path("blocks");
        if (!blocks.isArray()) {
            return desc.build();
        }
        int index = 0;
        for (JsonNode bn : blocks) {
            DescriptionBlock.Builder block = DescriptionBlock.newBuilder()
                    .setType(bn.path("type").asText(""))
                    .setId(bn.path("id").asText(""))
                    .setContent(bn.path("content").asText(""))
                    .setStyleJson(bn.path("styleJson").asText(""))
                    .setStagedToken(bn.path("stagedToken").asText(""))
                    .setAssetId(bn.path("assetId").asText(""))
                    .setObjectKey(bn.path("objectKey").asText(""))
                    .setUrl(bn.path("url").asText(""))
                    .setWidth(bn.path("width").asText(""))
                    .setHeight(bn.path("height").asText(""))
                    .setAlt(bn.path("alt").asText(""))
                    .setImportAssetId(bn.path("importAssetId").asText(""));
            applyInlineRunFields(bn, block);
            ProductImportItemAssetEntity asset = descriptionAssetsByBlock.get(index);
            if (asset != null && asset.getMediaImportAssetId() != null && !asset.getMediaImportAssetId().isBlank()) {
                block.setImportAssetId(asset.getMediaImportAssetId());
            }
            desc.addBlocks(block.build());
            index++;
        }
        return desc.build();
    }

    /**
     * 將 JSON block 的 inline-run 模型欄位（{@code runs} / {@code tableJson} / {@code listJson}）套上 proto
     * {@link DescriptionBlock.Builder}（02 §3.6；#1256 text runs / #1255 table / #1258 list）。
     *
     * <p>匯入產出的 description_json（{@code HtmlToBlocksConverter} / 編輯儲存）text block 內容存於
     * {@code runs}（非 legacy {@code content}）、table 存 {@code tableJson}、list 存 {@code listJson}；
     * 若不帶這些欄位，draft / commit proto 會丟失 inline 樣式與 table / list 結構，導致 commit 後描述變空 / 被
     * {@code ProductPublishValidator} 拒（DESC_BLOCK_INVALID）。draft 與 commit 兩處建 proto 共用本 helper。
     */
    public static void applyInlineRunFields(JsonNode bn, DescriptionBlock.Builder blk) {
        JsonNode runs = bn.path("runs");
        if (runs.isArray()) {
            for (JsonNode r : runs) {
                InlineRun.Builder run = InlineRun.newBuilder().setText(r.path("text").asText(""));
                if (r.path("bold").asBoolean(false)) run.setBold(true);
                if (r.path("italic").asBoolean(false)) run.setItalic(true);
                if (r.path("underline").asBoolean(false)) run.setUnderline(true);
                String color = r.path("color").asText("");
                if (!color.isEmpty()) run.setColor(color);
                String href = r.path("href").asText("");
                if (!href.isEmpty()) run.setHref(href);
                blk.addRuns(run.build());
            }
        }
        String tableJson = bn.path("tableJson").asText("");
        if (!tableJson.isEmpty()) blk.setTableJson(tableJson);
        String listJson = bn.path("listJson").asText("");
        if (!listJson.isEmpty()) blk.setListJson(listJson);
    }

    private static GalleryItem toGalleryItem(ProductImportItemAssetEntity a) {
        GalleryItem.Builder g = GalleryItem.newBuilder();
        if (a.getMediaImportAssetId() != null && !a.getMediaImportAssetId().isBlank()) {
            g.setAssetId(a.getMediaImportAssetId());
        }
        if (a.getObjectKey() != null && !a.getObjectKey().isBlank()) {
            g.setObjectKey(a.getObjectKey());
        }
        if (a.getSourceUrl() != null && !a.getSourceUrl().isBlank()) {
            g.setUrl(a.getSourceUrl());
        } else if (a.getPreviewUrl() != null && !a.getPreviewUrl().isBlank()) {
            g.setUrl(a.getPreviewUrl());
        }
        return g.build();
    }

    private static ProductSkuImport toProductSku(ProductImportItemSkuEntity s) {
        ProductSkuImport.Builder sb = ProductSkuImport.newBuilder()
                .setSkuName(orEmpty(s.getSkuName()))
                .setPrice(s.getPrice() == null ? "" : String.valueOf(s.getPrice()))
                .setSalePrice(s.getSalePrice() == null ? "" : String.valueOf(s.getSalePrice()))
                .setStockQty(s.getStockQty() == null ? "" : String.valueOf(s.getStockQty()))
                .setWeight(s.getWeight() == null ? "" : s.getWeight().toPlainString())
                .setUnit(orEmpty(s.getUnit()))
                .setLengthCm(s.getLengthCm() == null ? "" : s.getLengthCm().toPlainString())
                .setWidthCm(s.getWidthCm() == null ? "" : s.getWidthCm().toPlainString())
                .setHeightCm(s.getHeightCm() == null ? "" : s.getHeightCm().toPlainString());
        if (s.getShippingMethods() != null) {
            for (Map<String, Object> m : s.getShippingMethods()) {
                sb.addShippingMethods(ShippingMethod.newBuilder()
                        .setType(String.valueOf(m.getOrDefault("method", "")))
                        .setTemperature(String.valueOf(m.getOrDefault("temperature", "")))
                        .setFreeThreshold(String.valueOf(m.getOrDefault("freeThreshold", "")))
                        .setFeePayer(String.valueOf(m.getOrDefault("feePayer", "")))
                        .build());
            }
        }
        return sb.build();
    }

    private static String pickTargetStatus(String itemTarget, String fallback) {
        if (itemTarget != null && !itemTarget.isBlank()) return itemTarget;
        return fallback == null ? "" : fallback;
    }

    private static String orEmpty(String s) { return s == null ? "" : s; }
}
