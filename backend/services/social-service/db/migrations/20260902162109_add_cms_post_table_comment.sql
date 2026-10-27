-- Set comment to table: "cms_post"
COMMENT ON TABLE "cms_post" IS '媒體引用生命週期的 divergence 標記（image_asset_id／media_converged_asset_id／media_desired_asset_id）住在這張業務列上，不是獨立 outbox 表；待辦即兩者之差。因此本表不得硬刪：硬刪會把待辦連同唯一的收斂證據一起銷毀，對應的 asset_reference 從此無法被釋放，永久洩漏且事後無跡可循。日後若因 PII 清除需要硬刪，必須先讓該列收斂到空集合（media_desired_asset_id 與 media_converged_asset_id 皆為 NULL）再刪。見 #2772/#2773/#2774/#2775。';
