package io.github.will413028.hawkr.shipping.util;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;
import static org.junit.jupiter.api.Assertions.assertNull;

class AddressNormalizerTest {


    @Test
    void normalizeAddressDetail_stripsCityDistrictPrefix() {
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
        assertEquals("detail", AddressNormalizer.normalizeAddressDetail(null, "泰山區", "detail"));
        assertEquals("中山路1號", AddressNormalizer.normalizeAddressDetail("新北市", null, "貴和里5鄰中山路1號"));
    }


    @Test
    void normalizeAddressDetail_stripsVillageAndNeighborhood() {
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
        String result = AddressNormalizer.normalizeAddressDetail(
                "桃園市", "桃園區", "桃園市桃園區三民里12鄰鎮撫街1號");
        assertEquals("鎮撫街1號", result);
    }


    @Test
    void normalizeFullAddress_stripsVillageNeighborhoodAfterDistrict() {
        String result = AddressNormalizer.normalizeFullAddress("桃園市桃園區三民里12鄰鎮撫街1號");
        assertEquals("桃園市桃園區鎮撫街1號", result);
    }

    @Test
    void normalizeFullAddress_preservesDistrictContainingLiCharacter() {
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
