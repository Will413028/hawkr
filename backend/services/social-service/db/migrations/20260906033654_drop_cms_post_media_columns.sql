-- Drop index "idx_cms_post_media_diverged" from table: "cms_post"
DROP INDEX "idx_cms_post_media_diverged";
-- Modify "cms_post" table
ALTER TABLE "cms_post" DROP COLUMN "media_converged_asset_id", DROP COLUMN "media_next_attempt_at", DROP COLUMN "media_desired_asset_id";
-- Set comment to table: "cms_post"
COMMENT ON TABLE "cms_post" IS '本表不得硬刪（DELETE FROM）。image_asset_id 是這一列宣告的媒體引用真相，media reconciler 每輪只重放 ListLiveMediaRefs 看得到的列來收斂（level-triggered，無佇列無租約）。media 沒有任何 RPC 能列舉某 ref_type 的全部引用，所以硬刪之後這一列是從候選集合裡消失、而不是回傳空集合，reconciler 永遠不會再為它送出 Release，它在 media 上已建立的引用就永久留存且事後無跡可循——硬刪＝銷毀 root。軟刪保住那一列因而保住 root：資產欄位仍指著原資產，引用刻意保留不釋放；代價是軟刪列的資產永遠不會被回收，這是刻意接受的存儲成本，不是待修的洩漏。真正需要清除時由該情境自己的流程先釋放引用，帳號終止走 seller.deleted handler（internal/worker/seller_deleted.go）。完整契約見 pkg/mediaref/column_contract.md §1。見 #2772/#2773/#2774/#2775。';
