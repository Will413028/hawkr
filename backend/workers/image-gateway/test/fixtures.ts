/**
 * 18x18 的合法 PNG（1490 bytes，取自 Cloudflare workers-sdk 官方
 * vitest-pool-workers-examples/images fixture 的既有測試素材），供 Images binding 真轉換
 * 測試用。刻意不用 1x1 極小圖：本機 Miniflare Images 模擬器對 1x1 圖做 fit=cover 放大到
 * 240x240 等 preset 轉換時會靜默回傳 200 + 0 bytes（見 src/index.ts 的空輸出防呆註解），
 * 18x18 圖可穩定產出非空轉換結果，才驗得到真正的轉換/格式協商行為。
 */
function base64ToBytes(base64: string): Uint8Array {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

const SAMPLE_PNG_BASE64 =
  "iVBORw0KGgoAAAANSUhEUgAAABIAAAASCAYAAABWzo5XAAAABGdBTUEAALGPC/xhBQAAACBjSFJNAAB6JgAAgIQAAPoAAACA6AAAdTAAAOpgAAA6mAAAF3CculE8AAAAeGVYSWZNTQAqAAAACAAEARIAAwAAAAEAAQAAARoABQAAAAEAAAA+ARsABQAAAAEAAABGh2kABAAAAAEAAABOAAAAAAAAAEgAAAABAAAASAAAAAEAA6ABAAMAAAABAAEAAKACAAQAAAABAAAAEqADAAQAAAABAAAAEgAAAAB1NxziAAAACXBIWXMAAAsTAAALEwEAmpwYAAACkmlUWHRYTUw6Y29tLmFkb2JlLnhtcAAAAAAAPHg6eG1wbWV0YSB4bWxuczp4PSJhZG9iZTpuczptZXRhLyIgeDp4bXB0az0iWE1QIENvcmUgNi4wLjAiPgogICA8cmRmOlJERiB4bWxuczpyZGY9Imh0dHA6Ly93d3cudzMub3JnLzE5OTkvMDIvMjItcmRmLXN5bnRheC1ucyMiPgogICAgICA8cmRmOkRlc2NyaXB0aW9uIHJkZjphYm91dD0iIgogICAgICAgICAgICB4bWxuczp0aWZmPSJodHRwOi8vbnMuYWRvYmUuY29tL3RpZmYvMS4wLyIKICAgICAgICAgICAgeG1sbnM6ZXhpZj0iaHR0cDovL25zLmFkb2JlLmNvbS9leGlmLzEuMC8iPgogICAgICAgICA8dGlmZjpZUmVzb2x1dGlvbj43MjwvdGlmZjpZUmVzb2x1dGlvbj4KICAgICAgICAgPHRpZmY6WFJlc29sdXRpb24+NzI8L3RpZmY6WFJlc29sdXRpb24+CiAgICAgICAgIDx0aWZmOk9yaWVudGF0aW9uPjE8L3RpZmY6T3JpZW50YXRpb24+CiAgICAgICAgIDxleGlmOlBpeGVsWERpbWVuc2lvbj42NDwvZXhpZjpQaXhlbFhEaW1lbnNpb24+CiAgICAgICAgIDxleGlmOkNvbG9yU3BhY2U+MTwvZXhpZjpDb2xvclNwYWNlPgogICAgICAgICA8ZXhpZjpQaXhlbFlEaW1lbnNpb24+NjQ8L2V4aWY6UGl4ZWxZRGltZW5zaW9uPgogICAgICA8L3JkZjpEZXNjcmlwdGlvbj4KICAgPC9yZGY6UkRGPgo8L3g6eG1wbWV0YT4KaMKrAwAAAiZJREFUOBHdU0toE1EUPe8zmUmsxS5U0EWNpUEEQehOwU9dSamI2OBGEFwVBBGxO+2odeVCBfeCy7Qr10qzcePCjSCCGBSiYC1a25hkJvPe87z0gxFExJVecrlvcs879557Z4B/ytwElKvQHUT3HEP/sQBP8KtLnvjnXE+FOIac3kvQSwhRRpbeGBpx0lzIDEpSYklBzuWu1h4IAXIRAzCs2gazI4mIYdcT6UzxLJEPtRKw1jHJTCDRamZzhWvvJlylomZRRrkM4+90idZJVuKhbby2X2rbgBVPJBB1LFLiKNM3AdPXnw+TL68uRTdx1xN0R1Am2g9RzMI0Z0o7hUufRoEc/JbKjNe8bHYoyLdqQuZMYD8oMzD+On9sdDJ5ce5tdAZvfFajQjD70qIzFkR6cGW5nQhbD6ned7BB4sFOQrUX4PJHx4axWH2s5GZjHo3cUSeqV4SLd0WJFLdZ+rSx2XbIfoi+YTaiBQQVdZ18QsNlLeg9xxEeOunMs3vWNWsy6G+I7PPClE6UuB8W5PlGgx3YZYOoJGRxlFMJ6VSnAopj9DvKRQgPjvOZNXYcABq7O+bjfE66+qTIbhWd6q7EL5SVXRtI3nslPK+5P3uRS/SLzxnngdplINzHTQ7AtHVHp5k9pZTyvROV0Qv8sVqP+XeTyytsgVqsQdWngE3cu+F/SYtvfvq1B/43Dxz2776fI2v8VUaeD08zXqf/YJ+2+iH8r/YdiNHHTTMbtjAAAAAASUVORK5CYII=";

export function samplePngBytes(): Uint8Array {
  return base64ToBytes(SAMPLE_PNG_BASE64);
}

export function corruptJpegBytes(): Uint8Array {
  return new TextEncoder().encode("this is not a real jpeg, transform must degrade to original");
}

export function pdfBytes(): Uint8Array {
  return new TextEncoder().encode("%PDF-1.4 fake pdf body for content-type passthrough test");
}

/**
 * 18x18、3 幀的合法 animated GIF（307 bytes，PIL 產生）。
 *
 * 尺寸刻意與 SAMPLE_PNG 一致：本機 Miniflare Images 模擬器對極小圖轉換會靜默回
 * 200 + 0 bytes（見 src/index.ts 的空輸出防呆），18x18 才穩定產得出非空結果。
 * 多幀是重點——`motion` preset 的整個存在理由就是輸出還會動，單幀 GIF 驗不到那件事。
 */
const ANIMATED_GIF_BASE64 =
  "R0lGODlhEgASAIEAAP8AAAAA/wAAAAAAACH/C05FVFNDQVBFMi4wAwEAAAAh+QQADAAAACwAAAAAEgASAAAIMQADCBwYAIDBgwgBEByYsOFCgQ0TPiwY8eDEihYfYjR4caPHjyBDihxJsqTJkyhTBgQAIfkEAQwAAgAsAAAAABIAEgCBAAD/AP8AAAAAAAAACEEABQgcKACAwYMIARAcmLDhQoENEz4sGPEgwQAYMwaIeFEjRo4DPX6saFDkRpIATKJMKXKlypUwY8qcSbOmTZoBAQAh+QQBDAACACwAAAAAEgASAIH/AAAA/wAAAAAAAAAIPQADCBxIsKDBgwgTKlyoUIDDhwIWQnwocWLEhhYrOgTAsSOAgxA9dgT5UCTHhSY/KkyJ0mRLkQxjypw5MCAAOw==";

export function animatedGifBytes(): Uint8Array {
  return base64ToBytes(ANIMATED_GIF_BASE64);
}
