package io.github.will413028.hawkr.common.config;

import org.junit.jupiter.api.Test;

import static org.junit.jupiter.api.Assertions.assertEquals;

/**
 * MediaUrlProperties 單元測試。
 *
 * 重點：buildUrl(objectKey) 走 Image Gateway 原圖契約（base + "/i/original/" + objectKey，49 §6.1/§6.5）；
 * 另覆蓋 preset 變體（base + "/i/{preset}/" + objectKey）。
 */
class MediaUrlPropertiesTest {

    private MediaUrlProperties props(String baseUrl) {
        MediaUrlProperties p = new MediaUrlProperties();
        p.setBaseUrl(baseUrl);
        return p;
    }

    @Test
    void buildUrl_producesImageGatewayOriginalPath() {
        MediaUrlProperties p = props("https://test-media.example.com");
        assertEquals(
            "https://test-media.example.com/i/original/public/product/20260304/x.jpg",
            p.buildUrl("public/product/20260304/x.jpg"));
    }

    @Test
    void buildUrl_hasNoDefaultDomain() {
        // 無預設域名（backend#2283）：媒體域名不做新舊相容，dev 一律 test-media、prod 一律 media，
        // 一律由 MEDIA_BASE_URL 注入。舊實作預設回退一個已退役的 OSS 網域，DNS 一撤就靜默吐死連結。
        assertEquals("", new MediaUrlProperties().getBaseUrl());
    }

    @Test
    void buildUrl_emptyBaseReturnsBareObjectKey() {
        // 對齊 Go pkg/mediaurl.BuildImageURL：base 未設時回裸 objectKey，讓前端 buildImageUrl
        // 補上自己的 media base。**不可**回 "/i/original/..." 相對路徑，前端會再組一次而
        // 變成 /i/original/i/original/...。
        assertEquals("public/product/x.jpg", props("").buildUrl("public/product/x.jpg"));
        assertEquals("public/product/x.jpg", props("").buildUrl("/public/product/x.jpg"));
        assertEquals("public/product/x.jpg", props("").buildUrl("public/product/x.jpg", "thumb"));
        assertEquals("", props("").buildUrl(""));
        assertEquals(
            "https://lh3.google.com/a.jpg", props("").buildUrl("https://lh3.google.com/a.jpg"));
    }

    @Test
    void buildUrl_emptyReturnsEmpty() {
        assertEquals("", props("https://test-media.example.com").buildUrl(null));
        assertEquals("", props("https://test-media.example.com").buildUrl(""));
    }

    @Test
    void buildUrl_httpPassthrough() {
        assertEquals(
            "https://lh3.google.com/avatar.jpg",
            props("https://test-media.example.com").buildUrl("https://lh3.google.com/avatar.jpg"));
    }

    @Test
    void buildUrl_trimsTrailingSlashOnBase() {
        assertEquals(
            "https://test-media.example.com/i/original/a/b.jpg",
            props("https://test-media.example.com/").buildUrl("a/b.jpg"));
    }

    @Test
    void buildPresetUrl_producesImageGatewayPath() {
        MediaUrlProperties p = props("https://media.example.com");
        assertEquals(
            "https://media.example.com/i/thumb/public/product/x.jpg",
            p.buildUrl("public/product/x.jpg", "thumb"));
        assertEquals(
            "https://media.example.com/i/original/a/b.jpg",
            p.buildUrl("/a/b.jpg", "original"));
    }
}
