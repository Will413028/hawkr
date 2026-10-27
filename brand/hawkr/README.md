# Hawkr 品牌素材

作品集改名後的產品名稱與 logo。圖形是一個攤位：遮雨棚加兩根柱子與檯面組成 H，比喻「一個屋頂下有很多攤位」的多賣家平台。字標是 [Nunito](https://github.com/googlefonts/nunito)（SIL Open Font License 1.1）ExtraBold 以 HarfBuzz 排字（含 kerning）後轉成的外框，不依賴字型。

| 檔 | 用途 |
|---|---|
| `hawkr-mark.svg`／`hawkr-mark-dark.svg` | 圖形，淺色底／深色底 |
| `hawkr-wordmark.svg`／`hawkr-wordmark-dark.svg` | 字標 |
| `hawkr-lockup.svg`／`hawkr-lockup-dark.svg` | 圖形加字標的橫式組合 |
| `hawkr-app-icon.svg` | App 圖示（1024×1024，圓角底） |
| `hawkr-favicon.svg` | 16–48px 用的簡化版：不畫條紋、柱子加粗；更大的尺寸用 App 圖示 |

所有 SVG 由 `build.py` 產生，不手改；用法見檔頭（需要 fonttools、uharfbuzz 與 Nunito 的 variable font）。

顏色（三端的 design token 與契約 `website/design-system/cross-platform-tokens.json` 的 `brand.*`）：

| Token | 值 | 用途 |
|---|---|---|
| `brand.primary` | `#E4572E` | logo、大標（24px 以上，或 18.66px 以上粗體）、裝飾；不配一般尺寸的白字（3.68:1） |
| `brand.primaryStrong` | `#B8441F` | 互動元件：主按鈕底、連結、選取態、focus；白字 5.41:1 |
| `brand.primaryDeep` | `#8F3315` | hover、pressed；白字 7.92:1 |
| `brand.secondary` | `#F2A541` | 輔色底，配 `brand.ink` 文字（7.19:1）；不配白字 |
| `brand.ink` | `#1F2933` | Hawkr 的文字色 |
| `brand.inkOnDark` | `#F5F7FA` | 深色底上的文字 |
| `brand.primarySoft` | `#FCEBE6` | primary 12% 混白；淺底、標籤，配 `brand.ink` 文字不低於 4.5:1 |
| `brand.secondarySoft` | `#FDF1E1` | secondary 16% 混白；淺底、季節裝飾 |
| `light.text.accentOnTint` | `#8F3315`（同 `brand.primaryDeep`） | primary 半透明選取底與 `surface-panel` 上的強調文字；`brand.primaryStrong` 在這些底上低於 4.5:1 |
| `ink.50`–`ink.950` | `#F5F7FA` 到 `#161C23` | 以 `brand.ink`（900）與 `brand.inkOnDark`（50）為兩端的中性藍灰階，給分隔線、陰影、淡底與暗色模式用 |

暗色模式的底（page、muted、card、panel、elevated）是以 `brand.ink` 為基底、由深到淺的中性深色階，橘色只用在選取項與主按鈕；`brand.primary` 與 `brand.primaryStrong` 在暗色底上不當一般文字用（暗色的強調文字用 `brand.secondary`）。

接進三端時改用各端的 design token，不在元件裡寫死色碼。狀態色（成功、警告、危險、資訊）是通用色，不屬於品牌色票。

中英文介面的品牌名都是「Hawkr」，不另取中文名。

## App 圖示與啟動畫面的產生方式

兩支 App 的 iOS 圖示與 App 內的品牌圖由下面的腳本從本目錄的 SVG 產生，不手改 PNG。工具是 `@resvg/resvg-js@2.6.2`（沒有裝進任何專案，在暫存目錄安裝），另需 macOS 的 `sips`（讀既有 PNG 的像素寬度，輸出尺寸照原檔）。

```sh
mkdir /tmp/hawkr-assets && cd /tmp/hawkr-assets
npm init -y && npm pkg set type=module && npm i @resvg/resvg-js@2.6.2
# 把下面的腳本存成 /tmp/hawkr-assets/gen-brand-assets.mjs，然後在 repo 根目錄執行：
node /tmp/hawkr-assets/gen-brand-assets.mjs
```

- 輸入：`hawkr-app-icon.svg`（圖示，產生時把圓角 `rx` 改成 0，iOS 自己加遮罩）、`hawkr-favicon.svg`（像素寬度 48 以下的圖示）、`hawkr-mark.svg`／`hawkr-mark-dark.svg`／`hawkr-lockup.svg`／`hawkr-lockup-dark.svg`／`hawkr-wordmark.svg`。
- 輸出：`buyer-app`、`seller-app` 的 `ios/Runner/Assets.xcassets/{dev,stage,prod}AppIcon.appiconset/*.png`（RGB、無 alpha；dev 與 stage 的 1024 圖右下角保留紅色 DEV／STAGE 斜帶當 flavor 標記，斜帶的字用系統的 Helvetica Neue）、`assets/brand/` 下的 `logo-mark`、`logo-grass`、`logo-horizontal`、`logo-horizontal-light`、`wordmark`、`logo-four-color`（SVG 直接複製；PNG 依原檔寬度渲染）、`splash_mark*.png`、`assets/hawkrlogo.png`（圓形）。檔名與路徑不變，程式與 `pubspec.yaml` 不用改。
- 沒有用 `flutter_launcher_icons`：兩支 App 沒有 Android 專案目錄，iOS 圖示的 `Contents.json` 與檔名維持原樣即可。
- 原生啟動畫面維持透明圖加底色（`flutter_native_splash` 設定不變）；`splash_mark*.png` 只在重新執行 `fvm dart run flutter_native_splash:create` 時才會被用到。

```js
// 用法（在 repo 根目錄）：node gen-brand-assets.mjs
// 依賴：@resvg/resvg-js@2.6.2。輸入是 brand/hawkr/*.svg，輸出是兩支 App 的圖示與品牌圖。
import { Resvg } from '@resvg/resvg-js';
import { readFileSync, writeFileSync, readdirSync, copyFileSync, existsSync } from 'node:fs';
import { execFileSync } from 'node:child_process';
import { deflateSync } from 'node:zlib';

const brand = (n) => readFileSync(`brand/hawkr/${n}`, 'utf8');
const font = { loadSystemFonts: true, defaultFontFamily: 'Helvetica Neue' };

function render(svg, size) {
  return new Resvg(svg, { fitTo: { mode: 'width', value: size }, font }).render();
}

const crcTable = Array.from({ length: 256 }, (_, n) => {
  let c = n;
  for (let k = 0; k < 8; k++) c = c & 1 ? 0xedb88320 ^ (c >>> 1) : c >>> 1;
  return c >>> 0;
});
const crc32 = (buf) => {
  let c = 0xffffffff;
  for (const b of buf) c = crcTable[(c ^ b) & 0xff] ^ (c >>> 8);
  return (c ^ 0xffffffff) >>> 0;
};
const chunk = (type, data) => {
  const len = Buffer.alloc(4);
  len.writeUInt32BE(data.length);
  const body = Buffer.concat([Buffer.from(type, 'ascii'), data]);
  const crc = Buffer.alloc(4);
  crc.writeUInt32BE(crc32(body));
  return Buffer.concat([len, body, crc]);
};
// iOS 圖示不可以有 alpha 通道：把不透明的 RGBA 轉成 RGB 的 PNG。
function rgbPng(img) {
  const { width, height } = img;
  const px = img.pixels;
  const raw = Buffer.alloc((width * 3 + 1) * height);
  for (let y = 0; y < height; y++) {
    raw[y * (width * 3 + 1)] = 0;
    for (let x = 0; x < width; x++) {
      const s = (y * width + x) * 4;
      if (px[s + 3] !== 255) throw new Error('icon must be opaque');
      const d = y * (width * 3 + 1) + 1 + x * 3;
      raw[d] = px[s];
      raw[d + 1] = px[s + 1];
      raw[d + 2] = px[s + 2];
    }
  }
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(width, 0);
  ihdr.writeUInt32BE(height, 4);
  ihdr[8] = 8;
  ihdr[9] = 2;
  return Buffer.concat([
    Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]),
    chunk('IHDR', ihdr),
    chunk('IDAT', deflateSync(raw, { level: 9 })),
    chunk('IEND', Buffer.alloc(0)),
  ]);
}

const squareIcon = brand('hawkr-app-icon.svg').replace('rx="224"', 'rx="0"');
const squareFavicon = brand('hawkr-favicon.svg').replace('rx="7"', 'rx="0"');
const roundIcon = brand('hawkr-app-icon.svg').replace('rx="224"', 'rx="512"');

function ribbon(label, fontSize) {
  return (
    '<polygon points="728,1024 1024,728 1024,910 910,1024" fill="#D9283F"/>' +
    `<text x="0" y="0" transform="translate(919 919) rotate(-45)" text-anchor="middle" dominant-baseline="central" ` +
    `font-family="Helvetica Neue, Helvetica, Arial" font-weight="800" font-size="${fontSize}" fill="#FFFFFF">${label}</text>`
  );
}
function iconWithRibbon(label, fontSize) {
  return squareIcon.replace('</svg>', `${ribbon(label, fontSize)}</svg>`);
}

const pixelWidth = (f) =>
  Number(execFileSync('sips', ['-g', 'pixelWidth', f]).toString().match(/pixelWidth: (\d+)/)[1]);

const flavorRibbon = { dev: ['DEV', 84], stage: ['STAGE', 62], prod: null };
let count = 0;
for (const app of ['buyer-app', 'seller-app']) {
  for (const flavor of Object.keys(flavorRibbon)) {
    const dir = `${app}/ios/Runner/Assets.xcassets/${flavor}AppIcon.appiconset`;
    for (const f of readdirSync(dir).filter((n) => n.endsWith('.png'))) {
      const path = `${dir}/${f}`;
      const size = pixelWidth(path);
      let svg = size <= 48 ? squareFavicon : squareIcon;
      if (size === 1024 && flavorRibbon[flavor]) svg = iconWithRibbon(...flavorRibbon[flavor]);
      writeFileSync(path, rgbPng(render(svg, size)));
      count++;
    }
  }
  // App 內的品牌圖：檔名與路徑不變，內容換成 Hawkr。
  const a = `${app}/assets`;
  for (const [dst, src] of [
    ['brand/logo-mark.svg', 'hawkr-mark.svg'],
    ['brand/logo-grass.svg', 'hawkr-mark.svg'],
    ['brand/logo-horizontal.svg', 'hawkr-lockup.svg'],
    ['brand/logo-horizontal-light.svg', 'hawkr-lockup-dark.svg'],
    ['brand/wordmark.svg', 'hawkr-wordmark.svg'],
    ['brand/logo-four-color.svg', 'hawkr-mark.svg'],
  ]) {
    if (existsSync(`${a}/${dst}`)) copyFileSync(`brand/hawkr/${src}`, `${a}/${dst}`);
  }
  const png = (dst, svg) => {
    if (!existsSync(`${a}/${dst}`)) return;
    writeFileSync(`${a}/${dst}`, render(svg, pixelWidth(`${a}/${dst}`)).asPng());
  };
  png('brand/logo-four-color.png', brand('hawkr-mark.svg'));
  png('brand/splash_mark.png', brand('hawkr-mark.svg'));
  png('brand/splash_mark_android12.png', brand('hawkr-mark.svg'));
  png('brand/splash_mark_wheat.png', brand('hawkr-mark-dark.svg'));
  png('hawkrlogo.png', roundIcon);
}
console.log('icons written:', count);
```
