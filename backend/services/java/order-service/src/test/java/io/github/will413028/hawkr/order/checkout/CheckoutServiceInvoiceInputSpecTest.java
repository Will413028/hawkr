package io.github.will413028.hawkr.order.checkout;

import io.github.will413028.hawkr.common.utils.code.InvoiceCarrierType;
import io.github.will413028.hawkr.order.grpc.CheckoutInvoiceRequest;
import io.github.will413028.hawkr.invoice.grpc.InvoiceInputSpec;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import java.lang.reflect.Method;

import static org.assertj.core.api.Assertions.assertThat;

/**
 * {@code CheckoutService#toInvoiceInputSpec} 的跨服務契約。
 *
 * <p><b>這支測試存在的理由是一個 2026-09-01 實測抓到的缺口</b>：mapper 原本只在捐贈碼
 * 那一支設 {@code carrierType}，打統編那條路送出去的 {@code carrierType} 是空字串。
 * 而 invoice-service 的 {@code identityFromManualInput} 第一件事就是
 * {@code InvoiceCarrierTypeFromValue(spec.CarrierType)}，值域外一律回 9305
 * ——它的原地註解逐字寫著「統編必須明說 carrierType=TAX_ID」。
 *
 * <p>⇒ <b>打統編在 gateway 上結構性不可能成功</b>，而且完全靜默：order 側編譯過、
 * 單元測試全綠、CI 全綠、gateway 收得下這個欄位，只有真的打一次 dev 的 confirm 才會
 * 看到 9305。買家在結帳頁選「公司用發票」永遠拿到「格式不正確」。
 *
 * <p>🔴 <b>斷言錨在「carrierType 必須是 CodeMeta 的值」而不是「非空」</b>：
 * 非空的斷言擋不住有人填一個 invoice-service 不認得的字串，而那會落回同一個 9305。
 */
class CheckoutServiceInvoiceInputSpecTest {

    private static InvoiceInputSpec map(CheckoutInvoiceRequest in) throws Exception {
        Method m = CheckoutService.class.getDeclaredMethod("toInvoiceInputSpec", CheckoutInvoiceRequest.class);
        m.setAccessible(true);
        return (InvoiceInputSpec) m.invoke(null, in);
    }

    @Test
    @DisplayName("打統編必須明說 carrierType=TAX_ID，否則 invoice-service 回 9305")
    void taxIdSetsCarrierType() throws Exception {
        InvoiceInputSpec spec = map(CheckoutInvoiceRequest.newBuilder()
                .setBuyerTaxId("77777177")
                .setBuyerTitle("範例測試股份有限公司")
                .build());

        assertThat(spec.getCarrierType()).isEqualTo(InvoiceCarrierType.TAX_ID.value());
        assertThat(spec.getBuyerTaxId()).isEqualTo("77777177");
        assertThat(spec.getBuyerName()).isEqualTo("範例測試股份有限公司");
        // TAX_ID 的 value 必須留空（§6.1.1）：帶了值會被 validateIdentityShape 判成
        // 「前端把載具塞進了統編那一格」。
        assertThat(spec.getValue()).isEmpty();
    }

    @Test
    @DisplayName("捐贈碼走 DONATION ＋ value，統編那一格不動")
    void donationSetsCarrierTypeAndValue() throws Exception {
        InvoiceInputSpec spec = map(CheckoutInvoiceRequest.newBuilder()
                .setDonationCode("25885")
                .build());

        assertThat(spec.getCarrierType()).isEqualTo(InvoiceCarrierType.DONATION.value());
        assertThat(spec.getValue()).isEqualTo("25885");
        assertThat(spec.getBuyerTaxId()).isEmpty();
    }

    @Test
    @DisplayName("捐贈碼優先：兩者都帶時不設成 TAX_ID——三選一違規交給 A15 回 9306，不在 mapper 猜")
    void donationWinsWhenBothPresent() throws Exception {
        InvoiceInputSpec spec = map(CheckoutInvoiceRequest.newBuilder()
                .setDonationCode("25885")
                .setBuyerTaxId("77777177")
                .build());

        assertThat(spec.getCarrierType()).isEqualTo(InvoiceCarrierType.DONATION.value());
    }

    @Test
    @DisplayName("只給 profileId 時不設 carrierType——那條路走 identityFromPreference，不走手動輸入")
    void profileIdLeavesCarrierTypeEmpty() throws Exception {
        InvoiceInputSpec spec = map(CheckoutInvoiceRequest.newBuilder()
                .setProfileId("7499933495532191744")
                .build());

        assertThat(spec.getCarrierType()).isEmpty();
        assertThat(spec.getProfileId()).isEqualTo("7499933495532191744");
    }

    @Test
    @DisplayName("整包空的 invoice 不設 carrierType——那是 USE_BUYER_DEFAULT，已上架 App 的常態")
    void emptyRequestLeavesCarrierTypeEmpty() throws Exception {
        InvoiceInputSpec spec = map(CheckoutInvoiceRequest.newBuilder().build());

        assertThat(spec.getCarrierType()).isEmpty();
        assertThat(spec.getValue()).isEmpty();
    }
}
