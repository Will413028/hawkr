# AI 美化風格 — 範例縮圖製作規格

「AI 圖片美化」彈窗的風格選擇器需要每個風格一張範例縮圖，讓賣家一眼看懂這個風格會把照片變成什麼樣子。

## 檔案放哪裡

目前每個風格共用一張佔位圖 `public/placeholders/ai-style.svg`（`pnpm gen:placeholders` 產生），常數是 `lib/seller/ai-image.ts` 的 `AI_STYLE_THUMBNAIL`。

- 尺寸：**512 × 512**（選擇器是 `aspect-square`，`object-cover`）
- 後端範例圖的格式：`.webp`，品質 80，單檔 < 60 KB
- 後端 `/app/ai_styles` 若之後回傳自己的 `thumbnailUrl`，會**蓋過**本地檔案
- 縮圖載入失敗的風格，選擇器自動顯示 AI 星芒佔位圖，不會破圖

## 生成方式

每個風格請用**同一張基準照片**去跑，這樣四張排在一起時差異就是「風格」本身，而不是「不同題材」。

- 商品類（`product`）建議基準照：一籃剛採收的葉菜 + 番茄，自然光、木桌背景
- 貼文類（`post`）建議基準照：農夫在田邊遞出一籃蔬果，半身入鏡

生成時把下面「生成提示詞」整段丟給圖片模型即可（已經是中英雙語，可直接使用）。

---

## 商品圖風格（category: `product`）

### 1. `natural.webp` — 新鮮自然

> 為上傳的農產品照片進行自然清新修圖。提升蔬果的飽和度、明亮度及鮮度呈現，使綠色更翠綠、紅色更亮而不失真。光線自然柔和、整體呈現清爽健康的農作物形象。背景保持乾淨並略微柔化。Enhance the uploaded produce photo with a fresh and natural look. Boost color saturation and brightness while keeping the image realistic. Make greens more vibrant and reds naturally bright. Apply soft natural lighting and keep the background clean and slightly blurred.

### 2. `warm.webp` — 陽光暖色

> 將上傳的農產品照片修成陽光暖色調。增加金黃色與橙色色溫，使畫面呈現午後陽光般的溫暖氛圍。降低對比、柔化陰影，讓農產品看起來更柔和、溫潤且具農場自然感。Edit the uploaded produce photo with a warm sunlight tone. Add golden and orange hues to create a late-afternoon farm atmosphere. Use soft shadows, lower contrast, and warm lighting for a gentle, sunlit look.

### 3. `crisp.webp` — 清脆鮮豔

> 將上傳的農產品照片修成清晰鮮豔的高對比風格。強化蔬果表面細節、增加亮度與鮮明色彩，使紅色、綠色與黃色更突出。整體呈現乾淨、清晰、電商感強烈的風格。Apply a crisp and vibrant style to the uploaded produce photo. Increase contrast, enhance surface details, and boost vivid colors. Make reds, greens, and yellows pop while keeping the image clean and sharp—ideal for e-commerce.

### 4. `giftbox.webp` — 高級禮盒

> 將上傳的農產品照片修成高級禮盒風格。使用柔和光線、細緻反光、略帶暖調，提升高級感。增加立體度與質感，使農產品呈現精品禮盒般的精緻視覺效果。Transform the uploaded produce photo into a premium giftbox style. Use soft lighting, refined highlights, and mild warm tones. Enhance depth, smoothness, and elegance to make the produce look like a luxury gift set.

### 5. `documentary.webp` — 自然寫真

> 將上傳的農產品照片修成寫實農場感。微降低飽和度、增加顆粒與細節，保留些許自然瑕疵，使畫面呈現樸實、真實、有鄉村農場氛圍。Edit the uploaded produce photo with a documentary farm look. Slightly lower saturation, add texture and realism, and keep some natural imperfections. The result should feel authentic, rural, and grounded.

### 6. `kitchen.webp` — 料理素材

> 將上傳的農產品照片修成料理備料風格。加強切面細節、提升質地呈現，色溫略暖。光線柔和不刺眼，使農作物看起來像在料理檯上剛被處理，適合食譜與料理內容。Apply a kitchen prep style to the uploaded produce photo. Highlight cut-surface details and textures with a slightly warm tone. Use soft, natural lighting to create a fresh cooking-table look suitable for recipes and food content.

### 7. `clean.webp` — 白底極淨

> 將上傳的農產品照片修成純白極淨電商風。背景轉換為 #FFFFFF 純白，提升產品亮度與對比，使蔬果輪廓清晰、色彩乾淨飽滿，呈現專業商品主圖效果。Edit the uploaded produce photo in a pure white e-commerce style. Convert the background to #FFFFFF white, enhance brightness and contrast, and make the produce edges clean and colors vivid. The result should look like a professional product main image.

---

## 貼文圖風格（category: `post`）

### 1. `post_japanese_poster.webp` — 日系排版海報風

> 以高調映像風格優化這張照片，增強自然光線與溫暖奶油黃色調，營造日系雜誌報導感，保持人物或商品主體清晰。Enhance this photo with a Japanese high-key editorial style — warm natural light, soft creamy-yellow tones, and a clean magazine-report mood, while keeping the main subject sharp.

### 2. `post_mixed_collage.webp` — 混合媒體拼貼風

> 以混合媒體拼貼風格優化這張照片，提升色彩飽和度與對比，加入復古膠卷顆粒質感，營造活潑、有層次的社群視覺氛圍，保持主體清晰可辨。Enhance this photo with a mixed-media collage mood — boosted saturation and contrast, subtle vintage film grain, and a lively layered social-media feel, while keeping the subject clearly recognizable.

### 3. `post_grid_minimal.webp` — 九宮格網格風

> 以簡約網格美學優化這張照片，提升整體明亮度與通透感，柔化色調並降低雜訊，營造乾淨、輕盈的社群九宮格質感，保持構圖穩定。Enhance this photo with a clean grid-aesthetic look — brighter and airier overall tone, softened colors, reduced noise, for a tidy minimal social-feed feel while keeping the composition stable.

### 4. `post_event_pixel.webp` — 活動海報風

> 以活動海報風格優化這張照片，強化黑白對比與銳利度，加入鮮明色塊氛圍與略帶復古螢幕質感的色調，營造吸睛、有活動宣傳感的視覺效果，保持主體清晰。Enhance this photo with a bold event-poster style — stronger black-and-white contrast and sharpness, punchy color-block mood with a subtle retro-screen tint, for an eye-catching promotional feel while keeping the subject clear.

---

## 加新風格時

1. 後端 `/app/ai_styles` 新增風格 → 拿到它的 `id`
2. 用同樣基準照跑一張 512×512 範例圖，由後端以 `thumbnailUrl` 回傳；沒有的風格用佔位圖
