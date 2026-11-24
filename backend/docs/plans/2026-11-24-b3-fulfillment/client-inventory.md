# B3-0 client 盤點：誰在用出貨與物流（唯讀，快照 dev `496b048e2`）

標記：【量】＝跑過指令；【推】＝讀程式推論。路徑縮寫：`W=website`、`A=admin`、`BA=buyer-app/lib`、`SA=seller-app/lib`、`BAT=buyer-app/test`、`SAT=seller-app/test`。所有指令從 `/Users/will/second-brain/.projects/personal/hawkr` 執行，排除 `node_modules`、`.next`、`build`、`.dart_tool`、`*.g.dart`、`*.freezed.dart`、`gen/`。

依據：ADR `docs/adr/2026-11-15-fulfillment-seller-managed-shipping.md`（D1 自寄、D2 固定運費規則、D3 保留物流 port 與模擬 adapter、D4 不搬超商與宅配建單／門市地圖／追蹤號碼自動查詢／即時運費；「UI 流程改動先取得同意」）。「分類」欄全部是**建議**，由 Will 與後續決策簡報定案；「需 UI 同意?」＝該列的畫面或流程改動是否屬於 UI 流程改動。

重要前提【推】：目前**沒有任何 client 有「填物流業者＋追蹤號碼」的畫面**（`rg -il 'trackingNumber|tracking_no|追蹤號|carrierName'` 在四個 client 的非 codemeta、非生成檔裡只命中發票載具與 `TrackingClient` 的文案，沒有輸入欄位）。所以「改成自寄流程」的列，實際上都是新畫面，不是改造舊畫面。配送方式碼 `3005`（商家自送）在各 client 是被當成「不支援」擋掉的值（`shipping_presentation.dart`、`shipping-requirements.ts`）。

## 0. 產生數字的指令與命中數

| # | 指令 | 命中 | 說明 |
|---|---|---|---|
| C1 | `rg -l -f <p2> <client> -g '!*.md' -g '!*.json' -g '!*.lock' -g '!*.svg' -g '!**/codemeta/**' -g '!**/ui-text-ts/**' -g '!**/hawkr_ui_text/**'`，`<p2>` = `shipment\|logistics\|shipping[_A-Za-z]\|Shipping[A-Z]\|store_?pickup\|store_?map\|StorePickup\|StoreMap\|tcat\|taqbin\|seven_eleven\|deliveryTemp\|storeMapSession` | website 161 檔（其中路徑含 test／spec 64）、admin 19（7）、buyer-app 83（34；含 1 個 `ios/Runner.xcodeproj/project.pbxproj` 雜訊）、seller-app 105（47）、packages 0 | 【量】提到字樣的檔數，**含只是順帶提到的檔**（例如 `CouponSelectionSheet.tsx` 只因運費門檻文案命中）；表格列的是逐檔看過後真正有邏輯的核心檔，不等於此數 |
| C2 | 同 C1 但 `-c` 且不排除 codemeta | admin `codemeta/generated.ts` 102 行、website `codemeta/generated.ts` 97 行、buyer-app `code_meta.dart` 105、seller-app `code_meta.dart` 105 | 【量】CodeMeta 生成物，凍結且隨模組搬遷退場，不手改（根 `AGENTS.md` 契約表） |
| C3 | `rg -n -i '\buber\b\|uber[ _-]?direct\|\b3003\b'`（client 全域，排除 json） | 非 codemeta 的 Uber／3003 命中：`SA/core/codemeta/shipping_presentation.dart`、`SAT/core/codemeta/shipping_presentation_test.dart`、`SAT/features/order/shipping_provider_dispatch_test.dart`、`SA/features/order/presentation/order_card_widgets/order_bottom_row.dart`、`SA/features/order/presentation/after_sales_resolve_sheet.dart`、`BA/features/cart/presentation/widgets/shipping_section.dart`、`A/tests/shipping-provider-no-fallback.test.ts`；另 `packages/hawkr_auth/lib/sdk.dart` 的 `3003` 是 LINE SDK 取消錯誤碼，**與 Uber 無關** | 【量】Uber 即時配送的 UI 早已刪（`shipping_presentation.dart` 註明「功能已刪除（ADR 2026-10-27-general-ecommerce-scope-cut D4）」）；殘留的只有「3003／3005 不得落到黑貓或 7-11」的回歸測試與 codemeta 生成值 |
| C4 | `rg -o -N --no-filename -f <pat>`，`<pat>` = 出貨相關 REST 路徑字面值，各 client 排除 test | `shipments/anomalies` 27、`print_cvs_label` 17、`store_pickups` 14、`shipments/{order_no}` 13、`call_tcat` 13、`shipping_settings` 12、`shipping_onboardings` 10、`generate_home_label` 10、`/logistics` 10、`checkout/preview` 8、`store_map_sessions` 7、`shipping_business_days` 7、`checkout/validate` 6、`checkout/shipping_fee` 5 | 【量】出現次數（含註解與型別）；實際呼叫點見 §1-§4 |
| C5 | `rg -n --no-filename -o '/app/shipping_quotes\|/app/shipping_methods\|/app/shipping_onboardings\|/app/shipping_business_days' website seller-app/lib buyer-app/lib` | `shipping_quotes` 與 `shipping_methods` 只在商品設定（W `SkuShippingEditor`／`products/seller-repo.ts`、SA `product_repository.dart`） | 【量】商品設定的運送方式與報價是另一條線，見 §5 |
| C6 | `rg -l '<出貨 repo 檔>' website -g '!*test*'`（`seller/shipment`、`seller/shipping`、`shipping/anomalies`、`checkout/store-pickups`、`logistics/repo`、各 hook） | 22 個非 test 檔（`AddressesClient`、`TrackingClient`、`CartClient`、`SellerGroupCard`、`StorePickupPickerSheet`、`SellerOrdersPage`、`OrderDetailClient`、`DashboardPage`、`ShippingClient`、`SkuShippingEditor`、`ShipmentDialogs` 等） | 【量】website 端 REST 出貨程式的消費者清單 |
| C7 | `rg -c -i 'deliveryTemp\|delivery_temp\|DeliveryTemp\|deliveryTemperature\|temperature_zone\|TemperatureZone' -g '!*.json' -g '!*.md'`（client 全域） | website 14 檔、buyer-app 8 檔、seller-app 0 檔、admin 1 檔（`admin-avatar-contract.test.ts`，僅頭像測試的字樣，與配送溫層無關） | 【量】溫層由 B4／B6 擁有，見 §7 |
| C8 | `rg -n -i 'shipping\|shipment\|logistic' api/hawkr --glob '*.proto'` | 4 行：`notification.proto` 的 `NOTIFICATION_CATEGORY_SHIPMENT`／`SHIPMENT_ANOMALY`、`seller.proto` 的 `SELLER_CAPABILITY_SHIPPING_SETUP` 與 `pending_shipment_count`、`procurement.proto` 的 `shipping_amount`（詢價報價，非履約） | 【量】Connect 契約目前沒有任何履約 RPC；出貨全走舊 REST |

## 1. 買家：結帳的配送方式與門市

| 檔案／畫面 | 呼叫的端點 | 功能 | 分類（建議） | 需 UI 同意? | 測試檔 |
|---|---|---|---|---|---|
| `W/features/cart/components/ShippingMethodGroup.tsx`（275 行） | 無自己的呼叫；資料來自 `POST /app/checkout/validate` | 購物車每個賣家分組的配送方式選擇（黑貓宅配／7-11 超取、預計到貨日、運費由誰付） | 改成自寄流程：只剩賣家自訂的固定運費規則，無配送方式分組 | 是 | `W/features/cart/components/Cart.browser.test.tsx`、`W/tests/e2e/cart-seller-selection.spec.ts` |
| `W/features/cart/components/StorePickupPickerSheet.tsx`（439 行） | `POST /app/checkout/validate`、經 `store-pickups.ts` 的 `store_pickups` 與 `store_map_sessions` | 選取貨門市（開 7-11 電子地圖彈窗、已存門市清單、新增、編輯） | 刪除 | 是（刪除一個結帳步驟） | 無專屬測試；`W/features/cart/components/Cart.browser.test.tsx` 間接 |
| `W/lib/checkout/store-pickups.ts`、`W/lib/hooks/useStorePickups.ts` | `POST /app/checkout/preview`、`GET/POST /app/store_pickups`、`PUT/DELETE /app/store_pickups/{id}`、`POST /app/store_map_sessions` | 取貨門市 CRUD、電子地圖 session、`openStoreMapPopup` | 刪除 | 否（隨上列） | 無專屬 unit test；【量】`rg -l store-pickups website -g '*test*'` 命中 `W/features/cart/components/Cart.browser.test.tsx`、`W/features/seller-center/profile/shipping/ShippingClient.unit.test.ts`、`W/tests/seller-link-canonical.test.ts`、`W/tests/query-keys-csp.test.ts`（後兩者為路徑或 query key 引用） |
| `W/app/(commerce)/store-map/return/route.ts`（205 行） | 被 PayUni 的 `MapReturnURL` 回呼（後端 302 到此） | 電子地圖選完後 `postMessage` 回 opener | 刪除 | 否 | `W/tests/store-map-return-route.test.ts` |
| `W/features/cart/CartClient.tsx`、`W/features/cart/cartUiStore.ts`、`W/features/cart/components/SellerGroupCard.tsx` | `POST /app/checkout/validate`（經 `useShippingOptions`） | 購物車的配送方式選取、取貨門市與收件地址齊備判斷、`3005` 提示、運費門檻文案 | 改成自寄流程 | 是 | `W/tests/shipping-switch.test.ts`、`W/tests/checkout-free-shipping.test.ts`、`W/features/cart/components/Cart.browser.test.tsx` |
| `W/features/checkout/screens/CheckoutClient.tsx` | `POST /app/checkout/shipping_fee`（宅配精算）、`POST /app/checkout/preview` | 結帳頁的配送摘要、宅配運費精算 | 改成自寄流程（運費改讀固定規則，不呼叫精算） | 是 | `W/tests/checkout-repo.test.ts`、`W/lib/checkout/repo.unit.test.ts`、`W/tests/checkout-draft.test.ts` |
| `W/lib/checkout/repo.ts`、`W/lib/checkout/draft.ts`、`W/lib/checkout/shipping-requirements.ts`、`W/lib/checkout/free-shipping.ts` | 同上 | 配送選項型別、`deliveryTemp` 欄位、「這個配送方式需要什麼收件目標」的 fail-closed 判準、免運門檻 | 改成自寄流程；`free-shipping.ts` 對應固定規則可保留 | 否（內部契約，隨畫面） | `W/lib/checkout/shipping-requirements.unit.test.ts`、`W/lib/checkout/draft.unit.test.ts`、`W/tests/checkout-free-shipping.test.ts` |
| `W/lib/hooks/useShippingOptions.ts` | `POST /app/checkout/validate` | 取得每個賣家可選配送選項 | 改成自寄流程 | 否 | 同 `Cart.browser.test.tsx` |
| `W/features/account/screens/addresses/AddressesClient.tsx` | `GET /app/store_pickups`（經 hook） | 地址簿頁同時列「取貨門市」分頁 | 改成自寄流程：移除門市分頁，地址簿保留 | 是 | 無專屬測試 |
| `BA/features/cart/presentation/widgets/shipping_section.dart`（808 行） | 無自己的呼叫（吃 cart state） | 結帳的可折疊配送選項、收件地址卡、7-11 門市清單、「新增 7-11 門市」、預計工作天 | 改成自寄流程 | 是 | `BAT/features/cart/cart_shipping_selection_test.dart`、`BAT/features/cart/shipping_unsupported_filter_test.dart`、`BAT/features/cart/checkout_payment_sheet_layout_test.dart`、`BAT/features/cart/cart_surface_rwd_test.dart` |
| `BA/features/cart/presentation/store_map_webview_screen.dart`（74 行） | 透過 `POST /app/store_map_sessions` 取得的 URL | WebView 內的 7-11 電子地圖，deep link `hawkrbuyer://` 回傳 | 刪除 | 是（隨門市選擇一起刪） | 無專屬測試 |
| `BA/features/cart/presentation/store_pickup_form_screen.dart`（549 行） | 經 repository 打 `/app/store_pickups` | 新增、編輯取貨門市表單 | 刪除 | 是 | `BAT/features/address/store_pickup_form_sheet_test.dart` |
| `BA/features/address/data/store_pickup_model.dart`、`store_pickup_repository.dart`、`presentation/store_pickup_controller.dart`、`presentation/widgets/address_selection_widgets.dart`、`presentation/address_management_screen.dart` | `GET/POST /app/store_pickups`、`DELETE`、`POST /app/store_map_sessions` | 取貨門市資料層與在地址管理頁的門市分頁 | 刪除（地址管理頁其餘保留） | 是（地址管理頁移除分頁） | `BAT/features/address/store_pickup_model_test.dart`、`BAT/features/address/address_selection_widgets_test.dart` |
| `BA/features/cart/data/cart_repository.dart`、`cart_data.dart`、`presentation/cart_controller.dart`、`cart_state.dart`、`cart_screen.dart` | `GET /app/store_pickups`、`POST /app/checkout/validate`、`POST /app/checkout/shipping_fee` | 購物車的門市、運費精算、配送選項狀態 | 改成自寄流程 | 否（隨畫面） | `BAT/features/cart/cart_repository_shipping_fee_test.dart`、`BAT/features/cart/cart_controller_state_test.dart`、`BAT/features/cart/cart_store_selection_test.dart`、`BAT/features/cart/cart_control_consistency_test.dart` |
| `BA/features/cart/presentation/widgets/checkout_payment_sheet.dart`、`payment_details_section.dart` | `POST /app/checkout/preview` | 付款面板送出時帶 `shippingOption`、收件地址（不帶 storeId） | 改成自寄流程 | 否 | `BAT/features/cart/checkout_payment_sheet_layout_test.dart` |
| `BA/features/home/presentation/home_widget/home_address_section.dart`、`product_add_to_cart_sheet.dart`、`BA/features/product/presentation/widgets/product_purchase_panel.dart`、`BA/features/product/presentation/merchant_item_detail_screen.dart`、`W/features/product/components/ProductPurchaseBox.tsx` | 無（吃商品資料裡的配送文字） | 商品頁與加入購物車面板顯示配送方式或運費提示 | 待定（要看商品資料的 `shipping_methods` 在 B3 之後長什麼樣） | 是（顯示文案） | `BAT/features/home/home_address_section_test.dart`、`BAT/features/home/product_add_to_cart_sheet_test.dart`、`BAT/features/product/product_purchase_panel_test.dart`、`BAT/features/product/product_purchase_surface_parity_test.dart` |

## 2. 買家：訂單的物流追蹤

| 檔案／畫面 | 呼叫的端點 | 功能 | 分類（建議） | 需 UI 同意? | 測試檔 |
|---|---|---|---|---|---|
| `W/features/account/screens/orders/TrackingClient.tsx`（155 行）、`W/lib/logistics/repo.ts` | `GET /app/orders/{orderNo}/logistics`（後端當場向 PayUni 同步） | 買家單點貨態頁（託運單號、貨態、時間；PayUni 沒有多筆軌跡所以沒有時間線） | 改成自寄流程：顯示賣家填的物流業者與追蹤號碼，不再即時同步 | 是 | `W/features/account/screens/orders/TrackingClient.unit.test.ts` |
| `BA/features/order/presentation/widgets/order_logistics_section.dart`（439 行）、`BA/features/order/data/logistics_track.dart` | `GET /app/orders/{orderNo}/logistics`（`order_repository.dart:88`） | 訂單詳情的物流區塊，向後端要即時貨態 | 改成自寄流程 | 是 | `BAT/features/order/order_logistics_repository_test.dart`、`BAT/features/order/order_logistics_section_test.dart`、`BAT/features/order/pickup_timeline_narrow_test.dart` |
| `BA/features/order/presentation/widgets/return_logistics_timeline.dart`（840 行） | 無自己的呼叫（吃售後資料） | 退貨物流時間線（依 provider 分派） | 待定：屬售後模組（B3 範圍外？），但它依賴超商退貨的貨態 | 是 | `BAT/features/order/return_logistics_provider_dispatch_test.dart` |
| `W/features/orders/components/OrderStatusTimeline.tsx`、`orderStatusTimelineModel.ts`、`W/features/account/screens/orders/OrderDetailClient.tsx`、`OrderQuickActionDrawer.tsx`、`OrdersClient.tsx`、`statusBadgeStyles.ts` | 無自己的呼叫（訂單詳情資料） | 訂單狀態時間線與卡片的配送欄位（出貨、配送中、物流異常） | 保留（狀態機由伺服器決定，ADR D1）；其中配送資訊欄改顯示自寄資料 | 是（欄位內容變） | `W/tests/orders-repo.test.ts`、`W/lib/seller/shipment-progress-parity.unit.test.ts` |
| `BA/features/order/presentation/order_card.dart`、`order_detail_screen.dart`、`order_card_widgets/order_info_column.dart`、`order_expanded_section.dart`、`order_flag_banner.dart`、`order_status_presentation.dart`、`BA/features/order/data/order_model.dart`、`order_summary.dart`、`order_status.dart` | 無自己的呼叫 | 訂單卡片的配送方式、運費、物流異常旗標、狀態文字 | 保留，配送欄位改顯示自寄資料 | 是（欄位內容變） | `BAT/features/order/order_status_presentation_test.dart`、`BAT/features/order/order_card_redesign_test.dart`、`BAT/features/order/order_detail_redesign_test.dart`、`BAT/features/order/order_backend_contract_test.dart` |
| `BA/features/order/data/after_sale_model.dart`、`after_widgets/after_sale_detail_screen.dart`、`after_sale_view_screen.dart`、`W/features/account/screens/after-sales/AfterSaleDetailClient.tsx`、`W/lib/after-sales/return-failure-guidance.ts` 等 | 無（售後資料） | 退貨的取件與寄回物流（含「退貨便」失敗指引） | 待定：售後與退貨物流是否在 B3 內由伺服器改成自寄，要先看 B3 設計 | 是 | `W/tests/after-sales-return-failure-guidance.test.ts`、`W/tests/after-sales-status.test.ts`、`W/tests/after-sales-normalize.test.ts` |

## 3. 賣家：出貨建單、標籤、取號

| 檔案／畫面 | 呼叫的端點 | 功能 | 分類（建議） | 需 UI 同意? | 測試檔 |
|---|---|---|---|---|---|
| `W/features/seller/components/ShipmentDialogs.tsx`（666 行） | 經 `shipment.ts` | 三個對話框：黑貓預約（`TcatReservationDialog`）、叫車取件（`CallTcatDialog`）、7-11 交貨便（`SevenElevenShipDialog`），含出貨照片上傳 | 改成自寄流程：換成單一「填物流業者＋追蹤號碼」對話框（新畫面）；出貨照片上傳是否保留待定 | 是 | 無專屬測試 |
| `W/lib/seller/shipment.ts`（908 行） | `GET /app/shipping_business_days`、`POST /app/shipments/{order_no}/generate_home_label`、`.../call_tcat`、`.../print_cvs_label`、`.../confirm`、`GET .../tracking`、`POST /app/logistics`（`queryLogistics`） | 出貨動作、貨態碼、錯誤分類、冷卻與重試、`nextShipmentAction` | 改成自寄流程：只留 `confirm` 的等價物與狀態判斷；其餘刪 | 否（內部，隨畫面） | `W/lib/seller/shipment.unit.test.ts`、`W/lib/seller/shipment-provider.unit.test.ts`、`W/lib/seller/shipment-progress-parity.unit.test.ts` |
| `W/features/seller-center/orders/SellerOrdersPage.tsx`、`OrderDetailClient.tsx`、`SellerOrderDetailDrawer.tsx`、`orderTabs.ts`、`mobileOrderList.unit.test.ts` | 經 `shipment.ts` 與 `anomalies.ts` | 賣家訂單列表的「下一步動作」按鈕（預約取號、列印、出貨）、「配送異常」快篩與列內 badge、待處理分頁 | 改成自寄流程；異常快篩隨異常板的決定 | 是 | `W/features/seller-center/orders/mobileOrderList.unit.test.ts`、`W/tests/seller-order-tabs.test.ts`、`W/tests/e2e/seller-orders-count-vs-list.spec.ts` |
| `W/features/seller-center/orders/shipmentOperationStore.ts` | 無 | 黑貓預約進行中的本機操作狀態（`tcat-reserve`） | 刪除 | 否 | `W/features/seller-center/orders/shipmentOperationStore.unit.test.ts` |
| `SA/features/order/data/order_repository.dart` | `GET /api/v1/app/shipping_business_days`、`POST /api/v1/app/shipments/{no}/confirm`、`.../call_tcat`、`.../print_cvs_label`、`.../generate_home_label` | 出貨 API 呼叫、結果型別（`TcatResult`、`CvsLabelResult`、`HomeLabelResult`） | 改成自寄流程 | 否 | `SAT/features/order/order_backend_contract_test.dart`、`SAT/features/order/order_field_contract_test.dart` |
| `SA/features/order/presentation/tcat_reservation_dialog.dart`（453 行）、`call_tcat_dialog.dart`（280 行）、`seven_eleven_ship_dialog.dart`（261 行）、`cancel_shipment_dialog.dart`、`shipment_error_dispatch.dart`（217 行） | 同上 | 黑貓預約取號、叫車、7-11 交貨便、取消出貨、錯誤分派 | 刪除（取消出貨待定：改成自寄後是否仍可取消） | 是 | `SAT/features/order/shipping_provider_dispatch_test.dart` |
| `SA/features/order/presentation/order_card_widgets/order_bottom_row.dart` | 經 repository | 訂單卡底部按鈕列：依配送方式顯示「預約取號」「列印出貨單」，`3003／3005` 不得落到超商流程 | 改成自寄流程 | 是 | `SAT/features/order/order_card_compact_layout_test.dart`、`SAT/features/order/order_card_redesign_test.dart`、`SAT/features/order/shipping_provider_dispatch_test.dart` |
| `SA/features/order/presentation/logistics_tracking_screen.dart`（842 行）、`SA/features/order/data/payuni_logistics_service.dart`、路由 `AppRoutes.logistics = /order/logistics` | `POST /api/v1/app/logistics`（`lgsType`、`tradeType`、`returnOdno`） | 賣家端貨態查詢頁（PayUni 物流查詢） | 刪除 | 是 | `SAT/features/order/data/payuni_logistics_service_test.dart` |
| `SA/features/order/presentation/order_list_screen.dart`、`order_card.dart`、`shared_order_detail_screen.dart`、`order_payment_details_screen.dart`、`order_card_widgets/order_expanded_section.dart`、`order_info_column.dart`、`order_flag_banner.dart`、`SA/features/order/data/order_model.dart`、`order_tab_mapper.dart`、`SA/features/customer/presentation/customer_order_card_widget.dart` | 無自己的呼叫 | 訂單分頁（待出貨等）、運費與配送資訊顯示、物流異常旗標 | 保留，配送欄位改顯示自寄資料 | 是（欄位內容變） | `SAT/features/order/data/order_tab_mapper_test.dart`、`SAT/features/order/order_list_layout_test.dart`、`SAT/features/order/order_detail_redesign_test.dart`、`SAT/features/order/progress_parity_test.dart`、`SAT/features/order/order_status_presentation_test.dart` |

## 4. 賣家：物流開通、寄件資料、異常

| 檔案／畫面 | 呼叫的端點 | 功能 | 分類（建議） | 需 UI 同意? | 測試檔 |
|---|---|---|---|---|---|
| `W/features/seller-center/profile/shipping/ShippingClient.tsx`（741 行）、`prefill.ts`、`W/app/[locale]/(seller)/seller-center/me/shipping/page.tsx`、`W/lib/seller/shipping.ts`、`W/lib/hooks/useShippingOnboardings.ts` | `GET/POST /app/shipping_onboardings`、`POST .../home_delivery`、`POST .../cvs`、`POST .../{id}/toggle`、`POST /app/store_map_sessions` | 物流開通頁：超商開通（含選門市）、宅配寄件資料、啟停開關 | 刪除（寄件資料是否留作自寄地址待定，見開放問題） | 是 | `W/features/seller-center/profile/shipping/ShippingClient.unit.test.ts`、`prefill.unit.test.ts`、`W/lib/seller/shipping-onboarding.unit.test.ts` |
| `SA/features/settings/presentation/shipping_settings/shipping_settings_screen.dart`（625 行）、`widgets/cvs_shipping_section.dart`、`widgets/taqbin_shipping_section.dart`、`shipping_settings_controller.dart`、`store_map_webview_screen.dart` | 同上 | 物流設定畫面：7-11 賣貨便開通、黑貓宅配、寄件人（路由在 `app_router.dart:568`） | 刪除 | 是 | `SAT/features/settings/shipping_settings_architecture_test.dart`、`shipping_settings_prefill_test.dart`、`shipping_settings_responsive_test.dart`、`shipping_input_layout_test.dart` |
| `SA/features/settings/data/shipping_settings_remote_service.dart`、`shipping_settings_repository.dart`、`cvs_shipping_mutation.dart`、`shipping_sender.dart` | `GET /api/v1/app/shipping_onboardings`、`POST .../home_delivery`、`POST .../cvs`、`POST .../{id}/toggle` | 開通資料層與寄件人 | 刪除 | 否（隨畫面） | `SAT/features/settings/data/cvs_shipping_mutation_test.dart`、`SAT/features/settings/shipping_sender_test.dart` |
| `SA/features/seller/data/onboarding_tasks.dart`、`SA/features/seller/presentation/onboarding_progress_card.dart`、`profile_other_options_card.dart`、`profile_screen.dart`、`seller_certification_gate.dart`、`SA/core/router/route_capabilities.dart`、`W/features/seller-center/dashboard/onboardingTasks.ts`、`OnboardingProgressCard.tsx`、`DashboardPage.tsx`、`W/features/seller-center/components/pageCapabilities.ts` | 無自己的呼叫（吃 `shipping_onboardings` 與 `SELLER_CAPABILITY_SHIPPING_SETUP`） | 賣家上手清單裡的「物流設定」步驟與能力閘門 | 待定：步驟與能力 `SHIPPING_SETUP` 是否保留，要等契約決定（是否刪此 enum） | 是 | `SAT/features/seller/data/onboarding_tasks_test.dart`、`SAT/features/seller/presentation/onboarding_progress_card_test.dart`、`SAT/features/seller/presentation/profile_other_options_card_test.dart`、`SAT/features/seller/presentation/profile_screen_test.dart`、`SAT/features/seller/presentation/seller_certification_gate_test.dart`、`W/features/seller-center/dashboard/onboardingTasks.unit.test.ts`、`OnboardingProgressCard.unit.test.ts`、`W/features/seller-center/components/__tests__/pageCapabilities.unit.test.ts`、`W/tests/e2e/seller-dashboard-onboarding-actions.spec.ts` |
| `SA/features/shipment_anomaly/`（`data/shipment_anomaly_model.dart`、`shipment_anomaly_providers.dart`、`shipment_anomaly_repository.dart`、`presentation/shipment_anomaly_dashboard_screen.dart`（594 行）；生成檔 `.freezed.dart`、`.g.dart` 另計）、`SA/features/dashboard/presentation/shipment_anomaly_alert_banner.dart`、`SA/core/codemeta/shipment_anomaly_presentation.dart` | `GET /api/v1/app/shipments/anomalies`、`GET .../anomalies/stats`、`POST .../anomalies/{shipment_id}/{anomaly_type}/acknowledge`、`POST .../ignore` | 賣家異常看板與首頁橫幅 | 刪除（ADR 不搬異常板；異常改由伺服器狀態機轉售後） | 是 | `SAT/core/codemeta/shipment_anomaly_presentation_test.dart`、`SAT/features/dashboard/dashboard_screen_test.dart` |
| `W/lib/shipping/anomalies.ts`、`W/lib/hooks/useSellerShipmentAnomalies.ts`（含 `SellerOrdersPage` 的「配送異常」快篩、`DashboardPage`、`DelegatedDashboardPage`、`SellerInvoiceJobPage` 內的引用） | `GET /app/shipments/anomalies`、`GET .../stats`、`POST .../acknowledge`、`POST .../ignore` | website 賣家的異常資料與列內 badge | 刪除 | 是（移除快篩與 badge） | 無專屬測試；`W/tests/e2e/seller-orders-count-vs-list.spec.ts` 間接 |
| `A/src/app/admin/anomalies/page.tsx`（1238 行，5 個 tab 之一為 `shipping`） | `GET /api/v1/admin/shipments/anomalies?{params}`、`POST /api/v1/admin/shipments/anomalies/{shipmentId}/{anomalyType}/{action}` | 後台異常監控的「物流」tab 與處置動作 | 刪除 shipping tab；其餘 4 個 tab 保留 | 是 | `A/tests/anomaly-handoff-contract.test.ts` |
| `A/src/features/delegated-operations/anomaly-handoff.ts`、`W/features/delegated-operations/state/session.ts`（`delegatedAnomalyUrl`） | 無（組跨 host 連結） | 代行工作區訂單抽屜的「前往平台異常處置」連結，鍵名是跨 repo 契約 | 保留（其他 tab 仍用）；`shipping` tab 鍵移除要兩邊同波 | 否 | `A/tests/anomaly-handoff-contract.test.ts`、`A/tests/delegated-operations-contract.test.ts` |
| `A/src/app/admin/merchants/[id]/page.tsx`、`A/src/app/admin/transactions/page.tsx` | `GET /api/v1/admin/orders`（欄位 `shippingMethod`） | 後台訂單列表的配送方式欄（走 `SHIPPING_METHOD_LABEL`）；`transactions` 的 `SHIPPING_ABNORMAL` 狀態 | 保留，欄位內容隨訂單資料 | 是（欄位內容變） | 無專屬測試 |
| `W/features/delegated-operations/orders/DelegatedOrdersPage.tsx`、`DelegatedOrderDetailPanel.tsx`、`display.ts`、`repo.ts` | 代行 repo（經 website BFF） | 代行工作區的訂單列表配送欄、不顯示履約動作 | 保留；配送欄顯示隨自寄資料 | 是（欄位內容變） | `W/features/delegated-operations/orders/list-shipping-column.unit.test.ts`、`fulfilment-exclusion.unit.test.ts` |

## 5. 運費規則與商品的運送方式

| 檔案／畫面 | 呼叫的端點 | 功能 | 分類（建議） | 需 UI 同意? | 測試檔 |
|---|---|---|---|---|---|
| `W/features/seller-center/products/components/SkuShippingEditor.tsx`（498 行）、`ProductFormSkuEditor.tsx`、`W/features/seller-center/products/adapters/productFormAdapter.ts`、`W/lib/products/seller-repo.ts`（第 918-928 行） | `POST /app/shipping_quotes`（依包裹長寬高重量回傳可用物流商與運費） | SKU 的可配送方案：量尺寸後即時報價，再選運費誰付、免運門檻 | 改成自寄流程：移除尺寸報價，只留固定規則（買家付定額、賣家免運、滿額免運） | 是 | `W/features/seller-center/products/components/SkuShippingEditor.unit.test.ts`、`W/features/seller-center/products/adapters/__tests__/productFormAdapter.unit.test.ts`、`W/tests/product-form-adapter.test.ts` |
| `SA/features/product/presentation/product_setup_spec_screen.dart`、`widgets/shipping_spec_card.dart`（194 行）、`SA/features/product/data/product_repository.dart`（第 136-161 行）、`product_listing_model.dart`、`product_upload_service.dart` | `GET /api/v1/app/shipping_methods`、`POST /api/v1/app/shipping_quotes`、`POST /api/v1/app/store_map_sessions`（`product_repository.dart:178`，商品流程裡也有門市地圖入口） | 商品規格頁的運送方式多選、尺寸報價、運費規則 | 改成自寄流程；`store_map_sessions` 呼叫刪除 | 是 | `SAT/features/product/presentation/shipping_options_filter_test.dart`、`SAT/features/product/data/product_sku_payload_test.dart` |
| `W/features/cart/components/CouponSelectionSheet.tsx`、`W/features/cart/couponAvailability.ts`、`BA` 的 `cart_controller.dart` 等（運費與門檻相關） | 無 | 優惠券可用性受運費與免運門檻影響的文案 | 待定（B7 coupon 已遷；確認是否只是文案） | 否 | `W/tests/cart-coupon-availability.test.ts` |
| `A/src/app/admin/products/[id]/page.tsx`、`A/src/types/product.ts` | `GET /api/v1/admin/products/{id}`（欄位 `shippingMethods`） | 後台商品詳情的「運送方式」表（`SHIPPING_METHOD_LABEL`、`SHIPPING_FEE_PAYER_LABEL`） | 改成自寄流程（欄位內容隨固定規則） | 是（欄位內容變） | `A/tests/product-detail-contract.test.ts` |
| `W/features/delegated-operations/products/`（`DelegatedProductEditPage.tsx`、`edit-patch.ts`、`preview-props.ts`、`repo.ts`） | 代行 repo | 代行編輯商品的運送方式欄 | 改成自寄流程（隨商品表單） | 是 | `W/features/delegated-operations/products/edit-patch.unit.test.ts`、`conflict.unit.test.ts` |

## 6. CodeMeta、packages 與 Uber 3003 回歸測試

| 檔案／畫面 | 呼叫的端點 | 功能 | 分類（建議） | 需 UI 同意? | 測試檔 |
|---|---|---|---|---|---|
| `BA/core/codemeta/shipping_presentation.dart`、`SA/core/codemeta/shipping_presentation.dart`（手寫，四元分派 `3001`／`3002`／`3003`／`3005`，未知碼回 unsupported） | 無 | 配送方式碼值到畫面行為的分派 | 改成自寄流程：碼值集合隨 B3 重定（自寄成為唯一值時分派可大幅簡化）；待決定新碼值落在 proto enum 還是儲存碼 | 否 | `BAT/core/codemeta/shipping_presentation_test.dart`、`SAT/core/codemeta/shipping_presentation_test.dart` |
| `W/lib/codemeta/generated.ts`、`W/lib/codemeta/enums.ts`、`A/src/lib/codemeta/generated.ts`、`BA/core/codemeta/code_meta.dart`、`SA/core/codemeta/code_meta.dart` | 無 | 配送方式、貨態、異常類型（含 `UBER_*`）的生成碼表 | 保留到 CodeMeta 整體退場（凍結，不手改；根 `AGENTS.md`） | 否 | `A/tests/codemeta-no-second-copy.test.ts` |
| `A/tests/shipping-provider-no-fallback.test.ts`（守門）、`SAT/features/order/shipping_provider_dispatch_test.dart`、`SAT/core/codemeta/shipping_presentation_test.dart`（`3003 / 3005 → unsupported`）、`BAT/features/cart/shipping_unsupported_filter_test.dart`、`BAT/features/order/return_logistics_provider_dispatch_test.dart`、`W/lib/seller/shipment-provider.unit.test.ts`、`W/lib/checkout/shipping-requirements.unit.test.ts` | 無 | 「配送方式不得二元 fallback」的回歸測試（含 Uber 3003、商家自送 3005 不得落到黑貓或 7-11） | 改成自寄流程：配送方式集合改變後，測試要跟著改寫成新集合的 fail-closed 判準，而不是單純刪 | 否 | 同左（本列即測試） |
| `packages/hawkr_ui_text`、`packages/hawkr-ui-text-ts`（生成，來源 `backend/internal/platform/uitext/catalog/{zh-TW,en}.json`） | 無 | 通知類別 `NOTIFICATION_CATEGORY_SHIPMENT`／`SHIPMENT_ANOMALY` 的使用者面文字 | 待定：隨 proto enum 是否刪 `SHIPMENT_ANOMALY` | 否 | `packages/hawkr_ui_text/test/accepted_test.dart`（含 golden） |
| `packages/hawkr_api`、`packages/hawkr-api-ts`（生成；來源 `api/hawkr/notification/v1alpha1/notification.proto`、`seller/v1alpha1/seller.proto`） | 無（契約型別） | `NotificationCategory.SHIPMENT`／`SHIPMENT_ANOMALY`、`SellerCapability.SHIPPING_SETUP`、`pending_shipment_count`（賣家摘要） | 待定：契約改動要同次更新唯一來源與所有消費端（根 `AGENTS.md`），`buf breaking` 上線前不是約束 | 否 | `api-contract.yml`（CI） |
| `SA/features/notification/presentation/notification_settings_screen.dart`、`W/features/account/screens/notifications/PreferencesClient.tsx` | 通知偏好 RPC | 通知偏好頁的「出貨」「配送異常」兩個開關 | 待定：「出貨」開關保留；「配送異常」隨異常板決定 | 是（移除一個開關） | `W/features/account/screens/notifications/PreferencesClient.unit.test.ts` |
| `packages/hawkr_auth`（`sdk.dart`、`sdk_test.dart` 的 `3003`） | 無 | LINE SDK 使用者取消的錯誤碼 | 保留；**與 Uber 3003 無關**，刪 Uber 回歸測試時不要誤動 | 否 | `packages/hawkr_auth/test/sdk_test.dart` |

## 7. 溫層（deliveryTemp）：B4／B6 擁有，B3 不動

【量】C7：website 14 檔、buyer-app 8 檔、seller-app 0 檔。非 test 的使用點：

| 檔案 | 用法 | 歸屬 |
|---|---|---|
| `W/lib/checkout/repo.ts`（7 處）、`W/lib/checkout/draft.ts` | `deliveryTemp` 在驗證、預覽、運費精算的請求與回應欄位 | B4／B6（溫層是商品與結帳模組的資料）；若 B3 改運費規則，這條欄位是否仍需要要在 B4／B6 一併決定 |
| `W/features/cart/CartClient.tsx`（12 處）、`cartUiStore.ts`（6 處）、`W/features/cart/components/ShippingMethodGroup.tsx`（8 處）、`W/features/checkout/screens/CheckoutClient.tsx`（3 處） | 購物車按溫層分組選配送方式、以 `deliveryTemp` 比對選項 | B4／B6；但這些畫面本身在 §1 屬 B3 要改的畫面，**兩者在同一檔交會**，派工要排序 |
| `BA/features/cart/data/cart_repository.dart`、`BA/features/cart/presentation/widgets/checkout_payment_sheet.dart`（2 處） | 送精算與預覽時帶 `shippingOption['deliveryTemp']` | B4／B6，同上 |
| 測試：`W/tests/checkout-repo.test.ts`（10 處）、`W/tests/checkout-draft.test.ts`、`W/lib/checkout/draft.unit.test.ts`、`W/lib/checkout/repo.unit.test.ts`、`W/features/cart/components/Cart.browser.test.tsx`、`W/tests/e2e/cart-seller-selection.spec.ts`、`BAT/features/cart/cart_repository_shipping_fee_test.dart`、`BAT/features/cart/checkout_payment_sheet_layout_test.dart`（5 處）等 | 以 `deliveryTemp` 為欄位的斷言 | B4／B6 |

## 8. 彙總

| client | 逐檔看過的核心檔（不含生成檔） | 建議刪除 | 建議改成自寄流程 | 建議保留 | 待定 |
|---|---|---|---|---|---|
| website | 約 60 檔（lib／features／app） | 約 14 檔（門市、電子地圖、開通、異常、tcat 預約與標籤） | 約 22 檔（購物車、結帳、出貨、追蹤、運費規則） | 約 10 檔（訂單狀態顯示、代行契約） | 約 6 檔（售後物流、商品頁顯示、通知） |
| admin | 8 檔 | 1 個 tab（`anomalies/page.tsx` 的 `shipping`） | 1 檔（`products/[id]/page.tsx` 欄位內容） | 5 檔 | 0 |
| buyer-app | 約 40 檔 | 約 9 檔（`store_map_webview_screen`、`store_pickup_form_screen`、`store_pickup_*`） | 約 15 檔（`shipping_section`、cart、order 物流區塊） | 約 8 檔 | 約 6 檔 |
| seller-app | 約 50 檔 | 約 28 檔（物流設定、異常板、tcat 與超商對話框、PayUni 貨態頁） | 約 10 檔（訂單卡按鈕、`order_repository`、商品規格） | 約 6 檔 | 約 6 檔（onboarding、通知） |
| packages | 0 檔手寫；生成物 8 檔（C8） | 0 | 0 | 0 | 8（proto enum 是否刪值） |

【推】彙總的檔數是逐表列加總的概數，列內「等」代表同目錄同性質的檔合併成一列；精確數字用 C1 的指令。測試檔大多與其被測檔同進退（C1 的 test 欄：website 64、admin 7、buyer-app 34、seller-app 47，合計 152）。

## 9. 需要 Will 同意的 UI 變更（分組）

1. **結帳與購物車**：移除超商取貨與門市選擇（W `StorePickupPickerSheet`、BA `store_map_webview_screen`／`store_pickup_form_screen`、地址簿的門市分頁）；移除配送方式分組選擇；運費改為讀賣家固定規則。
2. **買家訂單**：物流追蹤頁與訂單物流區塊改為顯示賣家填的物流業者與追蹤號碼，不再即時同步貨態。
3. **賣家出貨**：用「填物流業者＋追蹤號碼」取代黑貓預約、叫車、7-11 交貨便三個對話框與列印標籤；訂單卡按鈕列、website 賣家訂單的「下一步動作」隨之改。**這是全新畫面，需要先出簡報再實作。**
4. **賣家物流開通**：刪除 website 的物流開通頁與 seller-app 的物流設定畫面，以及上手清單的「物流設定」步驟（若決定刪）。
5. **異常**：刪除 seller-app 異常看板與首頁橫幅、website 賣家的「配送異常」快篩與列內 badge、admin 異常頁的物流 tab、通知偏好的「配送異常」開關。
6. **商品設定**：移除依尺寸即時報價，改為固定運費規則輸入（W `SkuShippingEditor`、SA `product_setup_spec_screen`）。
7. **欄位內容變動**（不是結構變動，仍算 UI 文字變更）：admin 商品詳情與訂單列表的配送方式欄、訂單卡的配送資訊。

## 10. 開放問題

1. ADR D3 保留「物流 port＋模擬 adapter（建單、取標籤、貨態 webhook）」，但任務前提寫「不做標籤列印」。B3 的 client 是否完全不留標籤與貨態 webhook 的畫面？若模擬 adapter 只為測試存在，client 全刪即可；若要在作品集示範，則需要保留一個示範入口，這是範圍決定。
2. 售後的退貨物流（`return_logistics_timeline.dart` 840 行、`return-failure-guidance.ts`）依賴超商退貨；它屬 B3 還是售後模組，要在 B3 設計書先劃界。
3. 自寄流程的物流業者：自由輸入文字，還是固定清單（黑貓、新竹、郵局…）？這決定新畫面是文字欄還是下拉。
4. 賣家的「寄件地址」（`shipping_onboardings/home_delivery` 的資料）在自寄下是否還需要？若不需要，物流設定頁整個刪；若要，只保留地址一欄。
5. 配送方式碼值集合：自寄成為唯一值時，`3001`／`3002`／`3003`／`3005` 在 proto 與儲存碼怎麼收斂？client 的四元分派與 8 列守門測試要隨之改寫（§6）。
6. 購物車的溫層分組（`deliveryTemp`）與 B3 要改的配送方式分組在同一個檔案（`CartClient.tsx`、`ShippingMethodGroup.tsx`、`checkout_payment_sheet.dart`）。B4／B6 與 B3 的派工誰先、衝突怎麼排。
7. `SELLER_CAPABILITY_SHIPPING_SETUP`、`NOTIFICATION_CATEGORY_SHIPMENT_ANOMALY` 是否從 proto 刪除（影響 `api/`、`packages/hawkr_api`、`packages/hawkr-api-ts`、兩份 ui-text 目錄與三個 client）。
8. 商品的 `pending_shipment_count`（賣家摘要）在自寄下語意不變，預期保留；請確認。
9. website 與 seller-app 的商品流程各有一處呼叫 `store_map_sessions`（W `ShippingClient`、SA `product_repository.dart:178`），後者放在商品 repository 內看起來是歷史殘留，【推】可直接刪，未逐呼叫端驗證。
