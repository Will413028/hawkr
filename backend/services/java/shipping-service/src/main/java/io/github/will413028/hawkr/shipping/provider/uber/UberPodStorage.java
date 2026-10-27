package io.github.will413028.hawkr.shipping.provider.uber;

/**
 * 把 Uber 的送達證明轉存到我方儲存（66 §7.8）。
 *
 * <h2>為什麼要轉存，而不是存 Uber 的連結</h2>
 * POD 照片 Uber 只留 <b>7 天</b>（U-F），而爭議窗口是 D+7 ＋ 人工裁決
 * ⇒ 🔴 66 §0.5 逐字「差額可能為負」⇒ 必須主動抓回自存。
 * 存連結的後果是：爭議發生時照片剛好在第 7 天過期，而我們在那天才發現。
 *
 * <p>🔴 轉存的結果<b>只存 objectKey</b>，禁止完整 URL／presigned URL 落庫（10 檔 §25）。
 */
public interface UberPodStorage {

    /**
     * 存一份 POD，回我方的 objectKey。
     *
     * @param deliveryId Uber 的配送識別，當 {@code ref_id}
     * @return {@code secure/} 前綴的 objectKey
     */
    String store(String deliveryId, byte[] content, String contentType);
}
