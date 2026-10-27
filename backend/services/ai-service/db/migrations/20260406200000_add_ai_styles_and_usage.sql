-- Create "ai_styles" table
CREATE TABLE "ai_styles" (
  "id" bigserial NOT NULL,
  "name" text NOT NULL,
  "label" text NOT NULL,
  "prompt" text NOT NULL,
  "thumbnail_url" text NOT NULL DEFAULT '',
  "sort_order" integer NOT NULL DEFAULT 0,
  "is_default" boolean NOT NULL DEFAULT false,
  "enabled" boolean NOT NULL DEFAULT true,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("id"),
  CONSTRAINT "ai_styles_name_key" UNIQUE ("name")
);
-- Create partial unique index for is_default
CREATE UNIQUE INDEX "idx_ai_styles_default" ON "ai_styles" ("is_default") WHERE is_default = true;
-- Create index for sorting
CREATE INDEX "idx_ai_styles_sort" ON "ai_styles" ("enabled", "sort_order");

-- Create "ai_usage" table
CREATE TABLE "ai_usage" (
  "account_id" bigint NOT NULL,
  "week_start" date NOT NULL,
  "usage_count" integer NOT NULL DEFAULT 0,
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("account_id", "week_start")
);

-- Create "ai_quotas" table
CREATE TABLE "ai_quotas" (
  "account_id" bigint NOT NULL,
  "weekly_limit" integer NOT NULL,
  "memo" text NOT NULL DEFAULT '',
  "created_at" timestamptz NOT NULL DEFAULT now(),
  "updated_at" timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY ("account_id")
);

-- Seed: 7 種預設風格 + 1 筆 default prompt
INSERT INTO "ai_styles" ("name", "label", "prompt", "sort_order", "is_default", "enabled") VALUES
('natural', '新鮮自然',
 '為上傳的農產品照片進行自然清新修圖。提升蔬果的飽和度、明亮度及鮮度呈現，使綠色更翠綠、紅色更亮而不失真。光線自然柔和、整體呈現清爽健康的農作物形象。背景保持乾淨並略微柔化。Enhance the uploaded produce photo with a fresh and natural look. Boost color saturation and brightness while keeping the image realistic. Make greens more vibrant and reds naturally bright. Apply soft natural lighting and keep the background clean and slightly blurred.',
 1, false, true),

('warm', '陽光暖色',
 '將上傳的農產品照片修成陽光暖色調。增加金黃色與橙色色溫，使畫面呈現午後陽光般的溫暖氛圍。降低對比、柔化陰影，讓農產品看起來更柔和、溫潤且具農場自然感。Edit the uploaded produce photo with a warm sunlight tone. Add golden and orange hues to create a late-afternoon farm atmosphere. Use soft shadows, lower contrast, and warm lighting for a gentle, sunlit look.',
 2, false, true),

('crisp', '清脆鮮豔',
 '將上傳的農產品照片修成清晰鮮豔的高對比風格。強化蔬果表面細節、增加亮度與鮮明色彩，使紅色、綠色與黃色更突出。整體呈現乾淨、清晰、電商感強烈的風格。Apply a crisp and vibrant style to the uploaded produce photo. Increase contrast, enhance surface details, and boost vivid colors. Make reds, greens, and yellows pop while keeping the image clean and sharp—ideal for e-commerce.',
 3, false, true),

('giftbox', '高級禮盒',
 '將上傳的農產品照片修成高級禮盒風格。使用柔和光線、細緻反光、略帶暖調，提升高級感。增加立體度與質感，使農產品呈現精品禮盒般的精緻視覺效果。Transform the uploaded produce photo into a premium giftbox style. Use soft lighting, refined highlights, and mild warm tones. Enhance depth, smoothness, and elegance to make the produce look like a luxury gift set.',
 4, false, true),

('documentary', '自然寫真',
 '將上傳的農產品照片修成寫實農場感。微降低飽和度、增加顆粒與細節，保留些許自然瑕疵，使畫面呈現樸實、真實、有鄉村農場氛圍。Edit the uploaded produce photo with a documentary farm look. Slightly lower saturation, add texture and realism, and keep some natural imperfections. The result should feel authentic, rural, and grounded.',
 5, false, true),

('kitchen', '料理素材',
 '將上傳的農產品照片修成料理備料風格。加強切面細節、提升質地呈現，色溫略暖。光線柔和不刺眼，使農作物看起來像在料理檯上剛被處理，適合食譜與料理內容。Apply a kitchen prep style to the uploaded produce photo. Highlight cut-surface details and textures with a slightly warm tone. Use soft, natural lighting to create a fresh cooking-table look suitable for recipes and food content.',
 6, false, true),

('clean', '白底極淨',
 '將上傳的農產品照片修成純白極淨電商風。背景轉換為 #FFFFFF 純白，提升產品亮度與對比，使蔬果輪廓清晰、色彩乾淨飽滿，呈現專業商品主圖效果。Edit the uploaded produce photo in a pure white e-commerce style. Convert the background to #FFFFFF white, enhance brightness and contrast, and make the produce edges clean and colors vivid. The result should look like a professional product main image.',
 7, false, true),

('default', '預設',
 '優化這張農產品圖片，增強其視覺吸引力。保持產品的自然外觀，同時提升整體品質和專業感。Enhance this produce image to make it more visually appealing while maintaining its natural appearance.',
 0, true, false);
