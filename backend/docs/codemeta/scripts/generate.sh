#!/bin/bash

set -euo pipefail

export LC_ALL=C

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
MD_DIR="$(dirname "$SCRIPT_DIR")"
PROJECT_ROOT="$(cd "$MD_DIR/../.." && pwd)"

JAVA_OUTPUT_DIR="$PROJECT_ROOT/services/java/shared/common-core/src/main/java/io/github/will413028/hawkr/common/utils/code"
JAVA_ERROR_OUTPUT_FILE="$JAVA_OUTPUT_DIR/GrpcResponseCode.java"
GO_OUTPUT_DIR="$PROJECT_ROOT/internal/platform/codemeta"
GO_ERRORS_OUTPUT_FILE="$PROJECT_ROOT/pkg/errors/bizcode/bizcode.go"
GO_MEDIA_OUTPUT_FILE="$PROJECT_ROOT/pkg/media/ref_type.go"
GO_LABELS_OUTPUT_FILE="$PROJECT_ROOT/pkg/codemeta/labels.go"
GO_REGISTRY_OUTPUT_FILE="$PROJECT_ROOT/pkg/codemeta/registry.go"
MONOREPO_ROOT="$(cd "$PROJECT_ROOT/.." && pwd)"
FLUTTER_OUTPUT_FILES=("$MONOREPO_ROOT/buyer-app/lib/core/codemeta/code_meta.dart" "$MONOREPO_ROOT/seller-app/lib/core/codemeta/code_meta.dart")
WEB_OUTPUT_FILE="$MONOREPO_ROOT/website/lib/codemeta/generated.ts"
ADMIN_OUTPUT_FILE="$MONOREPO_ROOT/admin/src/lib/codemeta/generated.ts"
TMP_DIR="$(mktemp -d)"

MD_FILES=("$MD_DIR/business-status-codes.md" "$MD_DIR/business-type-codes.md" "$MD_DIR/internal-technical-codes.md")
ERROR_MD_FILE="$MD_DIR/error-codes.md"

GO_LABEL_ENUMS=(ProductStatus PostPublishStatus OrderStatus DelegationReasonCode DelegationAuditAction DelegationAuthorizationAction CertificationType)

GO_REGISTRY_ENUMS=(
    CommonStatus
    ProcurementRunStatus ProcurementStepStatus ProcurementLaneStatus ProcurementArtifactStatus
    ProcurementConfirmationStatus ProcurementRunFailureReason ProcurementErrorReason
    ProcurementLedgerEventType ProcurementArtifactType
    ProcurementRfqPublicationApprovalState ProcurementPublicationAudienceMode ProcurementWishPoolPolicy ProcurementPublicationState
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
    CommerceModel CollectionModel FulfillmentMode InvoiceModel
    LedgerOwnerType LedgerAccountCode LedgerTxnType
    FulfillmentAction UberDeliveryOutcome
    LogisticsReceivableStatus ResponsibilityCause
)

ADMIN_ENUMS=(
    ProcurementRunStatus ProcurementStepStatus ProcurementLaneStatus
    ProcurementArtifactStatus ProcurementConfirmationStatus
    ProcurementRfqPublicationApprovalState
    ProcurementRunFailureReason ProcurementErrorReason ProcurementLedgerEventType
    ProcurementArtifactType
    ProcurementPublicationAudienceMode ProcurementPublicationState
    ProcurementSellerResponseType ProcurementTaxInclusionBasis
    ProcurementSellerPartyType ProcurementSellerResponseStatus
    ProcurementQuoteAcceptability ProcurementOrderProvisioningState
    ProcurementWishPoolPolicy
    OrderStatus AfterSaleAppealState PaymentMethod PaymentTransactionStatus RefundStatus
    ShippingMethod ShippingTemperature ShippingFeePayer WeightUnit ShippingStatus
    CommerceModel CollectionModel FulfillmentMode InvoiceModel
    FulfillmentAction UberDeliveryOutcome ShippingType ReceiptStatus
    LedgerOwnerType LedgerAccountCode LedgerTxnType
    LogisticsReceivableStatus ResponsibilityCause
    ShipmentAnomalyType ShipmentAnomalySeverity ShipmentAnomalyAckAction
    ProviderSource DelegationAuditAction
    SellerSuspensionReason AfterSalePlatformDecisionOutcome
    ProductStatus ProductPublicationState
    ReportStatus ReportTargetType ReportReasonCode ReportResolveAction
    CertificationType ProductCertificationStatus ProductCertificationSource
    ProductCertificationGovVerifyState ProductCertificationValidity
    ProductCertificationReviewAction ProductCertificationActorRole
    ProductCertificationRejectReason
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
            # 「特殊方法」key 已廢棄（Java 微服務dev-guidelines §6），讀取後忽略
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
        echo " * <p>⚠ 此檔案由 generate.sh 自動生成，請以 docs/codemeta/error-codes.md 為唯一來源。</p>"
        echo " */"
        echo "public enum GrpcResponseCode {"
        echo ""

        local total_rows=0
        total_rows=$(grep -c '^ROW:' "$parsed_file" || true)
        local row_index=0
        while IFS= read -r line; do
            if [[ "$line" == SECTION:* ]]; then
                local section="${line#SECTION:}"
                if [[ "$section" =~ ^(通用錯誤碼|.*業務碼) ]]; then
                    if [[ $row_index -gt 0 ]]; then
                        echo ""
                    fi
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

generate_java_enum() {
    local enum_name="$1"
    local desc="$2"
    local special_methods="$3"
    local constants_file="$4"
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
        echo " * （見 docs/dev-guidelines/java-microservices-development-guidelines.md §6）。"
        echo " */"
        echo "public enum ${enum_name} implements CodeMeta {"
        echo ""

        local total_lines
        total_lines=$(wc -l < "$constants_file" | tr -d ' ')
        local line_num=0
        while IFS='|' read -r name code value display description; do
            line_num=$((line_num + 1))
            local suffix=","
            if [[ $line_num -eq $total_lines ]]; then suffix=";"; fi
            echo "    ${name}(\"$(java_escape "${code}")\", \"$(java_escape "${value}")\", \"$(java_escape "${display}")\", \"$(java_escape "${description}")\")${suffix}"
        done < "$constants_file"

        echo ""

        echo "    private final String code;"
        echo "    private final String value;"
        echo "    private final String label;"
        echo "    private final String description;"
        echo ""

        echo "    ${enum_name}(String code, String value, String label, String description) {"
        echo "        this.code = code;"
        echo "        this.value = value;"
        echo "        this.label = label;"
        echo "        this.description = description;"
        echo "    }"
        echo ""

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

        echo "    private static final Map<String, ${enum_name}> BY_CODE ="
        echo "        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(${enum_name}::code, e -> e));"
        echo ""
        echo "    private static final Map<String, ${enum_name}> BY_VALUE ="
        echo "        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(${enum_name}::value, e -> e));"
        echo ""

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
    local allowlist="${3:-}"
    local regen_flag="${4:---go}"

    mkdir -p "$(dirname "$output_file")"

    {
        echo "// Code generated by generate.sh from CodeMeta Markdown. DO NOT EDIT."
        echo "//"
        echo "// 此檔案由 generate.sh 自動生成，請勿手動修改。"
        echo "// 修改代碼值請編輯 docs/codemeta/*.md 後執行："
        echo "//   ./docs/codemeta/scripts/generate.sh ${regen_flag}"
        echo ""
        echo "package codemeta"
        echo ""
    } > "$output_file"

    _go_should_emit() {
        if [[ -z "$allowlist" ]]; then
            [[ "$visible" =~ ^[Yy] ]]
        else
            [[ " $allowlist " == *" $1 "* ]]
        fi
    }

    local enum_name="" desc="" visible=""
    local emitted="|"
    local constants_file="$TMP_DIR/go_constants"

    while IFS= read -r line; do
        if [[ "$line" == "===" ]]; then
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

    if [[ -n "$enum_name" ]] && _go_should_emit "$enum_name"; then
        _append_go_enum "$enum_name" "$desc" "$constants_file" "$output_file"
        emitted="${emitted}${enum_name}|"
    fi

    printf '%s\n' "$(cat "$output_file")" > "${output_file}.tmp" \
        && mv "${output_file}.tmp" "$output_file"

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
            printf '\t%-*s = %s{Code: "%s", Value: "%s", Label: "%s", Description: "%s"}\n' \
                "$max_var_len" "$var_name" "$enum_name" \
                "$(java_escape "$code")" "$(java_escape "$value")" \
                "$(java_escape "$display")" "$(java_escape "$description")"
        done < "$cfile"
        echo ")"
        echo ""

        echo "var All${enum_name}s = []${enum_name}{"
        while IFS='|' read -r name _ _ _ _; do
            local pascal
            pascal=$(to_pascal "$name")
            echo "	${enum_name}${pascal},"
        done < "$cfile"
        echo "}"
        echo ""

        echo "func ${enum_name}FromCode(code string) (${enum_name}, bool) {"
        echo "	for _, v := range All${enum_name}s {"
        echo "		if v.Code == code {"
        echo "			return v, true"
        echo "		}"
        echo "	}"
        echo "	return ${enum_name}{}, false"
        echo "}"
        echo ""

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
        echo "// 修改代碼值請編輯 backend/docs/codemeta/*.md 後重新生成（generate.sh --flutter）。"
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

to_snake_upper() {
    echo "$1" | sed -E 's/([a-z0-9])([A-Z])/\1_\2/g; s/([A-Z]+)([A-Z][a-z])/\1_\2/g' | tr '[:lower:]' '[:upper:]'
}

ts_escape() {
    local value="$1"
    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    printf '%s' "$value"
}

dart_escape() {
    printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e "s/'/\\\\'/g"
}

generate_ts_file() {
    local parsed_file="$1"
    local output_file="$2"
    local flag="$3"
    local allowlist="$4"

    {
        echo "// Code generated by generate.sh from CodeMeta Markdown. DO NOT EDIT."
        echo "//"
        echo "// 此檔案由 generate.sh 自動生成，請勿手動修改。"
        echo "// 修改代碼值請編輯 backend/docs/codemeta/*.md 後重新生成（generate.sh ${flag}）。"
        echo ""
    } > "$output_file"

    local enum_name="" desc="" visible=""
    local constants_file="$TMP_DIR/ts_constants"
    local emitted="|"

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

        echo "export const ${snake}_LABEL: Record<string, string> = {"
        local seen_values="|"
        while IFS='|' read -r name code value display description; do
            if [[ "$seen_values" == *"|${value}|"* ]]; then
                continue
            fi
            seen_values="${seen_values}${value}|"
            local clean_display="${display%（終態）}"
            echo "	\"$(ts_escape "$value")\": \"$(ts_escape "$clean_display")\","
        done < "$cfile"
        echo "};"
        echo ""
    } >> "$ofile"
}

generate_go_bizcode() {
    local parsed_error_file="$1"
    local output_file="$2"

    mkdir -p "$(dirname "$output_file")"

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
            IFS='|' read -r name code msg <<< "$data"
            if [[ "$code" =~ ^[0-9]+$ ]] && [[ "$code" -ge 1000 ]]; then
                printf '%s|%s|%s|%s\n' "$current_section" "$name" "$code" "$msg" >> "$biz_rows_file"
            fi
        fi
    done < "$parsed_error_file"

    local _s _n _c _m
    while IFS='|' read -r _s _n _c _m; do
        if [[ -z "$_m" ]]; then
            echo "   ⚠ 業務碼 ${_c}（${_n}）沒有 Message，中止 Go 業務碼生成" >&2
            return 1
        fi
    done < "$biz_rows_file"
    if [[ ! -s "$biz_rows_file" ]]; then
        echo "   ⚠ error-codes.md 解析不出任何業務碼（code >= 1000），中止 Go 業務碼生成" >&2
        return 1
    fi

    {
        echo "// Code generated by generate.sh from error-codes.md. DO NOT EDIT."
        echo "//"
        echo "// 修改業務碼請編輯 docs/codemeta/error-codes.md 後執行："
        echo "//   ./docs/codemeta/scripts/generate.sh --go-errors"
        echo ""
        echo "package bizcode"
        echo ""
        echo "const ("

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

        local msg_file="$TMP_DIR/biz_messages.txt"
        : > "$msg_file"
        local max_key=0
        while IFS='|' read -r section name code msg; do
            local key="${code}:"
            (( ${#key} > max_key )) && max_key=${#key}
            printf '%s|%s\n' "$key" "$(java_escape "$msg")" >> "$msg_file"
        done < "$biz_rows_file"

        echo ""
        echo "// bizCodeMessages 是error-codes.md 的 Message 欄逐字對照，與 Java 的"
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

generate_go_media() {
    local parsed_file="$1"
    local output_file="$2"

    mkdir -p "$(dirname "$output_file")"

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

    local max_name=0 max_val=0
    while IFS='|' read -r name code value display description; do
        local var_name="RefType$(to_pascal "$name")"
        (( ${#var_name} > max_name )) && max_name=${#var_name}
        local quoted="\"${value}\""
        (( ${#quoted} > max_val )) && max_val=${#quoted}
    done < "$rows_file"

    {
        echo "// Code generated by generate.sh from business-type-codes.md. DO NOT EDIT."
        echo "//"
        echo "// 修改多媒體資產引用類型請編輯 docs/codemeta/business-type-codes.md 後執行："
        echo "//   ./docs/codemeta/scripts/generate.sh --go-media"
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
        echo "// 修改代碼值或標籤請編輯 docs/codemeta/*.md 後執行："
        echo "//   ./docs/codemeta/scripts/generate.sh --go-labels"
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

    if [[ $do_diff -eq 1 ]]; then
        run_diff
        return
    fi

    if [[ $do_go -eq 1 ]]; then
        mkdir -p "$GO_OUTPUT_DIR"
        echo "🔵 生成 Go 常數定義..."
        generate_go_file "$parsed_file" "$GO_OUTPUT_DIR/code_meta.go"
        echo "   ✓ internal/platform/codemeta/code_meta.go"
        echo ""
    fi

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

    if [[ $do_go_media -eq 1 ]]; then
        echo "🔵 生成 Go media 引用類型常數..."
        generate_go_media "$parsed_file" "$GO_MEDIA_OUTPUT_FILE"
        echo "   ✓ pkg/media/ref_type.go"
        echo ""
    fi

    if [[ $do_go_labels -eq 1 ]]; then
        echo "🔵 生成 Go CodeMeta Label 對照..."
        generate_go_labels "$parsed_file" "$GO_LABELS_OUTPUT_FILE"
        echo "   ✓ pkg/codemeta/labels.go"
        echo ""
    fi

    if [[ $do_go_registry -eq 1 ]]; then
        echo "🔵 生成 Go CodeMeta registry..."
        generate_go_file "$parsed_file" "$GO_REGISTRY_OUTPUT_FILE" "${GO_REGISTRY_ENUMS[*]}" "--go-registry"
        echo "   ✓ pkg/codemeta/registry.go"
        echo ""
    fi

    if [[ $do_flutter -eq 1 ]]; then
        echo "💙 生成 Flutter Dart Enum..."
        generate_flutter_file "$parsed_file" "${FLUTTER_OUTPUT_FILES[0]}"
        cp "${FLUTTER_OUTPUT_FILES[0]}" "${FLUTTER_OUTPUT_FILES[1]}"
        echo "   ✓ buyer-app/lib/core/codemeta/code_meta.dart"
        echo "   ✓ seller-app/lib/core/codemeta/code_meta.dart"
        echo ""
    fi

    if [[ $do_web -eq 1 ]]; then
        echo "🌐 生成 Web TypeScript Enum..."
        generate_ts_file "$parsed_file" "$WEB_OUTPUT_FILE" "--web" ""
        echo "   ✓ website/lib/codemeta/generated.ts"
        echo ""
    fi

    if [[ $do_admin -eq 1 ]]; then
        echo "🛠  生成 ADM TypeScript Enum..."
        generate_ts_file "$parsed_file" "$ADMIN_OUTPUT_FILE" "--admin" "${ADMIN_ENUMS[*]}"
        echo "   ✓ admin/src/lib/codemeta/generated.ts"
        echo ""
    fi

    echo "✅ 生成完成！"
}

main "$@"
