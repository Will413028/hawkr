package io.github.will413028.hawkr.order.util;

import org.junit.jupiter.api.DisplayName;
import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

/** 56 §8.1.5 的訂單 PII 遮罩規則。 */
class MaskUtilsTest {

    @Test
    @DisplayName("手機保留前 4 後 3，中間固定三個 *")
    void maskPhone_keepsPrefixAndSuffix() {
        assertEquals("0912-***-234", MaskUtils.maskPhone("0912341234"));
        assertEquals("0287-***-678", MaskUtils.maskPhone("0287712345678"));
    }

    @Test
    @DisplayName("不足 8 碼全遮，否則遮罩後反而露出大部分")
    void maskPhone_shortNumberFullyMasked() {
        assertEquals("***", MaskUtils.maskPhone("1234567"));
        assertEquals("***", MaskUtils.maskPhone("12"));
    }

    @Test
    @DisplayName("null / 空字串原樣回傳")
    void maskPhone_nullSafe() {
        assertNull(MaskUtils.maskPhone(null));
        assertEquals("", MaskUtils.maskPhone(""));
    }

    @Test
    @DisplayName("直轄市地址止於區，不吐路名門牌樓層")
    void maskAddress_municipality() {
        assertEquals("台北市大安區",
                MaskUtils.maskAddressToDistrict("台北市大安區信義路四段 123 號 5 樓之 3"));
        assertEquals("高雄市三民區", MaskUtils.maskAddressToDistrict("高雄市三民區建工路 1 號"));
    }

    @Test
    @DisplayName("縣轄市無區鄉鎮字元，止於市——此時「市」正是它的 district")
    void maskAddress_countyAdministeredCity() {
        assertEquals("新竹縣竹北市", MaskUtils.maskAddressToDistrict("新竹縣竹北市光明六路 100 號"));
    }

    @Test
    @DisplayName("鄉／鎮同樣是行政區級終點")
    void maskAddress_townshipMarkers() {
        assertEquals("南投縣草屯鎮", MaskUtils.maskAddressToDistrict("南投縣草屯鎮中正路 9 號"));
        assertEquals("宜蘭縣冬山鄉", MaskUtils.maskAddressToDistrict("宜蘭縣冬山鄉照安路 88 號"));
    }

    @Test
    @DisplayName("找不到行政區字元回前 6 個字元（錯誤方向是遮太多）")
    void maskAddress_fallback() {
        assertEquals("信義路四段 ", MaskUtils.maskAddressToDistrict("信義路四段 123 號 5 樓"));
        assertEquals("短地址", MaskUtils.maskAddressToDistrict("短地址"));
    }

    @Test
    @DisplayName("null / 空字串原樣回傳")
    void maskAddress_nullSafe() {
        assertNull(MaskUtils.maskAddressToDistrict(null));
        assertEquals("", MaskUtils.maskAddressToDistrict(""));
    }
}
