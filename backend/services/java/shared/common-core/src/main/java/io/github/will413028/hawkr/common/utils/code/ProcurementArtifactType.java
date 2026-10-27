package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Hawkr Agent typed artifact 的種類（doc 57 §4.5）。每個值對應一份獨立、versioned 的 `data` payload 形狀；Track A／Track B **不得共用一個 generic schema**（例如 `RFQ_PUBLICATION_APPROVAL` 與 `CART_APPROVAL` 概念上都是「核准」，但欄位形狀必須各自獨立定義，不得抽出共用 `ApprovalData`）。對應的 Go 型別定義見 `services/ai-service/internal/procurement/domain/artifact.go`。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementArtifactType implements CodeMeta {

    REQUEST_SNAPSHOT("REQUEST_SNAPSHOT", "REQUEST_SNAPSHOT", "需求快照", "Track A。某一版 `procurement_request_revisions` 的公開投影（`slots`／`slotsHash`／`revisionNo`），`data` 形狀已可從既有 `domain.Slots` 推導"),
    CLARIFICATION("CLARIFICATION", "CLARIFICATION", "澄清問題", "Track A。承載 `domain.ClarifyQuestion` 的公開投影"),
    CANDIDATE_RESULTS("CANDIDATE_RESULTS", "CANDIDATE_RESULTS", "候選結果", "Track A。PLATFORM／EXTERNAL 兩條 lane 的候選清單，`data` 形狀已可從既有 `domain.CandidateLead` 欄位挑選層推導（不得含 email／phone／social／聯絡人，見 doc 57 §1.3）"),
    PRICING_PREVIEW("PRICING_PREVIEW", "PRICING_PREVIEW", "定價預覽", "Track A。上游流程屬 #2403（doc 57 §5.7），`data` 形狀**尚未展開**——本次只凍結型別名稱與共同 envelope"),
    CONFIRMED_RFQ("CONFIRMED_RFQ", "CONFIRMED_RFQ", "已確認詢價單", "Track A。#2403（doc 57 §5.7）的「不可變需求確認＝詢價單本體」，凍結 `snapshot`／`hash`／版本形狀時須讓商務線 CAS（`expectedRfqSnapshotHash`）用得上；`data` 形狀**尚未展開**"),
    RFQ_PUBLICATION_APPROVAL("RFQ_PUBLICATION_APPROVAL", "RFQ_PUBLICATION_APPROVAL", "發布核准", "Track A。屬商務線 #2450，`data` 形狀**尚未展開**；核准不得走 generic resume（doc 57 §4.6）"),
    RFQ_PUBLICATION_RESULT("RFQ_PUBLICATION_RESULT", "RFQ_PUBLICATION_RESULT", "發布結果", "Track A。屬商務線 #2450，`data` 形狀**尚未展開**；`PUBLISHED` 只表示 transaction 完成，Wish Pool indexed／Direct delivered／Seller viewed-quoted 需由 #2539 的 typed evidence 更新"),
    SELLER_QUOTE_RESULT("SELLER_QUOTE_RESULT", "SELLER_QUOTE_RESULT", "賣家報價結果", "Track A。屬商務線 #2538，`data` 形狀**尚未展開**"),
    ERROR_RECOVERY("ERROR_RECOVERY", "ERROR_RECOVERY", "錯誤復原引導", "Track A。承載 `ProcurementErrorReason`／`ProcurementRunFailureReason` 的公開復原建議，`data` 形狀已可從既有錯誤碼登記推導"),
    PURCHASE_PLAN("PURCHASE_PLAN", "PURCHASE_PLAN", "採買計畫", "Track B。上游 Cart 流程未在 doc 57 展開，`data` 形狀**尚未展開**"),
    CART_APPROVAL("CART_APPROVAL", "CART_APPROVAL", "購物車核准", "Track B。`data` 形狀**尚未展開**；不得與 `RFQ_PUBLICATION_APPROVAL` 共用 schema"),
    CART_RESULT("CART_RESULT", "CART_RESULT", "購物車結果", "Track B。`data` 形狀**尚未展開**");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementArtifactType(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementArtifactType> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementArtifactType::code, e -> e));

    private static final Map<String, ProcurementArtifactType> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementArtifactType::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementArtifactType fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementArtifactType fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
