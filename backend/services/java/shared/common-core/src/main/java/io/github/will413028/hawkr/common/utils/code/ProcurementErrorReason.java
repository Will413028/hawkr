package io.github.will413028.hawkr.common.utils.code;

import java.util.*;
import java.util.stream.Collectors;

/**
 * Hawkr Agent 對外錯誤 envelope 的 `reason` 欄位值域，即**機器可讀的錯誤身分**。
 *
 * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。
 * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class
 * （見 docs/開發規範/Java 微服務開發規範.md §6）。
 */
public enum ProcurementErrorReason implements CodeMeta {

    INVALID_ARGUMENT("INVALID_ARGUMENT", "INVALID_ARGUMENT", "參數錯誤", "400。schema／格式驗證失敗的通用碼（invalid requestId、clientRequestId is required…）"),
    PERMISSION_DENIED("PERMISSION_DENIED", "PERMISSION_DENIED", "無權操作", "403。呼叫端身分無法解析或無權存取此 request"),
    RESOURCE_NOT_FOUND("RESOURCE_NOT_FOUND", "RESOURCE_NOT_FOUND", "資源不存在", "404 通用碼。cross-buyer 與亂數 ID 共用同一外形，不洩漏存在性"),
    INTERNAL_ERROR("INTERNAL_ERROR", "INTERNAL_ERROR", "系統錯誤", "500"),
    RATE_LIMITED("RATE_LIMITED", "RATE_LIMITED", "請求過於頻繁", "429。可重試性由 `Retry-After` header 表達（串流路徑因 header 已 flush，改由 error frame 帶延遲）"),
    CAPABILITY_UNAVAILABLE("CAPABILITY_UNAVAILABLE", "CAPABILITY_UNAVAILABLE", "功能暫不可用", "503。capability 關閉時的 typed failure，不得靜默降級成舊行為"),
    SCHEMA_VERSION_UNSUPPORTED("SCHEMA_VERSION_UNSUPPORTED", "SCHEMA_VERSION_UNSUPPORTED", "版本不支援", "400。client 要求的 major schema 版本不受支援"),
    REQUEST_NOT_FOUND("REQUEST_NOT_FOUND", "REQUEST_NOT_FOUND", "需求不存在", "404。採購需求不存在或不屬於此買家"),
    IDEMPOTENCY_KEY_CONFLICT("IDEMPOTENCY_KEY_CONFLICT", "IDEMPOTENCY_KEY_CONFLICT", "冪等鍵衝突", "409。同一 clientRequestId 已綁定另一份 payload。**不是** #2446 原文的 IDEMPOTENCY_KEY_REUSED——單純 reuse（同鍵同內容）在本系統是成功的 replay 路徑，用 REUSED 命名等於拿正常路徑的名字命名錯誤"),
    STALE_REVISION("STALE_REVISION", "STALE_REVISION", "版本已過期", "409。expectedRevisionId 與現行 revision 不符"),
    REQUIREMENT_NOT_READY("REQUIREMENT_NOT_READY", "REQUIREMENT_NOT_READY", "需求尚未齊備", "409。必填欄位未填滿即嘗試進入候選比對"),
    ANSWER_SCHEMA_INVALID("ANSWER_SCHEMA_INVALID", "ANSWER_SCHEMA_INVALID", "回答格式不符", "422。答案不符合該問題卡的 schema（例如複合問題缺 unitOptionId）"),
    SLOT_NOT_SKIPPABLE("SLOT_NOT_SKIPPABLE", "SLOT_NOT_SKIPPABLE", "此欄位不可略過", "422。對必填欄位送出 skip"),
    CROP_CHOICE_INVALID("CROP_CHOICE_INVALID", "CROP_CHOICE_INVALID", "品項選擇無效", "422。crop-disambiguation 選項不在可選集合內"),
    INPUT_TOO_LONG("INPUT_TOO_LONG", "INPUT_TOO_LONG", "輸入過長", "422。訊息或自由文字超過長度上限"),
    AGENT_MONTHLY_QUOTA_EXCEEDED("AGENT_MONTHLY_QUOTA_EXCEEDED", "AGENT_MONTHLY_QUOTA_EXCEEDED", "本月 Agent 額度已用完", "429。本月額度已用盡，回 remaining=0 與 resetAt（下期起始時間）——doc 57 §7"),
    INVALID_OVERRIDE("INVALID_OVERRIDE", "INVALID_OVERRIDE", "覆寫欄位無效", "400。報價單邀請的買家覆寫欄位格式或值不合法"),
    CONVERSATION_TITLE_INVALID("CONVERSATION_TITLE_INVALID", "CONVERSATION_TITLE_INVALID", "對話標題無效", "422。改名的標題為空或超長"),
    CANDIDATE_FACTS_STALE("CANDIDATE_FACTS_STALE", "CANDIDATE_FACTS_STALE", "候選資料已過期", "409。候選事實的 factsAsOf 已早於現行需求版本，須重新比對"),
    LANE_MISMATCH("LANE_MISMATCH", "LANE_MISMATCH", "通道不符", "400。指定的 lane 與候選實際所屬通道不一致"),
    INVITE_ALREADY_RESOLVED("INVITE_ALREADY_RESOLVED", "INVITE_ALREADY_RESOLVED", "邀請已被處理", "409。該報價邀請已被確認或已失效（先到先得）"),
    PRICE_BASIS_UNAVAILABLE("PRICE_BASIS_UNAVAILABLE", "PRICE_BASIS_UNAVAILABLE", "報價基準無法解析", "422。currency/quantity/unit/conversion basis 無法 canonicalize，RFQ 報價預覽的 typed hard block，零 side effect、不建立 preview 列（issue #2403 Slice 2）"),
    RUN_NOT_ACTIVE("RUN_NOT_ACTIVE", "RUN_NOT_ACTIVE", "Run 非執行中", "409。對非 QUEUED/RUNNING/WAITING_FOR_INPUT 狀態的 run 送出需要 active run 的命令"),
    INTERRUPT_EXPIRED("INTERRUPT_EXPIRED", "INTERRUPT_EXPIRED", "中斷已逾期", "409。resume 命令對應的 interrupt 已超過 expiresAt"),
    APPROVAL_STALE("APPROVAL_STALE", "APPROVAL_STALE", "核准依據已過期", "409。resume/確認命令帶的 expectedVersion 與現行版本不符"),
    REPLAY_CURSOR_EXPIRED("REPLAY_CURSOR_EXPIRED", "REPLAY_CURSOR_EXPIRED", "Replay 游標已過期", "410。GET replay 的 Last-Event-ID 早於 minAvailableSeqId"),
    REPLAY_CURSOR_AHEAD("REPLAY_CURSOR_AHEAD", "REPLAY_CURSOR_AHEAD", "Replay 游標超前", "409。GET replay 的 Last-Event-ID 高於目前 high-water mark"),
    APPROVAL_ALREADY_RESOLVED("APPROVAL_ALREADY_RESOLVED", "APPROVAL_ALREADY_RESOLVED", "核准已被處理", "409。對已 resolved 的 interrupt 重複送出 resume/確認命令"),
    RUN_RESULT_NOT_READY("RUN_RESULT_NOT_READY", "RUN_RESULT_NOT_READY", "Run 結果尚未就緒", "422（#3330 起；原為 503 —— 服務端改用 codes.FailedPrecondition，而 procurement 走自己的對照表 gateway/internal/procurement/wire.go，那裡 FailedPrecondition 映 422、409 留給 Aborted）。命令的冪等鍵命中一個已建立的 run，但該 run 沒有可重播的結果。**兩種成因、處置相反，呼叫端不得一律重試**：(1) run 仍在執行中 → 稍後重試**同一把** clientRequestId 可能就緒；(2) run 已以 FAILED 收尾 → 該鍵永遠不會再產生可重播結果，必須改用**新的** clientRequestId 重下命令，重試同一把鍵會無限迴圈。本碼本身不區分兩者（區分需要讀 run 狀態），呼叫端若要自動重試必須設重試上限，或改查 run 狀態再決定。與 `RUN_NOT_ACTIVE` 是兩件事：那個講的是「run 的狀態不允許這個命令」，這個講的是「命令已受理、但沒有結果可回」"),
    APPROVAL_PENDING("APPROVAL_PENDING", "APPROVAL_PENDING", "待回覆的中斷", "409。該需求有一個尚未回覆的 interrupt（run 停在 WAITING_FOR_INPUT），必須改用 POST /runs/{runId}/resume 回覆；/answers 與 /crop-disambiguation 在此期間不得直接寫入，否則會讓 interrupt 孤兒化"),
    STALE_RFQ_SNAPSHOT("STALE_RFQ_SNAPSHOT", "STALE_RFQ_SNAPSHOT", "詢價快照已過期", "409。acceptance 的 expectedRfqSnapshotHash 與現行 confirmed snapshot 不符（需求被其他裝置改過）。零 side effect，不建立任何 acceptance 列（doc 64 §9.2，#2538）"),
    STALE_PUBLICATION("STALE_PUBLICATION", "STALE_PUBLICATION", "發布依據已過期", "409。賣家回價命令帶的 publication id／version／sellerFacingPayloadHash 與現行不符。零 side effect（doc 64 §9.2，#2538）"),
    STALE_SELLER_RESPONSE("STALE_SELLER_RESPONSE", "STALE_SELLER_RESPONSE", "供應商報價已變動", "409。expectedSellerResponseVersion 不是目前版本——賣家已改價或已撤回。⛔ **必須明確告知使用者報價已變動，不得靜默改用最新版重試**（doc 64 §9.2，#2538）"),
    RFQ_RESPONSE_ALREADY_SUBMITTED("RFQ_RESPONSE_ALREADY_SUBMITTED", "RFQ_RESPONSE_ALREADY_SUBMITTED", "已提交過正式回價", "409。賣家帶著「尚無正式回價」的 CAS（空 expectedSellerResponseVersion）提交，但該 publication 上已有一版。改版要帶目前版本（v6.0 Part III §13.1，#2538）"),
    RFQ_RESPONSE_EXPIRED("RFQ_RESPONSE_EXPIRED", "RFQ_RESPONSE_EXPIRED", "報價已逾有效期", "409。被 shortlist 或被接受的 sellerResponseVersion 已過 validUntil。⛔ 不可接受，也不得自動改接受別版（v6.0 Part III §13.1，#2538）"),
    RFQ_RECIPIENT_CLOSED("RFQ_RECIPIENT_CLOSED", "RFQ_RECIPIENT_CLOSED", "該收件人已停止寫入", "409。賣家已明確拒絕（decline），或 publication 已結案／他人成交後停止寫入（v6.0 Part III §13.1，#2538）"),
    RFQ_CHECKOUT_ALREADY_ACTIVE("RFQ_CHECKOUT_ALREADY_ACTIVE", "RFQ_CHECKOUT_ALREADY_ACTIVE", "已有進行中的成交", "409。同一 request 已有 ACTIVE 的 acceptance。回既有 orderNo 讓買家去付款，**不是**叫他重新接受（v6.0 Part III §13.1，#2538）"),
    RFQ_ALREADY_AWARDED("RFQ_ALREADY_AWARDED", "RFQ_ALREADY_AWARDED", "本詢價已成交", "409。同一 request 已有 payment-finalized 的成交。這是真正的終局封鎖，與 RFQ_CHECKOUT_ALREADY_ACTIVE 的差別是後者仍可付款、本碼不可（v6.0 Part III §13.1，#2538）"),
    QUOTE_AMOUNT_INCONSISTENT("QUOTE_AMOUNT_INCONSISTENT", "QUOTE_AMOUNT_INCONSISTENT", "報價金額不一致", "422。totalAmount ≠ goodsAmount + shippingAmount + taxAmount，或金額／幣別無法 canonicalize。零 side effect，不建立版本列（#2538）"),
    QUOTE_QUALITY_REQUIRED("QUOTE_QUALITY_REQUIRED", "QUOTE_QUALITY_REQUIRED", "未填實際品相", "422。正式回價未填 offeredQuality 的等級或驗收說明。⛔ 平台、LLM 或 Pricebook 不得代填——未填就不得成為買家可接受的版本（v6.0 V.7.8，#2538）");

    private final String code;
    private final String value;
    private final String label;
    private final String description;

    ProcurementErrorReason(String code, String value, String label, String description) {
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

    private static final Map<String, ProcurementErrorReason> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementErrorReason::code, e -> e));

    private static final Map<String, ProcurementErrorReason> BY_VALUE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(ProcurementErrorReason::value, e -> e));

    public static boolean isValidCode(String code) {
        return code != null && BY_CODE.containsKey(code);
    }

    public static boolean isValidValue(String value) {
        return value != null && BY_VALUE.containsKey(value);
    }

    public static ProcurementErrorReason fromCode(String code) {
        var e = BY_CODE.get(code);
        if (e == null) throw new IllegalArgumentException("Unknown code: " + code);
        return e;
    }

    public static ProcurementErrorReason fromValue(String value) {
        var e = BY_VALUE.get(value);
        if (e == null) throw new IllegalArgumentException("Unknown value: " + value);
        return e;
    }

    public static List<CodeMeta> codes() {
        return List.of(values());
    }
}
