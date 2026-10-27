package io.github.will413028.hawkr.common.utils.code;

import java.util.Arrays;
import java.util.Map;
import java.util.stream.Collectors;

/**
 * 跨服務統一 gRPC Response Code。
 *
 * <p>適用於所有使用 int32 code 模式的 gRPC Response。
 * 成功時 code=0，失敗時使用對應錯誤碼。</p>
 *
 * <p>{@link #message()} 為 API 回應的 {@code message} 欄位值（前端顯示值），
 * 禁止在各服務中硬編碼錯誤訊息字串。</p>
 *
 * <p>含 {@code %s} 的 message 為模板，由 BusinessException 透過
 * {@code String.format(message, args)} 組裝動態訊息。</p>
 *
 * <p>⚠ 此檔案由 generate.sh 自動生成，請以 docs/代碼管理（CodeMeta）/錯誤碼.md 為唯一來源。</p>
 */
public enum GrpcResponseCode {

    // ===== 通用錯誤碼（所有服務共用） =====
    SUCCESS(0, "success"),
    INVALID_PARAM(400, "參數錯誤"),
    UNAUTHORIZED(401, "未登入"),
    PERMISSION_DENIED(403, "無權操作"),
    NOT_FOUND(404, "資源不存在"),
    CONFLICT(409, "資源衝突"),
    INTERNAL_ERROR(500, "系統錯誤"),
    NOT_IMPLEMENTED(501, "功能暫不支援"),
    SERVICE_UNAVAILABLE(503, "服務暫時不可用"),

    // ===== Order 業務碼（4001~4099） =====
    INSUFFICIENT_STOCK(4001, "商品不可購買 — Checkout Preview / Confirm 時若 `data.unavailable_items` 非空，本碼為 envelope code；前端依 `unavailable_items[].reason`（`CartItemUnavailableReason`：DELISTED / OUT_OF_STOCK / INSUFFICIENT_STOCK / LOOKUP_FAILED）區分文案。Name 維持 `INSUFFICIENT_STOCK` 不重構，避免擴大改動範圍"),
    INVALID_STATUS(4002, "狀態不允許此操作"),
    CHECKOUT_EXPIRED(4006, "結帳已過期"),
    CHECKOUT_VALIDATION_FAILED(4007, "結帳驗證失敗：%s"),
    SHIPPING_METHOD_UNAVAILABLE(4008, "所選配送方式不可用"),
    SELLER_NOT_ONBOARDED(4009, "賣家尚未開通此配送方式"),
    SELLER_SHIPPING_INCOMPLETE(4010, "賣家物流設定不完整，請聯繫賣家"),
    COUPON_LOCK_FAILED(4011, "優惠券鎖定失敗：%s"),
    SKU_NOT_FOUND(4012, "商品不存在"),
    COUPON_NOT_AVAILABLE(4013, "優惠券不可用：%s"),
    ORDER_MISSING_TRANSACTION(4014, "訂單缺少交易編號，無法退款"),
    SELF_PURCHASE_NOT_ALLOWED(4015, "不可購買自己上架的商品"),
    SHIPPING_ALL_METHODS_EXCEED_LIMITS(4016, "商品尺寸或重量超過所有已開通配送方式的限制"),
    CHECKOUT_PRICE_CHANGED(4091, "訂單金額已變動"),
    ORDER_ALREADY_CREATED(4092, "您已有未付款訂單：%s，請先完成付款或取消後再下單"),

    // ===== Shipping 業務碼（5001~5099） =====
    SHIPMENT_NOT_FOUND(5001, "物流單不存在"),
    SHIPMENT_NO_TRADE_NO(5002, "物流單尚未取得序號，無法操作"),
    SHIPMENT_ALREADY_LABELED(5003, "已取號，不可重複產生"),
    SHIPMENT_SELLER_NOT_ONBOARDED(5004, "賣家尚未開通「%s」物流服務，請先完成建店申請"),
    SHIPMENT_PERMISSION_DENIED(5005, "賣家帳號不符，無權操作此物流單"),
    SHIPMENT_DATA_INCOMPLETE(5006, "訂單資料不完整：%s"),
    SHIPMENT_DATE_MUST_FUTURE(5007, "出貨日期必須大於今日"),
    SHIPMENT_DATE_HOLIDAY(5008, "%s %s 為假日（%s），請選擇工作天"),
    SHIPMENT_DATE_OUT_OF_RANGE(5009, "出貨日期超出可選範圍（最遠至 %s）"),
    SHIPMENT_DELIVERY_BEFORE_SHIP(5010, "配達日期必須大於出貨日期"),
    SHIPMENT_LABEL_REQUIRED(5011, "尚未取號，請先產生 PDF 後再呼叫黑貓"),
    SHIPMENT_TCAT_ALREADY_CALLED(5012, "已呼叫過黑貓，不可重複呼叫"),
    SHIPMENT_CONCURRENT_CONFLICT(5013, "操作失敗（併發衝突），請重試"),
    SHIPMENT_UNSUPPORTED_METHOD(5014, "不支援的配送方式：%s"),
    SHIPMENT_ORDER_QUERY_FAILED(5015, "訂單查詢失敗：%s"),
    BUYER_STORE_PICKUP_NOT_FOUND(5016, "常用門市不存在或無權限操作"),
    SHIPPING_ONBOARDING_NOT_FOUND(5017, "建店設定不存在"),
    SHIPMENT_PROVIDER_CREATE_FAILED(5018, "物流商建單失敗：%s"),
    RETURN_SHIPMENT_CREATE_FAILED(5019, "退貨物流建單失敗：%s"),
    SHIPPING_ONBOARDING_FAILED(5020, "物流建店失敗：%s"),
    SHIPPING_ONBOARDING_VIEW_DENIED(5021, "無權查詢此建店設定"),
    SHIPPING_ONBOARDING_UPDATE_DENIED(5022, "無權操作此建店設定"),
    SHIPPING_SELLER_META_NOT_FOUND(5023, "找不到賣家資訊"),
    SHIPPING_SELLER_PAYUNI_NOT_REGISTERED(5024, "賣家尚未註冊 PayUni 商家"),
    SHIPPING_ONBOARDING_PENDING_REVIEW(5025, "已有待審核的建店申請，請等待審核結果"),
    SHIPPING_ONBOARDING_PERMANENTLY_CLOSED(5026, "此物流類型已被永久關閉，無法重新申請"),
    SHIPPING_ONBOARDING_CLOSED(5027, "此物流類型已關閉，請聯繫客服"),
    SHIPPING_SELLER_META_QUERY_FAILED(5028, "賣家資訊查詢失敗：%s"),
    SHIPPING_GOODS_TYPE_INVALID(5029, "無效的貨物類型：%s，僅支援 常溫 / 冷凍"),
    SHIPPING_RETURN_STORE_CODE_INVALID(5030, "退貨門市代碼格式錯誤，需為 6 位數字"),
    SHIPPING_PRODUCT_CATEGORY_INVALID(5031, "商品類別代碼格式錯誤，需為 4 位數字（0001~0015）"),
    BUYER_STORE_PICKUP_LIMIT_EXCEEDED(5032, "常用門市數量已達上限（%s）"),
    SHIPMENT_LABEL_SCHEDULE_EMPTY(5033, "PayUni 預約取號回傳為空"),
    HOLIDAY_CSV_EMPTY(5034, "CSV 無有效假日資料"),
    SHIPMENT_LABEL_SCHEDULE_FAILED(5035, "PayUni 預約取號失敗：%s"),
    SHIPMENT_LABEL_FORM_ACTION_MISSING(5036, "無法從 PayUni HTML 回應中解析 form action URL"),
    SHIPMENT_LABEL_FORM_PARAMS_MISSING(5037, "無法從 PayUni HTML 回應中解析 form 參數"),
    SHIPMENT_LABEL_PDF_DOWNLOAD_FAILED(5038, "PDF 下載失敗"),
    SHIPMENT_LABEL_PDF_RESPONSE_INVALID(5039, "PDF 下載回應異常"),
    SHIPMENT_LABEL_FILE_DOWNLOAD_FAILED(5040, "檔案下載失敗"),
    HOLIDAY_CSV_DOWNLOAD_FAILED(5041, "CSV 下載失敗"),
    HOLIDAY_CSV_PARSE_FAILED(5042, "CSV 解析失敗"),
    SHIPMENT_TCAT_CALL_FAILED(5044, "呼叫黑貓失敗：%s"),
    SHIPMENT_NOT_A_RETURN(5045, "此物流單非退貨單，不可執行退貨列印重試"),
    SHIPMENT_PROVIDER_CALL_IN_PROGRESS(5047, "物流商呼叫進行中，請稍候再試"),
    SHIPMENT_PROVIDER_CALL_FAILED(5048, "物流商呼叫失敗：%s"),
    SHIPMENT_DATE_INVALID_FORMAT(5049, "%s 格式錯誤，需為 yyyy-MM-dd 或 yyyyMMdd：%s"),
    HOLIDAY_CSV_YEAR_MISMATCH(5050, "上傳的 CSV 不含指定年份 %s 的假日資料，請確認檔案與年份是否相符"),

    // ===== Cart 業務碼（6001~6099） =====
    CART_ITEM_NOT_FOUND(6001, "找不到購物車項目"),
    INVALID_SKU(6002, "商品不存在或已下架"),
    CART_ITEM_LOCKED(6003, "商品處於結帳鎖定狀態"),
    CHECKOUT_LOCK_FAILED(6005, "結帳鎖定失敗"),
    CART_SELF_ITEM_NOT_ALLOWED(6006, "不可將自己上架的商品加入購物車"),
    CART_PRODUCT_DATA_INVALID(6007, "商品資訊格式異常：%s"),
    CART_SKU_IDS_REQUIRED(6008, "sku_ids 不可為空"),
    CART_SKU_IDS_DUPLICATED(6009, "sku_ids 不可重複"),
    CART_QUANTITY_EXCEEDS_MAX_LIMIT(6010, "您已達該商品購買上限（上限 %d 件）"),

    // ===== Coupon 業務碼（7001~7099） =====
    COUPON_NOT_FOUND(7001, "優惠券不存在"),
    COUPON_EXPIRED(7002, "優惠券已過期"),
    COUPON_ALREADY_USED(7003, "優惠券已使用"),
    COUPON_NOT_APPLICABLE(7004, "優惠券不適用"),
    COUPON_QUOTA_EXCEEDED(7005, "賣家優惠券配額已滿"),
    COUPON_ISSUE_LIMIT_REACHED(7006, "優惠券發行上限已達"),
    COUPON_ALREADY_RECEIVED(7007, "已領取過此優惠券"),
    COUPON_RESERVE_FAILED(7008, "優惠券鎖定失敗"),
    COUPON_INVALID_STATUS(7009, "優惠券狀態不允許此操作"),
    COUPON_SELF_ISSUE_NOT_ALLOWED(7010, "不可將優惠券發給自己"),
    COUPON_NOT_CLAIMED(7011, "此優惠券尚未領取"),
    COUPON_CLAIM_NOT_REQUIRED(7012, "此優惠券不需要領取"),
    COUPON_PUBLISH_REQUIRES_DRAFT(7013, "僅 DRAFT 狀態可發佈"),
    COUPON_DISABLE_REQUIRES_ACTIVE(7014, "僅 ACTIVE 狀態可停用"),
    COUPON_CASH_DISCOUNT_REQUIRED(7015, "CASH 券需指定 discountAmount"),
    COUPON_PERCENT_OUT_OF_RANGE(7016, "PERCENT 券 discountPercent 範圍 1-99"),
    COUPON_PERCENT_MAX_DISCOUNT_REQUIRED(7017, "PERCENT 券需指定 maxDiscountAmount"),
    COUPON_TEMPLATE_TIME_INVALID(7019, "%s"),
    COUPON_CLAIM_REQUIRES_ACTIVE(7020, "優惠券模板非 ACTIVE 狀態"),
    COUPON_CLAIM_OUT_OF_WINDOW(7021, "優惠券不在有效期間"),
    COUPON_DISTRIBUTE_REQUIRES_ACTIVE(7022, "僅 ACTIVE 模板可執行代領"),
    COUPON_LOCKED_BY_OTHER_ORDER(7023, "此優惠券已被另一筆訂單鎖定，請取消後再試"),

    // ===== Payment 業務碼（8001~8099） =====
    PAYMENT_ALREADY_EXISTS(8001, "重複付款"),
    INVALID_PAYMENT_TYPE(8002, "不支援的支付方式：%s"),
    PAYMENT_STATUS_INVALID(8003, "交易狀態不允許此操作：%s"),
    REFUND_ALREADY_PROCESSING(8004, "該交易已有進行中的退款申請"),
    REFUND_BANK_INFO_REQUIRED(8005, "ATM 退款必須提供銀行帳戶資訊"),
    QUERY_TRANSACTION_FAILED(8006, "查詢交易狀態失敗：%s"),
    GRANT_FAILED(8007, "撥款指示失敗：%s"),
    ATM_CREATION_FAILED(8008, "ATM 建立失敗：%s"),
    CLOSE_STATUS_NOT_REFUNDABLE(8009, "交易請款狀態不允許退款：%s"),
    CLOSE_STATUS_UNAVAILABLE(8010, "無法取得請款狀態，交易可能非信用卡"),
    INVALID_MERCHANT_ID(8011, "代理商模式下必須提供子商家 MerID"),
    ORDER_QUERY_FAILED(8012, "查詢訂單失敗：%s"),
    NO_PERMISSION_FOR_PAYMENT(8013, "無權為此訂單付款"),
    ORDER_STATUS_NOT_PAYABLE(8014, "訂單狀態不允許付款：%s"),
    TRANSACTION_NOT_FOUND_FOR_ORDER(8015, "找不到訂單的付款交易"),
    TRANSACTION_NOT_PAID(8016, "交易尚未付款，無法撥款：%s"),
    CLOSE_STATUS_CANCELED(8017, "交易請款已取消，不可退款"),
    CLOSE_STATUS_NOT_APPLIED(8018, "交易尚未請款，不可退款"),
    CLOSE_STATUS_UNKNOWN(8019, "未知的請款狀態：%s"),
    PAYMENT_TYPE_REQUIRED(8020, "payment_type 不可為空"),
    PAYMENT_METHOD_AMOUNT_OUT_OF_RANGE(8021, "所選付款方式不支援此訂單金額：付款方式=%s、訂單金額=%d 元、允許範圍=%d~%d 元"),
    PAYMENT_DUPLICATE_DETECTED(8023, "偵測到重複付款，已自動觸發退款流程"),
    PAYMENT_AMOUNT_MISMATCH(8024, "重複付款金額不一致：existingPaid=%d, new=%d"),
    UNSUPPORTED_PAYMENT_TYPE_FOR_DUPLICATE(8025, "未支援的付款方式無法進入重複付款處理流程：%s"),
    ORDER_AMOUNT_EXCEEDS_LIMIT(8026, "訂單金額過高（%d 元），超出系統支援上限（%d 元），請分批下單或聯繫客服"),
    PAYMENT_ZERO_AMOUNT_NOT_REFUNDABLE(8027, "零元訂單無退款流程，請改由訂單退貨/取消流程處理"),
    APPLE_PAY_VALIDATION_URL_REJECTED(8028, "Apple Pay 驗證網址不符合白名單"),
    APPLE_PAY_CERT_LOAD_FAILED(8029, "Apple Pay 憑證載入失敗"),
    APPLE_PAY_MERCHANT_VALIDATION_FAILED(8030, "Apple Pay merchant 驗證失敗"),

    // ===== Product 業務碼（3001~3099） =====
    CERT_NO_REQUIRED(3001, "認證編號不可為空"),
    CERT_TYPE_INVALID(3002, "不合法的認證類型"),
    CERT_TYPE_NOT_VERIFIABLE(3003, "此認證類型暫不支援線上驗證"),
    GOV_CERT_API_UNAVAILABLE(3004, "無法連線至農業部資料查詢服務"),
    GOV_CERT_API_RESPONSE_INVALID(3005, "農業部資料查詢服務回應格式異常"),
    CATEGORY_NAME_RESERVED(3006, "名稱可能存在不合法字元，請檢查後再新增。如有其他問題請聯絡客服。"),
    CATEGORY_HAS_PRODUCTS(3007, "分類底下尚有商品，請先移除或轉移後再操作"),
    CATEGORY_HAS_CHILDREN(3008, "分類底下尚有啟用中子分類，請先處理子分類"),
    CATEGORY_NOT_DELETED(3009, "該分類非軟刪除狀態，無法還原"),
    CATEGORY_NAME_TAKEN(3010, "同名同父分類已存在，無法還原"),
    CATEGORY_PARENT_DELETED(3011, "父分類已被軟刪除，無法還原"),
    CATEGORY_LEVEL_INVALID(3012, "分類層級不符（批次更換來源/目標需為第三層）"),
    CATEGORY_NOT_FOUND(3013, "分類不存在或已被軟刪除"),
    INVALID_DESCRIPTION_HTML_FORBIDDEN(3014, "商品描述不可包含 HTML 標籤，請輸入純文字"),
    HASHTAG_INVALID_CHAR(3015, "hashtag 含不允許字元（僅接受中文/英文/數字，不可含空白、`#`、emoji 或標點）"),
    HASHTAG_LENGTH_EXCEEDED(3016, "hashtag 長度超出範圍（每個 hashtag 須為 1–40 字元）"),
    HASHTAG_COUNT_EXCEEDED(3017, "hashtag 數量超出上限（每個商品最多 10 個）"),
    HASHTAG_EMPTY_AFTER_NORMALIZE(3018, "hashtag 經規範化後為空，請重新輸入"),
    PRODUCT_EDIT_REQUIRES_INACTIVE(3019, "商品需先下架後再編輯（目前狀態：%s）"),
    INVALID_DESCRIPTION_HREF(3020, "商品描述連結網址格式不支援（僅接受 http/https）"),
    PRODUCT_NOT_FOUND(3021, "商品不存在"),
    PRODUCT_NOT_PUBLISHED(3022, "商品從未公開"),
    PRODUCT_REMOVED(3023, "商品已永久移除"),
    PRODUCT_SKU_NOT_FOUND(3024, "SKU 不存在或不屬於此商品（skuId: %s）"),
    CERT_APPLICATION_NOT_FOUND(3025, "認證申請不存在"),
    CERT_APPLICATION_VERSION_CONFLICT(3026, "認證申請已被其他操作變更，請重新載入後再試"),
    CERT_APPLICATION_STATE_INVALID(3027, "認證申請狀態不允許此操作"),
    CERT_REVIEW_REASON_REQUIRED(3028, "請選擇駁回或撤銷原因"),
    CERT_APPLICATION_DUPLICATE_PENDING(3029, "此商品的該認證類型已有待審申請"),
    CERT_REVERIFY_COOLDOWN(3030, "重新查驗冷卻中：%s。同一筆申請 %s 秒內只會實際查詢一次，避免重複消耗農業部每日配額"),

    // ===== IM 業務碼（9201~9202） =====
    SELF_MESSAGE_NOT_ALLOWED(9201, "不能傳訊息給自己"),
    BLOCKED_MESSAGE_NOT_ALLOWED(9202, "雙向封鎖時無法傳訊"),

    // ===== Account 自助刪除業務碼（9203~9209） =====
    BUYER_ACCOUNT_DELETING(9203, "帳戶刪除中，無法進行新交易"),
    ACCOUNT_ALREADY_IN_DELETION(9204, "帳號已在刪除流程中"),
    SELLER_HAS_ACTIVE_ORDERS(9205, "您有 %s 筆未完成訂單，請待結案後再申請刪除帳號"),
    RECEIVER_ACCOUNT_DELETED(9206, "IM：對方帳號已刪除，無法傳送訊息"),
    STORE_CLOSED_MESSAGE_NOT_ALLOWED(9207, "此商店已停業，無法傳送訊息"),

    // ===== 內容審核業務碼（9210~9219） =====
    MODERATION_REJECTED(9210, "內容不符社群守則"),
    REPORT_RATE_LIMITED(9211, "檢舉過於頻繁，請稍後再試"),

    // ===== Seller 媒體業務碼（9220~9229） =====
    MEDIA_OBJECT_KEY_REQUIRED(9220, "logo / cover_url 僅能透過頭像、封面圖片上傳流程設定，不可直接指定外部網址"),

    // ===== Seller 業務碼（9230~9239） =====
    SELLER_NOT_ACTIVE(9230, "賣家未啟用，無法補登食品業者登錄字號"),
    FOOD_BUSINESS_REG_NO_INVALID(9231, "食品業者登錄字號格式不正確或未填"),
    SELLER_WEBSITE_URL_INVALID(9232, "網站網址格式不正確"),
    SELLER_NOT_FOUND(9233, "找不到您的商家資料"),
    PAYUNI_MERCHANT_PROFILE_VERSION_CONFLICT(9234, "資料已被更新，請重新整理後再試"),
    PAYUNI_MERCHANT_PROFILE_IDEMPOTENCY_KEY_REUSED(9235, "這個操作已經處理過，但內容不同，請重新操作"),
    PAYUNI_MERCHANT_PROFILE_NOT_ACTIVE(9236, "商家尚未完成 PayUni 綁定或審核中，暫不可修改聯絡資訊"),

    // ===== 賣家停權業務碼（9240~9249） =====
    SELLER_SUSPENDED_OPERATION_BLOCKED(9240, "賣家帳號停權中，無法執行此操作"),
    SELLER_SUSPENDED_PRODUCT_UNAVAILABLE(9241, "賣家停權中，商品暫停販售"),

    // ===== AI 修圖業務碼（9250~9259） =====
    AI_TRANSFORM_NO_IMAGE(9250, "這次沒有成功產生圖片，請再試一次"),
    AI_TRANSFORM_SAFETY_BLOCKED(9251, "這張照片或描述沒有通過內容審查，請換一張照片或修改描述"),
    AI_TRANSFORM_RECITATION(9252, "產生的圖片與版權內容過於相似，請換一張照片或修改描述"),
    AI_PROVIDER_UNAVAILABLE(9253, "AI 美化暫時無法使用，請稍後再試"),
    AI_QUOTA_EXCEEDED(4291, "本週 AI 美化次數已用完（%d/%d），下週一重置"),

    // ===== 代行營運業務碼（9260~9299） =====
    DELEGATION_INVALID_COMMAND(9260, "代行請求參數不正確"),
    DELEGATION_ADMIN_SESSION_REQUIRED(9261, "管理員登入已失效，請重新登入"),
    DELEGATION_CAPABILITY_DENIED(9262, "此代行工作階段未取得該項操作權限"),
    DELEGATION_TARGET_MISMATCH(9263, "代行對象店家與請求的店家不符"),
    DELEGATION_AUTHORIZATION_REVOKED(9264, "此店家的代行營運授權已撤回或已到期"),
    DELEGATION_STORE_NOT_DELEGABLE(9265, "此店家目前不可代行"),
    DELEGATION_RESOURCE_VERSION_CONFLICT(9266, "資料已被更新，請重新載入後再試"),
    DELEGATION_IDEMPOTENCY_KEY_REUSED(9267, "冪等鍵已用於不同的請求內容"),
    DELEGATION_CONFLICT_ACTOR_OTHER_STORE(9268, "您已有進行中的代行工作階段，請先結束"),
    DELEGATION_CONFLICT_STORE_OTHER_ACTOR(9269, "此店家已有其他管理員正在代行"),
    DELEGATION_SESSION_EXPIRED(9270, "代行工作階段已逾時"),
    DELEGATION_SESSION_REVOKED(9271, "代行工作階段已被撤銷"),
    DELEGATION_STORE_NOT_OPERABLE(9272, "此店家已停權或關閉，無法代行"),
    DELEGATION_RATE_LIMITED(9273, "操作過於頻繁，請稍後再試"),
    DELEGATION_CREDENTIAL_INVALID(9274, "代行憑證無效，請重新建立工作階段"),
    DELEGATION_RESOURCE_NOT_OWNED(9275, "此資源不屬於代行對象店家"),
    DELEGATION_BATCH_PARTIAL_FAILED(9276, "批次操作未執行"),
    DELEGATION_SESSION_NOT_FOUND(9277, "找不到指定的代行工作階段"),
    DELEGATION_AUDIT_INTENT_FAILED(9279, "稽核紀錄寫入失敗，操作未執行"),
    DELEGATION_UPSTREAM_UNAVAILABLE(9280, "代行營運服務暫時無法使用，請稍後再試"),
    DELEGATION_ACTOR_MISMATCH(9281, "此代行工作階段不屬於目前登入的管理員"),
    DELEGATION_HANDOFF_INVALID(9282, "此代行連結已失效，請回管理後台重新開始"),
    DELEGATION_SCHEDULED_POST_NOT_PUBLISHABLE(9283, "排程發布時貼文已不符合發布條件"),
    DELEGATION_SCHEDULED_POST_RETRY_EXHAUSTED(9284, "排程發布重試次數已用盡"),
    DELEGATION_AUTHORIZATION_ALREADY_ACTIVE(9285, "此店家已開通代行營運授權"),
    DELEGATION_AUTHORIZATION_NOT_FOUND(9286, "找不到此店家的代行營運授權設定"),
    DELEGATION_CAPABILITY_DEPENDENCY_INCOMPLETE(9287, "代行能力組合不完整：選擇編輯或發布時必須同時包含對應的檢視權限"),
    DELEGATION_ADMIN_NOT_ALLOWLISTED(9288, "此管理員未開通代行營運治理權限"),
    DELEGATION_INTERNAL_CONTRACT_MISMATCH(9289, "代行營運服務設定異常，重試無效"),
    DELEGATION_RESOURCE_ALREADY_IN_STATE(9290, "此資源已處於你要求的狀態"),
    DELEGATION_FEATURE_DISABLED(9291, "代行營運功能目前未啟用"),

    // ===== Invoice 業務碼（9300~9399） =====
    INVOICE_DOCUMENT_NOT_FOUND(9301, "文件不存在"),
    INVOICE_PROFILE_NOT_FOUND(9302, "常用發票資料不存在"),
    INVOICE_PROFILE_LIMIT_EXCEEDED(9303, "常用發票資料已達 20 筆上限，請先刪除不需要的項目"),
    INVOICE_PROFILE_DEFAULT_REQUIRED(9304, "請先指定新的預設發票資料，才能刪除目前的預設項目"),
    INVOICE_CARRIER_INVALID(9305, "載具、統一編號或捐贈碼格式不正確"),
    INVOICE_TARGET_CONFLICT(9306, "統一編號、載具與捐贈碼三者只能擇一"),
    INVOICE_CORRECTION_NOT_ALLOWED(9307, "本張發票目前無法更正"),
    INVOICE_SNAPSHOT_VERSION_CONFLICT(9308, "發票資料已被更新，請重新整理後再試"),
    INVOICE_COMPLAINT_NOT_FOUND(9309, "申訴案件不存在"),
    INVOICE_COMPLAINT_DUPLICATE(9310, "本訂單已有處理中的申訴案件"),
    INVOICE_COMPLAINT_NOT_OPEN_YET(9311, "尚未到可提出發票申訴的時間"),
    INVOICE_PDF_UNAVAILABLE_CARRIER(9312, "載具發票於中獎前不提供發票檔案下載"),
    INVOICE_PDF_EXPIRED(9313, "已超過發票檔案的查詢期限"),
    INVOICE_DISCLOSURE_NOT_ACCEPTED(9314, "請先閱讀並同意電子發票法定告知事項"),
    SELLER_TAX_PROFILE_NOT_FOUND(9320, "賣家稅務資格資料不存在"),
    SELLER_TAX_PROFILE_STATE_INVALID(9321, "目前的稅務資格狀態不允許此操作"),
    SELLER_TAX_PROFILE_VERSION_CONFLICT(9322, "稅務資格資料已被更新，請重新整理後再試"),
    SELLER_TAX_EXTENSION_ALREADY_USED(9323, "審核期限已延長過一次，無法再次延長"),
    SELLER_TAX_REAPPLY_COOLDOWN(9324, "重新申請需等待冷卻期結束"),
    SELLER_SELF_ISSUE_TERMS_NOT_ACCEPTED(9325, "請先接受自行開立發票責任條款"),
    SELLER_INVOICE_INTAKE_LIMIT_REACHED(9326, "賣家於稅務資格審核期間的接單上限已達，暫時無法下單"),
    SELLER_INVOICE_CONSOLIDATED_NOT_SUPPORTED(9327, "合併結帳的訂單目前無法自動開立發票，請分批結帳"),
    INVOICE_PROVIDER_ACCOUNT_NOT_FOUND(9330, "尚未設定電子發票服務商連線"),
    INVOICE_PROVIDER_CREDENTIAL_INVALID(9331, "電子發票服務商憑證驗證失敗，請確認統一編號與 AppKey"),
    INVOICE_PROVIDER_UNAVAILABLE(9332, "電子發票服務商暫時無法連線，請稍後再試"),
    INVOICE_PROVIDER_STATE_INVALID(9333, "憑證尚未驗證通過，無法啟用或輪替"),
    INVOICE_PROVIDER_TAXID_MISMATCH(9334, "送出的統一編號與已核定的稅籍資料不符"),
    INVOICE_RECEIPT_AUTH_NOT_ELIGIBLE(9335, "目前的稅籍審核狀態無法授權平台代製收據"),
    INVOICE_COMPLAINT_ACTION_NOT_ALLOWED(9336, "這件申訴目前的狀態無法進行這個動作"),
    INVOICE_PROVIDER_CREDENTIAL_TAKEN(9337, "這組統一編號與 AppKey 已被其他帳戶綁定，請確認是否重複設定或聯絡客服"),
    INVOICE_JOB_NOT_FOUND(9340, "開票作業不存在"),
    INVOICE_JOB_NOT_RETRYABLE(9341, "本開票作業目前無法安全重試"),
    INVOICE_JOB_CANCEL_BLOCKED_UNKNOWN(9342, "服務商端結果尚未確認，無法取消本作業"),
    INVOICE_JOB_VERSION_CONFLICT(9343, "開票作業已被更新，請重新整理後再試"),
    INVOICE_MANUAL_UNLOCK_NEEDS_SECOND_APPROVAL(9344, "尚需第二位管理員確認才能解鎖人工處理"),
    INVOICE_MANUAL_UNLOCK_SAME_ACTOR(9345, "雙人確認不得由同一位管理員完成"),
    INVOICE_ISSUANCE_TARGET_INVALID(9346, "開票人與受票人的組合不合法"),
    INVOICE_DUPLICATE(9347, "相同的開票請求已建立過文件"),
    INVOICE_TAX_DECISION_INVALID(9348, "此審核決定與目前的稅務資格狀態不相容"),
    INVOICE_EXTERNAL_ISSUANCE_NOT_ALLOWED(9349, "本作業尚未開放自行開立回報，請等待平台通知"),
    INVOICE_IDEMPOTENCY_PAYLOAD_MISMATCH(9350, "相同的冪等鍵帶了不同的請求內容"),
    INVOICE_OPERATION_IN_PROGRESS(9351, "本張發票有處理中的作業，請稍後再試"),
    INVOICE_NOTIFICATION_RESEND_TOO_FREQUENT(9352, "通知重寄過於頻繁，請稍後再試"),
    INVOICE_TAX_APPROVAL_IDENTITY_INCOMPLETE(9353, "賣家尚未填寫統一編號與法定名稱，無法核定為應開統一發票，請先要求補件"),
    INVOICE_TAX_APPROVAL_RECEIPT_AUTH_MISSING(9354, "賣家尚未完成平台代製收據授權，無法核定為免用統一發票"),

    // ===== Buyer Membership 業務碼（9400~9409） =====
    INVALID_PLAN_CODE(9400, "無此方案代碼"),

    // ===== Seller Membership 業務碼（9410~9419） =====
    SELLER_INVALID_PLAN_CODE(9410, "無此賣家方案代碼"),

    // ===== Product Import 業務碼（1100~1199） =====
    PRODUCT_IMPORT_JOB_NOT_FOUND(1101, "找不到匯入任務"),
    PRODUCT_IMPORT_ITEM_NOT_FOUND(1102, "找不到匯入商品項"),
    PRODUCT_IMPORT_FILE_TOO_LARGE(1103, "CSV 檔案超過大小上限（%s bytes）"),
    PRODUCT_IMPORT_FILE_TOO_MANY_ROWS(1104, "CSV row 數超過上限（%s 筆）"),
    PRODUCT_IMPORT_FILE_ENCODING_UNSUPPORTED(1105, "CSV 編碼非 UTF-8，請以 UTF-8 重新匯出"),
    PRODUCT_IMPORT_FILE_HEADER_INVALID(1106, "CSV header 缺必要欄位：%s"),
    PRODUCT_IMPORT_RATE_LIMITED(1107, "同時進行的匯入任務或每日上傳次數已達上限"),
    PRODUCT_IMPORT_FILENAME_CONFLICT(1108, "同名 CSV 任務尚未結案，請先處理既有任務或更改檔名"),
    PRODUCT_IMPORT_JOB_WRONG_STATUS(1109, "任務目前狀態不允許此操作（%s）"),
    PRODUCT_IMPORT_ITEM_WRONG_STATUS(1110, "商品項目前狀態不允許編輯"),
    PRODUCT_IMPORT_OPTIMISTIC_LOCK_FAILED(1111, "資料已被其他操作更新，請重新整理後再試"),
    PRODUCT_IMPORT_NO_ELIGIBLE_ITEMS(1112, "沒有可上架的商品項，請先修正錯誤後再 commit"),
    PRODUCT_IMPORT_CATEGORY_INVALID(1113, "商品分類非合法葉節點"),
    PRODUCT_IMPORT_CONFIRMATION_TOKEN_INVALID(1114, "分類二次確認 token 無效或已過期，請重新選擇分類"),
    PRODUCT_IMPORT_ITEM_ALREADY_COMMITTED(1115, "該匯入商品已併入正式商品，無法再編輯或移除"),

    // ===== AfterSales 業務碼（9001~9099） =====
    AFTERSALE_NOT_FOUND(9001, "售後單不存在"),
    AFTERSALE_DUPLICATE(9002, "該訂單已有進行中的售後申請"),
    AFTERSALE_EXPIRED(9003, "已超過售後申請時限"),
    AFTERSALE_INVALID_STATUS(9004, "售後單狀態不允許此操作"),
    REFUND_AMOUNT_EXCEEDED(9006, "退款金額超過可退款上限"),
    COUPON_RULE_VIOLATED(9007, "優惠券不符合發放規則"),
    CANCEL_NOT_ALLOWED(9008, "訂單狀態不允許取消出貨"),
    CANCEL_DUPLICATE(9009, "該訂單已有取消出貨申請"),
    AFTERSALE_NOT_OWNER(9010, "無權操作此售後單"),
    RETURN_SHIPMENT_FAILED(9011, "建立退貨物流失敗：%s"),
    REFUND_PAGE_NOT_AVAILABLE(9012, "非 ATM 付款或退款任務狀態不允許"),
    AFTERSALE_LABEL_ALREADY_GENERATED(9013, "出貨便代碼已產生，無需重試"),
    AFTERSALE_LABEL_RETRY_NOT_APPLICABLE(9014, "售後案件狀態不允許重試列印（需為 7-11 退貨且處於待退貨 / 退貨運送中狀態）"),
    AFTERSALE_APPEAL_NOT_APPEALABLE(9015, "目前狀態無法申訴"),
    AFTERSALE_APPEAL_ALREADY_SUBMITTED(9016, "此案件已提交過申訴"),
    AFTERSALE_APPEAL_DEADLINE_EXCEEDED(9017, "申訴期限已過"),
    AFTERSALE_CASE_UNDER_APPEAL(9018, "案件由平台處理中，無法操作"),
    AFTERSALE_APPEAL_STATE_MISMATCH(9019, "申訴狀態不符預期（可能為並發衝突）"),
    AFTERSALE_PLATFORM_DECISION_INVALID_AMOUNT(9020, "裁決金額不合法"),
    AFTERSALE_CASE_NOT_UNDER_APPEAL(9021, "案件未申訴，無仲裁資料"),
    AFTERSALE_RESOLUTION_NOT_ALLOWED_FOR_ZERO_AMOUNT(9022, "零元訂單不允許此退款決議：%s（沒收錢無退款概念，僅允許退貨退款 / 拒絕 / 券補償）"),
    AFTERSALE_RETURN_METHOD_MISMATCH(9023, "退貨方式須與原出貨方式相同"),
    AFTERSALE_RETURN_NOT_SUPPORTED_FOR_UBER(9024, "Uber 即時配送訂單暫不支援退貨退款，請選擇其他售後決議"),
    AFTERSALE_COUPON_COMPENSATION_FAILED(9025, "補償券發放失敗：%s"),
    AFTERSALE_CONFIRM_RECEIPT_BLOCKED(9026, "訂單有進行中的退貨 / 退款 / 申訴售後案件，無法確認收貨"),

    // ===== 物流履約業務碼（9500~9699） =====
    FULFILLMENT_COMMERCE_MODEL_LOCKED(9501, "訂單的履約模式已鎖定，無法變更"),
    FULFILLMENT_CONSIGNMENT_AGREEMENT_REQUIRED(9502, "尚未完成受託代銷附約，暫時無法以此方式結帳"),
    FULFILLMENT_PROVIDER_NOT_ENABLED(9503, "此配送服務尚未開放"),
    FULFILLMENT_COMMERCE_MODEL_UNKNOWN(9504, "無法判定履約模式，請稍後再試"),
    PLATFORM_COLLECTION_NOT_CONFIGURED(9505, "平台代收設定尚未完成，暫時無法以此方式結帳"),
    SHIPPING_OPTION_CONFIG_MISSING(9510, "此配送方式暫不可用"),
    SHIPPING_OPTION_NO_FEE_TIER(9511, "此配送方式暫不可用"),
    SHIPPING_OPTION_EXCEEDS_LIMIT(9512, "商品超過此配送方式的材積或重量限制"),
    SHIPPING_OPTION_NONE_AVAILABLE_CONFIG(9513, "目前沒有可用的配送方式，我們正在處理"),
    SHIPPING_OPTION_NONE_AVAILABLE_LIMIT(9514, "商品超過所有配送方式的限制，請調整包裝或分批下單"),
    SHIPMENT_STATE_TRANSITION_REJECTED(9520, "目前的配送狀態不允許此操作"),
    SHIPMENT_CANCEL_ALREADY_PENDING(9521, "已有處理中的取消申請"),
    SHIPMENT_CANCEL_NOT_CANCELLABLE(9522, "此配送單目前無法申請取消"),
    SHIPMENT_CLAIM_ALREADY_OPEN(9523, "本配送單已有處理中的理賠案件"),
    SHIPMENT_CLAIM_NOT_ELIGIBLE(9524, "目前的配送狀態不可受理理賠"),
    SHIPMENT_RECREATE_NOT_ALLOWED(9525, "只有建單失敗的配送單可以重新建單"),
    SHIPMENT_PROVIDER_STATUS_UNKNOWN(9526, "物流商回報了目前無法辨識的狀態"),
    SHIPMENT_HANDOVER_NOT_ALLOWED(9527, "目前狀態不可回報交運"),
    SHIPMENT_PROVIDER_MISMATCH(9528, "此操作不適用於這張配送單的配送方式"),
    RECEIPT_CONFIRMATION_ALREADY_DONE(9530, "本訂單已完成收貨確認"),
    RECEIPT_CONFIRMATION_NOT_READY(9531, "商品尚未送達，暫時無法確認收貨"),
    RECEIPT_CONFIRMATION_DISPUTED(9532, "本訂單有處理中的爭議，暫時無法確認收貨"),
    RECEIPT_DISPUTE_ALREADY_OPEN(9533, "本訂單已有處理中的爭議"),
    RECEIPT_DISPUTE_NOT_ALLOWED(9534, "目前狀態不允許提出爭議"),
    RECEIPT_REVIEW_ALREADY_OPEN(9535, "本訂單已有進行中的人工裁決案"),
    SELLER_INVOICE_OBLIGATION_NOT_FOUND(9540, "發票任務不存在"),
    SELLER_INVOICE_OBLIGATION_STATE_INVALID(9541, "目前狀態不允許此操作"),
    SELLER_INVOICE_OBLIGATION_FILE_REQUIRED(9542, "請先上傳發票檔案"),
    SELLER_INVOICE_OBLIGATION_AMOUNT_MISMATCH(9543, "發票金額與應開金額不符"),
    SELLER_INVOICE_OBLIGATION_BUYER_MISMATCH(9544, "買受人統一編號與平台不符"),
    SELLER_INVOICE_OBLIGATION_REJECT_REASON_REQUIRED(9545, "退件必須填寫原因"),
    PROVIDER_STATEMENT_LINE_DUPLICATE(9546, "該帳單列已匯入過"),
    SETTLEMENT_NOT_FOUND(9550, "結算單不存在"),
    SETTLEMENT_PAYOUT_BLOCKED(9551, "尚未符合撥款條件"),
    SETTLEMENT_PAYOUT_NOT_APPLICABLE(9552, "本訂單不適用平台撥款"),
    SETTLEMENT_PAYOUT_INPUT_UNAVAILABLE(9553, "撥款資格的資料暫時讀不到，請稍後再試"),
    SETTLEMENT_ALREADY_HELD(9554, "本結算單已有未解除的暫停撥款"),
    SETTLEMENT_HOLD_NOT_FOUND(9555, "查無未解除的暫停撥款"),
    SETTLEMENT_HOLD_REASON_REQUIRED(9556, "暫停或解除撥款必須填寫原因"),
    SETTLEMENT_INSTRUCTION_ALREADY_ISSUED(9557, "本結算單已產生撥款指示"),
    SETTLEMENT_PERIOD_NOT_CLOSED(9558, "結算期尚未關帳"),
    FULFILLMENT_IDEMPOTENCY_PAYLOAD_MISMATCH(9560, "相同的請求識別碼帶了不同的內容"),
    FULFILLMENT_OPERATION_IN_PROGRESS(9561, "相同的操作正在處理中，請稍候"),
    LEDGER_ENTRY_NOT_FOUND(9565, "帳本分錄不存在"),
    LEDGER_TRANSACTION_UNBALANCED(9566, "分錄不平衡，已拒絕寫入"),
    LEDGER_ADJUSTMENT_REASON_REQUIRED(9567, "可稽核調整必須填寫原因與依據"),
    EXCEPTION_CASE_NOT_FOUND(9570, "案件不存在"),
    EXCEPTION_CASE_STATE_INVALID(9571, "案件目前狀態不允許此操作"),
    EXCEPTION_CASE_RESOLUTION_NOT_FOUND(9572, "處置提案不存在"),
    EXCEPTION_CASE_REVIEW_SELF_APPROVAL_DENIED(9573, "覆核者不得為提案者"),
    EXCEPTION_CASE_RESOLUTION_ALREADY_REVIEWED(9574, "本提案已完成覆核"),
    EXCEPTION_CASE_IMPACT_PREVIEW_STALE(9575, "影響預覽已過期，請重新產生後再覆核"),

    // ===== 平台功能開通業務碼（9700~9709） =====
    FEATURE_ADDRESS_LOOKUP_DISABLED(9700, "地址查詢服務尚未開通，請手動輸入完整地址"),
    FEATURE_AI_ASSISTANT_DISABLED(9703, "AI 助理尚未開通，請稍後再試"),
    FEATURE_PAYUNI_MERCHANT_DISABLED(9704, "金流商家建檔服務尚未開通，請稍後再試"),
    FEATURE_SHIPPING_PROVIDER_DISABLED(9705, "物流服務尚未開通，請稍後再試");

    private final int code;
    private final String message;

    GrpcResponseCode(int code, String message) {
        this.code = code;
        this.message = message;
    }

    public int code() {
        return code;
    }

    /**
     * API 回應的 message 欄位值（前端顯示值）。
     * 含 %s 的為模板，需透過 {@code String.format(message(), args)} 組裝。
     */
    public String message() {
        return message;
    }

    /**
     * 組裝動態訊息（含 %s 模板替換）。
     * 無參數時直接回傳 {@link #message()}。
     */
    public String formatMessage(Object... args) {
        if (args == null || args.length == 0) {
            return message;
        }
        return String.format(message, args);
    }

    private static final Map<Integer, GrpcResponseCode> BY_CODE =
        Arrays.stream(values()).collect(Collectors.toUnmodifiableMap(GrpcResponseCode::code, e -> e));

    public static GrpcResponseCode fromCode(int code) {
        var value = BY_CODE.get(code);
        if (value == null) throw new IllegalArgumentException("Unknown code: " + code);
        return value;
    }
}
