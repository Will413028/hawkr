#!/usr/bin/env node
/**
 * 對**已部署**的 Image Gateway 實測 `motion` preset。
 *
 * 為什麼需要它：動畫保不保留取決於 Cloudflare 的編碼器，本機 Miniflare 模擬器做不到
 * （`image/gif` 回 `GIF output is not supported in local mode`；`image/webp` 靜默吐單幀），
 * 所以單元測試只守得住我方那一側的前提（格式、anim 旗標）。「輸出真的會動而且真的變小」
 * 只有對真 binding 打一次才知道。
 *
 * 另一個必須實測的理由：Worker 的 degrade 語意是「轉換失敗就直出原檔」（49 號 §7.3）。
 * 對 motion 來說那等於**退回原本那顆 10 MB**，而 HTTP 仍然是 200 —— 只看狀態碼會把
 * 完全沒生效的情況當成成功。
 *
 * 用法：
 *   node scripts/verify-motion-preset.mjs <mediaBaseUrl> <objectKey>
 * 例：
 *   node scripts/verify-motion-preset.mjs https://test-media.example.com \
 *     public/post/2636840881/20260826/3df09cc3-e312-4f18-b099-2ad4d5845161.gif
 */

const [, , baseUrl, objectKey] = process.argv;
if (!baseUrl || !objectKey) {
  console.error("用法：node scripts/verify-motion-preset.mjs <mediaBaseUrl> <objectKey>");
  process.exit(2);
}

/**
 * 這顆輸出還會動嗎？**結構化解析，不做位元組樣式掃描。**
 *
 * ⚠️ 第一版是掃 `21 F9`（GIF 的 Graphic Control Extension 前綴）出現幾次。對同一顆
 * 真實貼文動圖，它數出 190，而 PIL 解碼出來是 **73**——壓縮後的影像資料裡碰巧含有
 * 那組位元組。方向錯得很危險：它會**高估**，而高估意味著一顆單幀 GIF 也可能被判成
 * 動圖通過，那正是這支腳本唯一要防的事。
 *
 * 改成走容器結構：GIF 逐區塊走訪、只數 Image Descriptor（0x2C）；WebP 走 RIFF
 * chunk 表、只認真正的 ANMF chunk。
 */
function inspectGif(bytes) {
  // Header(6) + Logical Screen Descriptor(7)
  if (bytes.length < 13) return null;
  let p = 10;
  const packed = bytes[p];
  p = 13;
  if (packed & 0x80) {
    p += 3 * (1 << ((packed & 0x07) + 1)); // Global Color Table
  }
  const skipSubBlocks = () => {
    while (p < bytes.length) {
      const size = bytes[p++];
      if (size === 0) return;
      p += size;
    }
  };
  let frames = 0;
  while (p < bytes.length) {
    const marker = bytes[p++];
    if (marker === 0x3b) break; // Trailer
    if (marker === 0x21) {
      p++; // extension label
      skipSubBlocks();
      continue;
    }
    if (marker === 0x2c) {
      frames++;
      p += 8; // left/top/width/height
      const local = bytes[p++];
      if (local & 0x80) p += 3 * (1 << ((local & 0x07) + 1)); // Local Color Table
      p++; // LZW minimum code size
      skipSubBlocks();
      continue;
    }
    return { frames, truncated: true };
  }
  return { frames, truncated: false };
}

function inspectWebp(bytes) {
  // RIFF____WEBP 之後是 chunk 序列：4 bytes fourCC + 4 bytes size(LE) + payload(偶數對齊)
  let p = 12;
  let anmf = 0;
  const fourCC = (at) => String.fromCharCode(bytes[at], bytes[at + 1], bytes[at + 2], bytes[at + 3]);
  while (p + 8 <= bytes.length) {
    const cc = fourCC(p);
    const size = bytes[p + 4] | (bytes[p + 5] << 8) | (bytes[p + 6] << 16) | (bytes[p + 7] << 24);
    if (cc === "ANMF") anmf++;
    p += 8 + size + (size % 2);
  }
  return { frames: anmf };
}

function looksAnimated(buffer) {
  const bytes = new Uint8Array(buffer);
  const head = Buffer.from(bytes.subarray(0, 12)).toString("latin1");
  if (head.startsWith("RIFF") && head.includes("WEBP")) {
    const { frames } = inspectWebp(bytes);
    return { animated: frames > 1, container: `webp(${frames} frames)` };
  }
  if (head.startsWith("GIF8")) {
    const info = inspectGif(bytes);
    if (info === null) return { animated: false, container: "gif(unparseable)" };
    return {
      animated: info.frames > 1 && !info.truncated,
      container: `gif(${info.frames} frames${info.truncated ? ", truncated" : ""})`,
    };
  }
  return { animated: false, container: `unknown(${head.slice(0, 4)})` };
}

async function probe(preset, accept) {
  const url = `${baseUrl.replace(/\/$/, "")}/i/${preset}/${objectKey}`;
  const started = Date.now();
  const res = await fetch(url, { headers: accept ? { Accept: accept } : {} });
  const body = await res.arrayBuffer();
  return {
    preset,
    accept: accept ?? "(none)",
    status: res.status,
    contentType: res.headers.get("content-type"),
    bytes: body.byteLength,
    ms: Date.now() - started,
    ...looksAnimated(body),
  };
}

const rows = [
  await probe("original", null),
  await probe("motion", "image/avif,image/webp,*/*"),
  await probe("motion", null),
];

for (const r of rows) {
  console.log(
    `${r.preset.padEnd(9)} accept=${r.accept.padEnd(26)} ${r.status} ${String(r.contentType).padEnd(11)} ` +
      `${String(r.bytes).padStart(9)} B  ${String(r.ms).padStart(5)} ms  animated=${r.animated}  ${r.container}`,
  );
}

const original = rows[0];
const motionWebp = rows[1];
const motionPlain = rows[2];

const failures = [];
if (original.status !== 200) failures.push(`原檔取不到（${original.status}），這顆 key 不能用來驗`);
if (!original.animated) failures.push("原檔本身就不是動圖，換一顆真的 GIF 再驗");
for (const r of [motionWebp, motionPlain]) {
  if (r.status !== 200) failures.push(`motion accept=${r.accept} 回 ${r.status}`);
  if (!r.animated) failures.push(`motion accept=${r.accept} 輸出不是動的（${r.container}）`);
  // degrade 直出時位元組數會與原檔一模一樣——這是「轉換其實沒發生」最明確的指紋。
  if (r.bytes === original.bytes) {
    failures.push(`motion accept=${r.accept} 與原檔位元組數相同，等於 degrade 直出、轉換沒生效`);
  }
  if (r.bytes >= original.bytes) {
    failures.push(`motion accept=${r.accept} 沒有變小（${r.bytes} >= ${original.bytes}）`);
  }
}

if (failures.length > 0) {
  console.error("\n❌ 驗證失敗：");
  for (const f of failures) console.error(`  - ${f}`);
  process.exit(1);
}

const saved = (1 - motionWebp.bytes / original.bytes) * 100;
console.log(`\n✅ motion 生效：動畫保留，webp 版省下 ${saved.toFixed(1)}%（${original.bytes} → ${motionWebp.bytes} B）`);
