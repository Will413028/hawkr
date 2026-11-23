# upload-routing（B1-0）

基準：HEAD `c8870685`。B1-3 搜尋指令原樣執行（命中 97 行，重跑該指令即可重現）。命中的全是傳輸層（gateway 路由與 handler、各 client 的上傳 helper、測試），不是功能層；本表是往上追到每個「拿 staged token 去用」的畫面或功能。

## 名詞

- `Go-owner`：請求走 Connect，由單體模組處理（handler 在 `backend/internal/httpapi/**`，轉正在 `backend/internal/modules/**` 的 `media.FinalizeField`／`FinalizeList`）。
- `Java`：請求走 `/api/v1` REST，經 gateway handler 呼叫 Java 服務的 gRPC。
- `other-legacy-Go`：請求走 `/api/v1` REST，經 gateway 呼叫舊的 Go 服務（invoice-service、ai-service），舊服務自己呼叫 Java media 轉正。review-service 與 im-service 已被 dev 的 A6-6 刪除，評價與聊天的 client 現在走 Connect 到單體的 review、im 模組（B1-9 合併後改標 Go-owner）。
- `none`：路由存在但固定回 503，沒有消費者。
- 證據等級：measured＝開過 client 呼叫點、帶 token 的請求、gateway handler 或單體 handler；inferred＝只靠名稱或欄位反推。

ref_type／scope：所有 client 建立會話時只送 `contentType`、`fileSize`、`filename`（機敏版另加 `isSecure: true`：website `lib/media/secure-upload.ts:66`、admin `src/services/media-upload.ts:24`、seller-app `media_repository.dart:100`）。沒有 client 送 ref_type 或 scope；`docType`（`appeal_evidence`、`inquiry_evidence`、`supplement`）只是標記，gateway 不依它分流（`website/features/seller/measures/appealEvidenceUpload.ts:4`）。ref_type 由消費端轉正時給（Go：`media.RefType*`；Java：`FinalizeImages` 的 ref_type）。

## 一、呼叫點總表

路徑相對 worktree 根目錄。

### website（25 列）

| # | 呼叫點 | 功能 | 帶 token 的請求 | 後端消費者 | 種類 | 證據 |
|---|---|---|---|---|---|---|
| W1 | `website/features/account/screens/profile/ProfileClient.tsx:55` | 買家換頭像 | Connect `BuyerAppService.UpdateProfile` 的 `avatar_staged_token`（`website/lib/profile/repo.ts:136`） | buyer 模組：`backend/internal/httpapi/buyer/app.go:202` → `modules/buyer/internal/profile/profile.go:305` | Go-owner（buyer） | measured |
| W2 | `website/features/seller/components/SellerProfileBranding.tsx:199`（賣家身分） | 賣家換店面 logo／封面 | Connect `SellerAppService.UpdateMyStore` 的 `logo_staged_token`／`cover_staged_token`（`website/lib/seller/repo.ts:145,148`） | seller 模組：`httpapi/seller/app.go:228` → `modules/seller/internal/profile/profile.go:423` | Go-owner（seller） | measured |
| W3 | 同上 `SellerProfileBranding.tsx:199`（代行身分，`scope.repos.media` 是 delegated 版） | 營運代行換店面 logo／封面 | REST `POST …/sellers/:store_id/storefront/logo`、`/cover`（`website/lib/seller-data/delegated-repos.ts:168,174`） | gateway `DelegatedStorefrontUnavailable`（`backend/gateway/internal/router/routes_delegated.go:88-89`、`handler/storefront_unavailable.go:11`）固定回 503，不讀不寫任何服務 | none | measured |
| W4 | `website/features/seller-center/posts/PostComposeClient.tsx:445,465`（賣家身分） | 賣家貼文圖 | Connect `SocialAppService.CreatePost`（`website/lib/posts/owner.ts:98`）的 `image_staged_token` | social 模組：`httpapi/social/owner.go:161` → `modules/social/internal/post/post.go:133` | Go-owner（social） | measured |
| W5 | 同上 `PostComposeClient.tsx:445,465`（代行身分） | 營運代行貼文圖 | BFF `website/app/(delegated)/adm/api/posts/[storeId]/route.ts` → Connect `SocialDelegatedService.CreateStorePost`；更新走 `UpdateStorePost` | social 模組：`httpapi/social/delegated.go:247,274` | Go-owner（social） | measured（`[postId]/route.ts` 沒開，由 `delegated.go:274` 反推） |
| W6 | `website/features/seller/components/DescriptionEditor.tsx:1848,2089`（賣家身分） | 商品說明內嵌圖 | REST `POST /app/products`、`PUT /app/products/:id` 的說明區塊 `stagedToken`（`website/lib/products/seller-repo.ts:389,404`；`lib/products/descriptionEditorJs.ts:500`） | gateway `handler/seller_product.go:535,675` → `h.Product.CreateProduct`／`UpdateProduct` → Java product-service（`DescriptionBlockDTO`、`ProductService`） | Java（product） | measured（到 gateway 呼叫）；Java 端靠 grep 命中 |
| W7 | 同上 `DescriptionEditor.tsx:1848,2089`（代行身分） | 營運代行商品說明圖 | REST 代行 `PATCH …/products/:productId`（`website/features/delegated-operations/products/repo.ts:296`） | gateway `handler/admin_delegated_product.go:394,588` → `CreateProductForDelegation`／`UpdateProductForDelegation` → Java product-service | Java（product） | inferred（代行商品頁是否掛 DescriptionEditor 沒逐一確認） |
| W8 | `website/features/seller/components/ProductImageUploader.tsx:83`（`seller-center/products/components/ProductFormSections.tsx:151` 使用） | 賣家商品圖庫 | 同 W6，`images[].stagedToken` | 同 W6 | Java（product） | measured |
| W9 | `website/lib/media/resolve-staged-images.ts:47`（`useProductForm.ts:212` 呼叫） | 編輯商品時把匯入來源圖下載後重傳 | 同 W8 | 同 W6 | Java（product） | measured |
| W10 | `website/features/delegated-operations/products/DelegatedProductCreatePage.tsx:292` | 營運代行建商品圖 | REST 代行建立商品（`repo.ts:95-97` 的 `{ stagedToken }`） | gateway `admin_delegated_product.go:394` → Java product-service | Java（product） | measured |
| W11 | `website/features/delegated-operations/products/DelegatedProductEditPage.tsx:386` | 營運代行編輯商品圖 | REST 代行 `PATCH …/products/:productId`（`repo.ts:296`） | gateway `admin_delegated_product.go:588` → Java product-service | Java（product） | measured |
| W12 | `website/features/seller/components/ShipmentDialogs.tsx:271`（`TcatReservationDialog`，`seller-center/orders/OrderDetailClient.tsx:298` 送出） | 黑貓宅配取號附出貨照片 | REST `POST /app/shipments/:orderNo/generate_home_label` 的 `stagedTokens`（`website/lib/seller/shipment.ts:160-165`） | gateway `handler/shipping.go:429` → `h.Shipping` 的 GenerateHomeLabel → Java shipping-service | Java（shipping） | measured |
| W13 | `ShipmentDialogs.tsx:587`（`SevenElevenShipDialog`，`OrderDetailClient.tsx:399` 送出） | 7-11 出貨確認附照片 | REST `POST /app/shipments/:orderNo/confirm` 的 `stagedTokens`（`shipment.ts:237-241`） | gateway `handler/shipping.go:1307` → `h.Shipping.ConfirmCvsShipment` → Java shipping-service | Java（shipping） | measured |
| W14 | `website/features/seller-center/orders/SellerOrdersPage.tsx:1786`（`:1872` 送出） | 訂單列表超商確認出貨照片 | 同 W13 | 同 W13 | Java（shipping） | measured |
| W15 | `website/features/account/screens/orders/ReviewClient.tsx:113` | 買家訂單評論圖 | Connect `ReviewAppService.CreateReview` 的 `image_staged_tokens`（`website/lib/reviews/repo.ts`） | 單體 `modules/review/internal/write/write.go` 提交時在 transaction 內 `media.ConfirmFields`（舊 review-service 與 gateway `POST /app/reviews` 已被 dev 的 A6-6 刪除） | Go-owner（review） | measured（dev 合併後 2026-11-23，B1-9）|
| W16 | `website/lib/hooks/useChat.ts:361` | 買家聊天傳圖 | Connect `ChatAppService` 送訊息的 `image_staged_token`（`website/lib/chat/repo.ts`） | 單體 `modules/im/internal/write/send.go`：`Send` 的 transaction 先取 request_id 的鎖與重送判定，之後才 `media.ConfirmField`（B1-D26；重送判定必須排在 Confirm 之前，否則並行或重送的同一個 request_id 會撞到 token 已確認，B1-D23） | Go-owner（im） | measured（dev 合併後 2026-11-23，B1-9）|
| W17 | `website/features/account/screens/orders/ReturnClient.tsx:271` | 買家申請售後附照片 | REST `POST /app/after_sales` 的 `photoTokens`（`website/lib/after-sales/buyer-repo.ts:45-51`） | gateway `handler/aftersales.go:365` → `h.Aftersales.CreateAfterSaleCase` → Java aftersales-service（`EvidenceService`） | Java（aftersales） | measured |
| W18 | `website/features/account/screens/after-sales/AfterSaleDetailClient.tsx:772` | 買家補件照片 | REST `POST /app/after_sales/:caseNo/supplement` 的 `photoTokens`（`buyer-repo.ts:76-83`） | gateway `aftersales.go:506` → `SupplementEvidence` → Java aftersales-service | Java（aftersales） | measured |
| W19 | `AfterSaleDetailClient.tsx:875` | 買家申訴（仲裁）佐證 | REST `POST /app/after_sales/:caseNo/appeal` 的 `evidencePhotoTokens`（`buyer-repo.ts:88-100`） | gateway `aftersales.go:1107` → `SubmitAppeal` → Java aftersales-service | Java（aftersales） | measured |
| W20 | `website/features/after-sales/components/AfterSaleConfirmReceiptDialog.tsx:167` | 賣家退貨驗貨照片 | REST `POST /app/seller/after_sales/:caseNo/confirm_receipt` 的 `inspectionPhotoTokens`（`lib/after-sales/seller-repo.ts:82-86`） | gateway `aftersales.go:786` → `ConfirmReturnReceipt` → Java aftersales-service | Java（aftersales） | measured |
| W21 | `website/features/report/ReportDialog.tsx:271` | 買家檢舉店家附佐證 | Connect `SellerReportAppService.CreateReport` 的 `evidence_staged_tokens`（`website/features/report/api.ts:16-22`） | seller 模組：`httpapi/seller/reports.go:115` → `modules/seller/internal/mediaref/mediaref.go:307`（`RefTypeGovernanceEvidence`） | Go-owner（seller） | measured |
| W22 | `website/features/seller/measures/SellerMeasureRequestPage.tsx:342`（機敏） | 賣家對措施申覆／補救附佐證 | Connect `SellerAppService.SubmitAppeal`／`RequestRemediation`（`website/lib/seller/measures.ts:112,120`） | seller 模組：`httpapi/seller/measures.go:79,105` | Go-owner（seller） | measured |
| W23 | `website/features/seller/inquiries/InquiryReplyForm.tsx:148`（機敏） | 賣家回覆平台詢問附件 | Connect `SellerAppService.ReplyToInquiry`（`website/lib/seller/inquiries.ts:88-91`） | seller 模組：`httpapi/seller/inquiries.go:92` | Go-owner（seller） | measured |
| W24 | `website/features/seller-center/profile/certifications/CertificationsClient.tsx:798`（機敏） | 賣家認證補充文件 | Connect `SaveKYCDraft`／`SubmitKYC` 的 `KYCDocument.staged_token`（`website/lib/seller/kyc.ts:186-187`） | seller 模組：`httpapi/seller/kyc.go:64,194` → `modules/seller/internal/kyc/documents.go:55` | Go-owner（seller） | measured |
| W25 | `website/features/seller-center/invoices/tax-status/TaxIdentityForm.tsx:66`（機敏） | 賣家稅籍證明 | REST `PUT /app/seller/invoices/tax_profile/draft` 的 `evidenceStagedTokens`（`website/lib/invoices/seller-repo.ts:1151-1170`） | gateway `handler/invoice_seller.go:131` → `h.Invoice.SaveSellerTaxProfileDraft` → 舊 Go invoice-service（`services/invoice-service/internal/usecase/tax_profile.go`） | other-legacy-Go（invoice-service，B10） | measured |

### admin（5 列）

| # | 呼叫點 | 功能 | 帶 token 的請求 | 後端消費者 | 種類 | 證據 |
|---|---|---|---|---|---|---|
| A1 | `admin/src/components/banner/BannerEditorClient.tsx:182` | 橫幅預覽圖與彈窗圖 | Connect `CreateBanner`／`UpdateBanner` 的 `preview_image_staged_token`、`popup_image_staged_token`（`BannerEditorClient.tsx:207-208`） | content 模組：`httpapi/content/admin.go:63,77` → `modules/content/internal/images/images.go:63` | Go-owner（content） | measured |
| A2 | `admin/src/app/admin/seller-prospects/[id]/ProspectMediaPanel.tsx:120`（經 `admin/src/services/seller-prospect-media.ts:10`） | 招商名單的 logo／封面 | Connect `SellerProspectAdminService.UpdateProspect`（`ProspectMediaPanel.tsx:61,134`） | seller 模組：`httpapi/seller/prospect.go:259` → `modules/seller/internal/prospect/images.go` | Go-owner（seller） | measured |
| A3 | `admin/src/app/admin/invoices/jobs/[jobId]/page.tsx:1063` | 發票作業手動解鎖的取消佐證 | REST 手動解鎖請求的 `cancelEvidenceStagedToken`（同檔 `:363`） | gateway `handler/invoice_admin.go:1109`（路由 `router/routes_invoice.go:162`）→ `h.Invoice.RequestManualUnlockForAdmin` → 舊 Go invoice-service | other-legacy-Go（invoice-service，B10） | measured |
| A4 | `admin/src/app/admin/ai-styles/page.tsx:168` | AI 風格縮圖 | REST 建立／更新 AI 風格的 `thumbnailStagedToken`（同檔 `:179,198`） | gateway `handler/ai.go:370` → `h.AI.CreateAiStyle` → 舊 Go ai-service | other-legacy-Go（ai-service） | measured（到 gateway；ai-service 內部靠 grep 命中） |
| A5 | `admin/src/app/admin/violation-reports/cases/[id]/RecordInquiryReplyDialog.tsx:26`（機敏） | 營運代記賣家的詢問回覆附件 | Connect `RecordCaseInquiryReply` 的 `evidence_tokens`（`violation-reports/inquiry-actions.ts:116-129`） | seller 模組：`httpapi/seller/governance_inquiries.go:120` | Go-owner（seller） | measured |

### seller-app（13 列）

| # | 呼叫點 | 功能 | 帶 token 的請求 | 後端消費者 | 種類 | 證據 |
|---|---|---|---|---|---|---|
| S1 | `seller-app/lib/features/seller/presentation/profile_screen.dart:370` | 店面 logo | Connect `UpdateMyStore`，`StoreChanges(logoStagedToken: …)` | seller 模組 `httpapi/seller/app.go:228` | Go-owner（seller） | measured |
| S2 | `profile_screen.dart:402` | 店面封面 | 同上，`coverStagedToken` | 同上 | Go-owner（seller） | measured |
| S3 | `seller-app/lib/features/post/presentation/post_compose_screen.dart:190`（經 `post_repository.dart:189`） | 貼文圖（新增） | Connect `createPost` 的 `imageStagedToken`（`post_repository.dart:166-169`） | social 模組 `httpapi/social/owner.go:161` | Go-owner（social） | measured |
| S4 | `post_edit_screen.dart:158`（經 `post_repository.dart:189`） | 貼文圖（編輯） | 更新貼文的 `imageStagedToken`（`post_repository.dart:365-366`） | social 模組（owner 的更新方法，未逐行開） | Go-owner（social） | measured（請求端）、inferred（handler） |
| S5 | `seller-app/lib/shared/widgets/rich_text_editor.dart:645` | 商品說明內嵌圖 | REST `POST/PUT /api/v1/app/products` 的 `descriptionStagedTokens`（`features/product/data/product_upload_service.dart:87,282,331`） | gateway `seller_product.go:535,675` → Java product-service | Java（product） | measured |
| S6 | `product_upload_service.dart:498` | 商品圖庫（新增） | REST `POST /api/v1/app/products`（`:282`）的 `images[].stagedToken`（`:516`） | 同 S5 | Java（product） | measured |
| S7 | `product_upload_service.dart:626` | 商品圖庫（編輯） | REST `PUT /api/v1/app/products/{id}`（`:331`）（`:633`） | 同 S5 | Java（product） | measured |
| S8 | `seller-app/lib/features/order/presentation/order_card_widgets/order_expanded_section.dart:23`（`order_bottom_row.dart:467`） | 宅配取號附出貨照片 | REST `POST /api/v1/app/shipments/$orderNo/generate_home_label`（`features/order/data/order_repository.dart:356`） | gateway `shipping.go:429` → Java shipping-service | Java（shipping） | measured |
| S9 | 同上（`order_bottom_row.dart:631`） | 超商確認出貨附照片 | REST `POST /api/v1/app/shipments/$orderNo/confirm`（`order_repository.dart:212-220`） | gateway `shipping.go:1307` → Java shipping-service | Java（shipping） | measured |
| S10 | `seller-app/lib/features/chat/presentation/chat_thread_screen.dart:609`（`seller_repository.dart:34`） | 賣家聊天傳圖 | Connect `ChatAppService` 送訊息的 `image_staged_token`（`seller-app` chat notifier `sendImage`） | 單體 `modules/im/internal/write/send.go`：`Send` 的 transaction 先取 request_id 的鎖與重送判定，之後才 `media.ConfirmField`（B1-D26；重送判定必須排在 Confirm 之前，否則並行或重送的同一個 request_id 會撞到 token 已確認，B1-D23） | Go-owner（im） | measured（dev 合併後 2026-11-23，B1-9）|
| S11 | `seller-app/lib/features/seller/presentation/seller_inquiry_reply_screen.dart:189`（機敏） | 回覆平台詢問附件 | Connect `replyToInquiry`（`seller/data/inquiries.dart:188-192`） | seller 模組 `httpapi/seller/inquiries.go:92` | Go-owner（seller） | measured |
| S12 | `seller_measure_request_form_screen.dart:273`（機敏） | 措施申覆／補救佐證 | Connect `submitAppeal`／`requestRemediation`（`seller/data/measures.dart:228-241`） | seller 模組 `httpapi/seller/measures.go:79,105` | Go-owner（seller） | measured |
| S13 | `seller-app/lib/features/seller/data/kyc_wizard.dart:678`（機敏） | 認證證件文件 | Connect `saveKYCDraft`（`kyc_wizard.dart:446`） | seller 模組 `httpapi/seller/kyc.go:64` | Go-owner（seller） | measured |

### buyer-app（8 列）

| # | 呼叫點 | 功能 | 帶 token 的請求 | 後端消費者 | 種類 | 證據 |
|---|---|---|---|---|---|---|
| B1 | `buyer-app/lib/features/settings/presentation/profile_edit_screen.dart:207` | 換頭像 | Connect `UpdateProfile` 的 `avatar_staged_token`（同檔約 `:222` 的 `updateAvatar`） | buyer 模組 `httpapi/buyer/app.go:202` | Go-owner（buyer） | measured |
| B2 | `buyer-app/lib/features/moderation/presentation/store_report_screen.dart:79`（`report_form_parts.dart:53`） | 檢舉店家佐證 | Connect `createReport`（`moderation/data/store_report_repository.dart:64-71`） | seller 模組 `httpapi/seller/reports.go:115` | Go-owner（seller） | measured |
| B3 | `store_report_supplement_screen.dart:68` | 檢舉補充佐證 | Connect `addReportSupplement`（`store_report_repository.dart:95`） | seller 模組 `httpapi/seller/reports.go:168` | Go-owner（seller） | measured |
| B4 | `buyer-app/lib/features/order/presentation/after_widgets/after_sale_screen.dart:133` | 申請售後照片 | REST `POST /app/after_sales`（`order/data/after_sale_repository.dart:34`）`photoTokens` | gateway `aftersales.go:365` → Java aftersales-service | Java（aftersales） | measured |
| B5 | `after_widgets/submit_appeal_screen.dart:87` | 售後申訴佐證 | REST `POST /app/after_sales/:caseNo/appeal`（`after_sale_repository.dart:179`）`evidencePhotoTokens` | gateway `aftersales.go:1107` → Java aftersales-service | Java（aftersales） | measured |
| B6 | `buyer-app/lib/features/order/presentation/confirm_receipt_dialog.dart:100` | 買家確認收貨附照片 | REST `POST /app/orders/{orderNo}/confirm_receipt`（`order/data/order_repository.dart:158`）`stagedTokens` | gateway `handler/order.go:862,865` → `h.Order.ConfirmReceipt` → Java order-service（`OrderGrpcService`） | Java（order） | measured |
| B7 | `buyer-app/lib/features/review/presentation/order_review_screen.dart:184` | 訂單評論圖 | Connect `ReviewAppService.CreateReview` 的 `image_staged_tokens`（`review/data/review_repository.dart`） | 單體 `modules/review/internal/write/write.go` 提交時在 transaction 內 `media.ConfirmFields`（舊 review-service 與 gateway `POST /app/reviews` 已被 dev 的 A6-6 刪除） | Go-owner（review） | measured（dev 合併後 2026-11-23，B1-9）|
| B8 | `buyer-app/lib/features/chat/presentation/chat_thread_screen.dart:1206` | 聊天傳圖 | Connect `ChatAppService` 送訊息的 `image_staged_token`（`buyer-app` chat notifier `sendImage`；送出中的氣泡用本機檔案預覽，B1-D18） | 單體 `modules/im/internal/write/send.go`：`Send` 的 transaction 先取 request_id 的鎖與重送判定，之後才 `media.ConfirmField`（B1-D26；重送判定必須排在 Confirm 之前，否則並行或重送的同一個 request_id 會撞到 token 已確認，B1-D23） | Go-owner（im） | measured（dev 合併後 2026-11-23，B1-9）|

## 二、`secure_download`／`secure_preview` URL 組裝與檔案擁有者

| 位置 | 組出的 URL | 檔案擁有者 | 備註 |
|---|---|---|---|
| `website/lib/invoices/seller-repo.ts:439`（`sellerTaxDocumentHref`，使用者 `SellerTaxStatusPage.tsx:42`） | `/api/v1/app/media/secure_download/{fileAssetId}` | 稅籍證明，invoice（舊 Go invoice-service，B10） | 檔是 W25 上傳 |
| `website/lib/invoices/seller-repo.ts:1068`（`receiptApprovalBasisHref`，使用者 `ReceiptAuthPage.tsx:94`） | 同上 | 收據授權核定文件，invoice | 同上 |
| `backend/internal/httpapi/seller/kyc.go:60-61,263`（`secureDownloadURI`，單體產生，client 直接用 `KYCDocument.download_uri`） | 同上 | 認證證件，seller 模組 | B1-4 的清單沒列這一處（單體端自己組這個 URL）；gateway 的 `secure_download` 刪除時要同步改 |
| `seller-app/lib/features/media/data/media_repository.dart:321`（`getSecureDownloadUrl`） | 同上 | 無 | 沒有呼叫端（`rg getSecureDownloadUrl seller-app buyer-app` 只命中定義），死碼 |
| `media_repository.dart:326`（`getSecurePreviewUrl`） | `…/secure_preview/{sessionId}` | 無 | 同上，沒有呼叫端 |

另外 seller-app 的 `protected_media_controller.dart:19`、`kyc_wizard_widgets.dart:459`、`product_setup_image_screen.dart:209` 走 `protectedMediaRepository.download(url)`，URL 來自 KYC 的 `download_uri`（上表第三列）。

## 三、B1-4 之後本機會失敗的流程（消費者是 Java 服務）

| Java 服務 | 流程 | 列數 |
|---|---|---|
| product | W6、W7（商品說明圖，賣家／代行）、W8、W9（商品圖庫）、W10、W11（代行建立／編輯商品）、S5（App 說明圖）、S6、S7（App 商品新增／編輯圖庫） | 9 |
| shipping | W12（黑貓取號出貨照片）、W13、W14（超商確認出貨）、S8（App 取號）、S9（App 超商確認） | 5 |
| aftersales | W17（申請售後）、W18（補件）、W19（買家申訴）、W20（賣家驗貨照片）、B4、B5 | 6 |
| order | B6（買家確認收貨照片） | 1 |
| 合計 | | 21 |

order 的 client 呼叫點只有 buyer-app 的 B6；website 的確認收貨（`website/lib/orders/repo.ts:1239`）沒有任何呼叫點帶 `stagedTokens`（`OrderDetailClient.tsx` 未見上傳）。

## 四、另需處理：舊 Go 服務的呼叫點（不是 Java，B1-4 後同樣收不下新 token）

| 舊 Go 服務 | 列 | 說明 |
|---|---|---|
| invoice-service | W25、A3 | B10 才搬；B1 計畫書沒寫這兩條流程在 B1-4 後的處置 |
| ai-service | A4 | 計畫書「不做」段提到 ai 等舊 Go 服務的 media client 隨服務目錄刪除，但 AI 風格縮圖這條流程的處置沒寫 |
| imageai 模組（尚未搬進單體，目前只有骨架；dev 的契約 `api/hawkr/admin/imageai/v1alpha1/imageai.proto` 的 `thumbnail_staged_token`） | A4（AI 風格縮圖） | 未來列：模組實作時 `ai_styles.thumbnail_asset_id` 在自己的 transaction 內 `Confirm(…, VisibilityPublic)`，更新與刪除時 `MarkDeleted` 被換掉的縮圖，並以 `media.NewFake()`／UUID 資產 ID 寫測試；在那之前 A4 仍是 ai-service 流程，本機失敗 |
| invoice-service（下載） | 第二節 `seller-repo.ts:439,1068` | 稅籍證明與收據授權核定文件的 `secure_download`：invoice 的檔案面只服務發票 PDF，沒有擁有者端點；B1-4 刪掉組 media 網址的程式，下載本機失敗到 B10（設計審查 2026-11-23） |

## 五、統計

| 維度 | 值 |
|---|---|
| 呼叫點總列數 | 51 |
| website | 25 |
| admin | 5 |
| seller-app | 13 |
| buyer-app | 8 |
| Go-owner | 26（website 10、admin 3、seller-app 8、buyer-app 5） |
| Java | 21（website 13、admin 0、seller-app 5、buyer-app 3） |
| other-legacy-Go | 3（website 1、admin 2；review-service、im-service 被 dev 刪除後，W15、W16、S10、B7、B8 改標 Go-owner，B1-9） |
| none（503） | 1（website W3） |
| 無法判定 | 0 |

Go-owner 26 列依單體模組：

- seller：W2、W21、W22、W23、W24、A2、A5、S1、S2、S11、S12、S13、B2、B3，共 14
- social：W4、W5、S3、S4，共 4
- buyer：W1、B1，共 2
- content：A1，共 1
- review：W15、B7，共 2
- im：W16、S10、B8，共 3

B1 計畫書寫「七個擁有者後端（buyer、content、im、moderation、review、seller、social）」。client 上傳呼叫點直達的單體模組是 seller、social、buyer、content，以及 dev 的 A6-6 把 client 切到單體之後的 review、im（B1-9）。moderation 沒有任何 client 上傳呼叫點直達（檢舉走 seller 模組的 `Reports`；佐證圖片由 moderation 在建快照時 `CopyPublic`，B1-D20）。

## 六、無法驗證或只到推論

- W7：代行商品頁是否掛 DescriptionEditor 沒逐一確認（inferred）。
- S4：賣家 App 貼文編輯對應的單體 handler 沒逐行開，只由請求欄位與 proto 反推。
- Java 端 token 轉正（`FinalizeImages`）靠 grep 命中檔名（`ProductService.java`、`ShippingService.java`、`OrderGrpcService.java`、`EvidenceService.java`），沒有逐行追到。
- 舊 Go 服務（invoice、ai）的轉正細節只開過 client 與 usecase 的命中行。
- website `lib/seller/shipment.ts:205-215` 的 `printCvsLabel` 有 `stagedTokens` 參數，但 gateway 的 `PrintCvsLabelReq` 沒有這個欄位，也沒有呼叫端傳值，屬死參數，不列為呼叫點。
- 沒有跑任何 build、測試或 client；全部是靜態閱讀。
