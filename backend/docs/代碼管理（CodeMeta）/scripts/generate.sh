#!/bin/bash
# =====================================================================
# 代碼管理（CodeMeta）Code Generate 腳本
#
# 解析 Markdown 檔案中的 Enum 定義，生成 Java / Go / Flutter / Web 程式碼。
# 僅依賴 macOS 內建工具（bash, awk, sed, diff）。
#
# 使用方式：
#   ./generate.sh --all            # 生成全部（等同 --java --go --go-errors
#                                  #   --go-media --go-labels --go-registry --flutter
#                                  #   --web --admin；不含 --diff）
#   ./generate.sh --java           # 僅 Java（直接覆蓋 common-core）
#   ./generate.sh --go             # 僅 Go（輸出到 internal/platform/codemeta/：代碼表與單體的錯誤原因）
#   ./generate.sh --go-errors      # 僅 Go 業務碼常數（直寫 pkg/errors/bizcode/bizcode.go）
#   ./generate.sh --go-media       # 僅 Go media 引用類型常數（直寫 pkg/media/ref_type.go 與單體的 internal/modules/media/ref_type.go）
#   ./generate.sh --go-labels      # 僅 Go Value→Label 對照（直寫 pkg/codemeta/labels.go）
#   ./generate.sh --go-registry    # 僅 Go registry：typed 常數 + AllXxx + FromCode/FromValue
#                                  #   （直寫 pkg/codemeta/registry.go；清單見 GO_REGISTRY_ENUMS）
#   ./generate.sh --flutter        # 僅 Flutter（直寫 buyer-app 與 seller-app 的 lib/core/codemeta/code_meta.dart）
#   ./generate.sh --web            # 僅 Web/TypeScript（直寫 website/lib/codemeta/generated.ts）
#   ./generate.sh --admin          # 僅 ADM/TypeScript（直寫 admin/src/lib/codemeta/generated.ts；清單見 ADMIN_ENUMS）
#   ./generate.sh --diff           # 比較生成結果與現行檔案差異（唯讀）
#   ./generate.sh --help           # 列出可用旗標（亦可用 -h）
#
# 以上即全部可接受的參數，傳入其他參數會報錯退出。
# =====================================================================

set -euo pipefail

# 強制 byte-consistent locale：macOS 內建 awk(BWK 20200816) 在 UTF-8 locale 下
# index()/substr() 對多位元組 key（如 `說明`）的 offset 會錯位，導致屬性值（class
# javadoc 的描述）被解析成空字串、進而讓所有 enum 檔產生假性 churn。LC_ALL=C 讓
# awk 以純 byte 處理，多位元組內容原樣穿透，跨環境結果一致。
export LC_ALL=C

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MD_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(cd "$MD_DIR/../.." && pwd)"

JAVA_OUTPUT_DIR="$PROJECT_ROOT/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code"
JAVA_ERROR_OUTPUT_FILE="$JAVA_OUTPUT_DIR/GrpcResponseCode.java"
GO_OUTPUT_DIR="$PROJECT_ROOT/internal/platform/codemeta"
GO_ERRORS_OUTPUT_FILE="$PROJECT_ROOT/pkg/errors/bizcode/bizcode.go"
GO_MEDIA_OUTPUT_FILE="$PROJECT_ROOT/pkg/media/ref_type.go"
# 單體的 media 模組（遷移期兩份並存；pkg/media 隨舊服務在遷移計畫階段 C 刪除）。
GO_MEDIA_MONOLITH_OUTPUT_FILE="$PROJECT_ROOT/internal/modules/media/ref_type.go"
GO_LABELS_OUTPUT_FILE="$PROJECT_ROOT/pkg/codemeta/labels.go"
GO_REGISTRY_OUTPUT_FILE="$PROJECT_ROOT/pkg/codemeta/registry.go"
# client 的生成物直接寫進 monorepo，CI（backend-monolith.yml）重新生成並檢查 drift。
MONOREPO_ROOT="$(cd "$PROJECT_ROOT/.." && pwd)"
FLUTTER_OUTPUT_FILES=("$MONOREPO_ROOT/buyer-app/lib/core/codemeta/code_meta.dart" "$MONOREPO_ROOT/seller-app/lib/core/codemeta/code_meta.dart")
WEB_OUTPUT_FILE="$MONOREPO_ROOT/website/lib/codemeta/generated.ts"
ADMIN_OUTPUT_FILE="$MONOREPO_ROOT/admin/src/lib/codemeta/generated.ts"
TMP_DIR="$(mktemp -d)"

# Markdown 來源
MD_FILES=("$MD_DIR/業務狀態碼.md" "$MD_DIR/業務類型碼.md" "$MD_DIR/內部技術碼.md")
ERROR_MD_FILE="$MD_DIR/錯誤碼.md"

# --go-labels 的涵蓋清單（見 generate_go_labels 的說明）。日後 Go 側再需要別的
# enum 的中文 Label，一律加進這個陣列，不得另開手寫表。
GO_LABEL_ENUMS=(ProductStatus PostPublishStatus OrderStatus DelegationReasonCode DelegationAuditAction DelegationAuthorizationAction CertificationType PayuniSellerFacingError PayuniErrorRemedy)

# --go-registry 的涵蓋清單：需要在 Go 端有 typed 常數 + AllXxx + FromCode/FromValue
# 的 enum。與 GO_LABEL_ENUMS 必須互斥——兩者同寫 package codemeta，對同一個 enum
# 會產生同名符號（例如 OrderStatusPaid）而編譯失敗。
GO_REGISTRY_ENUMS=(
    CommonStatus
    PricebookOverviewScope PricebookChangeDirection PricebookChangeBasis
    ProcurementRunStatus ProcurementStepStatus ProcurementLaneStatus ProcurementArtifactStatus
    ProcurementConfirmationStatus ProcurementRunFailureReason ProcurementErrorReason
    ProcurementLedgerEventType ProcurementArtifactType
    ProcurementRfqPublicationApprovalState ProcurementRfqInviteDeliveryState ProcurementPublicationAudienceMode ProcurementWishPoolPolicy ProcurementPublicationState
    ProcurementDealTemperatureStatus ProcurementDealTemperatureReason ProcurementDealTemperatureBand
    ProcurementSellerResponseType ProcurementTaxInclusionBasis ProcurementSellerPartyType
    ProcurementSellerResponseStatus ProcurementQuoteAcceptability ProcurementOrderProvisioningState
    ReportStatus ReportTargetType ReportReasonCode ReportResolveAction ModerationActionType
    InvoiceDocumentStatus InvoiceOperationStatus InvoiceAdjustmentStatus
    InvoiceCorrectionStatus InvoiceComplaintStatus SellerTaxReviewStatus InvoiceProviderAccountStatus
    InvoiceIssuerType InvoiceRecipientType InvoiceDocumentKind InvoiceFulfillmentMode InvoiceLineType
    InvoiceTaxCategory InvoiceCarrierType InvoiceOperationType InvoiceAdjustmentType
    InvoiceAdjustmentTrigger InvoiceCarrierSelectionMode RefundSource SellerTaxObligation
    SellerTaxEntityType SellerTaxReviewDecision InvoiceTaxRiskFlag
    InvoiceManualReason InvoiceNotApplicableReason InvoiceCancelReason InvoiceCancelEvidenceKind InvoiceComplaintEventType
    InvoiceComplaintAction InvoiceDocumentTimelineEvent InvoiceCorrectionRejectReason
    InvoiceInboxDisposition
    # 65 物流履約平台 Foundation。
    # ⚠️ 2026-09-12：原本的消費者是獨立的 Go settlement-service，該服務已隨架構裁決
    #    DEC-20260912-184025 併進 payment-service（Java）⇒ 這批 Go 常數目前零消費者。
    #    **刻意保留**：碼表屬於物流財務結算 domain，domain 還在、只是換了 runtime owner；
    #    刪掉會讓 ADMIN_ENUMS 那半也要重判，而 ADM 的帳本畫面確實會用到值域。
    CommerceModel CollectionModel FulfillmentMode InvoiceModel
    LedgerOwnerType LedgerAccountCode LedgerTxnType ProviderFeeType
    FulfillmentAction RecurringAction UberDeliveryOutcome
    # 70 物流財務結算（線 B）：應收狀態機與責任成因判定（runtime owner 已改為 payment-service）
    # 都在 Go 側，需要 typed 常數與 FromValue（狀態值用字串比對會讓 typo 在 runtime 才現形）。
    LogisticsReceivableStatus ResponsibilityCause
)

# --admin 的涵蓋清單。ADM 後台只顯示這些值域，不像 website 需要全量前端可見碼表，
# 所以走 allowlist 而非「前端可見: Yes」全掃——否則 admin 會帶進 50+ 個它永遠用不到
# 的 enum（例如 Procurement 那一組），review 時「這張表誰在用」就答不出來了。
#
# ⚠️ 這份清單**不受「前端可見」過濾**（機制見 generate_ts_file 的註解）：
# DelegationAuditAction 標 `前端可見: No`（不外洩給買賣家前端），但 ADM 的稽核動作
# 篩選下拉需要它。清單裡少掉任何一個 enum 會直接中止生成，不會靜默產出空表。
ADMIN_ENUMS=(
    # Hawkr Agent 採購線（#2538）：ADM 是內部後台，營運要看得懂一張採購單卡在哪一步、
    # 為什麼被擋。這些值域全部標「前端可見: Yes」但買賣家端各自只消費其中一部分，
    # **ADM 要拿到完整值域只能靠本 allowlist**——少了就只能在後台硬編字串。
    ProcurementRunStatus ProcurementStepStatus ProcurementLaneStatus
    ProcurementArtifactStatus ProcurementConfirmationStatus
    ProcurementRfqPublicationApprovalState ProcurementRfqInviteDeliveryState
    ProcurementRunFailureReason ProcurementErrorReason ProcurementLedgerEventType
    ProcurementArtifactType ProcurementDealTemperatureStatus
    ProcurementDealTemperatureReason ProcurementDealTemperatureBand
    ProcurementPublicationAudienceMode ProcurementPublicationState
    ProcurementSellerResponseType ProcurementTaxInclusionBasis
    ProcurementSellerPartyType ProcurementSellerResponseStatus
    ProcurementQuoteAcceptability ProcurementOrderProvisioningState
    ProcurementWishPoolPolicy
    OrderStatus AfterSaleAppealState PaymentMethod PaymentTransactionStatus RefundStatus
    ShippingMethod ShippingTemperature ShippingFeePayer WeightUnit ShippingStatus
    # 65 物流履約平台 Foundation：商流／履約維度 ＋ 動作值域 ＋ 帳本科目。
    # 帳本四個 enum 標「前端可見: No」（買賣家端不消費），**只能靠本 allowlist 進 ADM**——
    # 少任何一個，ADM 的帳本／撥款畫面就會拿不到值域而只能硬編碼字串。
    CommerceModel CollectionModel FulfillmentMode InvoiceModel
    FulfillmentAction RecurringAction UberDeliveryOutcome ShippingType ReceiptStatus
    LedgerOwnerType LedgerAccountCode LedgerTxnType ProviderFeeType
    # 70 物流財務結算：ADM 的收款異常佇列依 LogisticsReceivableStatus 篩 UNCONFIRMED／FAILED，
    # 對帳與爭議畫面依 ResponsibilityCause 分流歸屬。
    LogisticsReceivableStatus ResponsibilityCause
    ShipmentAnomalyType ShipmentAnomalySeverity ShipmentAnomalyAckAction
    # AppType 不在這裡：版本門檻頁改用 ContentAdminService 的 proto enum，顯示文字在頁面裡
    # （ADR 2026-09-25-codemeta-retirement D5）。
    ProviderSource DelegationAuditAction
    SellerSuspensionReason AfterSalePlatformDecisionOutcome
    ProductStatus ProductPublicationState
    ReportStatus ReportTargetType ReportReasonCode ReportResolveAction
    CertificationType ProductCertificationStatus ProductCertificationSource
    ProductCertificationGovVerifyState ProductCertificationValidity
    ProductCertificationReviewAction ProductCertificationActorRole
    ProductCertificationRejectReason
    PayuniMerchantVerificationStatus PayuniMemberVerificationStatus PayuniPaymentChannelStatus
    InvoiceDocumentStatus InvoiceOperationStatus InvoiceAdjustmentStatus InvoiceCorrectionStatus
    InvoiceComplaintStatus SellerTaxReviewStatus InvoiceProviderAccountStatus
    InvoiceIssuerType InvoiceRecipientType InvoiceDocumentKind InvoiceFulfillmentMode
    InvoiceLineType InvoiceTaxCategory InvoiceCarrierType InvoiceOperationType
    InvoiceAdjustmentType InvoiceAdjustmentTrigger SellerTaxObligation SellerTaxEntityType
    SellerTaxReviewDecision InvoiceTaxRiskFlag
    InvoiceManualReason InvoiceNotApplicableReason InvoiceDocumentTimelineEvent
    InvoiceCancelEvidenceKind
)

trap "rm -rf $TMP_DIR" EXIT

# docs/開發規範/Java 微服務開發規範.md §6（另見 docs/代碼管理（CodeMeta）/README.md）：
# CodeMeta enum 禁止包含業務輔助邏輯（自定義方法區塊）。
# 業務 helper 必須建立在使用該 enum 的服務或模組內（cross-service 共用時可
# 放 common-core/utils/code/helper/）。本 generator 不再支援自定義方法區塊。

# =====================================================================
# Markdown 解析器（awk）
# 輸出格式：每個 Enum 一段，用 === 分隔
# ENUM:name|category|interface|visible|services|dbField|desc|specialMethods
# ROW:name|code|value|label|description
# =====================================================================
parse_markdown() {
    local md_file="$1"
    awk '
    BEGIN { in_section=0; in_table=0; header_parsed=0 }

    /^## / {
        if (in_section) print "==="
        in_section=1; in_table=0; header_parsed=0
        enum_name=substr($0, 4)
        gsub(/[[:space:]]+$/, "", enum_name)
        # 初始化屬性
        category=""; iface="CodeMeta"; visible="No"; services=""
        db_field=""; desc=""; special=""; extra=""
        next
    }

    in_section && /^- \*\*/ {
        # 解析屬性：- **key**: value
        line=$0
        # 擷取 key（** 之間）
        gsub(/^- \*\*/, "", line)
        idx=index(line, "**:")
        if (idx > 0) {
            key=substr(line, 1, idx-1)
            val=substr(line, idx+3)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", key)
            gsub(/^[[:space:]]+|[[:space:]]+$/, "", val)
            if (key == "類別") category=val
            else if (key == "介面") iface=val
            else if (key == "前端可見") visible=val
            else if (key == "使用服務") services=val
            else if (key == "DB 儲存欄位") db_field=val
            else if (key == "說明") desc=val
            else if (key == "額外欄位") extra=val
            # 「特殊方法」key 已廢棄（Java 微服務開發規範 §6），讀取後忽略
        }
        next
    }

    in_section && /^\|.*\|$/ && /---/ { next }  # 跳過分隔線

    in_section && /^\|.*\|$/ && !header_parsed {
        # Header 行
        header_parsed=1
        # 輸出 ENUM 行
        printf "ENUM:%s|%s|%s|%s|%s|%s|%s|%s\n", \
            enum_name, category, iface, visible, services, db_field, desc, special
        in_table=1
        next
    }

    in_table && /^\|.*\|$/ {
        # 資料行
        n=split($0, cells, "|")
        # cells[1] 是空（左邊界），cells[2]~cells[n-1] 是資料
        name=cells[2]; code=cells[3]; value=cells[4]
        display=cells[5]; description=cells[6]
        # trim
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", name)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", code)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", display)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", description)
        if (name != "" && name != "Name")
            printf "ROW:%s|%s|%s|%s|%s\n", name, code, value, display, description
        next
    }

    in_section && in_table && !/^\|/ { in_table=0 }
    /^---$/ { in_table=0 }

    END { if (in_section) print "===" }
    ' "$md_file"
}

java_escape() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    printf '%s' "$value"
}

parse_error_markdown() {
    local md_file="$1"
    awk '
    BEGIN { in_enum=0; current_section="" }

    /^## GrpcResponseCode$/ {
        in_enum=1
        next
    }

    /^## / && in_enum {
        exit
    }

    !in_enum { next }

    /^### / {
        current_section=substr($0, 5)
        gsub(/[[:space:]]+$/, "", current_section)
        print "SECTION:" current_section
        next
    }

    /^\|.*\|$/ && /---/ { next }

    /^\|.*\|$/ {
        n=split($0, cells, "|")
        name=cells[2]
        code=cells[3]
        message=cells[4]

        gsub(/^[[:space:]]+|[[:space:]]+$/, "", name)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", code)
        gsub(/^[[:space:]]+|[[:space:]]+$/, "", message)

        if (name == "" || name == "Name") next
        if (name !~ /^[A-Z0-9_]+$/) next
        if (code !~ /^[0-9]+$/) next

        printf "ROW:%s|%s|%s\n", name, code, message
    }
    ' "$md_file"
}

generate_java_error_code() {
    local parsed_file="$1"
    local output_file="$2"

    {
        echo "package io.github.will413028.hawkr.common.utils.code;"
        echo ""
        echo "import java.util.Arrays;"
        echo "import java.util.Map;"
        echo "import java.util.stream.Collectors;"
        echo ""
        echo "/**"
        echo " * 跨服務統一 gRPC Response Code。"
        echo " *"
        echo " * <p>適用於所有使用 int32 code 模式的 gRPC Response。"
        echo " * 成功時 code=0，失敗時使用對應錯誤碼。</p>"
        echo " *"
        echo " * <p>{@link #message()} 為 API 回應的 {@code message} 欄位值（前端顯示值），"
        echo " * 禁止在各服務中硬編碼錯誤訊息字串。</p>"
        echo " *"
        echo " * <p>含 {@code %s} 的 message 為模板，由 BusinessException 透過"
        echo " * {@code String.format(message, args)} 組裝動態訊息。</p>"
        echo " *"
        echo " * <p>⚠ 此檔案由 generate.sh 自動生成，請以 docs/代碼管理（CodeMeta）/錯誤碼.md 為唯一來源。</p>"
        echo " */"
        echo "public enum GrpcResponseCode {"
        echo ""

        local total_rows=0
        total_rows=$(grep -c '^ROW:' "$parsed_file" || true)
        local row_index=0
        local last_was_section=0

        while IFS= read -r line; do
            if [[ "$line" == SECTION:* ]]; then
                local section="${line#SECTION:}"
                if [[ "$section" =~ ^(通用錯誤碼|.*業務碼) ]]; then
                    if [[ $row_index -gt 0 ]]; then
                        echo ""
                    fi
                    echo "    // ===== ${section} ====="
                    last_was_section=1
                fi
                continue
            fi

            if [[ "$line" == ROW:* ]]; then
                local data="${line#ROW:}"
                local name code message
                IFS='|' read -r name code message <<< "$data"
                row_index=$((row_index + 1))

                local suffix=","
                if [[ $row_index -eq $total_rows ]]; then
                    suffix=";"
                fi

                echo "    ${name}(${code}, \"$(java_escape "$message")\")${suffix}"
                last_was_section=0
            fi
        done < "$parsed_file"

        echo ""
        echo "    private final int code;"
        echo "    private final String message;"
        echo ""
        echo "    GrpcResponseCode(int code, String message) {"
        echo "        this.code = code;"
        echo "        this.message = message;"
        echo "    }"
        echo ""
        echo "    public int code() {"
        echo "        return code;"
        echo "    }"
        echo ""
        echo "    /**"
        echo "     * API 回應的 message 欄位值（前端顯示值）。"
        echo "     * 含 %s 的為模板，需透過 {@code String.format(message(), args)} 組裝。"
        echo "     */"
        echo "    public String message() {"
        echo "        return message;"
        echo "    }"
        echo ""
        echo "    /**"
        echo "     * 組裝動態訊息（含 %s 模板替換）。"
        echo "     * 無參數時直接回傳 {@link #message()}。"
        echo "     */"
        echo "    public String formatMessage(Object... args) {"
        echo "        if (args == null || args.length == 0) {"
        echo "            return message;"
        echo "        }"
        echo "        return String.format(message, args);"
        echo "    }"
        echo ""
        echo "    private static final Map<Integer, GrpcResponseCode> BY_CODE ="
        echo "        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(GrpcResponseCode::code, e -> e));"
        echo ""
        echo "    public static GrpcResponseCode fromCode(int code) {"
        echo "        var value = BY_CODE.get(code);"
        echo "        if (value == null) throw new IllegalArgumentException(\"Unknown code: \" + code);"
        echo "        return value;"
        echo "    }"
        echo "}"
    } > "$output_file"
}

# =====================================================================
# Java CodeMeta Enum 生成
# =====================================================================
generate_java_enum() {
    local enum_name="$1"
    local desc="$2"
    local special_methods="$3"  # 已廢棄（Java 微服務開發規範 §6），保留參數以維持呼叫端簽名
    local constants_file="$4"  # 暫存的常數列表
    local output_file="$5"

    {
        echo "package io.github.will413028.hawkr.common.utils.code;"
        echo ""
        echo "import java.util.*;"
        echo "import java.util.stream.Collectors;"
        echo ""
        echo "/**"
        echo " * ${desc}"
        echo " *"
        echo " * ⚠ 此檔案由 generate.sh 自動生成，請勿手動修改。"
        echo " * 業務輔助邏輯請在使用該 enum 的服務或模組內建立 helper class"
        echo " * （見 docs/開發規範/Java 微服務開發規範.md §6）。"
        echo " */"
        echo "public enum ${enum_name} implements CodeMeta {"
        echo ""

        # 常數
        local total_lines
        total_lines=$(wc -l < "$constants_file" | tr -d ' ')
        local line_num=0
        while IFS='|' read -r name code value display description; do
            line_num=$((line_num + 1))
            local suffix=","
            if [[ $line_num -eq $total_lines ]]; then suffix=";"; fi
            # 🔴 四個欄位都要過 java_escape：Description 是自由書寫的中文散文，
            # 出現 `"` 或 `\` 是完全合法的 Markdown，但直接吐進 Java 字串字面值會
            # 讓整個 enum 檔語法錯誤（首例：SellerTaxReviewDecision.EXTEND 的說明
            # 裡有 `decision = "EXTEND"`，common-core 整個模組編不過）。
            # ⚠️ 錯誤碼那一支（Message 欄）本來就有過 java_escape，只有本函式漏了。
            echo "    ${name}(\"$(java_escape "${code}")\", \"$(java_escape "${value}")\", \"$(java_escape "${display}")\", \"$(java_escape "${description}")\")${suffix}"
        done < "$constants_file"

        echo ""

        # 欄位
        echo "    private final String code;"
        echo "    private final String value;"
        echo "    private final String label;"
        echo "    private final String description;"
        echo ""

        # 建構子
        echo "    ${enum_name}(String code, String value, String label, String description) {"
        echo "        this.code = code;"
        echo "        this.value = value;"
        echo "        this.label = label;"
        echo "        this.description = description;"
        echo "    }"
        echo ""

        # CodeMeta 介面方法
        echo "    @Override"
        echo "    public String code() { return code; }"
        echo ""
        echo "    @Override"
        echo "    public String value() { return value; }"
        echo ""
        echo "    @Override"
        echo "    public String label() { return label; }"
        echo ""
        echo "    @Override"
        echo "    public String description() { return description; }"
        echo ""

        # 靜態 Map
        echo "    private static final Map<String, ${enum_name}> BY_CODE ="
        echo "        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(${enum_name}::code, e -> e));"
        echo ""
        echo "    private static final Map<String, ${enum_name}> BY_VALUE ="
        echo "        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(${enum_name}::value, e -> e));"
        echo ""

        # 標準靜態 helper（4 個固定不可變）
        echo "    public static boolean isValidCode(String code) {"
        echo "        return code != null && BY_CODE.containsKey(code);"
        echo "    }"
        echo ""
        echo "    public static boolean isValidValue(String value) {"
        echo "        return value != null && BY_VALUE.containsKey(value);"
        echo "    }"
        echo ""
        echo "    public static ${enum_name} fromCode(String code) {"
        echo "        var e = BY_CODE.get(code);"
        echo "        if (e == null) throw new IllegalArgumentException(\"Unknown code: \" + code);"
        echo "        return e;"
        echo "    }"
        echo ""
        echo "    public static ${enum_name} fromValue(String value) {"
        echo "        var e = BY_VALUE.get(value);"
        echo "        if (e == null) throw new IllegalArgumentException(\"Unknown value: \" + value);"
        echo "        return e;"
        echo "    }"
        echo ""

        echo "    public static List<CodeMeta> codes() {"
        echo "        return List.of(values());"
        echo "    }"
        echo "}"
    } > "$output_file"
}

# =====================================================================
# Go 生成（單一檔案）
# =====================================================================
to_pascal() {
    echo "$1" | awk '{
        n=split(tolower($0), parts, "_")
        result=""
        for (i=1; i<=n; i++) {
            result = result toupper(substr(parts[i],1,1)) substr(parts[i],2)
        }
        print result
    }'
}

generate_go_file() {
    local parsed_file="$1"
    local output_file="$2"
    # 第 3 參數：以空白分隔的 enum 白名單。留空 = 全部（原行為）。
    # 第 4 參數：header 要提示的重生成指令，預設 --go。
    local allowlist="${3:-}"
    local regen_flag="${4:---go}"

    mkdir -p "$(dirname "$output_file")"

    {
        echo "// Code generated by generate.sh from CodeMeta Markdown. DO NOT EDIT."
        echo "//"
        echo "// 此檔案由 generate.sh 自動生成，請勿手動修改。"
        echo "// 修改代碼值請編輯 docs/代碼管理（CodeMeta）/*.md 後執行："
        echo "//   ./docs/代碼管理（CodeMeta）/scripts/generate.sh ${regen_flag}"
        echo ""
        echo "package codemeta"
        echo ""
    } > "$output_file"

    # 空 allowlist ⇒ 看可見性；非空 allowlist ⇒ 只看名字在不在清單裡。
    # 與 _ts_should_emit 同一條規則，理由也相同：allowlist 是「這個 target 明確
    # 要哪幾個」的宣告，把它與「前端可見」AND 起來，等於讓一個標 No 的 enum 就算
    # 被逐字列進清單也永遠生不出來 —— 而且是靜默的。GO_REGISTRY_ENUMS 服務的是
    # Go 服務端的值域檢查（例如 ReportStatus 只寫進 DB、從不外送前端），標 No 才
    # 是誠實的旗標，不該因此被排除。
    _go_should_emit() {
        if [[ -z "$allowlist" ]]; then
            [[ "$visible" =~ ^[Yy] ]]
        else
            [[ " $allowlist " == *" $1 "* ]]
        fi
    }

    local enum_name="" desc="" visible=""
    # 已產出的 enum 名，供下方的缺項檢查用（格式 |A|B|）。
    local emitted="|"
    local constants_file="$TMP_DIR/go_constants"

    while IFS= read -r line; do
        if [[ "$line" == "===" ]]; then
            # 結束上一個 Enum，生成 Go 代碼
            if [[ -n "$enum_name" ]] && _go_should_emit "$enum_name"; then
                _append_go_enum "$enum_name" "$desc" "$constants_file" "$output_file"
                emitted="${emitted}${enum_name}|"
            fi
            enum_name=""
            > "$constants_file"
            continue
        fi

        if [[ "$line" == ENUM:* ]]; then
            local data="${line#ENUM:}"
            IFS='|' read -r enum_name _ _ visible _ _ desc _ <<< "$data"
            > "$constants_file"
            continue
        fi

        if [[ "$line" == ROW:* ]]; then
            echo "${line#ROW:}" >> "$constants_file"
        fi
    done < "$parsed_file"

    # 最後一個
    if [[ -n "$enum_name" ]] && _go_should_emit "$enum_name"; then
        _append_go_enum "$enum_name" "$desc" "$constants_file" "$output_file"
        emitted="${emitted}${enum_name}|"
    fi

    # 每個 enum 區塊尾端都有一個 echo "" 當分隔，最後一個因此在檔尾留下空行，
    # 而 gofmt 只允許檔尾恰好一個換行。生成物依 CodeMeta 規範不能手改，所以在這裡收掉。
    printf '%s\n' "$(cat "$output_file")" > "${output_file}.tmp" \
        && mv "${output_file}.tmp" "$output_file"

    # 清單裡有、但解析結果沒有 ⇒ 中止，不要產出一個少了東西的檔。
    # 與 generate_ts_file 尾端同一道檢查：少掉的那個 enum 在 Go 側是「呼叫端拿到
    # 一個不存在的識別字」，編譯期會炸但錯誤訊息已離現場很遠；更壞的情況是
    # 呼叫端還沒寫，於是清單說覆蓋了五個、實際只有兩個，而**沒有任何東西轉紅**
    # （registry_test.go 的檔頭早就記著這個坑，這是它的第二種形態）。
    if [[ -n "$allowlist" ]]; then
        local e
        for e in $allowlist; do
            if [[ "$emitted" != *"|${e}|"* ]]; then
                echo "   ⚠ 解析結果中找不到 ${e}，中止 ${regen_flag} 生成" >&2
                return 1
            fi
        done
    fi
}

_append_go_enum() {
    local enum_name="$1" desc="$2" cfile="$3" ofile="$4"

    {
        echo "// ===== ${enum_name} ====="
        echo "// ${desc}"
        echo ""
        echo "type ${enum_name} struct {"
        echo "	Code        string"
        echo "	Value       string"
        echo "	Label       string"
        echo "	Description string"
        echo "}"
        echo ""

        # 變數
        # gofmt 會把同一個 var 區塊的 `=` 對齊到最長的變數名。先量寬度再補空白，
        # 產出才是 gofmt-clean —— 否則 gofmt gate 會紅，而生成檔依 CodeMeta 規範不能手改，
        # 唯一的修法就是改這裡（同 generate_go_media 的做法）。
        local max_var_len=0
        while IFS='|' read -r name _ _ _ _; do
            local pascal
            pascal=$(to_pascal "$name")
            local vlen=$(( ${#enum_name} + ${#pascal} ))
            [ "$vlen" -gt "$max_var_len" ] && max_var_len=$vlen
        done < "$cfile"
        echo "var ("
        while IFS='|' read -r name code value display description; do
            local pascal
            pascal=$(to_pascal "$name")
            local var_name="${enum_name}${pascal}"
            # 四個欄位都要跳脫（Go 與 Java 的雙引號字串跳脫規則相同，共用 java_escape）：
            # Description / Label 是自由書寫的中文散文，出現 `"` 或 `\` 是完全合法的
            # Markdown，但直接吐進 Go 字串字面值會讓整個生成檔語法錯誤（gofmt gate 會紅，
            # 而生成檔依 CodeMeta 規範不能手改，唯一的修法就是改這裡）。
            printf '\t%-*s = %s{Code: "%s", Value: "%s", Label: "%s", Description: "%s"}\n' \
                "$max_var_len" "$var_name" "$enum_name" \
                "$(java_escape "$code")" "$(java_escape "$value")" \
                "$(java_escape "$display")" "$(java_escape "$description")"
        done < "$cfile"
        echo ")"
        echo ""

        # All slice
        echo "var All${enum_name}s = []${enum_name}{"
        while IFS='|' read -r name _ _ _ _; do
            local pascal
            pascal=$(to_pascal "$name")
            echo "	${enum_name}${pascal},"
        done < "$cfile"
        echo "}"
        echo ""

        # FromCode
        echo "func ${enum_name}FromCode(code string) (${enum_name}, bool) {"
        echo "	for _, v := range All${enum_name}s {"
        echo "		if v.Code == code {"
        echo "			return v, true"
        echo "		}"
        echo "	}"
        echo "	return ${enum_name}{}, false"
        echo "}"
        echo ""

        # FromValue
        echo "func ${enum_name}FromValue(value string) (${enum_name}, bool) {"
        echo "	for _, v := range All${enum_name}s {"
        echo "		if v.Value == value {"
        echo "			return v, true"
        echo "		}"
        echo "	}"
        echo "	return ${enum_name}{}, false"
        echo "}"
        echo ""
    } >> "$ofile"
}

# =====================================================================
# Flutter (Dart) 生成
# =====================================================================
to_camel() {
    echo "$1" | awk '{
        n=split(tolower($0), parts, "_")
        result=parts[1]
        for (i=2; i<=n; i++) {
            result = result toupper(substr(parts[i],1,1)) substr(parts[i],2)
        }
        print result
    }'
}

# Dart 保留字（reserved words）不能當識別字，撞到就加尾底線。
#
# 為什麼需要這一層：Dart 的 enum 成員名是識別字，而 CodeMeta 的 Name 欄是
# UPPER_SNAKE 的碼值，`to_camel` 之後有機會正好等於保留字。首例是
# `InvoiceOperationType.VOID` -> `void`，`dart format` 直接以
# 「Expected an identifier」解析失敗 ⇒ **整支 code_meta.dart 不可用**，
# 而 backend 這邊完全無感（Java／TS／Go 都合法，只有 Dart 炸）。
#
# 🔴 修法只能在生成器這一層。碼值本身不可改名——它與 DB CHECK 約束、
# provider 線上值逐字綁定（改 Name 就是改契約）。
#
# 下表是 Dart 的 reserved words 全集（built-in identifiers 如 `dynamic`／
# `abstract` 可以當一般識別字，不列入）。加尾底線而不是改字首，是為了讓
# 「原本是哪個碼」一眼可讀；Dart 對尾底線沒有任何 lint 限制。
DART_RESERVED="assert break case catch class const continue default do else enum extends false final finally for if in is new null rethrow return super switch this throw true try var void while with"

to_dart_ident() {
    local ident
    ident="$(to_camel "$1")"
    local w
    for w in $DART_RESERVED; do
        if [[ "$ident" == "$w" ]]; then
            printf '%s_' "$ident"
            return 0
        fi
    done
    printf '%s' "$ident"
}

generate_flutter_file() {
    local parsed_file="$1"
    local output_file="$2"

    {
        echo "// Code generated by generate.sh from CodeMeta Markdown. DO NOT EDIT."
        echo "//"
        echo "// 此檔案由 generate.sh 自動生成，請勿手動修改。"
        echo "// 修改代碼值請編輯 backend/docs/代碼管理（CodeMeta）/*.md 後重新生成（generate.sh --flutter）。"
        echo "// 生成器的輸出就是 commit 的內容，CI 逐位元比對，所以不經過 dart format。"
        echo ""
        echo "// dart format off"
        echo ""
    } > "$output_file"

    local enum_name="" desc="" visible=""
    local constants_file="$TMP_DIR/dart_constants"

    while IFS= read -r line; do
        if [[ "$line" == "===" ]]; then
            if [[ -n "$enum_name" && "$visible" =~ ^[Yy] ]]; then
                _append_dart_enum "$enum_name" "$desc" "$constants_file" "$output_file"
            fi
            enum_name=""
            > "$constants_file"
            continue
        fi

        if [[ "$line" == ENUM:* ]]; then
            local data="${line#ENUM:}"
            IFS='|' read -r enum_name _ _ visible _ _ desc _ <<< "$data"
            > "$constants_file"
            continue
        fi

        if [[ "$line" == ROW:* ]]; then
            echo "${line#ROW:}" >> "$constants_file"
        fi
    done < "$parsed_file"

    if [[ -n "$enum_name" && "$visible" =~ ^[Yy] ]]; then
        _append_dart_enum "$enum_name" "$desc" "$constants_file" "$output_file"
    fi
}

_append_dart_enum() {
    local enum_name="$1" desc="$2" cfile="$3" ofile="$4"

    {
        echo "/// ${desc}"
        echo "enum ${enum_name} {"

        # 列舉值
        local total
        total=$(wc -l < "$cfile" | tr -d ' ')
        local idx=0
        while IFS='|' read -r name code value display description; do
            idx=$((idx + 1))
            local camel
            camel=$(to_dart_ident "$name")
            local suffix=","
            if [[ $idx -eq $total ]]; then suffix=";"; fi
            echo "  /// ${display} — ${description}"
            echo "  ${camel}(code: '$(dart_escape "$code")', value: '$(dart_escape "$value")', label: '$(dart_escape "$display")')${suffix}"
        done < "$cfile"

        echo ""
        echo "  const ${enum_name}({"
        echo "    required this.code,"
        echo "    required this.value,"
        echo "    required this.label,"
        echo "  });"
        echo ""
        echo "  final String code;"
        echo "  final String value;"
        echo "  final String label;"
        echo ""
        echo "  /// 依 code 查找，找不到回傳 null"
        echo "  static ${enum_name}? fromCode(String code) {"
        echo "    for (final v in values) {"
        echo "      if (v.code == code) return v;"
        echo "    }"
        echo "    return null;"
        echo "  }"
        echo ""
        echo "  /// 依 value 查找，找不到回傳 null"
        echo "  static ${enum_name}? fromValue(String value) {"
        echo "    for (final v in values) {"
        echo "      if (v.value == value) return v;"
        echo "    }"
        echo "    return null;"
        echo "  }"
        echo "}"
        echo ""
    } >> "$ofile"
}

# =====================================================================
# Web (TypeScript) 生成
#
# 直接從已解析的 Markdown（$parsed_file）生成，不透過 Dart 中介檔案。
# 直寫 website/lib/codemeta/generated.ts，enums.ts 只 re-export ＋ 保留手寫 helper
# （不在 CodeMeta 生成範圍內）。
# =====================================================================
to_snake_upper() {
    # OrderCancelReason -> ORDER_CANCEL_REASON
    echo "$1" | sed -E 's/([a-z0-9])([A-Z])/\1_\2/g; s/([A-Z]+)([A-Z][a-z])/\1_\2/g' | tr '[:lower:]' '[:upper:]'
}

ts_escape() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    printf '%s' "$value"
}

# Dart 生成器用單引號字串，跳脫的是反斜線與**單**引號（不是雙引號）。
# 與 java_escape / ts_escape 是同一個病的第三份解法：Label / Code / Value 都是
# 自由書寫的欄位，出現引號是合法 Markdown，但直接吐進字面值會讓整個生成檔語法錯誤。
dart_escape() {
    # ⚠️ 不能寫成 `${value//\'/\\\'}`（java_escape / ts_escape 的形式）：**bash 3.2 與
    # bash 5 對替換字串裡的反斜線處理不同**，而本檔 shebang 是 `#!/bin/bash`＝macOS 的
    # bash 3.2。實測同一段程式碼 bash 5 產出 `\'`（正確）、bash 3.2 產出 `\\'`（兩個反斜線，
    # Dart 字串就地截斷 ⇒ 整個生成檔語法錯誤）。用 `bash` 手測會過、跑 generate.sh 會壞。
    # `\"` 那個形式兩版一致，所以只有本函式需要繞開；改用 sed 與 shell 版本無關。
    printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e "s/'/\\\\'/g"
}

# TypeScript 生成器。`--web` 與 `--admin` 共用同一份實作，差別只在兩個參數：
#
#   flag       ── 重新生成的旗標名（寫進檔頭）
#   allowlist  ── 空字串＝沿用「前端可見: Yes」全量過濾（--web 的原行為）；
#                 非空＝只產清單內的 enum，且**不受前端可見過濾**
#
# allowlist 不受可見性過濾是刻意的，理由與 GO_LABEL_ENUMS 相同（見該陣列註解）：
# 有些值域標「前端可見: No」是為了不外洩給買賣家前端，但 ADM 這種內部後台確實
# 要顯示它們（例如 DelegationAuditAction）。把 .md 改成 Yes 會讓那些值同時灌進
# website 與兩支 App 的生成物，是更糟的解法。
generate_ts_file() {
    local parsed_file="$1"
    local output_file="$2"
    local flag="$3"
    local allowlist="$4"

    {
        echo "// Code generated by generate.sh from CodeMeta Markdown. DO NOT EDIT."
        echo "//"
        echo "// 此檔案由 generate.sh 自動生成，請勿手動修改。"
        echo "// 修改代碼值請編輯 backend/docs/代碼管理（CodeMeta）/*.md 後重新生成（generate.sh ${flag}）。"
        echo ""
    } > "$output_file"

    local enum_name="" desc="" visible=""
    local constants_file="$TMP_DIR/ts_constants"
    # 已產出的 enum，供 allowlist 模式做「清單有但 .md 沒有」的缺項檢查。
    local emitted="|"

    # 空 allowlist ⇒ 看可見性；非空 allowlist ⇒ 只看名字在不在清單裡。
    _ts_should_emit() {
        local name="$1"
        if [[ -z "$allowlist" ]]; then
            [[ "$visible" =~ ^[Yy] ]]
        else
            [[ " $allowlist " == *" $name "* ]]
        fi
    }

    while IFS= read -r line; do
        if [[ "$line" == "===" ]]; then
            if [[ -n "$enum_name" ]] && _ts_should_emit "$enum_name"; then
                _append_ts_enum "$enum_name" "$desc" "$constants_file" "$output_file"
                emitted="${emitted}${enum_name}|"
            fi
            enum_name=""
            > "$constants_file"
            continue
        fi

        if [[ "$line" == ENUM:* ]]; then
            local data="${line#ENUM:}"
            IFS='|' read -r enum_name _ _ visible _ _ desc _ <<< "$data"
            > "$constants_file"
            continue
        fi

        if [[ "$line" == ROW:* ]]; then
            echo "${line#ROW:}" >> "$constants_file"
        fi
    done < "$parsed_file"

    if [[ -n "$enum_name" ]] && _ts_should_emit "$enum_name"; then
        _append_ts_enum "$enum_name" "$desc" "$constants_file" "$output_file"
        emitted="${emitted}${enum_name}|"
    fi

    # 清單裡少掉任何一個就中止（同 GO_LABEL_ENUMS 的理由）：靜默略過會讓消費端
    # 拿到一個不存在的識別字。TS 側是編譯期就炸，但那時錯誤訊息已離現場很遠，
    # 而且 CI 上先炸的會是前端 repo 而不是改 .md 的這個 PR。
    if [[ -n "$allowlist" ]]; then
        local e
        for e in $allowlist; do
            if [[ "$emitted" != *"|${e}|"* ]]; then
                echo "   ⚠ 解析結果中找不到 ${e}，中止 ${flag} 生成" >&2
                return 1
            fi
        done
    fi
}

_append_ts_enum() {
    local enum_name="$1" desc="$2" cfile="$3" ofile="$4"
    local snake
    snake=$(to_snake_upper "$enum_name")

    {
        if [[ -n "$desc" ]]; then
            echo "/** $(ts_escape "$desc") */"
        fi
        echo "export const ${enum_name} = {"
        while IFS='|' read -r name code value display description; do
            echo "	${name}: \"$(ts_escape "$value")\","
        done < "$cfile"
        echo "} as const;"
        echo "export type ${enum_name}Code = keyof typeof ${enum_name};"
        echo "export type ${enum_name}Value = (typeof ${enum_name})[${enum_name}Code];"
        echo ""

        # Record<string, string>（非 Record<${enum_name}Value, string>）：既有
        # web 慣例對部分 label map（如 ORDER_CANCEL_REASON_LABEL、
        # PAYMENT_METHOD_INELIGIBLE_REASON_LABEL）以未收斂型別的原始字串（後端
        # 傳來、未先轉型）直接查表，嚴格型別會在這類呼叫點產生 TS7053。統一放寬
        # 避免每個 caller 各自 cast。
        echo "export const ${snake}_LABEL: Record<string, string> = {"
        local seen_values="|"
        while IFS='|' read -r name code value display description; do
            if [[ "$seen_values" == *"|${value}|"* ]]; then
                continue
            fi
            seen_values="${seen_values}${value}|"
            # 去除 Label 尾綴「（終態）」標記（web 顯示不需要）
            local clean_display="${display%（終態）}"
            echo "	\"$(ts_escape "$value")\": \"$(ts_escape "$clean_display")\","
        done < "$cfile"
        echo "};"
        echo ""
    } >> "$ofile"
}

# =====================================================================
# Go 業務碼常數生成（bizcode package）
# 從錯誤碼.md 中過濾 code >= 1000 的業務碼，輸出為 Go const 定義。
# 通用碼（400~503）已在 pkg/errors/domain_error.go 具名定義，不重複生成。
#
# 同時輸出 code → Message 對照（Message()）。Go 側原本只有碼值沒有訊息，於是拿到
# 一個碼的地方（代行稽核面板、客服工單）只能印裸數字；前端自建對照表就是第二份
# CodeMeta 鏡像，它的失敗形狀是 drift 而非錯誤 —— Markdown 改了文案，副本不編譯
# 失敗、不轉紅，只是從那天起顯示舊字串。同 generate_go_labels／generate_go_media
# 的動機。
# =====================================================================
generate_go_bizcode() {
    local parsed_error_file="$1"
    local output_file="$2"

    mkdir -p "$(dirname "$output_file")"

    # 收集業務碼（code >= 1000），保留 section 資訊
    local biz_rows_file="$TMP_DIR/biz_rows.txt"
    > "$biz_rows_file"

    local current_section=""
    while IFS= read -r line; do
        if [[ "$line" == SECTION:* ]]; then
            current_section="${line#SECTION:}"
            continue
        fi
        if [[ "$line" == ROW:* ]]; then
            local data="${line#ROW:}"
            local name code msg
            # 第三個變數吃剩下的全部：訊息本身若含 `|` 會在 parse_error_markdown
            # 就先被切碎，這裡再切一次只會把它悄悄截短。
            IFS='|' read -r name code msg <<< "$data"
            if [[ "$code" =~ ^[0-9]+$ ]] && [[ "$code" -ge 1000 ]]; then
                printf '%s|%s|%s|%s\n' "$current_section" "$name" "$code" "$msg" >> "$biz_rows_file"
            fi
        fi
    done < "$parsed_error_file"

    # 訊息欄空掉就中止，不產出半張表。空訊息在下游是靜默的：Message() 回空字串，
    # 呼叫端 fallback 顯示原碼 —— 畫面看起來與「這個碼沒登記」一模一樣，不會有任何
    # 東西轉紅。抓錯欄位（少抓一欄／欄序改了）正是會全表空掉的那種失誤。
    # 檢查放在開檔重導向之前：先寫檔再中止會留下一個被截斷的生成物。
    local _s _n _c _m
    while IFS='|' read -r _s _n _c _m; do
        if [[ -z "$_m" ]]; then
            echo "   ⚠ 業務碼 ${_c}（${_n}）沒有 Message，中止 Go 業務碼生成" >&2
            return 1
        fi
    done < "$biz_rows_file"
    if [[ ! -s "$biz_rows_file" ]]; then
        echo "   ⚠ 錯誤碼.md 解析不出任何業務碼（code >= 1000），中止 Go 業務碼生成" >&2
        return 1
    fi

    {
        echo "// Code generated by generate.sh from 錯誤碼.md. DO NOT EDIT."
        echo "//"
        echo "// 修改業務碼請編輯 docs/代碼管理（CodeMeta）/錯誤碼.md 後執行："
        echo "//   ./docs/代碼管理（CodeMeta）/scripts/generate.sh --go-errors"
        echo ""
        echo "package bizcode"
        echo ""
        echo "const ("

        # gofmt 對 const 區塊的對齊規則是「以空行分隔的每一組各自對齊到組內最長的名字」，
        # 而這裡每個 section 之間正好會 echo 一個空行 —— 所以要逐 section 量寬度。
        # 不對齊的話產出就不是 gofmt-clean，而 pkg/errors/bizcode/bizcode.go 是
        # DO NOT EDIT 的生成物、依 CodeMeta 規範不能手改，gofmt gate 與 CodeMeta drift gate
        # 會互相打架（一個要求對齊、一個要求跟生成結果一致）。
        local pascal_file="$TMP_DIR/biz_pascal.txt"
        : > "$pascal_file"
        while IFS='|' read -r section name code msg; do
            printf '%s|%s|%s\n' "$section" "$(to_pascal "$name")" "$code" >> "$pascal_file"
        done < "$biz_rows_file"

        awk -F'|' '
            { s[NR]=$1; p[NR]=$2; c[NR]=$3
              if (length($2) > w[$1]) w[$1] = length($2)
              n = NR }
            END {
                last = ""
                for (i = 1; i <= n; i++) {
                    if (s[i] != last) {
                        if (last != "") print ""
                        printf "\t// %s\n", s[i]
                        last = s[i]
                    }
                    pad = w[s[i]] - length(p[i]); sp = ""
                    for (k = 0; k < pad; k++) sp = sp " "
                    printf "\t%s%s int32 = %s\n", p[i], sp, c[i]
                }
            }
        ' "$pascal_file"

        echo ")"

        # code → Message。key 一律是碼值不是常數名：常數名長度差距大，gofmt 會把
        # map literal 切成好幾個對齊區段，而本腳本的對齊是自算的（不依賴 gofmt
        # binary），區段一切開自算結果就對不上，CodeMeta drift gate 與 gofmt gate
        # 會互相打架。碼值全為等寬數字則只有一個區段。
        local msg_file="$TMP_DIR/biz_messages.txt"
        : > "$msg_file"
        local max_key=0
        while IFS='|' read -r section name code msg; do
            local key="${code}:"
            (( ${#key} > max_key )) && max_key=${#key}
            # Go 與 Java 的雙引號字串跳脫規則相同（反斜線與雙引號），共用同一支。
            printf '%s|%s\n' "$key" "$(java_escape "$msg")" >> "$msg_file"
        done < "$biz_rows_file"

        echo ""
        echo "// bizCodeMessages 是錯誤碼.md 的 Message 欄逐字對照，與 Java 的"
        echo "// GrpcResponseCode.message() 同源。含 %s 的是模板（由發出端 format），"
        echo "// 本表的用途是「拿到一個碼要說出它是什麼」，不是拿來組回應訊息。"
        echo "var bizCodeMessages = map[int32]string{"
        while IFS='|' read -r key msg; do
            printf '\t%-*s "%s",\n' "$max_key" "$key" "$msg"
        done < "$msg_file"
        echo "}"
        echo ""
        echo "// Message 回傳 code 對應的錯誤訊息；未登錄的碼回空字串，由呼叫端決定"
        echo "// 要顯示什麼。這裡不回 fallback 字串：一句「未知錯誤」會蓋掉呼叫端原本"
        echo "// 就有的碼值，而顯示原碼至少還追得到。"
        echo "func Message(code int32) string {"
        echo "	return bizCodeMessages[code]"
        echo "}"
    } > "$output_file"
}

# =====================================================================
# Go media 引用類型常數生成（media package，直寫 pkg/media/ref_type.go）
#
# AssetReferenceType 前端可見=No，不會被 --go（無 allowlist ⇒ 只輸出 Yes）納入，
# 但 Go media package 需要這組常數（各服務 FinalizeImages/LinkImages 帶 ref_type）。
# 故獨立 target 直寫，杜絕手維護副本 drift（issue #1530）。
# 命名前綴固定 RefType（既有 caller 慣例，如 RefTypeProduct），值取 Markdown Value。
# 對齊自算（純 bash，不依賴 gofmt binary），輸出即 gofmt-clean，供 light runner 的
# CodeMeta drift gate 純文字重跑後 diff 穩定。
# =====================================================================
generate_go_media() {
    local parsed_file="$1"
    local output_file="$2"

    mkdir -p "$(dirname "$output_file")"

    # 從已解析的 Markdown 抽出 AssetReferenceType 的資料列（無視「前端可見」過濾）。
    local rows_file="$TMP_DIR/media_rows.txt"
    > "$rows_file"
    local enum_desc="" in_target=0 cur_enum="" cur_desc=""

    while IFS= read -r line; do
        if [[ "$line" == "===" ]]; then
            in_target=0; cur_enum=""
            continue
        fi
        if [[ "$line" == ENUM:* ]]; then
            local data="${line#ENUM:}"
            IFS='|' read -r cur_enum _ _ _ _ _ cur_desc _ <<< "$data"
            if [[ "$cur_enum" == "AssetReferenceType" ]]; then
                in_target=1
                enum_desc="$cur_desc"
            else
                in_target=0
            fi
            continue
        fi
        if [[ "$line" == ROW:* && $in_target -eq 1 ]]; then
            echo "${line#ROW:}" >> "$rows_file"
        fi
    done < "$parsed_file"

    if [[ ! -s "$rows_file" ]]; then
        echo "   ⚠ 解析結果中找不到 AssetReferenceType，跳過 Go media 生成" >&2
        return 1
    fi

    # 對齊寬度：模擬 gofmt const 區塊，name 欄與 value 欄各自補到最長者。
    local max_name=0 max_val=0
    while IFS='|' read -r name code value display description; do
        local var_name="RefType$(to_pascal "$name")"
        (( ${#var_name} > max_name )) && max_name=${#var_name}
        local quoted="\"${value}\""
        (( ${#quoted} > max_val )) && max_val=${#quoted}
    done < "$rows_file"

    {
        echo "// Code generated by generate.sh from 業務類型碼.md. DO NOT EDIT."
        echo "//"
        echo "// 修改多媒體資產引用類型請編輯 docs/代碼管理（CodeMeta）/業務類型碼.md 後執行："
        echo "//   ./docs/代碼管理（CodeMeta）/scripts/generate.sh --go-media"
        echo ""
        echo "package media"
        echo ""
        echo "// AssetReferenceType ${enum_desc}"
        echo "const ("
        while IFS='|' read -r name code value display description; do
            local var_name="RefType$(to_pascal "$name")"
            local quoted="\"${value}\""
            if [[ -n "$description" ]]; then
                printf '\t%-*s = %-*s // %s\n' "$max_name" "$var_name" "$max_val" "$quoted" "$description"
            else
                printf '\t%-*s = %s\n' "$max_name" "$var_name" "$quoted"
            fi
        done < "$rows_file"
        echo ")"
    } > "$output_file"
}

# =====================================================================
# Go CodeMeta Label 生成（codemeta package，直寫 pkg/codemeta/labels.go）
#
# Go 側中文 Label 的唯一來源。走不了 --go：該 target 不帶 allowlist ⇒ 只輸出「前端可見=Yes」
# 的 enum，而 DelegationAuditAction / DelegationReasonCode 都是 No；而且 --go 的輸出在
# 單體的 internal/platform/codemeta，舊的各服務 module import 不到。手寫第二份中文對照表的失敗形狀是 drift
# 不是錯誤 —— CodeMeta 改了 Label，副本不編譯失敗、不轉紅，只是從那天起稽核與
# 通知顯示舊字串（56 號設計書 §6.1.12）。同 generate_go_media 的動機（issue #1530）。
#
# 涵蓋清單見 GO_LABEL_ENUMS。
#
# 值常數只對「Value 全為數字」的 enum 生成：4402 這種數字在 Go 端無法自我說明，
# 正是禁硬編碼要消滅的字面值來源；UPPER_SNAKE / snake_case 的 Value 本身就是
# 識別字，各服務已有自己的常數，再生成一份只是多一個同義名字。
#
# 對齊自算（純 bash，不依賴 gofmt binary），輸出即 gofmt-clean，供 CodeMeta drift
# gate 純文字重跑後 diff 穩定。
# =====================================================================
generate_go_labels() {
    local parsed_file="$1"
    local output_file="$2"

    mkdir -p "$(dirname "$output_file")"

    local work="$TMP_DIR/labels"
    rm -rf "$work"
    mkdir -p "$work"

    local cur_enum="" cur_desc="" keep=0 want=""
    while IFS= read -r line; do
        if [[ "$line" == "===" ]]; then
            keep=0; cur_enum=""
            continue
        fi
        if [[ "$line" == ENUM:* ]]; then
            local data="${line#ENUM:}"
            IFS='|' read -r cur_enum _ _ _ _ _ cur_desc _ <<< "$data"
            keep=0
            for want in "${GO_LABEL_ENUMS[@]}"; do
                if [[ "$cur_enum" == "$want" ]]; then keep=1; break; fi
            done
            if [[ $keep -eq 1 ]]; then
                printf '%s\n' "$cur_desc" > "$work/$cur_enum.desc"
                : > "$work/$cur_enum.rows"
            fi
            continue
        fi
        if [[ "$line" == ROW:* && $keep -eq 1 ]]; then
            echo "${line#ROW:}" >> "$work/$cur_enum.rows"
        fi
    done < "$parsed_file"

    # 清單裡少掉任何一個就中止：靜默略過會讓消費端拿到一個不存在的識別字（編譯
    # 失敗，還算好）或一張空表（Label 顯示成原始碼值，沒有任何東西會報錯）。
    local e
    for e in "${GO_LABEL_ENUMS[@]}"; do
        if [[ ! -s "$work/$e.rows" ]]; then
            echo "   ⚠ 解析結果中找不到 ${e}，中止 Go labels 生成" >&2
            return 1
        fi
    done

    {
        echo "// Code generated by generate.sh from CodeMeta Markdown. DO NOT EDIT."
        echo "//"
        echo "// 修改代碼值或標籤請編輯 docs/代碼管理（CodeMeta）/*.md 後執行："
        echo "//   ./docs/代碼管理（CodeMeta）/scripts/generate.sh --go-labels"
        echo ""
        echo "package codemeta"
    } > "$output_file"

    for e in "${GO_LABEL_ENUMS[@]}"; do
        _append_go_labels_enum "$e" "$work" "$output_file"
    done
}

_append_go_labels_enum() {
    local enum_name="$1" work="$2" ofile="$3"
    local rows="$work/$enum_name.rows"
    local desc
    desc="$(cat "$work/$enum_name.desc")"
    local lower="$(echo "${enum_name:0:1}" | tr '[:upper:]' '[:lower:]')${enum_name:1}"

    # 註解只取說明的第一句：完整敘述在 Markdown SSOT，生成檔只需要指認這是什麼。
    if [[ "$desc" == *"。"* ]]; then
        desc="${desc%%。*}。"
    fi

    local numeric=1
    local name code value label description
    while IFS='|' read -r name code value label description; do
        [[ "$value" =~ ^[0-9]+$ ]] || numeric=0
    done < "$rows"

    {
        echo ""
        echo "// ${enum_name} ${desc}"

        if [[ $numeric -eq 1 ]]; then
            local max_name=0 max_val=0
            while IFS='|' read -r name code value label description; do
                local var_name="${enum_name}$(to_pascal "$name")"
                (( ${#var_name} > max_name )) && max_name=${#var_name}
                (( ${#value} > max_val )) && max_val=${#value}
            done < "$rows"

            echo "const ("
            while IFS='|' read -r name code value label description; do
                local var_name="${enum_name}$(to_pascal "$name")"
                printf '\t%-*s = %-*s // %s\n' "$max_name" "$var_name" "$max_val" "$value" "$label"
            done < "$rows"
            echo ")"
            echo ""
        fi

        # 寬度要量**跳脫後**的長度，否則含引號的 value 會讓對齊算錯而 gofmt gate 轉紅。
        local max_key=0
        while IFS='|' read -r name code value label description; do
            local key="\"$(java_escape "$value")\":"
            (( ${#key} > max_key )) && max_key=${#key}
        done < "$rows"

        echo "var ${lower}Labels = map[string]string{"
        while IFS='|' read -r name code value label description; do
            local key="\"$(java_escape "$value")\":"
            printf '\t%-*s "%s",\n' "$max_key" "$key" "$(java_escape "$label")"
        done < "$rows"
        echo "}"
        echo ""
        echo "// ${enum_name}Label 回傳 value 對應的中文標籤。未登錄的值原樣回傳："
        echo "// 顯示原始碼值仍看得出發生了什麼，回空字串則會在畫面上留下一個無法追查的空格。"
        echo "func ${enum_name}Label(value string) string {"
        echo "	if l, ok := ${lower}Labels[value]; ok {"
        echo "		return l"
        echo "	}"
        echo "	return value"
        echo "}"
    } >> "$ofile"
}

# =====================================================================
# Diff 模式 — 比較生成結果與現行檔案
# =====================================================================
run_diff() {
    echo "📊 比較生成結果與現行 Java 檔案..."
    echo ""

    local diff_count=0
    local same_count=0
    local new_count=0

    for gen_file in "$TMP_DIR/java/"*.java; do
        local basename
        basename=$(basename "$gen_file")
        local existing="$JAVA_OUTPUT_DIR/$basename"

        if [[ ! -f "$existing" ]]; then
            echo "🆕 $basename （新檔案）"
            new_count=$((new_count + 1))
            continue
        fi

        # 比較時忽略自定義方法區塊（只比常數和標準方法）
        if diff -q "$gen_file" "$existing" > /dev/null 2>&1; then
            same_count=$((same_count + 1))
        else
            echo "⚠ $basename 有差異："
            diff --color=auto -u "$existing" "$gen_file" | head -30 || true
            echo "   ... (使用 diff '$existing' '$gen_file' 查看完整差異)"
            echo ""
            diff_count=$((diff_count + 1))
        fi
    done

    echo ""
    echo "結果：✅ 相同 $same_count | ⚠ 差異 $diff_count | 🆕 新增 $new_count"
}

# =====================================================================
# 主流程
# =====================================================================
main() {
    local do_java=0 do_go=0 do_go_errors=0 do_go_media=0 do_go_labels=0 do_go_registry=0 do_flutter=0 do_web=0 do_admin=0 do_diff=0

    for arg in "$@"; do
        case "$arg" in
            --java)       do_java=1 ;;
            --go)         do_go=1 ;;
            --go-errors)  do_go_errors=1 ;;
            --go-media)   do_go_media=1 ;;
            --go-labels)  do_go_labels=1 ;;
            --go-registry) do_go_registry=1 ;;
            --flutter)    do_flutter=1 ;;
            --web)        do_web=1 ;;
            --admin)      do_admin=1 ;;
            --all)        do_java=1; do_go=1; do_go_errors=1; do_go_media=1; do_go_labels=1; do_go_registry=1; do_flutter=1; do_web=1; do_admin=1 ;;
            --diff)       do_diff=1 ;;
            --help|-h)
                echo "使用方式: $0 [--java] [--go] [--go-errors] [--go-media] [--go-labels] [--go-registry] [--flutter] [--web] [--admin] [--all] [--diff]"
                echo "  --go-errors  生成 Go 業務碼常數（pkg/errors/bizcode/bizcode.go）"
                echo "  --go-media   生成 Go media 引用類型常數（pkg/media/ref_type.go；AssetReferenceType）"
                echo "  --go-labels  生成 Go Value→Label 對照（pkg/codemeta/labels.go；清單見 GO_LABEL_ENUMS）"
                echo "  --go-registry 生成 Go registry 常數（pkg/codemeta/registry.go；清單見 GO_REGISTRY_ENUMS）"
                echo "  --flutter    生成 Flutter（buyer-app 與 seller-app 的 lib/core/codemeta/code_meta.dart）"
                echo "  --web        生成 Web/TypeScript（website/lib/codemeta/generated.ts；全部前端可見碼表）"
                echo "  --admin      生成 ADM/TypeScript（admin/src/lib/codemeta/generated.ts；清單見 ADMIN_ENUMS）"
                exit 0 ;;
            *)
                echo "❌ 未知參數: $arg"
                exit 1 ;;
        esac
    done

    if [[ $do_java -eq 0 && $do_go -eq 0 && $do_go_errors -eq 0 && $do_go_media -eq 0 && $do_go_labels -eq 0 && $do_go_registry -eq 0 && $do_flutter -eq 0 && $do_web -eq 0 && $do_admin -eq 0 && $do_diff -eq 0 ]]; then
        echo "使用方式: $0 [--java] [--go] [--go-errors] [--go-media] [--go-labels] [--go-registry] [--flutter] [--web] [--admin] [--all] [--diff]"
        exit 1
    fi

    # 解析所有 Markdown
    echo "📖 解析 Markdown 檔案..."
    local parsed_file="$TMP_DIR/parsed.txt"
    > "$parsed_file"
    for md_file in "${MD_FILES[@]}"; do
        if [[ -f "$md_file" ]]; then
            parse_markdown "$md_file" >> "$parsed_file"
        else
            echo "   ⚠ 找不到 ${md_file}，跳過"
        fi
    done
    local enum_count
    enum_count=$(grep -c '^ENUM:' "$parsed_file" || true)
    echo "   找到 ${enum_count} 個 Enum 定義"
    local parsed_error_file="$TMP_DIR/parsed_errors.txt"
    if [[ -f "$ERROR_MD_FILE" ]]; then
        parse_error_markdown "$ERROR_MD_FILE" > "$parsed_error_file"
        local error_count
        error_count=$(grep -c '^ROW:' "$parsed_error_file" || true)
        echo "   找到 ${error_count} 個錯誤碼定義"
    else
        > "$parsed_error_file"
        echo "   ⚠ 找不到 ${ERROR_MD_FILE}，跳過錯誤碼生成"
    fi
    echo ""

    # Java 生成
    if [[ $do_java -eq 1 || $do_diff -eq 1 ]]; then
        local java_target_dir="$JAVA_OUTPUT_DIR"
        if [[ $do_diff -eq 1 ]]; then
            java_target_dir="$TMP_DIR/java"
        fi
        mkdir -p "$java_target_dir"

        echo "☕ 生成 Java CodeMeta Enum..."
        local current_enum="" current_desc="" current_special=""
        local current_iface=""
        local constants_tmp="$TMP_DIR/constants"
        > "$constants_tmp"

        while IFS= read -r line; do
            if [[ "$line" == "===" ]]; then
                if [[ -n "$current_enum" && "$current_iface" != "Legacy" ]]; then
                    generate_java_enum "$current_enum" "$current_desc" "$current_special" \
                        "$constants_tmp" "$java_target_dir/${current_enum}.java"
                    if [[ $do_diff -eq 0 ]]; then
                        echo "   ✓ ${current_enum}.java"
                    fi
                elif [[ -n "$current_enum" && "$current_iface" == "Legacy" ]]; then
                    if [[ $do_diff -eq 0 ]]; then
                        echo "   ⏭ ${current_enum}.java (Legacy，跳過)"
                    fi
                fi
                current_enum=""
                > "$constants_tmp"
                continue
            fi

            if [[ "$line" == ENUM:* ]]; then
                local data="${line#ENUM:}"
                IFS='|' read -r current_enum _ current_iface _ _ _ current_desc current_special <<< "$data"
                > "$constants_tmp"
                continue
            fi

            if [[ "$line" == ROW:* ]]; then
                echo "${line#ROW:}" >> "$constants_tmp"
            fi
        done < "$parsed_file"

        # 最後一個
        if [[ -n "$current_enum" && "$current_iface" != "Legacy" ]]; then
            generate_java_enum "$current_enum" "$current_desc" "$current_special" \
                "$constants_tmp" "$java_target_dir/${current_enum}.java"
            if [[ $do_diff -eq 0 ]]; then
                echo "   ✓ ${current_enum}.java"
            fi
        elif [[ -n "$current_enum" && "$current_iface" == "Legacy" ]]; then
            if [[ $do_diff -eq 0 ]]; then
                echo "   ⏭ ${current_enum}.java (Legacy，跳過)"
            fi
        fi

        if [[ -s "$parsed_error_file" ]]; then
            generate_java_error_code "$parsed_error_file" "$java_target_dir/GrpcResponseCode.java"
            if [[ $do_diff -eq 0 ]]; then
                echo "   ✓ GrpcResponseCode.java"
            fi
        fi
        echo ""
    fi

    # Diff 模式
    if [[ $do_diff -eq 1 ]]; then
        run_diff
        return
    fi

    # Go 生成
    if [[ $do_go -eq 1 ]]; then
        mkdir -p "$GO_OUTPUT_DIR"
        echo "🔵 生成 Go 常數定義..."
        generate_go_file "$parsed_file" "$GO_OUTPUT_DIR/code_meta.go"
        echo "   ✓ internal/platform/codemeta/code_meta.go"
        echo ""
    fi

    # Go 業務碼生成
    if [[ $do_go_errors -eq 1 ]]; then
        echo "🔵 生成 Go 業務碼常數..."
        local parsed_error_file="$TMP_DIR/errors_parsed.txt"
        parse_error_markdown "$ERROR_MD_FILE" > "$parsed_error_file"
        if [[ -s "$parsed_error_file" ]]; then
            generate_go_bizcode "$parsed_error_file" "$GO_ERRORS_OUTPUT_FILE"
            echo "   ✓ pkg/errors/bizcode/bizcode.go"
        else
            echo "   ⚠ 無錯誤碼資料，跳過 Go 業務碼生成"
        fi
        echo ""
    fi

    # Go media 引用類型生成（AssetReferenceType 前端可見=No，不走 --go，獨立直寫）
    if [[ $do_go_media -eq 1 ]]; then
        echo "🔵 生成 Go media 引用類型常數..."
        generate_go_media "$parsed_file" "$GO_MEDIA_OUTPUT_FILE"
        echo "   ✓ pkg/media/ref_type.go"
        generate_go_media "$parsed_file" "$GO_MEDIA_MONOLITH_OUTPUT_FILE"
        echo "   ✓ internal/modules/media/ref_type.go"
        echo ""
    fi

    # Go Value→Label 對照生成（含前端可見=No 的 enum，不走 --go，獨立直寫）
    if [[ $do_go_labels -eq 1 ]]; then
        echo "🔵 生成 Go CodeMeta Label 對照..."
        generate_go_labels "$parsed_file" "$GO_LABELS_OUTPUT_FILE"
        echo "   ✓ pkg/codemeta/labels.go"
        echo ""
    fi

    # Go registry 生成（typed 常數 + AllXxx + FromCode/FromValue，直寫 Go module）
    if [[ $do_go_registry -eq 1 ]]; then
        echo "🔵 生成 Go CodeMeta registry..."
        generate_go_file "$parsed_file" "$GO_REGISTRY_OUTPUT_FILE" "${GO_REGISTRY_ENUMS[*]}" "--go-registry"
        echo "   ✓ pkg/codemeta/registry.go"
        echo ""
    fi

    # Flutter 生成
    if [[ $do_flutter -eq 1 ]]; then
        echo "💙 生成 Flutter Dart Enum..."
        generate_flutter_file "$parsed_file" "${FLUTTER_OUTPUT_FILES[0]}"
        cp "${FLUTTER_OUTPUT_FILES[0]}" "${FLUTTER_OUTPUT_FILES[1]}"
        echo "   ✓ buyer-app/lib/core/codemeta/code_meta.dart"
        echo "   ✓ seller-app/lib/core/codemeta/code_meta.dart"
        echo ""
    fi

    # Web 生成
    if [[ $do_web -eq 1 ]]; then
        echo "🌐 生成 Web TypeScript Enum..."
        generate_ts_file "$parsed_file" "$WEB_OUTPUT_FILE" "--web" ""
        echo "   ✓ website/lib/codemeta/generated.ts"
        echo ""
    fi

    # ADM 生成
    if [[ $do_admin -eq 1 ]]; then
        echo "🛠  生成 ADM TypeScript Enum..."
        generate_ts_file "$parsed_file" "$ADMIN_OUTPUT_FILE" "--admin" "${ADMIN_ENUMS[*]}"
        echo "   ✓ admin/src/lib/codemeta/generated.ts"
        echo ""
    fi

    echo "✅ 生成完成！"
}

main "$@"
