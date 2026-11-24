# B4 初版分類清單草稿（D8）

來源：Shopify Standard Product Taxonomy `2026-11-unstable`（MIT，`https://raw.githubusercontent.com/Shopify/product-taxonomy/main/dist/en/categories.txt`，2026-11-24 下載，14,609 行）。每個 Shopify ID 都用 `grep` 從該檔取得【量】。注意：該 repo README 說 `dist/` 已標為 deprecated，正式匯入時改用 release asset（`releases/latest/download/categories.en.json.gz`）並釘版本。

## 設計前提（草稿裡發現、要一起確認）

1. **Shopify 的深度不一致**：例如「手機」在第 4 層（`el-4-8-5`），「洋裝」在第 3 層（`aa-1-4`），「背包」在第 2 層（`lb-1`）。所以「兩層、結構照 Shopify」做不到；草稿的做法是：**我們的兩層是對使用者的分組，每個葉節點對照一個 Shopify 節點**（`shopify_taxonomy_id`）。這等於追加簡報的 D 選項（內部結構自定、Shopify ID 當對照），與 D8 拍板的「結構照標準」不同，請確認。業界同樣做法：蝦皮、momo 的前台類目都是自己的分組，對外再對照 Google 類目【文件】。
2. **受管制品類**：ADR `2026-10-27-general-ecommerce-scope-cut` 的重評條件寫「要賣食品、保健品、化妝品、醫療器材時，再做品類資格審核」。草稿**不放**食品飲料、彩妝保養、保健醫療、菸酒（基準：Amazon gated categories）。若要放，就要先做資格審核，範圍變大。

## 草稿（10 個頂層、52 個葉節點）

| 頂層 | 葉節點 | Shopify ID | Shopify 名稱 |
|---|---|---|---|
| 女裝男裝 | 上衣 | `aa-1-13` | Clothing Tops |
| | 褲子 | `aa-1-12` | Pants |
| | 裙子 | `aa-1-15` | Skirts |
| | 洋裝 | `aa-1-4` | Dresses |
| | 外套 | `aa-1-10` | Outerwear |
| | 運動服 | `aa-1-1` | Activewear |
| | 睡衣居家服 | `aa-1-17` | Sleepwear & Loungewear |
| | 襪子 | `aa-1-18` | Socks |
| | 泳裝 | `aa-1-20` | Swimwear |
| 鞋包配件 | 鞋子 | `aa-8` | Shoes |
| | 皮夾手提包 | `aa-5` | Handbags, Wallets & Cases |
| | 飾品 | `aa-6` | Jewelry |
| | 服飾配件 | `aa-2` | Clothing Accessories |
| | 後背包 | `lb-1` | Backpacks |
| | 托特包 | `lb-13` | Tote Bags |
| | 行李箱 | `lb-12` | Suitcases |
| 3C 電子 | 手機 | `el-4-8-5` | Mobile & Smart Phones |
| | 手機配件 | `el-4-8-4` | Mobile & Smart Phone Accessories |
| | 平板 | `el-6-8` | Tablet Computers |
| | 筆電 | `el-6-6` | Laptops |
| | 耳機 | `el-2-2-7` | Headphones & Headsets |
| | 智慧手錶 | `aa-6-12` | Smart Watches |
| | 電玩主機 | `el-19` | Video Game Consoles |
| | 電子配件 | `el-7` | Electronics Accessories |
| 居家生活 | 家飾 | `hg-3` | Decor |
| | 寢具 | `hg-15` | Linens & Bedding |
| | 燈具 | `hg-13` | Lighting |
| | 收納 | `hg-10-16` | Storage & Organization |
| | 清潔用品 | `hg-10-6` | Household Cleaning Supplies |
| | 衛浴用品 | `hg-1` | Bathroom Accessories |
| | 園藝 | `hg-12` | Lawn & Garden |
| 廚房餐具 | 鍋具烘焙 | `hg-11-2` | Cookware & Bakeware |
| | 廚房工具 | `hg-11-8` | Kitchen Tools & Utensils |
| | 保鮮容器 | `hg-11-4` | Food Storage |
| | 廚房家電 | `hg-11-7` | Kitchen Appliances |
| 母嬰用品 | 童裝 | `aa-1-25` | Baby & Children's Clothing |
| | 哺育用品 | `bt-10` | Nursing & Feeding |
| | 尿布 | `bt-9` | Diapering |
| | 嬰兒推車 | `bt-6` | Baby Transport |
| 運動戶外 | 健身器材 | `sg-2` | Fitness & General Exercise Equipment |
| | 露營登山 | `sg-4-2` | Camping & Hiking |
| | 單車 | `sg-4-4` | Cycling |
| | 水上運動 | `sg-4-1` | Boating & Water Sports |
| 寵物用品 | 貓用品 | `ap-2-2` | Cat Supplies |
| | 狗用品 | `ap-2-3` | Dog Supplies |
| 文具辦公 | 文具 | `os-4` | General Office Supplies |
| | 收納歸檔 | `os-3` | Filing & Organization |
| 玩具遊戲 | 積木 | `tg-5-7` | Building Toys |
| | 公仔娃娃 | `tg-5-8` | Dolls, Playsets & Toy Figures |
| | 桌遊 | `tg-2` | Games |
| | 拼圖 | `tg-4` | Puzzles |
| | 益智玩具 | `tg-5-9` | Educational Toys |

說明：
- 寵物食品在 `ap-2-2-1`、`ap-2-3-3` 之下，屬「貓／狗用品」，不另立葉節點（寵物食品在臺灣也受飼料管理法規範，資格審核另議【推】）。
- en 名稱用 Shopify 名稱；zh-TW 名稱照 `docs/共通規範/使用者面用語術語表.md` 校過再定案（B4-2）。
- 「其他」類不設：每個商品必須掛到明確葉節點；找不到時是分類法要擴充的訊號。

## 要 Will 確認的三件事

1. 結構：接受「我們的兩層分組＋每個葉節點對照 Shopify ID」（D 選項），而不是照 Shopify 結構。
2. 受管制品類：初版不放食品、彩妝保養、保健醫療、菸酒。
3. 葉節點清單本身（增刪、改名）。
