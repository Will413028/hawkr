package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.order.grpc.CheckoutInvoiceRequest;

import java.util.regex.Pattern;

/**
 * 把結帳請求的**選填** {@code invoice} 區塊轉成 {@code order_invoice_snapshots} 的欄位
 * （58 號 §5.4.3 DDL、§6.1.2(b)、§7.1.2 步驟 1）。
 *
 * <p><b>純函式、無 IO</b>：不打任何 gRPC、不查 DB。載具預設的解析（{@code USE_BUYER_DEFAULT}）
 * 刻意<b>不</b>在結帳當下做——那要打 invoice-service 一次 gRPC，而這裡是結帳 hot path
 * （§5.4.3 表註解逐字）。
 *
 * <p><b>缺省行為（硬規則）</b>：整包 {@code invoice} 省略、或每個欄位都空
 * ⇒ {@code invoice_kind = B2C}、{@code carrier_selection_mode = USE_BUYER_DEFAULT}。
 * invoice-service 建立文件時才依 D6 fallback 鏈解析「買家指定的預設 → 無載具發票」。
 * 🔴 這個分支<b>必須永遠成立</b>：{@code hawkr_buyer} 已上架 App 直接呼叫
 * {@code /app/checkout/confirm} 且完全不知道 {@code invoice} 這個欄位存在
 * （{@code lib/features/cart/presentation/widgets/checkout_payment_sheet.dart}），
 * 任何把它變成必填的改動都會立刻打爛線上結帳。
 *
 * <p><b>本層的驗證只是 fail-fast</b>：格式與互斥性在這裡擋（讓買家當場改），
 * 但 {@code buyer_invoice_preference_id} 的<b>歸屬驗證（IDOR，I-47）收斂點在 invoice-service</b>
 * ——它是唯一一條寫進發票的路徑，且 {@code buyer_id} 只能取自訂單、不得取自請求。
 * order-service 這裡拿得到的 buyerId 來自 Gateway 注入，用它做歸屬判斷會是「拿請求端的值
 * 驗請求端的值」，沒有意義。
 *
 * <p>值域一律走 CodeMeta 生成的 enum（{@link InvoiceDocumentKind}／
 * {@link InvoiceCarrierSelectionMode}），不寫字串字面值。
 */
public final class InvoiceIntakeResolver {

    /** 統編 8 碼數字（{@code chk_ois_b2b}）。 */
    private static final Pattern TAX_ID = Pattern.compile("^[0-9]{8}$");

    /** 捐贈碼 3–7 碼數字（{@code chk_ois_donation_fmt}）。 */
    private static final Pattern DONATION_CODE = Pattern.compile("^[0-9]{3,7}$");

    /**
     * 與 {@code chk_ois_notify_email} 逐字相同的形狀檢查。兩邊必須一致：DB 較嚴會在建單交易
     * 內炸 23514（買家看到的是 500 而不是可修正的 9305），DB 較鬆則這裡形同虛設。
     */
    private static final Pattern NOTIFY_EMAIL =
            Pattern.compile("^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$");

    /** AMEGO BuyerName 上限（{@code buyer_title VARCHAR(60)}，以字元計）。 */
    static final int BUYER_TITLE_MAX = 60;

    /** {@code notify_email VARCHAR(254)}：RFC 5321 §4.5.3.1.3 的 path 上限。 */
    static final int NOTIFY_EMAIL_MAX = 254;

    private InvoiceIntakeResolver() {
    }

    /**
     * 解析結果。全部欄位可為 {@code null}（＝該欄在 DB 為 NULL）；
     * {@code invoiceKind} 與 {@code carrierSelectionMode} 恆非 null（DDL 為 NOT NULL）。
     */
    public record Snapshot(
            String invoiceKind,
            String carrierSelectionMode,
            Long buyerInvoicePreferenceId,
            String buyerTaxId,
            String buyerTitle,
            String donationCode,
            String notifyEmail
    ) {
    }

    /** 缺省快照：買家沒帶 invoice 區塊時的形狀。 */
    public static Snapshot defaultSnapshot() {
        return new Snapshot(
                InvoiceDocumentKind.B2C.value(),
                InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value(),
                null, null, null, null, null);
    }

    /**
     * @param request     {@code CheckoutConfirmRequest.invoice}；未帶時傳 {@code null}
     * @param hasInvoice  proto 的 {@code hasInvoice()}（proto3 message 欄位的 presence）。
     *                    ⚠️ 不可用「每個欄位都空」代替：那兩者今天等價，但把
     *                    presence 判斷寫成「內容判斷」會在未來加上 bool 欄位時靜默改變語意。
     * @throws OrderBusinessException 9305（格式不正確）／9306（統編、載具、捐贈碼三者只能擇一）
     */
    public static Snapshot resolve(CheckoutInvoiceRequest request, boolean hasInvoice) {
        if (!hasInvoice || request == null) {
            return defaultSnapshot();
        }

        String profileId = trimToNull(request.getProfileId());
        String buyerTaxId = trimToNull(request.getBuyerTaxId());
        String buyerTitle = trimToNull(request.getBuyerTitle());
        String donationCode = trimToNull(request.getDonationCode());
        String notifyEmail = trimToNull(request.getNotifyEmail());

        // "0" 是前端「已清除預設」的哨兵（§6.1.2 端點 2 註）：pkg/tsid 的 time-based 高位恆非零，
        // 不會產生 id = 0 ⇒ 拿它當「未指定」不會與任何真實 profile 撞號。
        if ("0".equals(profileId)) {
            profileId = null;
        }

        int targets = (profileId != null ? 1 : 0)
                + (buyerTaxId != null ? 1 : 0)
                + (donationCode != null ? 1 : 0);
        if (targets > 1) {
            // 我方強制；AMEGO 不擋（58 號 I-04、§6.1.2 confirm 失敗分支表）
            throw new OrderBusinessException(GrpcResponseCode.INVOICE_TARGET_CONFLICT);
        }

        Long preferenceId = null;
        if (profileId != null) {
            preferenceId = parsePositiveId(profileId);
        }
        if (buyerTaxId != null && !TAX_ID.matcher(buyerTaxId).matches()) {
            throw new OrderBusinessException(GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }
        if (donationCode != null && !DONATION_CODE.matcher(donationCode).matches()) {
            throw new OrderBusinessException(GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }
        if (buyerTaxId != null) {
            // chk_ois_b2b 要求 B2B 必有抬頭；讓它在這裡以 9305 失敗，而不是在建單交易內炸 23514
            if (buyerTitle == null) {
                throw new OrderBusinessException(GrpcResponseCode.INVOICE_CARRIER_INVALID);
            }
            // 用**字元數**而非 displayLength：DB 是 VARCHAR(60)（PostgreSQL 以字元計），
            // 而 displayLength 把全形算 2 ⇒ 會把「30 個中文字的合法公司名」擋掉，
            // 那是採集端多擋、使用者看得到的失敗。
            if (buyerTitle.codePointCount(0, buyerTitle.length()) > BUYER_TITLE_MAX) {
                throw new OrderBusinessException(GrpcResponseCode.INVOICE_CARRIER_INVALID);
            }
        } else if (buyerTitle != null) {
            // 沒有統編的抬頭在 AMEGO 無處可放（B2C 的 BuyerName 恆為預設值）⇒ 不靜默丟棄，
            // 靜默丟棄會讓買家以為自己填的抬頭會出現在發票上。
            throw new OrderBusinessException(GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }
        if (notifyEmail != null
                && (notifyEmail.length() > NOTIFY_EMAIL_MAX || !NOTIFY_EMAIL.matcher(notifyEmail).matches())) {
            throw new OrderBusinessException(GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }

        String kind = buyerTaxId != null
                ? InvoiceDocumentKind.B2B.value()
                : InvoiceDocumentKind.B2C.value();

        String mode;
        if (buyerTaxId != null) {
            // B2B 一律 NONE（chk_ois_b2b 逐字要求）
            mode = InvoiceCarrierSelectionMode.NONE.value();
        } else if (donationCode != null) {
            mode = InvoiceCarrierSelectionMode.DONATION.value();
        } else if (preferenceId != null) {
            mode = InvoiceCarrierSelectionMode.EXPLICIT_PREFERENCE.value();
        } else {
            mode = InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value();
        }

        return new Snapshot(kind, mode, preferenceId, buyerTaxId, buyerTitle, donationCode, notifyEmail);
    }

    private static Long parsePositiveId(String raw) {
        try {
            long v = Long.parseLong(raw);
            if (v <= 0) {
                throw new OrderBusinessException(GrpcResponseCode.INVOICE_CARRIER_INVALID);
            }
            return v;
        } catch (NumberFormatException e) {
            throw new OrderBusinessException(GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }
    }

    private static String trimToNull(String v) {
        if (v == null) {
            return null;
        }
        String t = v.trim();
        return t.isEmpty() ? null : t;
    }
}
