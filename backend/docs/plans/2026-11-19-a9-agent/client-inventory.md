# A9-0 四個 client 的 AI 助手／採購呼叫點盤點

快照：dev `bc2c732c`（worktree `.worktrees/a9-plan`），只讀。標記：【量】＝跑過指令或打開檔案看過；【推】＝讀程式推論。
縮寫：W=`website`、A=`admin`、BA=`buyer-app/lib`、SA=`seller-app/lib`。路徑都相對 hawkr 根目錄。
範圍是 gateway 上屬於 ai-service 的路由（`backend/gateway/internal/router/routes_ai.go`、`routes_procurement_quote.go`）。
形狀沿用 `backend/docs/plans/2026-11-19-a6-review-im-crm/client-inventory.md`。去向（proto service 名稱）A9 還沒定案，不替它們命名。

## 0. gateway 路由表（核對 client 有沒有打到不存在的路徑）

來源 `backend/gateway/internal/router/routes_ai.go`、`routes_procurement_quote.go`【量】。

| 組 | 路由 | 數 | SSE |
|---|---|---|---|
| 賣家 AI 助手（CapMarketing） | `POST /app/sellers/me/assistant/{chat,orders,inventory,payment,product-listing/chat}`、`POST …/product-autofill`、`GET …/sessions/latest` | 7 | 5 條 SSE（chat、orders、inventory、payment、product-listing/chat） |
| 買家 AI 助手（requireBuyer） | `POST /app/buyers/me/assistant/{orders,tracking,return-policy,seller-match,chat}`、`GET …/sessions/latest` | 6 | 5 條 SSE |
| 買家採購（requireBuyer） | `/app/buyers/me/procurement/` 下 turns、turns/stream、requests/:id 下 snapshot、candidate-searches（POST、GET :runId）、selections、answers、rewinds、rfq-price-previews、rfq-confirmations、rfq-publications（含 /preview）、seller-responses、shortlist（PUT、DELETE）、buyer-quote-acceptances；conversations（GET、GET :id/messages、GET :id/runtime-snapshot、PATCH :id、DELETE :id）；runs/:runId/{events,cancel,resume} | 27 | 2 條 SSE（`turns/stream`、`runs/:runId/events`） |
| 賣家採購（CapMarketing） | `/app/sellers/me/procurement/publications` GET、`…/{id}` GET、`…/response-draft` PUT、`…/seller-responses` POST、`…/withdraw` POST、`…/decline` POST | 6 | 無 |
| AI 圖片 | `GET /app/ai_styles`、`POST /app/image_transformations` | 2 | 無 |
| admin AI | `/admin/ai_styles` GET/POST/PUT/DELETE、`/admin/ai_quotas/:store_id` GET/PUT/DELETE | 7 | 無 |

SSE 合計 5＋5＋2＝12【量：與委派說的 12 條吻合】。`product-autofill` 與 `sessions/latest` 是 unary。
已經是 Connect 的鄰近物：`api/hawkr/agent/v1alpha1/agent.proto` 只有 `VendorPortalPublicService`、`VendorPortalInviteeService`（站外供應商入口，W 經 `website/lib/connect/vendor-portal.ts` 呼叫）【量：`rg -n "^service|rpc" api/hawkr/agent/v1alpha1/agent.proto`】。其餘 AI／採購沒有任何 proto。

## 1. 指令與命中

排除項：`--glob '!node_modules' --glob '!.next' --glob '!**/*.md' --glob '!**/gen/**' --glob '!**/*.g.dart' --glob '!**/*.freezed.dart'`；測試另帶 `--glob '!**/*test*' --glob '!**/*spec*'`。

| # | 指令（rg -n，省略排除項） | 結果 |
|---|---|---|
| W1 | `rg -n "/(buyers\|sellers)/me/(assistant\|procurement)\|image_transformations\|ai/styles\|ai_styles" website/lib website/features website/components website/app` | 買家採購約 21 行（見 §2.1）、seller-match 2 行、賣家 product-listing 2 行、AI 圖 3 檔 |
| W2 | `rg -n "sellers/me/procurement\|me/procurement/publications" website`（排除 md、`openapi-types.ts`） | 0 檔【量】：website 沒有賣家採購畫面 |
| W3 | `rg -n "product-autofill\|sellers/me/assistant" website/lib website/features` | 只有 `ai-listing-chat.ts` 的 product-listing/chat；autofill、賣家 chat/orders/inventory/payment 都沒有 |
| W4 | `rg -n "@ag-ui\|copilotkit\|assistant-ui" website/package.json website/lib website/features` | `@ag-ui/core` 1.0.2（`package.json:43`）；CopilotKit 0；assistant-ui 只在 `capabilities.ts` 的註解 |
| A1 | `rg -n -i "assistant\|procurement\|ai_styles\|ai_quotas\|ag-ui" admin/src`（排除 `generated.ts`） | 只有 `app/admin/ai-styles/page.tsx`（8 個請求行）與 `Sidebar.tsx:180`；採購、助手 0 |
| B1 | `rg -n "assistant\|procurement\|agui" buyer-app/lib` | `features/ai_assistant/`（4 檔）、`core/networking/agui_sse.dart`、`chat_list_screen.dart`、router 與 `route_access_policy.dart`；procurement 只在 `code_meta.dart` 生成物，0 個手寫檔【量】 |
| S1 | `rg -n "assistant\|procurement\|agui\|image_transformations\|ai_styles" seller-app/lib` | `features/ai_assistant/`、`features/product/…product_listing_chat*`、`features/procurement/`（5 檔）、`features/photo_editor/data/ai_*`、`core/networking/agui_sse.dart`、router |
| P1 | `rg -il "ag-ui\|agui\|assistant\|procurement" packages`（排除 gen） | `packages/hawkr-api-ts/src/gen/hawkr/admin/seller/v1alpha1/prospect_pb.ts` 與 `hawkr_ui_text` 的 enum manifest；沒有任何手寫 AI 呼叫【量】 |

### 各端摘要

| 端 | 不重複的 REST／SSE 請求（method＋path） | 備註 |
|---|---|---|
| website | 買家採購 unary 約 20＋SSE 3（turns/stream、runs/events、seller-match）＋賣家 product-listing SSE 1（旗標關）＋AI 圖 2 | `HawkrAgentChatWorkspace` 已無路由（§5） |
| admin | AI 風格與額度 7（`/admin/ai_styles`、`/admin/ai_quotas`） | 沒有採購、助手 |
| buyer-app | 買家助手 SSE 4（chat、orders、tracking、return-policy）＋`sessions/latest` 1 | 沒有 seller-match、沒有任何採購、沒有圖片 AI |
| seller-app | 賣家助手 SSE 4（chat、orders、inventory、payment）＋product-listing SSE 1＋`sessions/latest` 1＋賣家採購 6＋AI 圖 2 | 沒有 `product-autofill` |

## 2. AG-UI 用法

### 2.1 套件與解析方式【量】

| 端 | 函式庫 | wire 怎麼吃 |
|---|---|---|
| website | `@ag-ui/core@1.0.2`（`website/package.json:43`、`pnpm-lock.yaml:718`）只拿來做 `EventSchemas.safeParse` 與 `RunFinishedOutcome` 型別（`website/lib/ai-chat/agui-dispatch.ts:13-14`）；沒有 `@ag-ui/client`、沒有 CopilotKit | 手寫：`fetch` ＋ `res.body.getReader()` ＋ `TextDecoder`，自己切 frame、併 `data:` 行（`website/lib/ai-chat/stream-client.ts`，POST 通道）；GET 通道用 `website/lib/sse/client.ts` 的 `SseClient`（fetch＋ReadableStream，不用 EventSource） |
| buyer-app | 無 AG-UI 套件（`pubspec.yaml` 只有 `dio`） | 手寫 `AgUiSseDecoder`（`buyer-app/lib/core/networking/agui_sse.dart`，217 行）；`dio` `ResponseType.stream` |
| seller-app | 同上 | 同檔複本（`seller-app/lib/core/networking/agui_sse.dart`，217 行，與 buyer-app 只差一行註解，`diff` 只有 `:5`） |

結論：**三端都直接吃 AG-UI wire，沒有 AG-UI client 函式庫**；website 的 `@ag-ui/core` 只是 schema 驗證。AG-UI 的 `@ag-ui/core` 1.0.2 與程式註解不一致：註解多處寫 `0.0.58`（`agui-dispatch.ts:106`、`stream-client.unit.test.ts:452`）【推：註解過期，不影響行為】。

frame 形狀：只有 `data:`（POST 通道），或 `id:`＋`data:`（`runs/:runId/events` 重播通道，`id` 是十進位 ledger seq），**沒有 `event:` 行**；事件類型靠 JSON `type`（`stream-client.ts` 檔頭、`run-events-stream.ts:19-22`）。

### 2.2 消費的事件型別【量】

| 端 | 檔案:行 | 處理的 `type` |
|---|---|---|
| website | `lib/ai-chat/agui-dispatch.ts:91-145` | `TEXT_MESSAGE_CONTENT`、`RUN_FINISHED`（含 `outcome.interrupt`）、`RUN_ERROR`、`TOOL_CALL_START`（記 `toolCallId→toolCallName`）、`TOOL_CALL_RESULT`（轉成以工具名為名的 named event）、`CUSTOM`（轉 `onNamedEvent(name, JSON(value))`）；其餘 `default: break` |
| website 採購 | `features/procurement-assistant/turn-stream.ts:7-11`、`run-events-stream.ts:134-144` | CUSTOM `step_detail`、`activity`、`artifact`（gateway 把 `ARTIFACT_UPSERTED` 投影成 CUSTOM `name="artifact"`）；`RUN_FINISHED.result` 帶完整 `TurnResult`；CUSTOM `run_status`、`confirmation` 目前忽略 |
| buyer-app／seller-app | `agui_sse.dart:129-182` | `TEXT_MESSAGE_CONTENT`、`RUN_FINISHED`、`RUN_ERROR`、`TOOL_CALL_START`、`TOOL_CALL_RESULT`、`CUSTOM`；轉成 App 內部事件 `text/done/error/tool_result/draft_patch/step_detail`，終止看 `terminal` 欄位不看 name（`agui_sse.dart:6-20`） |

### 2.3 重連、取消、逾時【量】

| 通道 | 行為 | 檔案:行 |
|---|---|---|
| W POST SSE（seller-match、product-listing、`turns/stream`） | **不重連**；`AbortSignal` 由呼叫端傳入；30 秒讀不到 chunk 丟 `AiStreamTimeoutError`（對齊 gateway `aiStreamChunkTimeout`）；開場 non-2xx 丟 `AiStreamHttpError`，404／405／429 時 `turns/stream` 降級走同步 `POST /turns` | `lib/ai-chat/stream-client.ts:31-60,155`；`turn-stream.ts:224,356-360` |
| W GET SSE（`runs/:runId/events`） | `SseClient` 指數退避重連、帶 `Last-Event-ID` cursor、heartbeat watchdog 35 秒（gateway 每 20 秒 `: ping`）、429／503 停止重試；**伺服器不會在 run 終態關連線**，只有 gateway 120 秒 `streamTurnDeadline` 會收，呼叫端必須自己 `close()`；prod 的 `PROCUREMENT_REPLAY_SSE_ENABLED` 當時是關的，固定 503（註解自述） | `run-events-stream.ts:10-27`；`useRunActivityStream.ts:43` |
| W 取消 run | unary `POST /runs/{runId}/cancel`（`conversations.ts:233-262`）與 `POST /runs/{interruptId}/resume`（`useProcurementSession.ts:918`）；session hook 另用 `operation.signal` abort 本地 fetch（`useProcurementSession.ts:1154,1177,1416,1532`） | |
| W 同源反代 | `app/api/v1/[...path]/route.ts:64,130,316` 對 `text/event-stream` 做特例（不快取、不緩衝） | |
| BA／SA SSE | **不重連**；`receiveTimeout` 3 分鐘（助手，`ai_assistant_repository.dart:84`）或 `Duration.zero` 無上限（SA product-listing，`product_listing_chat_repository.dart:39`）；取消只有 controller 取消訂閱（`ai_assistant_controller.dart:15,128`），**沒有 `CancelToken`**【量：`rg -n CancelToken` 在這三檔 0 筆】；non-200 丟 `Exception('AI service error: …')`，停權 403 走 `isRestSuspension` | |

### 2.4 tool-call 呈現【量】【推】

- W seller-match：`TOOL_CALL_RESULT` 以工具名 `tool_result` 轉出，`{type:"seller_candidates",candidates:[…]}` 逐次累加（`lib/buyer/seller-match-chat.ts:7-8,68-70`）；畫面是已退場的 `HawkrAgentChatWorkspace`（§5）。
- W 採購：不渲染通用 tool call；以 typed CUSTOM／artifact 驅動 `AgentActivityRail`、`CandidateCanvas`、`BuyerQuoteComparison` 等專用元件【推：由 §2.2 的事件清單與 `features/procurement-assistant/` 檔名推論】。
- BA／SA：`tool_result` 事件進 `ai_message_model.dart`；`assistant_tool_result.sse` fixture 是唯一覆蓋（見 §3.3）。

## 3. 手寫型別與 client（Connect 生成後會被取代的）、測試

### 3.1 website（全部手寫，無生成碼）

| 檔案 | 內容 | 行數 |
|---|---|---|
| `website/lib/ai-chat/stream-client.ts`、`agui-dispatch.ts` | 通用 SSE 傳輸＋AG-UI 分派 | 234＋157【量：`wc -l`】 |
| `website/lib/buyer/seller-match-chat.ts` | seller-match SSE（死碼，§5） | 151 |
| `website/lib/seller/ai-listing-chat.ts` | product-listing SSE＋`AiDraftPatch` 型別（被 `useProductForm.ts:31`、`productFormAdapter.ts:34` 引用） | 107 |
| `website/features/procurement-assistant/turn-stream.ts` | `turns/stream` 客戶端 | 429 |
| `…/run-events-stream.ts`、`useRunActivityStream.ts` | `runs/:runId/events` | 222＋ |
| `…/conversations.ts`、`useAgentConversations.ts` | conversations 列表、messages、rename、delete、cancel | 271＋ |
| `…/useProcurementSession.ts` | 採購主 hook，直接 `apiClient.authed` 呼叫 turns、resume、snapshot、answers、rewinds、candidate-searches、selections、rfq-price-previews、rfq-confirmations、rfq-publications(/preview)；有手寫 `normalizeCropDisambiguation`（`:267`） | 2366 |
| `…/useBuyerQuoteComparison.ts`、`useRfqRuntimeSnapshot.ts` | seller-responses、shortlist PUT／DELETE（`:212`）、quote acceptance | — |
| `…/*-contract.ts`、`runtime-snapshot-contract.ts`、`types.ts`、`activity.ts` | 手寫 zod／型別（`buyer-quote-contract.ts`、`rfq-price-contract.ts`、`rfq-publication-contract.ts`） | — |
| `website/features/procurement-rfq-demo/load.ts:72,121,159,172` | demo 路由 `hawkr-agent-rfq-demo` 的 4 個採購請求 | — |
| `website/lib/seller/ai-image.ts` | `ai_styles`、`image_transformations`（`:267,364`；含內建風格 fallback 清單） | — |
| `website/lib/api/openapi-types.ts` | 舊 OpenAPI 型別（含採購路徑，非 AI 專用，【推】隨 A9 搬走刪對應段） | — |

website 已有的 Connect：只有 `lib/connect/vendor-portal.ts`（`VendorPortalPublicService`、`VendorPortalInviteeService`）；`lib/vendor-portal/repo.ts` 是其包裝。

### 3.2 admin、buyer-app、seller-app、packages

| 端 | 手寫檔 | 備註 |
|---|---|---|
| admin | `admin/src/app/admin/ai-styles/page.tsx`：頁面內直接 `api.get/put/post/delete("/api/v1/admin/ai_styles…")` 與 `ai_quotas`（`:99,181,200,219,228,244,267,284`），型別 `ListResponse`、`QuotaResponse` 內聯 | 無獨立 repo 層 |
| buyer-app | `ai_assistant_repository.dart`（`BuyerAssistantEndpoint` enum 4 路徑、`sessions/latest:137`）、`ai_message_model.dart`、`agui_sse.dart` | 與 SA 的 `ai_assistant_repository.dart` 結構相同，各一份 |
| seller-app | `ai_assistant_repository.dart`（`SellerAssistantEndpoint` 4 路徑）、`product_listing_chat_repository.dart`＋`product_listing_model.dart`、`procurement/data/seller_quote_repository.dart`（`basePath` ＋ 6 個請求）＋`seller_quote_model.dart`＋`procurement_error.dart`、`photo_editor/data/ai_image_service.dart`（`:127,195`）＋`ai_style_model.dart` | procurement 與採購共用的 `ErrorReason` 來自 `core/codemeta/code_meta.dart`（生成物，CodeMeta 凍結） |
| packages | 0 | `hawkr_ui_text`、`hawkr-ui-text-ts` 的 enum manifest 含 ai-service 的 StepCode／artifact kind（`code_meta.dart:1161,3389` 註解指 `services/ai-service/internal/procurement/domain/steps.go`）；A9 改契約時要同步 |

### 3.3 mock 與測試【量：`rg -l` 與 `ls`】

| 端 | 單元／元件測試 | e2e／fixture |
|---|---|---|
| website | `lib/ai-chat/*.unit.test.ts`（stream-client 823 行、agui-dispatch）、`lib/seller/ai-listing-chat.unit.test.ts`、`ai-image-quota.unit.test.ts`、`lib/buyer/seller-match-chat.unit.test.ts`、`features/procurement-assistant/*.unit.test.ts`、`*.browser.test.tsx`；`website/tests/` 下 `hawkr-agent-*`／`procurement-assistant-*` 共 25 個；`ai-chat/__tests__/agui-fixtures.unit.test.ts` 驗 AG-UI 信封 | Playwright 12 個 spec（`hawkr-agent-*`、`procurement-*`、`vendor-rfq-landing`），mock 在 `tests/e2e/helpers/mock-procurement-rfq.ts`（18 處 `route(`）；`hawkr-agent-live.spec.ts` 是對真後端的 live 版 |
| admin | 0（`admin/tests/codemeta-no-second-copy.test.ts` 只擋 CodeMeta 複本） | 0 |
| buyer-app | `test/features/ai_assistant/` 5 檔（controller terminal、lifecycle、error_event、chat_parity、repository_agui）、`test/core/networking/agui_sse_decoder_test.dart`、`suspension_test.dart`、`chat_list_widgets_test.dart`、`social_connect_guard_test.dart` | `test/fixtures/agui/` 4 個 `.sse`（`assistant_text_done`、`assistant_tool_result`、`assistant_keepalive_ping`、`procurement_keepalive_steps`） |
| seller-app | `test/features/ai_assistant/` 5 檔、`test/features/product/product_listing_chat_*` 4 檔、`test/features/procurement/` 7 檔、`test/features/photo_editor/` 2 檔、`agui_sse_decoder_test.dart`、`app_route_inventory_test.dart`、`chat_icon_policy_test.dart` | `test/fixtures/agui/` 5 個 `.sse`（多 `listing_draft_patch`） |

注意：兩支 App 的 `procurement_keepalive_steps.sse` fixture 在 BA 沒有任何採購程式、SA 也沒有採購 SSE 呼叫，只是 decoder 的 CUSTOM 樣本【推】。

## 4. 已死或壞掉的畫面／功能

| # | 項目 | file:line | 狀態 |
|---|---|---|---|
| 1 | W `HawkrAgentChatWorkspace`（seller-match 對話）與 `streamSellerMatchChat` | `website/features/procurement-assistant/HawkrAgentChatWorkspace.tsx:160`；`website/lib/buyer/seller-match-chat.ts` | **無路由、無 import**：只被 `tests/hawkr-agent-entry-contract.test.ts:328-331` 斷言「路由不 import 它」，`hawkr-agent-live.spec.ts:16` 註解寫「依 §V.14.4-2 先留著」；knip baseline 也列它（`website/docs/plans/2026-11-15-remove-storybook/knip-baseline.json:2669`）【量】。seller-match 這條 gateway 路由的 web 消費者因此為 0 |
| 2 | W 賣家 AI 刊登助手 | `website/features/seller/components/AiAssistantPanel.tsx:12`（`AI_LISTING_ASSISTANT_ENABLED = false`）；`features/seller-center/ai/AiPage.tsx:27`、`ProductFormPage.tsx:171,191` | 編譯期常數關，程式在但永遠不渲染；`stream-client.ts` 檔頭自稱「feature-flag 關閉但是真的在跑」，與現況不符【推：檔頭過期】 |
| 3 | W 整個 Hawkr Agent | `website/lib/config/env.ts:199-206`（`NEXT_PUBLIC_HAWKR_AGENT_ENABLED`，未設時只有 dev flavor 為 true）；`.env.example:19` 為 true | build-time 閘，prod 預設關；`capabilities.ts` 另有 4 支 `WEB_PROCUREMENT_*` 旗標預設全 OFF（`:30-37`）。**是否在 prod 開著 website 內查不到**【未量：看 Vercel 設定，不在 repo】 |
| 4 | SA 賣家助手入口 | `seller-app/lib/features/chat/presentation/chat_list_screen.dart:455`（`kAiAssistantChatEntryEnabled = false`）；`router` `app_router.dart:533` 仍註冊 `/chat/ai-assistant` | 入口隱藏，route 仍可 deep link；整組 4 條 SSE 沒有 UI 入口【量】 |
| 5 | SA 賣家刊登助手 | `seller-app/lib/features/product/presentation/ai_assistant_button.dart:10`（`kAiProductListingAssistantEnabled = false`，四個 setup 畫面都以它擋 `AiAssistantButton`） | 入口隱藏；`product-listing/chat` SSE 無 UI 入口 |
| 6 | BA 買家助手 | `buyer-app/lib/features/chat/presentation/chat_list_screen.dart:209,224`：FAB 只在「已載入且有對話」時顯示，空列表走 `onStartAssistant:334` | 活著，無旗標【量】 |
| 7 | gateway 有、client 沒人打 | `POST /sellers/me/assistant/product-autofill`；W 與 SA 都沒有 `product-autofill` 字串【量：`rg -n product-autofill website buyer-app/lib seller-app/lib` 0】；買家 `seller-match` 只剩 #1 的死碼 | 候選刪除，待 A9 決定 |
| 8 | pricebook／行情殘留（ADR D2：`docs/adr/2026-10-27-general-ecommerce-scope-cut.md:34`，採購只解析到分類樹、不對作物與行情） | W `features/procurement-assistant/` 的 `CropDisambiguationCard.tsx`、`types.ts` 的 `CropDisambiguation`、`steps.ts:71,115` 的 `CROP_GATE`「核對品項分類」、`useProcurementSession.ts:267,334` | 「crop」命名仍在 UI 與型別；ai-service 仍有 `SlotCropDisambiguation`（`backend/services/ai-service/internal/procurement/domain/pick_next_question.go:33`）與 `procurement_handler.go:711-718` 驗證，所以是**連線的活碼，不是孤兒**，只是名稱與語意殘留【推：要不要改名是 A9 的設計決定】 |
| 9 | 行情字樣（非採購） | `website/features/seller/landing/sellerFeatureOverviewData.ts:130,135`（「農業部批發市場實際行情、條件式七日預測」）；`website/components/shared/SellerNavbar.tsx:164` 註解、`styles/features/seller-center.css:11003` | 與 AI 無關，屬 pricebook 移除後的文案殘留，順帶記錄 |
| 10 | 舊 CodeMeta 複本 | `admin/src/lib/codemeta/generated.ts`、`buyer-app/lib/core/codemeta/code_meta.dart`、`seller-app/lib/core/codemeta/code_meta.dart` 含 ai-service 的 StepCode／ErrorReason | CodeMeta 凍結，隨模組搬遷退場；A9 搬 ai-service 時要處理其中 ai-service 的條目 |

沒有找到 client 打到 gateway 已不存在的 AI 路由【量：W、BA、SA 的每個請求路徑都逐一對到 §0 的路由表】。

## 5. 各功能的使用角色

| 功能 | buyer | seller | admin | 依據 |
|---|---|---|---|---|
| 買家助手 chat／orders／tracking／return-policy | BA（活）；W 無 | – | – | `requireBuyer()`（`routes_ai.go`） |
| 買家 seller-match | W 死碼 | – | – | §4 #1 |
| 買家採購（turns、candidate-searches、rfq、shortlist、quote acceptance、conversations、runs） | W（受 Hawkr Agent 閘控）；BA 沒有 | – | – | `requireBuyer()`；W `features/procurement-assistant/` |
| 賣家助手 chat／orders／inventory／payment | – | SA（入口隱藏） | – | `sellerHandle … CapMarketing` |
| 賣家 product-listing 助手 | – | SA、W 皆被旗標關掉 | – | §4 #2、#5 |
| 賣家採購回價（publications、response-draft、seller-responses、withdraw、decline） | – | SA 活（`profile_other_options_card.dart:57` 進入）；W 沒有；另有站外供應商走 `VendorPortal*Service`（Connect，無站內帳號） | – | `routes_procurement_quote.go`；`route_capabilities.dart:94-95` |
| AI 圖片轉換 | – | W（`AiImageEnhanceDialog` 被 `ProductImageUploader.tsx:194`、`PostComposeClient.tsx:723` 使用）、SA（`photo_editor`） | – | `lib/seller/ai-image.ts` |
| AI 風格與額度管理 | – | – | A（`ai-styles/page.tsx`，Sidebar「AI 修圖管理」） | `routes_ai.go` admin 群組 |

## 6. 給 A9 計畫的幾點（【推】）

1. 12 條 SSE 的實際消費者：W 3 條（`turns/stream`、`runs/:runId/events`、死的 seller-match）＋1 條旗標關（product-listing）；BA 4 條；SA 5 條（4 助手＋1 刊登，入口全隱藏）。**有活 UI 的 SSE 只有 BA 助手 4 條、W 採購 2 條**；其餘是死碼或隱藏入口，遷移前可先決定刪還是保留。
2. 三端都手寫 AG-UI 解析，沒有函式庫：換 Connect server-streaming 時 W 的 `stream-client.ts`／`agui-dispatch.ts`／`turn-stream.ts`／`run-events-stream.ts` 與兩支 App 的 `agui_sse.dart` 都是替換對象；BA、SA 的 `agui_sse.dart` 是逐字複本，且 decoder 以 `terminal` 欄位判終止的設計要保留語意。
3. `runs/:runId/events` 是唯一帶 replay cursor（`Last-Event-ID`）的通道，Connect 對應要有 cursor 欄位；其餘 POST 通道目前不重連，換 Connect 後不必新增重連行為除非 A9 要。
4. 採購 unary 約 27 條幾乎全由 W 的 `useProcurementSession.ts`（2366 行）等手寫；BA 沒有採購，SA 只有賣家 6 條。
5. 該計畫的 ADR D2 殘留：`crop` 命名與 `SlotCropDisambiguation` 仍活，不是死碼。
