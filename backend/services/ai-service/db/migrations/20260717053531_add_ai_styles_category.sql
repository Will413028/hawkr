-- Drop index "idx_ai_styles_default" from table: "ai_styles"
DROP INDEX "idx_ai_styles_default";
-- Modify "ai_styles" table
ALTER TABLE "ai_styles" DROP CONSTRAINT "ai_styles_name_key", ADD CONSTRAINT "ai_styles_category_check" CHECK (category = ANY (ARRAY['product'::text, 'post'::text])), ADD COLUMN "category" text NOT NULL DEFAULT 'product';
-- Create index "idx_ai_styles_default" to table: "ai_styles"
CREATE UNIQUE INDEX "idx_ai_styles_default" ON "ai_styles" ("category") WHERE (is_default = true);
-- Create index "idx_ai_styles_category_name" to table: "ai_styles"
CREATE UNIQUE INDEX "idx_ai_styles_category_name" ON "ai_styles" ("category", "name");

-- Seed: 4 組貼文預設風格 + 1 筆貼文 default prompt
-- NOTE: id 從 11 起（非 brief 原定的 9）——dev DB 既有一筆未預期的手動測試資料
-- (id=10, name='test', 2026-04-08 建立)，brief 假設的「id 1-8 用掉」不成立，9 空但 10 被佔用，
-- 為避免與既有資料交錯、保持區塊連續，改用緊接目前最大 id(10) 之後的 11-15。
INSERT INTO "ai_styles" ("id", "name", "label", "prompt", "sort_order", "is_default", "enabled", "category") VALUES
(11, 'post_japanese_poster', '日系排版海報風',
 '以高調映像風格優化這張照片，增強自然光線與溫暖奶油黃色調，營造日系雜誌報導感，保持人物或商品主體清晰。Enhance this photo with a Japanese high-key editorial style — warm natural light, soft creamy-yellow tones, and a clean magazine-report mood, while keeping the main subject sharp.',
 1, false, true, 'post'),

(12, 'post_mixed_collage', '混合媒體拼貼風',
 '以混合媒體拼貼風格優化這張照片，提升色彩飽和度與對比，加入復古膠卷顆粒質感，營造活潑、有層次的社群視覺氛圍，保持主體清晰可辨。Enhance this photo with a mixed-media collage mood — boosted saturation and contrast, subtle vintage film grain, and a lively layered social-media feel, while keeping the subject clearly recognizable.',
 2, false, true, 'post'),

(13, 'post_grid_minimal', '九宮格網格風',
 '以簡約網格美學優化這張照片，提升整體明亮度與通透感，柔化色調並降低雜訊，營造乾淨、輕盈的社群九宮格質感，保持構圖穩定。Enhance this photo with a clean grid-aesthetic look — brighter and airier overall tone, softened colors, reduced noise, for a tidy minimal social-feed feel while keeping the composition stable.',
 3, false, true, 'post'),

(14, 'post_event_pixel', '活動海報風',
 '以活動海報風格優化這張照片，強化黑白對比與銳利度，加入鮮明色塊氛圍與略帶復古螢幕質感的色調，營造吸睛、有活動宣傳感的視覺效果，保持主體清晰。Enhance this photo with a bold event-poster style — stronger black-and-white contrast and sharpness, punchy color-block mood with a subtle retro-screen tint, for an eye-catching promotional feel while keeping the subject clear.',
 4, false, true, 'post'),

(15, 'default', '預設',
 '優化這張貼文照片，增強其視覺吸引力與社群感，保持照片原有風格與主體清晰。Enhance this post image to make it more visually engaging and social-media friendly while preserving its original character and subject clarity.',
 0, true, false, 'post');
