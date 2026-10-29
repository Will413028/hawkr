# Hawkr Screen-API Mapping

> 每個前端頁面對應的 API 清單

---

## Buyer App

### 1. Onboarding / Auth

#### welcome_screen (歡迎頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/health` | GET | 檢查服務狀態 |

#### force_update_screen (強制更新頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/public/app-version-gate` | GET | 檢查 App 版本是否需要強制更新 (`docs/modules/content.md` §5) |

#### auth_screen (登入註冊頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/auth/login` | POST | 帳密登入 |
| `/api/v1/auth/register` | POST | 用戶註冊 |
| `/api/v1/auth/google` | POST | Google OAuth 登入 |
| `/api/v1/auth/google/check` | GET | 檢查 Google 帳號是否存在 |
| `/api/v1/auth/firebase` | POST | Firebase 手機驗證 |

#### google_verification_screen (Google 驗證頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/auth/google` | POST | Google Token 交換 |
| `/api/v1/auth/google/check` | GET | 檢查帳號是否存在 |

---

### 2. Home / Discovery

#### home_screen (首頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/banners` | GET | 取得 Banner 列表 |
| `/api/v1/products` | GET | 取得推薦商品列表 |
| `/api/v1/categories/tree` | GET | 取得分類樹 |
| `/api/v1/search/hot` | GET | 取得熱門搜尋關鍵字 |

#### home_discover_screen (探索頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/posts` | GET | 取得貼文 Feed |
| `/api/v1/posts/:id/like` | POST | 按讚貼文 |
| `/api/v1/posts/:id/save` | POST | 收藏貼文 |

#### marketplace_screen (市集頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/products` | GET | 取得商品列表 |
| `/api/v1/search/products/filter` | POST | 篩選商品 |
| `/api/v1/categories` | GET | 取得分類 |

#### news_screen (新聞頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/posts` | GET | 取得新聞列表 (type=news) |

---

### 3. Search / Category

#### search_screen (搜尋頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/search/suggest` | GET | 取得搜尋建議 |
| `/api/v1/search/hot` | GET | 取得熱門搜尋 |
| `/api/v1/search/history` | GET | 取得搜尋歷史 |
| `/api/v1/search/history` | DELETE | 清除搜尋歷史 |

#### search_results_screen (搜尋結果頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/search/products` | GET | 搜尋商品 |
| `/api/v1/search/products/filter` | POST | 篩選商品 |

#### category_screen (分類頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/categories/tree` | GET | 取得分類樹 |
| `/api/v1/categories` | GET | 取得分類列表 |

#### category_landing_screen (分類落地頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/categories/:id` | GET | 取得分類詳情 |
| `/api/v1/categories/:id/children` | GET | 取得子分類 |
| `/api/v1/products` | GET | 取得該分類商品 |

#### category_filter_screen (分類篩選頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/search/products/filter` | POST | 篩選商品 |

---

### 4. Product

#### item_detail_screen (商品詳情頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/products/:id` | GET | 取得商品詳情 |
| `/api/v1/products/:id/skus` | GET | 取得 SKU 列表 |
| `/api/v1/products/:id/reviews` | GET | 取得商品評價 |
| `/api/v1/products/:id/rating` | GET | 取得商品評分 |
| `/api/v1/products/:id/related` | GET | 取得相關商品 |
| `/api/v1/skus/:id` | GET | 取得 SKU 詳情 |
| `/api/v1/skus/:id/price` | GET | 取得 SKU 價格 |
| `/api/v1/inventory/:skuId` | GET | 查詢庫存 |
| `/api/v1/user/favorites/:id/check` | GET | 檢查是否收藏 |
| `/api/v1/user/favorites` | POST | 加入收藏 |
| `/api/v1/user/favorites/:id` | DELETE | 取消收藏 |
| `/api/v1/cart` | POST | 加入購物車 |
| `/api/v1/user/history` | POST | 記錄瀏覽歷史 |

#### product_reviews_screen (商品評價頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/products/:id/reviews` | GET | 取得商品評價列表 |
| `/api/v1/products/:id/rating` | GET | 取得評分統計 |

#### reserve_screen (預購頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/reservations/*` | GET | 取得預購商品 |

---

### 5. Seller

#### seller_screen (商家頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/sellers/:id` | GET | 取得商家資訊 |
| `/api/v1/sellers/:id/stats` | GET | 取得商家統計 |
| `/api/v1/sellers/:id/products` | GET | 取得商家商品 |
| `/api/v1/sellers/:id/posts` | GET | 取得商家貼文 |
| `/api/v1/user/following/:id/check` | GET | 檢查是否關注 |
| `/api/v1/user/following/:id` | POST | 關注/取消關注 |

#### seller_item_detail_screen (商家商品詳情)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/products/:id` | GET | 取得商品詳情 |
| `/api/v1/sellers/:id/products/:pid/images` | GET | 取得商品圖片 |

#### seller_posts_feed_screen (商家貼文列表)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/sellers/:id/posts` | GET | 取得商家貼文 |

---

### 6. Cart / Checkout

#### cart_screen (購物車頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/cart` | GET | 取得購物車列表 |
| `/api/v1/cart/item` | PUT | 更新購物車商品數量 |
| `/api/v1/cart/:id` | DELETE | 刪除購物車商品 |
| `/api/v1/cart/check` | POST | 勾選商品 |
| `/api/v1/cart/check-all` | POST | 全選商品 |
| `/api/v1/inventory/:skuId` | GET | 查詢庫存 |

#### confirm_order_screen (確認訂單頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/addresses` | GET | 取得地址列表 |
| `/api/v1/user/addresses/default` | GET | 取得預設地址 |
| `/api/v1/coupons/my` | GET | 取得我的優惠券 |
| `/api/v1/orders` | POST | 建立訂單 |

#### payment_detail_screen (支付頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/payment/credit` | POST | 信用卡支付 |
| `/api/v1/payment/atm` | POST | ATM 虛擬帳號 |
| `/api/v1/payment/atm/banks` | GET | 取得銀行列表 |
| `/api/v1/payment/apple-pay` | POST | Apple Pay 支付 |
| `/api/v1/payment/apple-pay/validate-merchant` | POST | Apple Pay 商家驗證 |
| `/api/v1/payment/google-pay` | POST | Google Pay 支付 |

#### coupon_management_screen (優惠券管理頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/coupons` | GET | 取得可領取優惠券 |
| `/api/v1/coupons/my` | GET | 取得我的優惠券 |
| `/api/v1/coupons/:id/receive` | POST | 領取優惠券 |

---

### 7. Order

#### order_list_screen (訂單列表頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders` | GET | 取得買家訂單列表 |

#### order_detail_screen (訂單詳情頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders/:order_no` | GET | 取得訂單詳情 |
| `/api/v1/app/orders/:order_no/cancel` | POST | 取消訂單 |
| `/api/v1/app/orders/:order_no/refund` | POST | 申請退款 |
| `/api/v1/app/orders/:order_no/refund/method` | PUT | 選擇退款方式 |

#### order_matched_screen (訂單配對頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders/:order_no` | GET | 取得訂單狀態 |

#### order_details_delivering_screen (配送中訂單頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders/:order_no` | GET | 取得訂單詳情 |
| `/api/v1/shipping/*` | GET | 取得物流資訊 |

#### order_payment_details_screen (訂單支付詳情)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders/:order_no` | GET | 取得訂單詳情 |

#### order_receipts_screen (訂單收據頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/payments/:trade_no` | GET | 取得支付詳情 |

#### order_review_screen (訂單評價頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/products/:id/reviews` | POST | 提交商品評價 |
| `/api/v1/media/*` | POST | 上傳評價圖片 |

#### tracking_screen (物流追蹤頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/shipping/*` | GET | 取得物流追蹤資訊 |

#### dispute_screen (爭議處理頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/disputes/*` | POST | 提交爭議 |
| `/api/v1/media/*` | POST | 上傳證據圖片 |

#### order_dispute_screen (訂單爭議詳情)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/disputes/*` | GET | 取得爭議詳情 |

---

### 8. Social / Feed

#### post_detail_screen (貼文詳情頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/posts/:id` | GET | 取得貼文詳情 |
| `/api/v1/posts/:id/comments` | GET | 取得留言列表 |
| `/api/v1/posts/:id/comments` | POST | 發表留言 |
| `/api/v1/posts/:id/like` | POST | 按讚貼文 |
| `/api/v1/posts/:id/save` | POST | 收藏貼文 |
| `/api/v1/posts/:id/share` | POST | 分享貼文 |
| `/api/v1/posts/:id/share/count` | GET | 取得分享次數 |
| `/api/v1/comments/:id/like` | POST | 按讚留言 |
| `/api/v1/comments/:id` | PUT | 更新留言 |
| `/api/v1/comments/:id` | DELETE | 刪除留言 |

---

### 9. Chat / Message

#### chat_list_screen (聊天列表頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/chat/conversations` | GET | 取得對話列表 |
| `/api/v1/chat/unread` | GET | 取得未讀數 |

#### chat_room_screen (聊天室頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/chat/ws` | WS | WebSocket 連線 |
| `/api/v1/chat/history/:targetId` | GET | 取得聊天記錄 |
| `/api/v1/chat/messages` | POST | 發送訊息 |
| `/api/v1/chat/conversations/:targetId/read` | POST | 標記已讀 |
| `/api/v1/media/*` | POST | 上傳圖片 |

#### notifications_screen (通知列表頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/notifications` | GET | 取得通知列表 |
| `/api/v1/user/notifications/unread` | GET | 取得未讀數 |
| `/api/v1/user/notifications/:id/read` | PUT | 標記已讀 |
| `/api/v1/user/notifications/read-all` | PUT | 全部標記已讀 |
| `/api/v1/user/notifications/:id` | DELETE | 刪除通知 |

---

### 10. Profile / Wallet

#### wallet_screen (錢包頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/points` | GET | 取得積分餘額 |
| `/api/v1/user/points/history` | GET | 取得積分歷史 |

#### wallet_coupon_detail_screen (優惠券詳情頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/coupons/my` | GET | 取得優惠券詳情 |

#### spending_history_screen (消費記錄頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/orders` | GET | 取得訂單記錄 |
| `/api/v1/user/points/history` | GET | 取得積分使用記錄 |

#### profile_edit_screen (編輯個人資料頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/profile` | GET | 取得個人資料 |
| `/api/v1/user/profile` | PUT | 更新個人資料 |
| `/api/v1/media/*` | POST | 上傳頭像 |

#### address_management_screen (地址管理頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/addresses` | GET | 取得地址列表 |
| `/api/v1/user/addresses` | POST | 新增地址 |
| `/api/v1/user/addresses/:id` | PUT | 更新地址 |
| `/api/v1/user/addresses/:id` | DELETE | 刪除地址 |
| `/api/v1/user/addresses/:id/default` | PUT | 設為預設 |

#### my_subscription_screen (我的訂閱頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/following` | GET | 取得關注列表 |
| `/api/v1/user/followers` | GET | 取得粉絲列表 |

---

### 11. Settings

#### settings_screen (設定頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/profile` | GET | 取得使用者資訊 |
| `/api/v1/auth/logout` | POST | 登出 |

---

## Seller App

### 1. Onboarding

#### welcome_screen (歡迎頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/health` | GET | 檢查服務狀態 |
| `/api/v1/user/merchant` | GET | 檢查商家狀態 |

#### force_update_screen (強制更新頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/public/app-version-gate` | GET | 檢查 App 版本是否需要強制更新 (`docs/modules/content.md` §5) |

#### auth_screen (登入註冊頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/auth/login` | POST | 商家登入 |
| `/api/v1/auth/google` | POST | Google OAuth 登入 |

#### person_setup1-4 (個人資料設定)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/profile` | PUT | 更新個人資料 |
| `/api/v1/media/*` | POST | 上傳證件照片 |

#### merchant_setup1-5 (商家設定)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/sellers/apply` | POST | 申請成為商家 |
| `/api/v1/categories/tree` | GET | 取得分類 |
| `/api/v1/media/*` | POST | 上傳商家圖片 |

#### real_name_verification_page (實名認證頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/media/*` | POST | 上傳證件 |
| `/api/v1/user/profile` | PUT | 提交認證資料 |

#### merchant_certification_screen (商家認證頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/merchant` | GET | 取得認證狀態 |

---

### 2. Dashboard

#### dashboard_screen (儀表板頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders?view=selling` | GET | 取得賣家最新訂單 |
| `/api/v1/chat/unread` | GET | 取得未讀訊息數 |

---

### 3. Product Management

#### product_edit_screen (商品編輯頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/products` | POST | 新增商品 |
| `/api/v1/app/products/:id` | PUT | 更新商品 |
| `/api/v1/app/categories/tree` | GET | 取得分類（含自己建立的審核中分類） |
| `/api/v1/app/categories` | POST | 提交新分類 |

#### product_image_setup_screen (商品圖片設定頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/media/*` | POST | 上傳商品圖片 |
| `/api/v1/media/*` | DELETE | 刪除商品圖片 |

#### product_spec_setup_screen (商品規格設定頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/products/:id/variants` | GET | 取得規格列表 |

#### product_info_setup_screen (商品資訊設定頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/categories/tree` | GET | 取得分類（含自己建立的審核中分類） |
| `/api/v1/app/categories` | POST | 提交新分類 |

#### product_list_screen (商品列表管理頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/products` | GET | 取得賣家商品列表 |

#### product_reviews_screen (商品評價頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/products/:id/reviews` | GET | 取得商品評價 |
| `/api/v1/reviews/:id/reply` | POST | 回覆評價 |

---

### 4. Order Management

> 賣家訂單透過 `?view=selling` 查詢參數切換視角，不使用獨立路由

#### order_list_screen (訂單列表頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders?view=selling` | GET | 取得賣家訂單列表 |

#### order_matched_screen (訂單配對頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders/:order_no?view=seller` | GET | 取得訂單詳情（賣家視角） |

#### order_details_delivering_screen (配送中訂單頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/app/orders/:order_no?view=seller` | GET | 取得訂單詳情（賣家視角） |
| `/api/v1/app/orders/:order_no/ship` | POST | 發貨（填寫物流單號） |

#### order_dispute_screen (訂單爭議頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/disputes/*` | GET | 取得爭議詳情 |
| `/api/v1/disputes/*` | PUT | 處理爭議 |

#### logistics_tracking_screen (物流追蹤頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/shipping/*` | GET | 取得物流資訊 |

#### after_sales_screen (售後服務頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/disputes/*` | GET | 取得售後列表 |

---

### 5. Coupon Management

#### coupon_management_screen (優惠券管理頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/seller/coupons` | GET | 取得優惠券列表 |
| `/api/v1/seller/coupons/:id` | DELETE | 刪除優惠券 |

#### coupon_editor_screen (優惠券編輯頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/seller/coupons` | POST | 新增優惠券 |
| `/api/v1/seller/coupons/:id` | GET | 取得優惠券詳情 |
| `/api/v1/seller/coupons/:id` | PUT | 更新優惠券 |

---

### 6. Social / Post

#### post_compose_screen (發佈貼文頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/seller/posts` | POST | 發佈貼文 |
| `/api/v1/media/*` | POST | 上傳貼文圖片 |
| `/api/v1/seller/products` | GET | 取得商品列表 (標記商品) |

#### post_detail_screen (貼文詳情頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/posts/:id` | GET | 取得貼文詳情 |
| `/api/v1/posts/:id/comments` | GET | 取得留言列表 |
| `/api/v1/seller/posts/:id` | PUT | 編輯貼文 |
| `/api/v1/seller/posts/:id` | DELETE | 刪除貼文 |

---

### 7. Chat / Message

#### chat_list_screen (聊天列表頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/chat/conversations` | GET | 取得對話列表 |
| `/api/v1/chat/unread` | GET | 取得未讀數 |

#### chat_thread_screen (聊天對話頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/chat/ws` | WS | WebSocket 連線 |
| `/api/v1/chat/history/:targetId` | GET | 取得聊天記錄 |
| `/api/v1/chat/messages` | POST | 發送訊息 |
| `/api/v1/chat/conversations/:targetId/read` | POST | 標記已讀 |
| `/api/v1/media/*` | POST | 上傳圖片 |

#### notifications_screen (通知列表頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/notifications` | GET | 取得通知列表 |
| `/api/v1/user/notifications/unread` | GET | 取得未讀數 |
| `/api/v1/user/notifications/:id/read` | PUT | 標記已讀 |

#### notification_settings_screen (通知設定頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/profile` | GET | 取得通知設定 |
| `/api/v1/user/profile` | PUT | 更新通知設定 |

---

### 8. Merchant Settings

#### merchant_info_screen (商家資訊頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/merchant` | GET | 取得商家資訊 |

#### merchant_edit_screen (商家資訊編輯頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/merchant` | GET | 取得商家資訊 |
| `/api/v1/sellers/apply` | POST | 更新商家資訊 |
| `/api/v1/media/*` | POST | 上傳商家圖片 |

#### profile_screen (個人中心頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/profile` | GET | 取得個人資料 |
| `/api/v1/user/merchant` | GET | 取得商家資訊 |

#### settings_screen (設定頁面)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/profile` | GET | 取得使用者資訊 |
| `/api/v1/auth/logout` | POST | 登出 |

#### help_center_screen (幫助中心頁)
| API | Method | 說明 |
|-----|--------|------|
| (靜態內容) | - | FAQ 等內容 |

---

### 9. Media Tools

#### photo_editor_screen (圖片編輯器)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/media/*` | POST | 上傳編輯後圖片 |
| `/api/v1/ai/*` | POST | AI 圖片處理 |

#### image_editor_ai_tab (AI 圖片工具)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/ai/*` | POST | AI 去背/美化 |

---

### 10. PAYUNi / Wallet

#### payuni_signup_screen (PAYUNi 註冊頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/seller/me/payuni/register` | POST | 註冊 PAYUNi |
| `/api/v1/seller/me/payuni/status` | GET | 查詢註冊狀態 |
| `/api/v1/seller/me/payuni/retry` | POST | 重試註冊 |
| `/api/v1/seller/me/payuni/refresh-status` | POST | 刷新狀態 |

#### wallet_screen (商家錢包頁)
| API | Method | 說明 |
|-----|--------|------|
| `/api/v1/user/points` | GET | 取得積分餘額 |
| `/api/v1/user/points/history` | GET | 取得交易記錄 |

---

## API 總覽

### 公開 API (無需認證)
- `GET /api/v1/auth/google/check`
- `POST /api/v1/auth/login`
- `POST /api/v1/auth/register`
- `POST /api/v1/auth/google`
- `GET /api/v1/categories/*`
- `GET /api/v1/products/*`
- `GET /api/v1/skus/*`
- `GET /api/v1/sellers/*`
- `GET /api/v1/posts/*`
- `GET /api/v1/banners/*`
- `GET /api/v1/public/app-version-gate`
- `GET /api/v1/search/*`
- `GET /api/v1/coupons`
- `GET /api/v1/inventory/*`

### 需認證 API（/api/v1/app/*）
- `/api/v1/app/orders` - 訂單（買家/賣家透過 `?view=selling` 切換）
- `/api/v1/app/products` - 賣家商品管理
- `/api/v1/app/categories` - 賣家分類管理
- `/api/v1/app/carts` - 購物車
- `/api/v1/app/payments` - 支付
- `/api/v1/app/media` - 媒體上傳

### 需認證 API（/api/v1/*，非三大 Namespace）
- `/api/v1/user/*` - 用戶相關
- `/api/v1/chat/*` - 聯天
- `/api/v1/seller/*` - 賣家管理（優惠券、貼文、認證、CRM）
- `/api/v1/sellers/me` - 賣家個人資訊
- `/api/v1/seller/me/payuni/*` - PAYUNi 金流
- `/api/v1/ai/*` - AI 功能
