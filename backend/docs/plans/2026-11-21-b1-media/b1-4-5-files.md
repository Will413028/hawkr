# B1-4＋B1-5 逐檔清單

基準：HEAD `a93b7aac`。來源：`upload-routing.md`（51 列呼叫點）、計畫 B1-4／B1-5 段、B1-D18、B1-D19。產生指令：

```bash
grep -rn 'upload_sessions\|secure_upload\|secure_preview' backend/gateway website admin seller-app buyer-app --include='*.go' --include='*.ts' --include='*.tsx' --include='*.dart' | grep -v node_modules | cut -d: -f1 | sort -u   # 40 檔
grep -rln 'mediaref\|media\.Sync\|RefSyncer\|FinalizeArgs\|media\.Ref' backend/internal backend/cmd --include='*.go'          # 76 檔
grep -rln 'media_sync\|media_sweep' backend/internal --include='*.go'
```

欄位：新增（A）、修改（M）、刪除（D）。「車道」是派工分組，同一檔只屬一個車道。

## 共用上傳流程（車道 L1 TS、L2 Dart）

| 動作 | 檔 | 內容 |
|---|---|---|
| A | `packages/hawkr-api-ts/src/connect/upload.ts`（＋測試） | `BeginUpload`→預簽 PUT（MD5、回應指定標頭）→`CompleteUpload`→`GetUpload` 輪詢到 READY／REJECTED；`BeginSecureUpload`→分段 PUT 與 409 續傳；HEIC 由呼叫端轉檔後傳入（client 已有轉檔的沿用） |
| M | `packages/hawkr-api-ts/package.json` | subpath export `./connect/upload` |
| M | `backend/internal/platform/uitext/catalog/`、`make uitext` 生成的三端檔 | `RejectReason` 每個值的文字（B1-D18） |
| A | `packages/hawkr_api/lib/upload.dart`（＋測試） | 同 TS 版語意 |
| M | `AGENTS.md`（根目錄地圖） | `@hawkr/api` 補 `connect/upload`、`hawkr_api` 補 `upload.dart` |

## website（車道 L3）

| 動作 | 檔 | 內容 |
|---|---|---|
| M | `website/lib/media/upload.ts`、`website/lib/media/secure-upload.ts` | 改成呼叫 `@hawkr/api/connect/upload`，刪手寫 REST 呼叫與型別；對外函式簽名維持，呼叫端（W1–W25）不必逐一改 |
| M | `website/app/(delegated)/adm/api/media/upload_sessions/route.ts`、`.../upload_sessions/[sessionId]/complete/route.ts`（＋新增 GetUpload 的轉發） | 代行公開圖片改轉發 `MediaDelegatedService` 的 `BeginUpload`／`CompleteUpload`／`GetUpload`（B1-D21） |
| 保留 | `.../secure_uploads/[token]/route.ts`、`.../secure_uploads/[token]/chunks/[index]/route.ts` | B1-3c3 建的機敏代行 BFF（代行憑證是 HttpOnly cookie，瀏覽器不能自己帶標頭） |
| M | `website/app/api/files/[...path]/route.ts`、`website/lib/api/local-proxy.unit.test.ts`、`website/tests/api-proxy-rewrites.test.ts`、`website/tests/sentry-trace-tag.test.ts` | 拿掉舊 media 路徑的分支與斷言，補新路徑 |
| M | `website/lib/seller-data/delegated-repos.ts`、`website/lib/seller-data/repo-injection.unit.test.ts` | 代行 media repo 改用新流程 |
| M | `website/lib/seller/shipment.ts` | 刪 `printCvsLabel` 的死參數 `stagedTokens` |
| M | `website/lib/invoices/seller-repo.ts:439,1068` | 刪 `sellerTaxDocumentHref`、`receiptApprovalBasisHref` 組 media 網址；使用者 `SellerTaxStatusPage.tsx`、`ReceiptAuthPage.tsx` 拿掉連結（本機失敗到 B10，不改指 invoice） |
| M／D | `website/lib/media/__tests__/upload.unit.test.ts`、`website/features/delegated-operations/secure-upload-bff.unit.test.ts`（D）、`website/tests/delegated-bff-routes.test.ts` | 跟著改 |
| M | 顯示 REJECTED 的既有錯誤提示：`ImageUploader.tsx`、`ProductImageUploader.tsx`、`AfterSalePhotoUploader.tsx`、`AttachmentPicker.tsx` 等呼叫 `lib/media/upload` 的元件 | 錯誤句用 `@hawkr/ui-text` 的 `RejectReason` 文字，不新增版面（B1-D18） |

## admin（車道 L4）

| 動作 | 檔 | 內容 |
|---|---|---|
| M | `admin/src/services/media-upload.ts`、`admin/src/services/seller-prospect-media.ts` | 改用 `@hawkr/api/connect/upload` |
| M | `admin/src/components/banner/BannerEditorClient.tsx`、`ProspectMediaPanel.tsx`、`RecordInquiryReplyDialog.tsx`、`invoices/jobs/[jobId]/page.tsx`、`ai-styles/page.tsx` | REJECTED 走既有錯誤提示；A3、A4 只換上傳 helper（後端收不下，本機失敗） |
| M | `admin/tests/media-upload.test.ts`、`admin/tests/seller-prospect-media.test.ts`、`admin/tests/invoice-unlock-evidence-secure-upload.test.ts` | 跟著改 |

admin 的 BFF 若有轉發 `/api/v1/app/media/*` 的路徑一併刪（實作者以 `rg -n 'media/' admin/src/app/api` 核對）。

## seller-app 與 buyer-app（車道 L5）

| 動作 | 檔 | 內容 |
|---|---|---|
| M | `seller-app/lib/features/media/data/media_repository.dart` | 改呼叫 `hawkr_api` 的 `upload.dart`；刪 `getSecureDownloadUrl`、`getSecurePreviewUrl`（死碼） |
| M | `seller-app/lib/features/media/data/private_document_uploader.dart`＋`.g.dart`（重生成） | 機敏改用 `upload.dart` 的分段上傳 |
| M | `seller-app/lib/features/seller/data/kyc_wizard.dart`、`protected_media_*` | 讀證件只用 `download_uri`（B1-D19） |
| M | `seller-app/test/features/media/*`、`post_gif_upload_contract_test.dart`、`kyc_wizard_test.dart` 等 | 跟著改 |
| M | `buyer-app/lib/core/networking/oss_service.dart`＋`.g.dart`、`buyer-app/lib/features/media/data/media_repository.dart` | 改呼叫 `upload.dart`，刪手寫 REST 與型別；只剩死碼時刪檔 |
| M | 兩支 App 顯示上傳錯誤的既有提示 | `hawkr_ui_text` 的 `RejectReason` 文字 |

## gateway（車道 L6）

| 動作 | 檔 |
|---|---|
| D | `gateway/internal/router/routes_media.go`、`gateway/internal/handler/media.go`、`gateway/internal/handler/admin_delegated_media.go`、`gateway/internal/grpc/` 的 `media_client.go` |
| M | `router/constants.go:178-183`、`router/routes_delegated.go`、`router/seller_route_assert.go`、`router/delegated_capability_test.go`、`router/openapi_coverage_test.go`（binaryProxy 例外三列）、`middleware/body_guard.go`＋`_test.go`、`middleware/logger.go`、`authz/admin_capability.go`、`proxy/reverse_proxy_rewrite_test.go`、`handler/holder.go`、`handler/di.go`、`grpc/di.go`（`MediaClient`） |
| M | `backend/api/openapi/openapi.yaml`（刪兩個 operation） |
| 保留 | `router/admin_secure_media_removed_test.go` 的不變量（後台沒有通用機敏下載）；`handler/order_review.go` 的 `stagedTokens` |

B1-1 的 REST driver media 情境與 `backend-characterization` job 的 media 服務項一起退場（job 留著）。

## 單體：擁有者模組（車道 L7 buyer＝5a、L8 seller、L9 其餘五個＝5b）

每個模組同一套：

| 動作 | 檔 |
|---|---|
| D | `internal/modules/<m>/internal/mediaref/`、`media_sync`／`media_sweep` job 註冊與 sqlc 查詢、`retired_assets`／`media_ref_sync` 表 |
| A | `internal/modules/<m>/db/migrations/` 新 migration（`*_asset_id` 改 UUID、drop 舊表；Atlas 生成，不改歷史） |
| M | `db/schema.sql`、`db/queries/`、`internal/store`（`make sqlc`）、寫入 use case：transaction 內 `Confirm`、換掉／終態 `MarkDeleted`（moderation 只 `Confirm`） |
| M | 模組根 `<m>.go` 的依賴介面（改吃新 media port）、整合測試 `*_integration_test.go`（換圖、回滾、終態三條） |

- L7 buyer：另做動態 WebP 單邊超過 16383 改標頭階段永久拒收（`internal/modules/media`、`RejectReason`）。
- L8 seller：KYC 與治理佐證讀取改 `ReadSecure`；B1-D19 新增 `GET /api/files/seller/kyc/{document}`（`internal/httpapi/seller/`，店擁有者授權、`Cache-Control: private, no-store`）；`internal/httpapi/seller/kyc.go:60-61,263` 的 `secureDownloadURI` 改指向它。
- L9：content、im、review、social、moderation。

## 單體：收尾刪除（車道 L10，L7–L9 完成後）

| 動作 | 檔 |
|---|---|
| M | `cmd/hawkr/main.go:182,384`、`cmd/hawkr/seller_commands.go:68` 的 `mediaservice.New` 改接 Go media |
| D | `internal/modules/media/finalize.go`、`finalize_args.go`、`finalize_test.go`、`ref_type.go`、`internal/modules/media/internal/refsync/`、`pkg/media/ref_type.go`、`internal/httpapi/characterize/fakemedia.go` |
| M | `internal/modules/media/fake.go`（新介面的 fake）、`media.go`（刪 Finalize／Link／Unlink） |
| M | CodeMeta 裡 Java 仍用的 ref_type 值標擁有者 Java media |
| A | `mutation/b1-4-5.json`（彙整各車道的 mutant，CI 跑） |
