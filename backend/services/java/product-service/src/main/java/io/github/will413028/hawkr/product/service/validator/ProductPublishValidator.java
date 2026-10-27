package io.github.will413028.hawkr.product.service.validator;

import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.code.CertificationType;
import io.github.will413028.hawkr.common.utils.code.CodeMetaValidator;
import io.github.will413028.hawkr.common.utils.code.ProductStatus;
import io.github.will413028.hawkr.common.utils.code.ShippingTemperature;
import io.github.will413028.hawkr.common.utils.code.WeightUnit;
import io.github.will413028.hawkr.common.utils.validation.InputValidator;
import io.github.will413028.hawkr.product.domain.hashtag.HashtagInputValidator;
import io.github.will413028.hawkr.product.entity.CategoryEntity;
import io.github.will413028.hawkr.product.grpc.Certification;
import io.github.will413028.hawkr.product.grpc.DescriptionBlock;
import io.github.will413028.hawkr.product.grpc.GalleryItem;
import io.github.will413028.hawkr.product.grpc.InlineRun;
import io.github.will413028.hawkr.product.grpc.ProductDescription;
import io.github.will413028.hawkr.product.grpc.ProductDraft;
import io.github.will413028.hawkr.product.grpc.ProductSkuImport;
import io.github.will413028.hawkr.product.grpc.ShippingMethod;
import io.github.will413028.hawkr.product.grpc.ValidateProductDraftRequest.ValidationContext;
import io.github.will413028.hawkr.product.grpc.client.ShippingGrpcClient;
import io.github.will413028.hawkr.product.service.CategoryCacheService;
import io.github.will413028.hawkr.product.service.CategoryService;
import io.github.will413028.hawkr.product.service.GovCertificationService;
import com.google.protobuf.InvalidProtocolBufferException;
import org.springframework.stereotype.Component;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.List;
import java.util.Optional;

/**
 * 商品上架共用驗證器（40 號 §4.3a）。
 *
 * <p>從 {@link io.github.will413028.hawkr.product.service.ProductImportService#doCreate} 抽出，
 * 並補上 §4.3a 規則目錄中所列的新規則。三時機點共用（CSV_PARSE / IMPORT_UPDATE / IMPORT_COMMIT / PUBLISH），
 * 由 {@link ValidationContext} 控制部分規則的嚴格度。
 *
 * <p>與舊邏輯的關鍵差異：
 * <ul>
 *   <li>違規一律 <b>收集</b> 後回傳，不再首錯即拋例外 — 由 caller 自行決定是否阻擋。</li>
 *   <li>{@link ValidationResult#hasBlockingError()} = true 才阻擋寫入。</li>
 *   <li>欄位 code 嚴格對齊 §4.3a 規則目錄表 — 用於落地 {@code product_import_item_error.error_type}。</li>
 * </ul>
 *
 * <p>本 validator 不負責 hashtag 字面格式 / SKU 內部欄位的詳細格式檢核（仍由 caller 透過
 * {@link HashtagInputValidator} 等做 fail-fast）；本層僅針對 §4.3a 規則目錄中明列的 code 做覆蓋。
 */
@Component
public class ProductPublishValidator {

    private static final ObsLogger log = ObsLogger.forClass(ProductPublishValidator.class);

    /** 名稱長度上限（§4.3a NAME_LENGTH_EXCEEDS）。 */
    public static final int MAX_NAME_LENGTH = 80;

    /** Gallery 圖片數量上限（§4.3a GALLERY_EXCEEDS_LIMIT，常數對應設計書 {@code ProductPublishConstants.MAX_GALLERY_IMAGES}）。 */
    public static final int MAX_GALLERY_IMAGES = 9;

    /** SKU 數量上限（與 02 號設計書對齊；超過則 SKUS_EXCEEDS_LIMIT）。 */
    public static final int MAX_SKUS = 50;

    // description table / list block 上限（對齊 02 §3.6 / ProductService.validateDescriptionDtoTextBlocks）
    private static final int MAX_TABLE_ROWS = 50;
    private static final int MAX_TABLE_COLS = 10;
    private static final int MAX_CELL_TEXT_LEN = 200;
    private static final int MAX_LIST_ITEMS = 50;
    private static final int MAX_DESC_JSON_BYTES = 20 * 1024;
    private static final com.fasterxml.jackson.databind.ObjectMapper DESC_MAPPER =
            new com.fasterxml.jackson.databind.ObjectMapper();

    private final CategoryService categoryService;
    private final CategoryCacheService categoryCacheService;
    private final GovCertificationService govCertificationService;
    private final ShippingGrpcClient shippingGrpcClient;

    public ProductPublishValidator(CategoryService categoryService,
                                   CategoryCacheService categoryCacheService,
                                   GovCertificationService govCertificationService,
                                   ShippingGrpcClient shippingGrpcClient) {
        this.categoryService = categoryService;
        this.categoryCacheService = categoryCacheService;
        this.govCertificationService = govCertificationService;
        this.shippingGrpcClient = shippingGrpcClient;
    }

    /**
     * 驗證入口。
     *
     * @param storeId 賣家 store 代理鍵（用於 ShippingValidation 等需賣家上下文的規則）
     * @param draft     待驗證 draft
     * @param context   呼叫時機點；validator 依此放寬部分規則（如 CSV_PARSE 不檢 GALLERY_EXCEEDS_LIMIT）
     */
    public ValidationResult validate(long storeId, ProductDraft draft, ValidationContext context) {
        List<Violation> violations = new ArrayList<>();
        if (draft == null) {
            violations.add(error("draft", "DRAFT_REQUIRED", "draft 不可為空", null));
            return new ValidationResult(violations);
        }
        ValidationContext ctx = context == null ? ValidationContext.UNSPECIFIED : context;

        validateName(draft, violations);
        validateCategory(draft, violations);
        validateGallery(draft, ctx, violations);
        validateSkus(storeId, draft, violations);
        validateCertifications(draft, violations);
        validateDescription(draft, violations);
        validateHashtags(draft, violations);
        validateTargetStatus(draft, violations);

        return new ValidationResult(violations);
    }

    // ---------------- NAME ----------------

    private void validateName(ProductDraft draft, List<Violation> violations) {
        String name = draft.getName();
        if (name == null || name.isBlank()) {
            violations.add(error("name", "NAME_REQUIRED", "商品名稱不可為空", name));
            return;
        }
        if (name.length() > MAX_NAME_LENGTH) {
            violations.add(error("name", "NAME_LENGTH_EXCEEDS",
                    "商品名稱長度超過 " + MAX_NAME_LENGTH + " 字", name));
        }
    }

    // ---------------- CATEGORY ----------------

    private void validateCategory(ProductDraft draft, List<Violation> violations) {
        String raw = draft.getCategoryId();
        if (raw == null || raw.isBlank()) {
            violations.add(error("category_id", "CATEGORY_NOT_FOUND",
                    "category_id 不可為空", raw));
            return;
        }
        Long categoryId;
        try {
            categoryId = Long.parseLong(raw.trim());
        } catch (NumberFormatException e) {
            violations.add(error("category_id", "CATEGORY_NOT_FOUND",
                    "category_id 必須為數字", raw));
            return;
        }
        Optional<CategoryEntity> leafOpt = categoryCacheService.findById(categoryId);
        if (leafOpt.isEmpty()) {
            violations.add(error("category_id", "CATEGORY_NOT_FOUND",
                    "category_id=" + categoryId + " 不存在", raw));
            return;
        }
        CategoryEntity leaf = leafOpt.get();
        if (!io.github.will413028.hawkr.common.utils.code.CommonStatus.ACTIVE.value().equals(leaf.getStatus())) {
            violations.add(error("category_id", "CATEGORY_NOT_FOUND",
                    "category_id=" + categoryId + " 已停用", raw));
            return;
        }
        if (categoryService.detectCategoryLevel(categoryId) != 3) {
            violations.add(error("category_id", "CATEGORY_NOT_LEAF",
                    "category_id=" + categoryId + " 非葉節點", raw));
        }
    }

    // ---------------- GALLERY ----------------

    private void validateGallery(ProductDraft draft, ValidationContext ctx, List<Violation> violations) {
        List<GalleryItem> gallery = draft.getGalleryList();
        if (gallery == null || gallery.isEmpty()) {
            violations.add(error("gallery_images", "GALLERY_REQUIRED",
                    "商品圖至少 1 張", null));
            return;
        }
        // CSV_PARSE 階段已自行截斷至 9 張並補 WARNING；validator 不重複觸發
        if (ctx != ValidationContext.CSV_PARSE && gallery.size() > MAX_GALLERY_IMAGES) {
            violations.add(error("gallery_images", "GALLERY_EXCEEDS_LIMIT",
                    "商品圖數量超過上限 " + MAX_GALLERY_IMAGES + "（當前 " + gallery.size() + "）",
                    String.valueOf(gallery.size())));
        }
        // ASSET_NOT_READY 僅 IMPORT_COMMIT 嚴格檢
        // TODO(40 §4.3a)：commit 階段應對每筆 gallery item 透過 media-service 確認 asset 已 UPLOADED。
        // 目前 product-service 未持有 media-service client（MediaGrpcClient 屬 product-service 既有但限商品/上傳場景），
        // import-service 端 §5.7 commit worker 在呼叫本 RPC 前已透過 import_asset.status 自行確認；
        // PUBLISH context 走 staged_token + finalize 流程，asset 必為 UPLOADED。
        // 待 media-service 暴露 BatchGetAssetStatusInternal 後補上 product-service 端 final guard。
    }

    // ---------------- SKUS ----------------

    private void validateSkus(long storeId, ProductDraft draft, List<Violation> violations) {
        List<ProductSkuImport> skus = draft.getSkusList();
        if (skus == null || skus.isEmpty()) {
            violations.add(error("skus", "SKUS_REQUIRED", "至少需要 1 個 SKU", null));
            return;
        }
        if (skus.size() > MAX_SKUS) {
            violations.add(error("skus", "SKUS_EXCEEDS_LIMIT",
                    "SKU 數量超過上限 " + MAX_SKUS + "（當前 " + skus.size() + "）",
                    String.valueOf(skus.size())));
            return;
        }
        for (int i = 0; i < skus.size(); i++) {
            ProductSkuImport sku = skus.get(i);
            validateSkuPrice(sku, i, violations);
            validateSkuShippingTemperatureBasics(sku, i, violations);
        }
        // 賣家層級的 shipping 開通與材積限制 — 一次性批次呼叫 shipping-service
        validateShippingEnabled(storeId, skus, violations);
    }

    private void validateSkuPrice(ProductSkuImport sku, int idx, List<Violation> violations) {
        String price = sku.getPrice();
        Long priceValue = parsePositiveLong(price);
        if (priceValue == null) {
            violations.add(error("skus[" + idx + "].price", "PRICE_INVALID",
                    "售價必須為大於 0 的整數", price));
            return;
        }
        String salePrice = sku.getSalePrice();
        if (salePrice != null && !salePrice.isBlank()) {
            try {
                long sale = Long.parseLong(salePrice.trim());
                if (sale <= 0 || sale >= priceValue) {
                    violations.add(error("skus[" + idx + "].sale_price", "PRICE_INVALID",
                            "特價必須 > 0 且 < 售價", salePrice));
                }
            } catch (NumberFormatException e) {
                violations.add(error("skus[" + idx + "].sale_price", "PRICE_INVALID",
                        "特價必須為整數", salePrice));
            }
        }
    }

    private void validateSkuShippingTemperatureBasics(ProductSkuImport sku, int idx, List<Violation> violations) {
        // 配送方式溫層 enum 合法性（值不合法本身不在 §4.3a 規則目錄，但為避免 downstream NPE，
        // 在此標為 SHIPPING_NOT_ENABLED 一併彙整給賣家；後續會再透過 shipping-service 做業務驗證）
        for (ShippingMethod sm : sku.getShippingMethodsList()) {
            if (sm.getTemperature() != null && !sm.getTemperature().isBlank()) {
                try {
                    CodeMetaValidator.requireValidValue(
                            ShippingTemperature.class, sm.getTemperature(), "sku.shipping_methods.temperature");
                } catch (IllegalArgumentException e) {
                    violations.add(error("skus[" + idx + "].shipping_methods.temperature",
                            "SHIPPING_NOT_ENABLED",
                            "配送溫層代碼不合法：" + sm.getTemperature(), sm.getTemperature()));
                }
            }
        }
        if (sku.getUnit() != null && !sku.getUnit().isBlank()) {
            try {
                CodeMetaValidator.requireValidValue(WeightUnit.class, sku.getUnit(), "sku.unit");
            } catch (IllegalArgumentException e) {
                violations.add(error("skus[" + idx + "].unit", "PRICE_INVALID",
                        "單位代碼不合法：" + sku.getUnit(), sku.getUnit()));
            }
        }
    }

    /**
     * 透過 shipping-service ValidateProductShipping 一次批次驗證所有 SKU 的配送方式：
     * <ul>
     *   <li>賣家是否已開通該配送方式 → 未開通則 SHIPPING_NOT_ENABLED (ERROR)</li>
     *   <li>材積 / 重量是否超出該配送方式限制 → 超出則 SHIPPING_DIM_WEIGHT_TEMP_DOWNGRADED (WARNING)</li>
     * </ul>
     *
     * <p>shipping-service 失敗時降級：log warn 但不阻擋 validate（避免 shipping 暫時不可用阻斷上架評估）。
     * IMPORT_COMMIT context 由 caller 端另行加固（defense-in-depth）。
     */
    private void validateShippingEnabled(long storeId, List<ProductSkuImport> skus, List<Violation> violations) {
        if (storeId <= 0) {
            // 缺賣家上下文（理論上 import / publish 兩路徑都會帶）— 直接放過，但留 log
            log.warn("ProductPublishValidator.validateShippingEnabled.skip",
                    "缺 storeId，跳過 shipping-service 驗證");
            return;
        }
        List<io.github.will413028.hawkr.shipping.grpc.ValidateShippingItem> items = new ArrayList<>();
        // index 對應 skus 索引以便回填 violations
        List<int[]> indexMap = new ArrayList<>(); // [skuIdx, methodIdx]
        for (int i = 0; i < skus.size(); i++) {
            ProductSkuImport sku = skus.get(i);
            String weightGrams = toGramsString(sku.getWeight(), sku.getUnit());
            for (int m = 0; m < sku.getShippingMethodsCount(); m++) {
                ShippingMethod sm = sku.getShippingMethods(m);
                if (sm.getType() == null || sm.getType().isBlank()) {
                    continue;
                }
                items.add(io.github.will413028.hawkr.shipping.grpc.ValidateShippingItem.newBuilder()
                        .setSkuName(sku.getSkuName() == null ? "sku#" + i : sku.getSkuName())
                        .setShippingMethod(sm.getType())
                        .setTemperature(sm.getTemperature() == null ? "" : sm.getTemperature())
                        .setLengthCm(nullSafe(sku.getLengthCm()))
                        .setWidthCm(nullSafe(sku.getWidthCm()))
                        .setHeightCm(nullSafe(sku.getHeightCm()))
                        .setWeightGrams(weightGrams)
                        .build());
                indexMap.add(new int[]{i, m});
            }
        }
        if (items.isEmpty()) {
            return;
        }
        io.github.will413028.hawkr.shipping.grpc.ValidateProductShippingResponse resp;
        try {
            resp = shippingGrpcClient.validateProductShipping(storeId, items);
        } catch (Exception e) {
            // 22 號設計書 §3.6：shipping-service 不可用 / 回 INVALID_PARAM → degraded return，不阻擋上架；
            // log ERROR 標明降級事件供查（非單純 warn，避免被當無事件忽略）
            log.error("ProductPublishValidator.validateShippingEnabled.degraded",
                    "shipping-service 呼叫失敗，跳過 shipping 驗證：" + e.getMessage(), e);
            return;
        }
        if (resp == null || resp.getAllValid()) {
            return;
        }
        for (int k = 0; k < resp.getResultsCount() && k < indexMap.size(); k++) {
            io.github.will413028.hawkr.shipping.grpc.ValidateShippingResult r = resp.getResults(k);
            if (r.getValid()) continue;
            int[] map = indexMap.get(k);
            String field = "skus[" + map[0] + "].shipping_methods[" + map[1] + "]";
            String reason = r.getRejectionReason();
            String msg = r.getRejectionMessage() == null || r.getRejectionMessage().isBlank()
                    ? "配送方式驗證失敗：" + reason
                    : r.getRejectionMessage();
            if ("DIMENSION_SUM_EXCEEDED".equals(reason)
                    || "SINGLE_SIDE_EXCEEDED".equals(reason)
                    || "WEIGHT_EXCEEDED".equals(reason)) {
                violations.add(warning(field, "SHIPPING_DIM_WEIGHT_TEMP_DOWNGRADED", msg, r.getShippingMethod()));
            } else {
                // NOT_ONBOARDED / METHOD_NOT_SUPPORTED / 其他 → SHIPPING_NOT_ENABLED
                violations.add(error(field, "SHIPPING_NOT_ENABLED", msg, r.getShippingMethod()));
            }
        }
    }

    private static String toGramsString(String weight, String unit) {
        if (weight == null || weight.isBlank()) {
            return "0";
        }
        try {
            BigDecimal w = new BigDecimal(weight.trim());
            // KG → 1000，G → 1；其他單位（如 ML / 件）以原值轉 g 估算（CSV 路徑會 fail-fast 在 CodeMetaValidator）
            if (unit != null && "KG".equalsIgnoreCase(unit)) {
                w = w.multiply(BigDecimal.valueOf(1000));
            }
            return w.toBigInteger().toString();
        } catch (NumberFormatException e) {
            return "0";
        }
    }

    // ---------------- CERTIFICATIONS ----------------

    /**
     * 認證驗證（設計書 19 §6.5.3）。
     *
     * <p>🔴 <b>本方法一律不發外部呼叫。</b>它跑在 IMPORT_COMMIT／PUBLISH 等<b>同步寫入路徑</b>上
     * （{@code ProductImportService} 的 commit final guard ＋ {@code ProductGrpcService.validateProductDraft}）。
     * 舊版在這裡直接呼叫 {@code govCertificationService.verify()}：一旦逾時／IO 例外就會打開
     * per-type、TTL 60 秒、<b>Redis 全域共享</b>的 circuit breaker ⇒ 緊接著排出去的背景查驗
     * 落在那 60 秒窗口內被短路成 UNAVAILABLE ⇒ §8.3 整套背景查驗機制被同步階段自己封死。
     * ⇒ 改為<b>只讀 §8.3.1 的結構化快取</b>（{@link GovCertificationService#readCachedResult}）。
     * 「零外部呼叫」因此是結構保證，不是一句約定。
     *
     * <p>兩個發出點拆碼且都降為 warning：
     * <ul>
     *   <li>類型不合法 → 既有的 {@code CERT_ENUM_INVALID}（既有文案「已自動略過該筆認證」正好吻合）</li>
     *   <li>查無／已失效 → 維持 {@code CERT_VERIFY_FAILED}，語意改為「已轉送人工審核」</li>
     * </ul>
     * 兩者共用一碼的話只能有一份文案，而它們要說的是兩件不同的事，必有一邊是錯的。
     *
     * <p>⚠️ {@code CERT_VERIFY_PENDING} 的 catch 分支已隨外部呼叫一併移除（不再有例外可 catch）。
     * <b>碼值本身保留在 CodeMeta，不得刪除</b>：{@code product_import_item_error} 既有列可能仍存著它，
     * 刪碼會讓歷史列的 {@code error_type} 變成無法翻譯的孤兒值（與 40 號 §11.3a 一致）。
     */
    private void validateCertifications(ProductDraft draft, List<Violation> violations) {
        List<Certification> certs = draft.getCertificationsList();
        for (int i = 0; i < certs.size(); i++) {
            Certification cert = certs.get(i);
            String name = cert.getName();
            String certNo = cert.getCertNo();
            if (name == null || name.isBlank() || certNo == null || certNo.isBlank()) {
                continue;
            }
            CertificationType type;
            try {
                CodeMetaValidator.requireValidValue(CertificationType.class, name, "certification.name");
                type = CertificationType.fromValue(name);
            } catch (IllegalArgumentException e) {
                violations.add(warning("certifications[" + i + "].name", "CERT_ENUM_INVALID",
                        "認證代碼不在可選清單內，已自動略過該筆認證：" + name, name));
                continue;
            }

            // 只讀結構化快取：命中且有效 → 無事；未命中 → 無事（交給背景查驗，§8.2）。
            GovCertificationService.GovVerifyResult cached =
                    govCertificationService.readCachedResult(certNo, type);
            if (cached != null && !cached.exists()) {
                violations.add(warning("certifications[" + i + "].cert_no", "CERT_VERIFY_FAILED",
                        "認證編號查驗未通過（" + cached.validity() + "），已轉送人工審核：" + certNo, certNo));
            }
        }
    }

    // ---------------- DESCRIPTION ----------------

    private void validateDescription(ProductDraft draft, List<Violation> violations) {
        com.google.protobuf.ByteString bytes = draft.getDescriptionJson();
        if (bytes == null || bytes.isEmpty()) {
            return;
        }
        // ProductDraft.description_json 為 ProductDescription proto 序列化後 bytes；
        // 解析失敗 → DESC_BLOCK_INVALID；text block 含 HTML tag → DESC_BLOCK_INVALID
        ProductDescription desc;
        try {
            desc = ProductDescription.parseFrom(bytes);
        } catch (InvalidProtocolBufferException e) {
            violations.add(error("description_json", "DESC_BLOCK_INVALID",
                    "商品描述格式無法辨識，請重新編輯商品描述", null));
            return;
        }
        validateDescriptionBlocks(desc, violations);
    }

    /**
     * inline-run 內容模型（02 §3.6）：text（runs）/ image / table（tableJson）/ list（listJson）四型別。
     * text block 內容存於 {@code runs}（inline-run），legacy {@code content} 僅相容 fallback；table / list 為
     * opaque JSON 字串，上限與 run 內文驗證對齊 {@link io.github.will413028.hawkr.product.service.ProductService} 的 DTO 版。
     */
    private void validateDescriptionBlocks(ProductDescription desc, List<Violation> violations) {
        List<DescriptionBlock> blocks = desc.getBlocksList();
        for (int i = 0; i < blocks.size(); i++) {
            DescriptionBlock block = blocks.get(i);
            String type = block.getType();
            String base = "description_json.blocks[" + i + "]";
            if (type == null || type.isBlank()) {
                violations.add(error(base + ".type", "DESC_BLOCK_INVALID", "description block type 不可為空", type));
                continue;
            }
            switch (type) {
                case "text" -> validateTextBlockRuns(block, base, violations);
                case "table" -> validateTableBlockJson(block.getTableJson(), base, violations);
                case "list" -> validateListBlockJson(block.getListJson(), base, violations);
                case "image" -> { /* 圖片 block 無文字驗證 */ }
                default -> violations.add(error(base + ".type", "DESC_BLOCK_INVALID",
                        "description block type 不合法（僅支援 text/image/table/list）：" + type, type));
            }
        }
    }

    /** text block：runs 逐一驗 text 禁 HTML + href scheme；runs 缺時 fallback 驗 legacy content。 */
    private void validateTextBlockRuns(DescriptionBlock block, String base, List<Violation> violations) {
        if (block.getRunsCount() > 0) {
            for (InlineRun run : block.getRunsList()) {
                checkRunText(run.getText(), base + ".content", violations);
                checkHref(run.getHref(), base + ".content", violations);
            }
        } else {
            checkRunText(block.getContent(), base + ".content", violations);
        }
    }

    /** table block：tableJson 上限 + cell run 驗證（對齊 02 §3.6 / #1255）。 */
    private void validateTableBlockJson(String tableJson, String base, List<Violation> violations) {
        if (tableJson == null || tableJson.isBlank()) {
            violations.add(error(base + ".tableJson", "DESC_BLOCK_INVALID", "表格內容不可為空", null));
            return;
        }
        if (tableJson.getBytes(java.nio.charset.StandardCharsets.UTF_8).length > MAX_DESC_JSON_BYTES) {
            violations.add(error(base + ".tableJson", "DESC_BLOCK_INVALID", "表格資料過大（上限 20KB）", null));
            return;
        }
        final com.fasterxml.jackson.databind.JsonNode root;
        try {
            root = DESC_MAPPER.readTree(tableJson);
        } catch (Exception e) {
            violations.add(error(base + ".tableJson", "DESC_BLOCK_INVALID", "表格資料格式錯誤", null));
            return;
        }
        com.fasterxml.jackson.databind.JsonNode rows = root.path("rows");
        if (!rows.isArray() || rows.isEmpty()) {
            violations.add(error(base + ".tableJson", "DESC_BLOCK_INVALID", "表格至少需 1 列 1 欄", null));
            return;
        }
        if (rows.size() > MAX_TABLE_ROWS) {
            violations.add(error(base + ".tableJson", "DESC_BLOCK_INVALID",
                    "表格列數超出上限（最多 " + MAX_TABLE_ROWS + " 列）", null));
        }
        for (com.fasterxml.jackson.databind.JsonNode row : rows) {
            if (!row.isArray() || row.isEmpty()) {
                violations.add(error(base + ".tableJson", "DESC_BLOCK_INVALID", "表格至少需 1 列 1 欄", null));
                continue;
            }
            if (row.size() > MAX_TABLE_COLS) {
                violations.add(error(base + ".tableJson", "DESC_BLOCK_INVALID",
                        "表格欄數超出上限（最多 " + MAX_TABLE_COLS + " 欄）", null));
            }
            for (com.fasterxml.jackson.databind.JsonNode cell : row) {
                com.fasterxml.jackson.databind.JsonNode cellRuns = cell.path("runs");
                if (!cellRuns.isArray()) continue;
                StringBuilder cellText = new StringBuilder();
                for (com.fasterxml.jackson.databind.JsonNode run : cellRuns) {
                    String t = run.path("text").asText("");
                    cellText.append(t);
                    checkRunText(t, base + ".tableJson", violations);
                    checkHref(run.path("href").asText(null), base + ".tableJson", violations);
                }
                if (cellText.length() > MAX_CELL_TEXT_LEN) {
                    violations.add(error(base + ".tableJson", "DESC_BLOCK_INVALID",
                            "表格單一儲存格字數超出上限（最多 " + MAX_CELL_TEXT_LEN + " 字）", null));
                }
            }
        }
    }

    /** list block：listJson 上限 + item run 驗證（對齊 02 §3.6 / #1257 / #1258）。 */
    private void validateListBlockJson(String listJson, String base, List<Violation> violations) {
        if (listJson == null || listJson.isBlank()) {
            violations.add(error(base + ".listJson", "DESC_BLOCK_INVALID", "清單內容不可為空", null));
            return;
        }
        if (listJson.getBytes(java.nio.charset.StandardCharsets.UTF_8).length > MAX_DESC_JSON_BYTES) {
            violations.add(error(base + ".listJson", "DESC_BLOCK_INVALID", "清單資料過大（上限 20KB）", null));
            return;
        }
        final com.fasterxml.jackson.databind.JsonNode root;
        try {
            root = DESC_MAPPER.readTree(listJson);
        } catch (Exception e) {
            violations.add(error(base + ".listJson", "DESC_BLOCK_INVALID", "清單資料格式錯誤", null));
            return;
        }
        com.fasterxml.jackson.databind.JsonNode items = root.path("items");
        if (!items.isArray() || items.isEmpty()) {
            violations.add(error(base + ".listJson", "DESC_BLOCK_INVALID", "清單至少需 1 個項目", null));
            return;
        }
        if (items.size() > MAX_LIST_ITEMS) {
            violations.add(error(base + ".listJson", "DESC_BLOCK_INVALID",
                    "清單項目數超出上限（最多 " + MAX_LIST_ITEMS + " 項）", null));
        }
        for (com.fasterxml.jackson.databind.JsonNode item : items) {
            com.fasterxml.jackson.databind.JsonNode itemRuns = item.path("runs");
            if (!itemRuns.isArray()) continue;
            StringBuilder itemText = new StringBuilder();
            for (com.fasterxml.jackson.databind.JsonNode run : itemRuns) {
                String t = run.path("text").asText("");
                itemText.append(t);
                checkRunText(t, base + ".listJson", violations);
                checkHref(run.path("href").asText(null), base + ".listJson", violations);
            }
            if (itemText.length() > MAX_CELL_TEXT_LEN) {
                violations.add(error(base + ".listJson", "DESC_BLOCK_INVALID",
                        "清單單一項目字數超出上限（最多 " + MAX_CELL_TEXT_LEN + " 字）", null));
            }
        }
    }

    /** run text 禁 HTML tag → DESC_BLOCK_INVALID 違規。 */
    private void checkRunText(String text, String field, List<Violation> violations) {
        if (text == null) return;
        try {
            InputValidator.requireNoHtmlTags(text, "商品描述");
        } catch (IllegalArgumentException e) {
            violations.add(error(field, "DESC_BLOCK_INVALID", e.getMessage(), text));
        }
    }

    /** href 僅接受 http(s):// 絕對 URL；空值放行。 */
    private void checkHref(String href, String field, List<Violation> violations) {
        if (href == null || href.isBlank()) return;
        String lower = href.trim().toLowerCase();
        if (!lower.startsWith("http://") && !lower.startsWith("https://")) {
            violations.add(error(field, "DESC_BLOCK_INVALID",
                    "商品描述連結網址格式不支援（僅接受 http/https）", href));
        }
    }

    // ---------------- HASHTAGS ----------------

    private void validateHashtags(ProductDraft draft, List<Violation> violations) {
        List<String> hashtags = draft.getHashtagsList();
        if (hashtags == null || hashtags.isEmpty()) {
            return;
        }
        try {
            HashtagInputValidator.requireHashtagSet(new ArrayList<>(hashtags));
        } catch (Exception e) {
            // §4.3a 規則目錄未列 hashtag 專屬 code，併入 DESC_BLOCK_INVALID 不合適 — 標為 NAME_REQUIRED 也不對；
            // 為了不偏離 §4.3a 表，hashtag 格式錯誤這裡僅 log，由 caller 端 fail-fast（既有 doCreate 邏輯保留）
            log.warn("ProductPublishValidator.validateHashtags.invalid",
                    "hashtag 格式錯誤（不列入 validator 違規，留給 caller fail-fast）：" + e.getMessage());
        }
    }

    // ---------------- TARGET STATUS ----------------

    private void validateTargetStatus(ProductDraft draft, List<Violation> violations) {
        String raw = draft.getTargetStatus();
        if (raw == null || raw.isBlank()) {
            violations.add(error("target_status", "TARGET_STATUS_INVALID",
                    "target_status 為必填（ACTIVE / DRAFT / PENDING_REVIEW）", raw));
            return;
        }
        if (!ProductStatus.isValidCode(raw) && !ProductStatus.isValidValue(raw)) {
            violations.add(error("target_status", "TARGET_STATUS_INVALID",
                    "target_status 不合法：" + raw, raw));
            return;
        }
        ProductStatus ps = ProductStatus.isValidCode(raw)
                ? ProductStatus.fromCode(raw)
                : ProductStatus.fromValue(raw);
        if (ps == ProductStatus.INACTIVE) {
            violations.add(error("target_status", "TARGET_STATUS_INVALID",
                    "target_status 不可為 INACTIVE", raw));
        }
    }

    // ---------------- 共用 helper ----------------

    private static Long parsePositiveLong(String raw) {
        if (raw == null || raw.isBlank()) return null;
        try {
            long v = Long.parseLong(raw.trim());
            return v > 0 ? v : null;
        } catch (NumberFormatException e) {
            return null;
        }
    }

    private static String nullSafe(String raw) {
        return raw == null ? "" : raw;
    }

    private static Violation error(String field, String code, String message, String rawValue) {
        return new Violation(field, code, "ERROR", message, rawValue == null ? "" : rawValue);
    }

    private static Violation warning(String field, String code, String message, String rawValue) {
        return new Violation(field, code, "WARNING", message, rawValue == null ? "" : rawValue);
    }

    // ==================== 巢狀型別 ====================

    /**
     * 單筆違規（對應 proto {@code DraftViolation}）。
     */
    public record Violation(String field, String code, String severity, String message, String rawValue) {}

    /**
     * 驗證結果集合。
     */
    public record ValidationResult(List<Violation> violations) {

        public ValidationResult {
            violations = violations == null ? List.of() : List.copyOf(violations);
        }

        /** 是否含任一 severity=ERROR 的違規（caller 用此決定是否阻擋寫入）。 */
        public boolean hasBlockingError() {
            for (Violation v : violations) {
                if ("ERROR".equals(v.severity())) return true;
            }
            return false;
        }

        public int errorCount() {
            int n = 0;
            for (Violation v : violations) if ("ERROR".equals(v.severity())) n++;
            return n;
        }
    }
}
