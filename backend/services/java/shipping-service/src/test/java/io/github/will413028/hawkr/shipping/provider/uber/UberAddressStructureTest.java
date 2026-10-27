package io.github.will413028.hawkr.shipping.provider.uber;

import com.fasterxml.jackson.databind.ObjectMapper;
import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Nested;
import org.junit.jupiter.api.Test;

import static org.assertj.core.api.Assertions.assertThat;
import static org.assertj.core.api.Assertions.assertThatThrownBy;

/**
 * 收件地址結構化（66 §6.5）。
 *
 * <h2>🔴 這份測試守的是「切點」與「fail-closed」，不是「欄名對不對」</h2>
 * 欄名（street／city／state／country）是<b>未查證的外部契約</b>，
 * 用測試把它釘死等於把一個猜測升級成保證。⇒ 這裡只斷言
 * 「值有沒有落到我方決定的那一格」，欄名本身由 {@code KEY_*} 常數集中管理。
 */
@DisplayName("Uber 收件地址結構化")
class UberAddressStructureTest {

    private final ObjectMapper objectMapper = new ObjectMapper();

    @Nested
    @DisplayName("樓層切點")
    class FloorSplit {

        @Test
        @DisplayName("🔴 樓層進 notes 不進 street（塞進 street 會讓地理編碼失敗）")
        void floorGoesToNotes() {
            var s = UberAddressStructure.of(objectMapper, "台北市", "大安區", "忠孝東路四段1號5樓");
            assertThat(s.addressJson()).contains("忠孝東路四段1號").doesNotContain("5樓");
            assertThat(s.notes()).isEqualTo("5樓");
        }

        @Test
        @DisplayName("🔴「之」不能當錨：「1之2號」的「之」屬於門牌")
        void hyphenIsNotTheAnchor() {
            var s = UberAddressStructure.of(objectMapper, "台北市", "大安區", "忠孝東路1之2號5樓之3");
            // 用「之」當錨會切成「忠孝東路1」，把門牌截斷。
            assertThat(s.addressJson()).contains("忠孝東路1之2號");
            assertThat(s.notes()).isEqualTo("5樓之3");
        }

        @Test
        @DisplayName("🔴 錨是**第一個**「號」——樓層自己也可以含「號」")
        void anchorIsFirstNumberSign() {
            // 🔴 這一支是為了讓 indexOf/lastIndexOf 的變異有鑑別力而存在。
            //    上面那支只有一個「號」⇒ 兩種寫法結果相同 ⇒ 對這個變異完全看不見
            //    （2026-09-12 實測：把 lastIndexOf 改成 indexOf，全部 11 支照樣綠）。
            //    真實情境：樓層含「號」的地址（「B1號櫃」「1樓之2號房」）。
            var s = UberAddressStructure.of(objectMapper, "台北市", "信義區", "松高路1號B1號櫃");
            assertThat(s.addressJson())
                    .as("門牌止於第一個「號」")
                    .contains("松高路1號")
                    .doesNotContain("B1號櫃");
            assertThat(s.notes())
                    .as("「號」之後全部進 notes，含樓層自己的那個「號」")
                    .isEqualTo("B1號櫃");
        }

        @Test
        @DisplayName("沒有樓層 ⇒ notes 為 null，street 完整保留")
        void noFloor() {
            var s = UberAddressStructure.of(objectMapper, "新竹縣", "竹北市", "光明六路1號");
            assertThat(s.notes()).isNull();
            assertThat(s.addressJson()).contains("光明六路1號");
        }

        @Test
        @DisplayName("沒有「號」⇒ 不猜切點，整串當門牌")
        void noNumberSignDoesNotGuess() {
            var s = UberAddressStructure.of(objectMapper, "台北市", "大安區", "台大校總區");
            assertThat(s.notes()).isNull();
            assertThat(s.addressJson()).contains("台大校總區");
        }
    }

    @Nested
    @DisplayName("🔴 detail 的前置正規化（沿用 AddressNormalizer，不自己再寫一份）")
    class DetailNormalization {

        @Test
        @DisplayName("detail 重複帶了縣市＋區前綴 ⇒ strip 掉，不讓結構化地址出現兩次城市名")
        void duplicatedPrefixIsStripped() {
            // 真實情境：前端把完整地址塞進 detail 欄。不 strip 的話 state=台北市、
            // city=大安區，而 street 又是「台北市大安區忠孝東路一段1號」⇒ 地理編碼失敗。
            var s = UberAddressStructure.of(objectMapper, "台北市", "大安區", "台北市大安區忠孝東路一段1號5樓");
            assertThat(s.addressJson())
                    .as("street 不得再帶一次縣市與區")
                    .doesNotContain("台北市大安區忠孝東路")
                    .contains("忠孝東路一段1號");
            assertThat(s.notes()).isEqualTo("5樓");
        }

        @Test
        @DisplayName("里／鄰被 strip（PayUni 明文不接受，Uber 沿用同一個保守處置）")
        void villageAndNeighborhoodStripped() {
            var s = UberAddressStructure.of(objectMapper, "台中市", "西屯區", "何安里5鄰台灣大道三段99號");
            assertThat(s.addressJson())
                    .doesNotContain("鄰")
                    .contains("台灣大道三段99號");
        }

        @Test
        @DisplayName("正規化後變成空字串 ⇒ fail-closed，不送空門牌給 Uber")
        void blankAfterNormalizationFailsClosed() {
            assertThatThrownBy(() ->
                    UberAddressStructure.of(objectMapper, "台北市", "大安區", "台北市大安區"))
                    .isInstanceOf(UberAddressUnparseableException.class);
        }
    }

    @Nested
    @DisplayName("行政層級")
    class AdministrativeLevels {

        @Test
        @DisplayName("縣市落 state、鄉鎮市區落 city（台灣兩級對國際欄位的定案）")
        void levelsMapping() {
            var s = UberAddressStructure.of(objectMapper, "新竹縣", "竹北市", "光明六路1號");
            assertThat(s.addressJson())
                    .contains("\"" + UberAddressStructure.KEY_STATE + "\":\"新竹縣\"")
                    .contains("\"" + UberAddressStructure.KEY_CITY + "\":\"竹北市\"");
        }

        @Test
        @DisplayName("🔴「臺」與「台」是同一個行政區，兩種寫法都收")
        void taiVariantAccepted() {
            var s = UberAddressStructure.of(objectMapper, "臺中市", "西屯區", "台灣大道三段99號");
            assertThat(s.addressJson()).contains("台中市");
        }

        @Test
        @DisplayName("country 固定 TW")
        void countryIsTw() {
            var s = UberAddressStructure.of(objectMapper, "高雄市", "苓雅區", "四維三路2號");
            assertThat(s.addressJson()).contains("\"" + UberAddressStructure.KEY_COUNTRY + "\":\"TW\"");
        }

        @Test
        @DisplayName("🔴 zip 不送：沒有權威對照表，送一個我編的號碼比不送更糟")
        void zipIsNotEmitted() {
            var s = UberAddressStructure.of(objectMapper, "台北市", "大安區", "忠孝東路四段1號");
            assertThat(s.addressJson()).doesNotContain("zip");
        }
    }

    @Nested
    @DisplayName("🔴 fail-closed（都是 ADDRESS_UNPARSEABLE，不是靜默帶壞值去打 Uber）")
    class FailClosed {

        @Test
        @DisplayName("縣市不在 22 個第一級行政區內 ⇒ 拒絕")
        void unknownCountyRejected() {
            assertThatThrownBy(() -> UberAddressStructure.of(objectMapper, "東京都", "澀谷區", "1-1-1"))
                    .isInstanceOf(UberAddressUnparseableException.class)
                    .hasMessageContaining("東京都");
        }

        @Test
        @DisplayName("🔴「大安區」放進縣市欄 ⇒ 拒絕（欄位語意被寫反時必須擋下來）")
        void districtInCountyFieldRejected() {
            // 這是最可能的真實錯誤：上游把兩欄對調。它不會有任何型別錯誤。
            assertThatThrownBy(() -> UberAddressStructure.of(objectMapper, "大安區", "台北市", "忠孝東路1號"))
                    .isInstanceOf(UberAddressUnparseableException.class);
        }

        @Test
        @DisplayName("任一欄為空 ⇒ 拒絕")
        void blankFieldsRejected() {
            assertThatThrownBy(() -> UberAddressStructure.of(objectMapper, null, "大安區", "忠孝東路1號"))
                    .isInstanceOf(UberAddressUnparseableException.class);
            assertThatThrownBy(() -> UberAddressStructure.of(objectMapper, "台北市", "  ", "忠孝東路1號"))
                    .isInstanceOf(UberAddressUnparseableException.class);
            assertThatThrownBy(() -> UberAddressStructure.of(objectMapper, "台北市", "大安區", ""))
                    .isInstanceOf(UberAddressUnparseableException.class);
        }
    }
}
