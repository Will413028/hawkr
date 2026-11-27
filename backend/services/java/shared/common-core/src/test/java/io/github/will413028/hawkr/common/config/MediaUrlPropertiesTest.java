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
        assertEquals("", new MediaUrlProperties().getBaseUrl());
    }

    @Test
    void buildUrl_emptyBaseReturnsBareObjectKey() {
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
