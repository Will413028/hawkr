package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierSelectionMode;
import io.github.will413028.hawkr.common.utils.code.InvoiceDocumentKind;
import io.github.will413028.hawkr.order.exception.OrderBusinessException;
import io.github.will413028.hawkr.order.grpc.CheckoutInvoiceRequest;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * 結帳發票採集的映射與 fail-fast 驗證（58 號 §5.4.3 DDL、§6.1.2(b)、§7.1.2 步驟 1）。
 *
 * <p><b>本檔守的第一件事是「選填」</b>：{@code hawkr_buyer} 已上架 App 直接呼叫
 * {@code /app/checkout/confirm} 且完全不知道 {@code invoice} 這個欄位存在
 * ⇒ 把它改成必填會立刻打爛線上結帳。鑑別力：把 {@code resolve} 對
 * {@code hasInvoice == false} 改成拋例外 ⇒ {@code OptionalBlock#omittedBlockFallsBackToBuyerDefault} 轉紅。
 *
 * <p>第二件是<b>值域必須落在 DDL 的 CHECK 之內</b>：這裡產出的每一組
 * {@code (invoice_kind, carrier_selection_mode, …)} 都要能通過 {@code chk_ois_*}。
 * 產出違反 CHECK 的組合不會在這裡失敗，會在建單交易內炸 23514——買家看到 500 而不是可修正的 9305。
 * DDL 那一側由 {@code OrderInvoiceSnapshotConstraintIT} 用真 PostgreSQL 守。
 */
class InvoiceIntakeResolverTest {

    private static CheckoutInvoiceRequest.Builder req() {
        return CheckoutInvoiceRequest.newBuilder();
    }

    private static InvoiceIntakeResolver.Snapshot resolve(CheckoutInvoiceRequest.Builder b) {
        return InvoiceIntakeResolver.resolve(b.build(), true);
    }

    private static void assertBizCode(Runnable r, GrpcResponseCode expected) {
        assertThatThrownBy(r::run)
                .isInstanceOf(OrderBusinessException.class)
                .satisfies(e -> assertThat(((OrderBusinessException) e).code()).isEqualTo(expected.code()));
    }

    @Nested
    @DisplayName("選填：沒帶 invoice 區塊的路徑必須永遠成立（線上 App 走的就是它）")
    class OptionalBlock {

        @Test
        @DisplayName("整包省略 → B2C ＋ USE_BUYER_DEFAULT，其餘欄位全 null")
        void omittedBlockFallsBackToBuyerDefault() {
            InvoiceIntakeResolver.Snapshot s = InvoiceIntakeResolver.resolve(null, false);

            assertThat(s.invoiceKind()).isEqualTo(InvoiceDocumentKind.B2C.value());
            assertThat(s.carrierSelectionMode())
                    .isEqualTo(InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value());
            assertThat(s.buyerInvoicePreferenceId()).isNull();
            assertThat(s.buyerTaxId()).isNull();
            assertThat(s.buyerTitle()).isNull();
            assertThat(s.donationCode()).isNull();
            assertThat(s.notifyEmail()).isNull();
        }

        @Test
        @DisplayName("帶了但每欄都空 → 同缺省（前端把「不使用載具」radio 送成空 invoice 也要成立）")
        void emptyBlockFallsBackToBuyerDefault() {
            InvoiceIntakeResolver.Snapshot s = resolve(req());

            assertThat(s.invoiceKind()).isEqualTo(InvoiceDocumentKind.B2C.value());
            assertThat(s.carrierSelectionMode())
                    .isEqualTo(InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value());
        }

        @Test
        @DisplayName("profileId=\"0\"（前端已清除預設的哨兵）視同未指定，不是一個 id")
        void zeroProfileIdIsSentinelNotAnId() {
            InvoiceIntakeResolver.Snapshot s = resolve(req().setProfileId("0"));

            assertThat(s.carrierSelectionMode())
                    .isEqualTo(InvoiceCarrierSelectionMode.USE_BUYER_DEFAULT.value());
            assertThat(s.buyerInvoicePreferenceId()).isNull();
        }
    }

    @Nested
    @DisplayName("三選一的映射（chk_ois_exclusive／chk_ois_pref_pair／chk_ois_donation_pair）")
    class TargetMapping {

        @Test
        @DisplayName("統編＋抬頭 → B2B ＋ NONE（chk_ois_b2b 三個條件同時成立）")
        void taxIdMapsToB2b() {
            InvoiceIntakeResolver.Snapshot s =
                    resolve(req().setBuyerTaxId("12345678").setBuyerTitle("範例生鮮股份有限公司"));

            assertThat(s.invoiceKind()).isEqualTo(InvoiceDocumentKind.B2B.value());
            assertThat(s.carrierSelectionMode()).isEqualTo(InvoiceCarrierSelectionMode.NONE.value());
            assertThat(s.buyerTaxId()).isEqualTo("12345678");
            assertThat(s.buyerTitle()).isEqualTo("範例生鮮股份有限公司");
            // chk_ois_exclusive：另外兩個目標必須為 null
            assertThat(s.buyerInvoicePreferenceId()).isNull();
            assertThat(s.donationCode()).isNull();
        }

        @Test
        @DisplayName("常用資料 id → EXPLICIT_PREFERENCE ＋ B2C，id 原樣帶下去（歸屬驗證在 invoice-service）")
        void profileIdMapsToExplicitPreference() {
            InvoiceIntakeResolver.Snapshot s = resolve(req().setProfileId("2649605557"));

            assertThat(s.invoiceKind()).isEqualTo(InvoiceDocumentKind.B2C.value());
            assertThat(s.carrierSelectionMode())
                    .isEqualTo(InvoiceCarrierSelectionMode.EXPLICIT_PREFERENCE.value());
            assertThat(s.buyerInvoicePreferenceId()).isEqualTo(2649605557L);
        }

        @Test
        @DisplayName("捐贈碼 → DONATION ＋ B2C")
        void donationCodeMapsToDonation() {
            InvoiceIntakeResolver.Snapshot s = resolve(req().setDonationCode("5238"));

            assertThat(s.invoiceKind()).isEqualTo(InvoiceDocumentKind.B2C.value());
            assertThat(s.carrierSelectionMode())
                    .isEqualTo(InvoiceCarrierSelectionMode.DONATION.value());
            assertThat(s.donationCode()).isEqualTo("5238");
        }

        @Test
        @DisplayName("統編 ＋ 捐贈碼 → 9306（AMEGO 不擋，我方強制）")
        void taxIdPlusDonationIsConflict() {
            assertBizCode(() -> resolve(req().setBuyerTaxId("12345678")
                            .setBuyerTitle("X").setDonationCode("5238")),
                    GrpcResponseCode.INVOICE_TARGET_CONFLICT);
        }

        @Test
        @DisplayName("統編 ＋ 常用資料 → 9306")
        void taxIdPlusProfileIsConflict() {
            assertBizCode(() -> resolve(req().setBuyerTaxId("12345678")
                            .setBuyerTitle("X").setProfileId("100")),
                    GrpcResponseCode.INVOICE_TARGET_CONFLICT);
        }

        @Test
        @DisplayName("常用資料 ＋ 捐贈碼 → 9306")
        void profilePlusDonationIsConflict() {
            assertBizCode(() -> resolve(req().setProfileId("100").setDonationCode("5238")),
                    GrpcResponseCode.INVOICE_TARGET_CONFLICT);
        }
    }

    @Nested
    @DisplayName("格式 fail-fast（9305）——讓買家當場改，而不是建單交易內炸 23514")
    class Format {

        @Test
        @DisplayName("統編非 8 碼數字")
        void malformedTaxId() {
            assertBizCode(() -> resolve(req().setBuyerTaxId("1234567").setBuyerTitle("X")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
            assertBizCode(() -> resolve(req().setBuyerTaxId("1234567A").setBuyerTitle("X")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }

        @Test
        @DisplayName("有統編沒抬頭（chk_ois_b2b 會拒，這裡先擋）")
        void taxIdWithoutTitle() {
            assertBizCode(() -> resolve(req().setBuyerTaxId("12345678")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }

        @Test
        @DisplayName("抬頭超過 60 字元（AMEGO BuyerName 上界 ＝ buyer_title VARCHAR(60)）")
        void titleTooLong() {
            assertBizCode(() -> resolve(req().setBuyerTaxId("12345678")
                            .setBuyerTitle("公".repeat(61))),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }

        @Test
        @DisplayName("有抬頭沒統編 → 拒絕，不靜默丟棄（丟棄會讓買家以為抬頭會印在發票上）")
        void titleWithoutTaxIdIsRejectedNotDropped() {
            assertBizCode(() -> resolve(req().setBuyerTitle("範例生鮮")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }

        @Test
        @DisplayName("捐贈碼非 3–7 碼數字")
        void malformedDonationCode() {
            assertBizCode(() -> resolve(req().setDonationCode("52")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
            assertBizCode(() -> resolve(req().setDonationCode("12345678")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
            assertBizCode(() -> resolve(req().setDonationCode("52A8")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }

        @Test
        @DisplayName("profileId 非正整數")
        void malformedProfileId() {
            assertBizCode(() -> resolve(req().setProfileId("abc")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
            assertBizCode(() -> resolve(req().setProfileId("-1")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }
    }

    @Nested
    @DisplayName("notify_email：漏了它，invoice_documents.recipient_email 恆空、通知寄不出去而不報錯")
    class NotifyEmail {

        @Test
        @DisplayName("合法信箱原樣帶下去（缺這一欄 ＝ §5.4.3 DDL 的漏列，本案補上）")
        void validEmailIsCarried() {
            assertThat(resolve(req().setNotifyEmail("buyer@example.com")).notifyEmail())
                    .isEqualTo("buyer@example.com");
        }

        @Test
        @DisplayName("留空 → null（由 invoice-service 依 D6 fallback 鏈解析）")
        void blankEmailBecomesNull() {
            assertThat(resolve(req().setNotifyEmail("   ")).notifyEmail()).isNull();
        }

        @Test
        @DisplayName("形狀不成立 → 9305（與 chk_ois_notify_email 同一組述詞）")
        void malformedEmail() {
            assertBizCode(() -> resolve(req().setNotifyEmail("not-an-email")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
            assertBizCode(() -> resolve(req().setNotifyEmail("a@b")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
            assertBizCode(() -> resolve(req().setNotifyEmail("a b@c.com")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }

        @Test
        @DisplayName("超過 254 字元 → 9305（notify_email VARCHAR(254)）")
        void emailTooLong() {
            String local = "a".repeat(250);
            assertBizCode(() -> resolve(req().setNotifyEmail(local + "@ex.com")),
                    GrpcResponseCode.INVOICE_CARRIER_INVALID);
        }

        @Test
        @DisplayName("信箱可以與任一目標並存，不算三選一衝突")
        void emailCoexistsWithAnyTarget() {
            assertThat(resolve(req().setDonationCode("5238").setNotifyEmail("a@b.co")).notifyEmail())
                    .isEqualTo("a@b.co");
        }
    }
}
