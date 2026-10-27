/**
 * Preset 白名單（全平台圖片轉換規格唯一定義點）。
 *
 * 對應 49 號設計書 §4.4。新增/調整 preset：改本檔一處＋設計書表格同步＋部署 Worker，
 * 前端不需改版（前端只准傳 preset 名，不可自訂 width/height/quality/format）。
 *
 * 新增或刪除 preset 名稱時，跑 `node backend/workers/image-gateway/scripts/gen-client-presets.mjs`
 * 更新各 client 的 preset 常數（ADR 2026-09-25-api-image-uri-template D4；CI 檢查 drift）。上線後
 * preset 只增不刪：舊版 App 還會送舊的名稱。
 */

/**
 * Cloudflare Images binding `.transform()` 接受的參數子集（本專案實際用到的欄位）。
 * 型別對齊 `wrangler types` 生成的 `ImageTransform`（`fit` 聯合型別含
 * scale-down/contain/pad/squeeze/cover/crop，本專案只用 cover 與 scale-down）。
 */
export interface ImageTransformOptions {
  width?: number;
  height?: number;
  fit?: "scale-down" | "cover";
}

export interface ImagePreset {
  transform: ImageTransformOptions;
  /**
   * 保留來源的動畫幀（animated GIF/WebP）。
   *
   * 影響兩件事：① 格式協商只在**能動**的格式之間選（webp → gif），絕不落到 jpeg/avif
   * ——靜態格式會把動圖壓成單張，那正是這個 preset 要避免的；② 顯式傳 `anim: true`
   * 給 Images binding 的 `.output()`。runtime 型別註明 anim 預設為 true，但這裡不靠
   * 預設值：這個 preset 存在的唯一理由就是動畫，預設值改變就等於功能靜默消失。
   *
   * 背景：貼文卡片的動圖原本走 `original`（不轉換、直出原檔），實測單檔 10,233,546 B
   * ——同一顆物件的 `card` 是 32,815 B。首頁一次載 100+ 張圖共用同一個 media host，
   * 那一顆把頁面 settle 從 ~9 秒拖到 ~28 秒（hawkr-website#528）。
   */
  animated?: true;
  /**
   * 49 號設計書假設 preset 可直接宣告 `format=auto`，但實測（`wrangler types` 生成的
   * `ImageOutputOptions.format`）只接受具體 MIME（image/jpeg｜png｜gif｜webp｜avif｜
   * rgb｜rgba），無 "auto" 字面值。故 preset 只定義 quality，實際 format 由 index.ts
   * 依請求 Accept header 現場協商（見 negotiateFormat）——效果等價於 cf.image 的
   * format=auto，只是協商邏輯從平台內建搬到本檔明寫。
   */
  quality: number;
}

export const PRESETS = {
  avatar: {
    transform: { width: 160, height: 160, fit: "cover" },
    quality: 60,
  },
  thumb: {
    transform: { width: 240, height: 240, fit: "cover" },
    quality: 70,
  },
  card: {
    transform: { width: 720, height: 540, fit: "cover" },
    quality: 75,
  },
  detail: {
    transform: { width: 1280, fit: "scale-down" },
    quality: 80,
  },
  banner: {
    transform: { width: 1600, fit: "scale-down" },
    quality: 85,
  },
  chat: {
    transform: { width: 720, fit: "scale-down" },
    quality: 75,
  },
  /**
   * 動圖專用：有界尺寸 + 保留動畫。貼文卡片的 GIF 走這裡，而不是 `original`。
   *
   * 用 scale-down 而非 card 的 cover 裁切：動圖的構圖是作者逐幀調過的
   * （賣家端有「拖曳 GIF 調整每一幀取景」），裁掉等於推翻他的取景。
   */
  motion: {
    transform: { width: 720, fit: "scale-down" },
    quality: 70,
    animated: true,
  },
} as const satisfies Record<string, ImagePreset>;

/** `original` 不經 Images binding 轉換，故不在 PRESETS（見 index.ts 的直出分支）。 */
export type TransformPresetName = keyof typeof PRESETS;
export const ORIGINAL_PRESET_NAME = "original" as const;

/** Worker 路由段可接受的全部 preset 名（含 original），供白名單判核用。 */
export type PresetName = TransformPresetName | typeof ORIGINAL_PRESET_NAME;

export function isKnownPreset(preset: string): preset is PresetName {
  return preset === ORIGINAL_PRESET_NAME || Object.prototype.hasOwnProperty.call(PRESETS, preset);
}

export function isTransformPreset(preset: string): preset is TransformPresetName {
  return Object.prototype.hasOwnProperty.call(PRESETS, preset);
}

/**
 * 這個 preset 要不要保留動畫。
 *
 * 需要一個存取器而不是直接讀 `PRESETS[preset].animated`：`as const satisfies` 保留了
 * 每個 preset 的字面型別，沒宣告 `animated` 的那幾個在型別上**不存在**這個屬性，
 * 直接讀會編譯錯。從 ImagePreset 這個介面讀就一致了。
 */
export function isAnimatedPreset(preset: TransformPresetName): boolean {
  return (PRESETS[preset] as ImagePreset).animated === true;
}
