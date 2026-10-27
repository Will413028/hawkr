package io.github.will413028.hawkr.shipping.util;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

class AddressNormalizerTest {

    // ---- normalizeAddressDetail：city+district 前綴 strip ----

    @Test
    void normalizeAddressDetail_stripsCityDistrictPrefix() {
        // 前端誤塞全址 → strip 重複前綴 + strip 里
        String result = AddressNormalizer.normalizeAddressDetail(
                "新北市", "泰山區", "新北市泰山區貴和里明志路三段341巷5弄1號");
        assertEquals("明志路三段341巷5弄1號", result);
    }

    @Test
    void normalizeAddressDetail_preservesDetailWithoutPrefix() {
        String result = AddressNormalizer.normalizeAddressDetail(
                "新北市", "泰山區", "貴和里明志路三段341巷5弄1號");
        assertEquals("明志路三段341巷5弄1號", result);
    }

    @Test
    void normalizeAddressDetail_trimsWhitespace() {
        String result = AddressNormalizer.normalizeAddressDetail(
                "新北市", "泰山區", "  新北市泰山區貴和里明志路三段341巷5弄1號  ");
        assertEquals("明志路三段341巷5弄1號", result);
    }

    @Test
    void normalizeAddressDetail_detailEqualsPrefix_returnsEmpty() {
        String result = AddressNormalizer.normalizeAddressDetail(
                "新北市", "泰山區", "新北市泰山區");
        assertEquals("", result);
    }

    @Test
    void normalizeAddressDetail_conflictingDistrictPrefix_conservativelyUnchanged() {
        // 矛盾資料：detail 內行政區「新莊區」與 district 參數「泰山區」不符。
        // 保守：前綴不符不 strip（避免誤刪 city）；里鄰不在開頭（藏在重複區之後）亦不動，
        // 避免對畸形資料誤刪路名。正常成員點 detail 的里鄰都在開頭，能正確 strip。
        String result = AddressNormalizer.normalizeAddressDetail(
                "新北市", "泰山區", "新北市新莊區貴和里中山路1號");
        assertEquals("新北市新莊區貴和里中山路1號", result);
    }

    @Test
    void normalizeAddressDetail_nullDetail_returnsNull() {
        assertNull(AddressNormalizer.normalizeAddressDetail("新北市", "泰山區", null));
    }

    @Test
    void normalizeAddressDetail_blankDetail_returnsBlank() {
        assertEquals("", AddressNormalizer.normalizeAddressDetail("新北市", "泰山區", ""));
    }

    @Test
    void normalizeAddressDetail_nullCityOrDistrict_stillStripsVillageNeighborhood() {
        // city/district 為 null 時跳過前綴 strip，但里鄰仍要 strip
        assertEquals("detail", AddressNormalizer.normalizeAddressDetail(null, "泰山區", "detail"));
        assertEquals("中山路1號", AddressNormalizer.normalizeAddressDetail("新北市", null, "貴和里5鄰中山路1號"));
    }

    // ---- normalizeAddressDetail：里/村、鄰 strip（本次 DEF01003 修復核心）----

    @Test
    void normalizeAddressDetail_stripsVillageAndNeighborhood() {
        // prod DEF01003 觸發案例
        String result = AddressNormalizer.normalizeAddressDetail(
                "桃園市", "桃園區", "三民里12鄰鎮撫街1號");
        assertEquals("鎮撫街1號", result);
    }

    @Test
    void normalizeAddressDetail_stripsNeighborhoodOnly() {
        String result = AddressNormalizer.normalizeAddressDetail(
                "桃園市", "桃園區", "12鄰鎮撫街1號");
        assertEquals("鎮撫街1號", result);
    }

    @Test
    void normalizeAddressDetail_stripsVillageOnly() {
        String result = AddressNormalizer.normalizeAddressDetail(
                "桃園市", "桃園區", "三民里鎮撫街1號");
        assertEquals("鎮撫街1號", result);
    }

    @Test
    void normalizeAddressDetail_stripsVillageOfTypeCun() {
        // 郊區「村」
        String result = AddressNormalizer.normalizeAddressDetail(
                "南投縣", "魚池鄉", "大雁村5鄰中山路1號");
        assertEquals("中山路1號", result);
    }

    @Test
    void normalizeAddressDetail_noVillageNoNeighborhood_unchanged() {
        String result = AddressNormalizer.normalizeAddressDetail(
                "新北市", "泰山區", "明志路三段341巷5弄1號");
        assertEquals("明志路三段341巷5弄1號", result);
    }

    @Test
    void normalizeAddressDetail_prefixPlusVillageNeighborhood() {
        // 前端誤塞全址 + 里鄰：前綴 + 里 + 鄰 全部去除
        String result = AddressNormalizer.normalizeAddressDetail(
                "桃園市", "桃園區", "桃園市桃園區三民里12鄰鎮撫街1號");
        assertEquals("鎮撫街1號", result);
    }

    // ---- normalizeFullAddress：買家收件地址（單一合併字串）----

    @Test
    void normalizeFullAddress_stripsVillageNeighborhoodAfterDistrict() {
        String result = AddressNormalizer.normalizeFullAddress("桃園市桃園區三民里12鄰鎮撫街1號");
        assertEquals("桃園市桃園區鎮撫街1號", result);
    }

    @Test
    void normalizeFullAddress_preservesDistrictContainingLiCharacter() {
        // 關鍵回歸：台中市大里區的「里」屬行政區，不可誤刪；里鄰只 strip 區之後
        String result = AddressNormalizer.normalizeFullAddress("臺中市大里區內新里5鄰大明路1號");
        assertEquals("臺中市大里區大明路1號", result);
    }

    @Test
    void normalizeFullAddress_withLeadingZipCode() {
        String result = AddressNormalizer.normalizeFullAddress("330桃園市桃園區三民里12鄰鎮撫街1號");
        assertEquals("330桃園市桃園區鎮撫街1號", result);
    }

    @Test
    void normalizeFullAddress_noVillageNeighborhood_unchanged() {
        String result = AddressNormalizer.normalizeFullAddress("臺北市大安區忠孝東路四段1號");
        assertEquals("臺北市大安區忠孝東路四段1號", result);
    }

    @Test
    void normalizeFullAddress_noCityDistrictPrefix_removesNeighborhoodConservatively() {
        // 缺縣市的畸形輸入：保守只移除「鄰」，里保留（避免誤刪路名）
        String result = AddressNormalizer.normalizeFullAddress("三民里12鄰鎮撫街1號");
        assertEquals("三民里鎮撫街1號", result);
    }

    @Test
    void normalizeFullAddress_nullOrBlank_returnsSame() {
        assertNull(AddressNormalizer.normalizeFullAddress(null));
        assertEquals("", AddressNormalizer.normalizeFullAddress(""));
        assertEquals("   ", AddressNormalizer.normalizeFullAddress("   "));
    }
}
