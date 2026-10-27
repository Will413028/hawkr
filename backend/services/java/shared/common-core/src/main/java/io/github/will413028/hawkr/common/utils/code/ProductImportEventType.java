package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * 商品匯入精靈服務的 MQ 事件類型，對齊 `{publisher_service}.{domain}.{action}` 命名規則。Code 與 Value 一致（UPPER_SNAKE_CASE），與 routing key 不同 — routing key 為點分式 `csv.parse` / `html.fetch` / `asset.download` / `commit` / `gc.expire_job`，寫於 `MqConstants`；event_type 作 ObsLogger event name 與 trace metadata。對齊 OrderEventType / BuyerEventType 設計。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProductImportEventType implements CodeMeta {

    PRODUCT_IMPORT_CSV_PARSE("PRODUCT_IMPORT_CSV_PARSE", "PRODUCT_IMPORT_CSV_PARSE", "CSV 解析事件", "Routing key `csv.parse`；CreateImportJob 入口 publish"),
    PRODUCT_IMPORT_HTML_FETCH("PRODUCT_IMPORT_HTML_FETCH", "PRODUCT_IMPORT_HTML_FETCH", "HTML 拉取事件", "Routing key `html.fetch`；description_source=URL 時 csv.parse worker publish"),
    PRODUCT_IMPORT_ASSET_DOWNLOAD("PRODUCT_IMPORT_ASSET_DOWNLOAD", "PRODUCT_IMPORT_ASSET_DOWNLOAD", "圖片下載派工事件", "Routing key `asset.download`；多入口 publisher（csv.parse / html.fetch / Edit / Retry）"),
    PRODUCT_IMPORT_COMMIT("PRODUCT_IMPORT_COMMIT", "PRODUCT_IMPORT_COMMIT", "轉正事件", "Routing key `commit`；CommitImportJob 入口 publish"),
    PRODUCT_IMPORT_GC_EXPIRE_JOB("PRODUCT_IMPORT_GC_EXPIRE_JOB", "PRODUCT_IMPORT_GC_EXPIRE_JOB", "過期清理事件（跨 Media 消費）", "Routing key `gc.expire_job`；ProductImportGcJob publish，**media-service consume**"),
    PRODUCT_IMPORT_JOB_PARSED("PRODUCT_IMPORT_JOB_PARSED", "PRODUCT_IMPORT_JOB_PARSED", "解析+下載完成推播（跨團隊 notification）", "Routing key `job.parsed`；MediaCallbackListener / CallbackTimeoutGc 在 job 第一次完成解析+下載時 publish，每個 job 只發一次（job.parsed_event_published_at flag）"),
    PRODUCT_IMPORT_JOB_COMMIT_FINISHED("PRODUCT_IMPORT_JOB_COMMIT_FINISHED", "PRODUCT_IMPORT_JOB_COMMIT_FINISHED", "commit 完成推播（跨團隊 notification）", "Routing key `job.commit_finished`；commit worker 在 job 由 COMMITTING 轉出時 publish（每輪 commit 完成都發）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProductImportEventType(String code, String value, String label, String description) {
        this.code = code;
        this.value = value;
        this.label = label;
        this.description = description;
    }

    @Override
    public String code() { return code; }

    @Override
    public String value() { return value; }

    @Override
    public String label() { return label; }

    @Override
    public String description() { return description; }

    private static final Map<String, ProductImportEventType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductImportEventType::code, e -> e));

    private static final Map<String, ProductImportEventType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProductImportEventType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProductImportEventType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProductImportEventType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
