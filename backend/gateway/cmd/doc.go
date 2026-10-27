// Package main is the entry point for the Hawkr API Gateway.
//
// @title           Hawkr API Gateway
// @version         1.0
// @description     Hawkr 生鮮電商平台 API Gateway，提供帳號、商家、商品、訂單等完整服務
// @termsOfService  https://example.com/terms/
//
// @contact.name   Hawkr API Support
// @contact.url    https://example.com/support
// @contact.email  support@example.com
//
// @license.name  Apache 2.0
// @license.url   http://www.apache.org/licenses/LICENSE-2.0.html
//
// @host      localhost:9900
// @BasePath  /api/v1
//
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description JWT Token，格式：Bearer {token}
//
// @tag.name 認證 (Auth)
// @tag.description 用戶認證相關 API，包含 Google OAuth、電話驗證、Token 刷新等
//
// @tag.name 商家 (Seller)
// @tag.description 商家相關 API，包含商家資訊、商品、訂單管理等
//
// @tag.name 商品 (Product)
// @tag.description 商品瀏覽、搜尋、評價等
//
// @tag.name 訂單 (Order)
// @tag.description 訂單建立、查詢、支付等
//
// @tag.name 購物車 (Cart)
// @tag.description 購物車管理
//
// @tag.name 優惠券 (Coupon)
// @tag.description 優惠券領取與管理
//
// @tag.name 支付 (Payment)
// @tag.description 支付相關功能
//
// @tag.name 媒體 (Media)
// @tag.description 檔案上傳與下載
//
// @tag.name 搜尋 (Search)
// @tag.description 商品搜尋與建議
//
// @tag.name 內容 (Content)
// @tag.description 文章、食譜、橫幅等內容
//
// @tag.name 聊天 (Chat)
// @tag.description 即時聊天功能
//
// @tag.name 管理員 (Admin)
// @tag.description 管理員操作
//
// @tag.name 通知 (Notification)
// @tag.description 通知管理
//
// @tag.name 地址 (Address)
// @tag.description 收貨地址管理
//
// @tag.name 收藏 (Favorite)
// @tag.description 商品收藏
//
// @tag.name 瀏覽記錄 (History)
// @tag.description 瀏覽歷史
//
// @tag.name 裝置 (Device)
// @tag.description 裝置管理
//
// @tag.name 買家 (Buyer)
// @tag.description 買家資料
//
// @tag.name 分類 (Category)
// @tag.description 商品分類
//
// @tag.name 評論 (Review)
// @tag.description 商品評論
//
// @tag.name 留言 (Comment)
// @tag.description 貼文留言
//
// @tag.name 物流 (Shipping)
// @tag.description 物流出貨、標籤列印、物流建店、運費試算
//
// @tag.name AI
// @tag.description AI 功能
//
// @tag.name 商家商品 (Seller Product)
// @tag.description 商家商品管理
//
// @tag.name 商家CRM (Seller CRM)
// @tag.description 商家客戶管理
//
// @tag.name 貼文 (Post)
// @tag.description 貼文發布與互動
//
// @tag.name 討論區 (Forum)
// @tag.description 商家討論區
//
// @tag.name 訂閱 (Subscription)
// @tag.description 訂閱商家
//
// @tag.name 結帳 (Checkout)
// @tag.description 結帳流程
//
// @tag.name 開發工具 (Dev)
// @tag.description 開發環境工具
//
// @tag.name 商家治理 (Governance)
// @tag.description B-169 商家違規檢舉、食安通報、申訴流程
//
// @tag.name 健康檢查 (Health)
// @tag.description 系統健康檢查端點
package main
