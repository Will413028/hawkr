// dart format off
// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。

// ignore_for_file: unnecessary_parenthesis, unused_import, prefer_single_quotes, unnecessary_string_interpolations

import 'package:connectrpc/connect.dart' show Code;
import 'package:hawkr_api/gen/hawkr/assistant/v1alpha1/assistant.pbenum.dart' as hawkr_assistant_v1alpha1_assistant;
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart' as hawkr_buyer_v1alpha1_buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart' as hawkr_common_v1alpha1_policy_category;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pbenum.dart' as hawkr_common_v1alpha1_store_card;
import 'package:hawkr_api/gen/hawkr/crm/v1alpha1/crm.pbenum.dart' as hawkr_crm_v1alpha1_crm;
import 'package:hawkr_api/gen/hawkr/moderation/v1alpha1/moderation.pbenum.dart' as hawkr_moderation_v1alpha1_moderation;
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart' as hawkr_payment_v1alpha1_payment;
import 'package:hawkr_api/gen/hawkr/procurement/v1alpha1/procurement.pbenum.dart' as hawkr_procurement_v1alpha1_procurement;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/report.pbenum.dart' as hawkr_seller_v1alpha1_report;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart' as hawkr_seller_v1alpha1_seller;
import 'package:hawkr_api/gen/hawkr/social/v1alpha1/social.pbenum.dart' as hawkr_social_v1alpha1_social;

import '../hawkr_ui_text.dart' show UiLocale;

String? _assistantAssistantTool(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '正在查詢你的訂單',
(UiLocale.en, 1) => 'Looking up your orders',
(UiLocale.zhTW, 2) => '正在查詢訂單詳情',
(UiLocale.en, 2) => 'Looking up the order details',
(UiLocale.zhTW, 3) => '正在查詢包裹物流',
(UiLocale.en, 3) => 'Checking your package tracking',
_ => null,
};
}

extension AssistantAssistantToolUiText on hawkr_assistant_v1alpha1_assistant.AssistantTool {
  /// assistant.AssistantTool 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _assistantAssistantTool(l, value);
}

String? _buyerGender(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '男性',
(UiLocale.en, 1) => 'Male',
(UiLocale.zhTW, 2) => '女性',
(UiLocale.en, 2) => 'Female',
(UiLocale.zhTW, 3) => '其他',
(UiLocale.en, 3) => 'Other',
_ => null,
};
}

extension BuyerGenderUiText on hawkr_buyer_v1alpha1_buyer.Gender {
  /// buyer.Gender 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _buyerGender(l, value);
}

String? _commonPolicyCategory(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '食品安全',
(UiLocale.en, 1) => 'Food safety',
(UiLocale.zhTW, 2) => '違法或禁售商品',
(UiLocale.en, 2) => 'Illegal or prohibited goods',
(UiLocale.zhTW, 3) => '未經許可販售',
(UiLocale.en, 3) => 'Unlicensed sale',
(UiLocale.zhTW, 4) => '不實或誤導資訊',
(UiLocale.en, 4) => 'False or misleading information',
(UiLocale.zhTW, 5) => '假認證',
(UiLocale.en, 5) => 'Fake certification',
(UiLocale.zhTW, 6) => '仿冒商品',
(UiLocale.en, 6) => 'Counterfeit goods',
(UiLocale.zhTW, 7) => '侵害智慧財產權',
(UiLocale.en, 7) => 'Intellectual property infringement',
(UiLocale.zhTW, 8) => '詐欺或操縱交易',
(UiLocale.en, 8) => 'Fraud or transaction manipulation',
(UiLocale.zhTW, 9) => '繞過平台交易',
(UiLocale.en, 9) => 'Off-platform transactions',
(UiLocale.zhTW, 10) => '不當使用個人資料',
(UiLocale.en, 10) => 'Misuse of personal data',
(UiLocale.zhTW, 11) => '冒用或誤導交易主體',
(UiLocale.en, 11) => 'Impersonation',
(UiLocale.zhTW, 12) => '干擾或濫用平台',
(UiLocale.en, 12) => 'Platform abuse',
(UiLocale.zhTW, 13) => '其他',
(UiLocale.en, 13) => 'Other',
(UiLocale.zhTW, 14) => '垃圾訊息',
(UiLocale.en, 14) => 'Spam',
(UiLocale.zhTW, 15) => '騷擾或霸凌',
(UiLocale.en, 15) => 'Harassment or bullying',
(UiLocale.zhTW, 16) => '仇恨或歧視言論',
(UiLocale.en, 16) => 'Hate or discriminatory speech',
(UiLocale.zhTW, 17) => '色情或性相關內容',
(UiLocale.en, 17) => 'Sexual content',
_ => null,
};
}

extension CommonPolicyCategoryUiText on hawkr_common_v1alpha1_policy_category.PolicyCategory {
  /// common.PolicyCategory 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _commonPolicyCategory(l, value);
}

String? _commonStoreCardStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '營業中',
(UiLocale.en, 1) => 'Open',
(UiLocale.zhTW, 2) => '暫停營業',
(UiLocale.en, 2) => 'Temporarily closed',
(UiLocale.zhTW, 3) => '已關閉',
(UiLocale.en, 3) => 'Closed',
_ => null,
};
}

extension CommonStoreCardStatusUiText on hawkr_common_v1alpha1_store_card.StoreCardStatus {
  /// common.StoreCardStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _commonStoreCardStatus(l, value);
}

String? _crmCustomerSort(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '首購時間',
(UiLocale.en, 1) => 'First order date',
(UiLocale.zhTW, 2) => '消費金額',
(UiLocale.en, 2) => 'Total spent',
(UiLocale.zhTW, 3) => '訂單數',
(UiLocale.en, 3) => 'Order count',
_ => null,
};
}

extension CrmCustomerSortUiText on hawkr_crm_v1alpha1_crm.CustomerSort {
  /// crm.CustomerSort 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _crmCustomerSort(l, value);
}

String? _crmCustomerType(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '一般客戶',
(UiLocale.en, 1) => 'Regular customer',
(UiLocale.zhTW || UiLocale.en, 2) => 'VIP',
_ => null,
};
}

extension CrmCustomerTypeUiText on hawkr_crm_v1alpha1_crm.CustomerType {
  /// crm.CustomerType 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _crmCustomerType(l, value);
}

String? _moderationReportOutcome(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 2) => '已處理（違規）',
(UiLocale.en, 2) => 'Resolved (violation)',
(UiLocale.zhTW, 3) => '已處理（不違規）',
(UiLocale.en, 3) => 'Resolved (no violation)',
_ => null,
};
}

extension ModerationReportOutcomeUiText on hawkr_moderation_v1alpha1_moderation.ReportOutcome {
  /// moderation.ReportOutcome 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _moderationReportOutcome(l, value);
}

String? _moderationReportTargetKind(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '貼文',
(UiLocale.en, 1) => 'Post',
(UiLocale.zhTW, 2) => '留言',
(UiLocale.en, 2) => 'Comment',
(UiLocale.zhTW, 3) => '討論區留言',
(UiLocale.en, 3) => 'Forum post',
(UiLocale.zhTW, 4) => '使用者',
(UiLocale.en, 4) => 'User',
(UiLocale.zhTW, 5) => '評價',
(UiLocale.en, 5) => 'Review',
(UiLocale.zhTW, 6) => '聊天訊息',
(UiLocale.en, 6) => 'Chat message',
(UiLocale.zhTW, 7) => '評價回覆',
(UiLocale.en, 7) => 'Review reply',
_ => null,
};
}

extension ModerationReportTargetKindUiText on hawkr_moderation_v1alpha1_moderation.ReportTargetKind {
  /// moderation.ReportTargetKind 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _moderationReportTargetKind(l, value);
}

String? _paymentPaymentAccountStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '準備中',
(UiLocale.en, 1) => 'Preparing',
(UiLocale.zhTW, 2) => '審核中',
(UiLocale.en, 2) => 'Under review',
(UiLocale.zhTW, 3) => '待設定',
(UiLocale.en, 3) => 'Setup needed',
(UiLocale.zhTW, 4) => '已啟用',
(UiLocale.en, 4) => 'Active',
(UiLocale.zhTW, 5) => '無法啟用',
(UiLocale.en, 5) => 'Unavailable',
_ => null,
};
}

extension PaymentPaymentAccountStatusUiText on hawkr_payment_v1alpha1_payment.PaymentAccountStatus {
  /// payment.PaymentAccountStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentAccountStatus(l, value);
}

String? _paymentPaymentAnomalyKind(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '付款',
(UiLocale.en, 1) => 'Payment',
(UiLocale.zhTW, 2) => '轉帳',
(UiLocale.en, 2) => 'Transfer',
(UiLocale.zhTW, 3) => '退款',
(UiLocale.en, 3) => 'Refund',
(UiLocale.zhTW, 4) => '收回',
(UiLocale.en, 4) => 'Recovery',
(UiLocale.zhTW, 5) => '補差額轉帳',
(UiLocale.en, 5) => 'Top-up transfer',
_ => null,
};
}

extension PaymentPaymentAnomalyKindUiText on hawkr_payment_v1alpha1_payment.PaymentAnomalyKind {
  /// payment.PaymentAnomalyKind 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentAnomalyKind(l, value);
}

String? _paymentPaymentAnomalyOutcome(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '金流商已執行',
(UiLocale.en, 1) => 'Provider executed it',
(UiLocale.zhTW, 2) => '金流商沒有執行',
(UiLocale.en, 2) => 'Provider did not execute it',
_ => null,
};
}

extension PaymentPaymentAnomalyOutcomeUiText on hawkr_payment_v1alpha1_payment.PaymentAnomalyOutcome {
  /// payment.PaymentAnomalyOutcome 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentAnomalyOutcome(l, value);
}

String? _paymentPaymentRefundStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '待送出',
(UiLocale.en, 1) => 'Pending',
(UiLocale.zhTW, 2) => '退款中',
(UiLocale.en, 2) => 'Refunding',
(UiLocale.zhTW, 3) => '已退款',
(UiLocale.en, 3) => 'Refunded',
(UiLocale.zhTW, 4) => '退款失敗',
(UiLocale.en, 4) => 'Refund failed',
(UiLocale.zhTW, 5) => '待人工處理',
(UiLocale.en, 5) => 'Needs review',
_ => null,
};
}

extension PaymentPaymentRefundStatusUiText on hawkr_payment_v1alpha1_payment.PaymentRefundStatus {
  /// payment.PaymentRefundStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentRefundStatus(l, value);
}

String? _paymentPaymentReversalStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '無需收回',
(UiLocale.en, 1) => 'Nothing to recover',
(UiLocale.zhTW, 2) => '等待收回',
(UiLocale.en, 2) => 'Waiting',
(UiLocale.zhTW, 3) => '收回中',
(UiLocale.en, 3) => 'Recovering',
(UiLocale.zhTW, 4) => '已收回',
(UiLocale.en, 4) => 'Recovered',
(UiLocale.zhTW, 5) => '不再收回',
(UiLocale.en, 5) => 'Not recovered',
(UiLocale.zhTW, 6) => '待人工處理',
(UiLocale.en, 6) => 'Needs review',
_ => null,
};
}

extension PaymentPaymentReversalStatusUiText on hawkr_payment_v1alpha1_payment.PaymentReversalStatus {
  /// payment.PaymentReversalStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentReversalStatus(l, value);
}

String? _paymentPaymentStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '建立中',
(UiLocale.en, 1) => 'Setting up',
(UiLocale.zhTW, 2) => '待付款',
(UiLocale.en, 2) => 'Awaiting payment',
(UiLocale.zhTW, 3) => '處理中',
(UiLocale.en, 3) => 'Processing',
(UiLocale.zhTW, 4) => '已付款',
(UiLocale.en, 4) => 'Paid',
(UiLocale.zhTW, 5) => '已取消',
(UiLocale.en, 5) => 'Canceled',
(UiLocale.zhTW, 6) => '建立失敗',
(UiLocale.en, 6) => 'Could not start',
(UiLocale.zhTW, 7) => '待人工處理',
(UiLocale.en, 7) => 'Needs review',
_ => null,
};
}

extension PaymentPaymentStatusUiText on hawkr_payment_v1alpha1_payment.PaymentStatus {
  /// payment.PaymentStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentStatus(l, value);
}

String? _paymentPaymentTopupStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '無需補差額',
(UiLocale.en, 1) => 'No top-up',
(UiLocale.zhTW, 2) => '等待補差額',
(UiLocale.en, 2) => 'Waiting',
(UiLocale.zhTW, 3) => '補差額中',
(UiLocale.en, 3) => 'Sending',
(UiLocale.zhTW, 4) => '已補差額',
(UiLocale.en, 4) => 'Topped up',
(UiLocale.zhTW, 5) => '不再補差額',
(UiLocale.en, 5) => 'Not topped up',
(UiLocale.zhTW, 6) => '待人工處理',
(UiLocale.en, 6) => 'Needs review',
_ => null,
};
}

extension PaymentPaymentTopupStatusUiText on hawkr_payment_v1alpha1_payment.PaymentTopupStatus {
  /// payment.PaymentTopupStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentTopupStatus(l, value);
}

String? _paymentPaymentTransferStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '待轉帳',
(UiLocale.en, 1) => 'Pending',
(UiLocale.zhTW, 2) => '已轉帳',
(UiLocale.en, 2) => 'Transferred',
(UiLocale.zhTW, 3) => '轉帳失敗',
(UiLocale.en, 3) => 'Transfer failed',
(UiLocale.zhTW, 4) => '待人工處理',
(UiLocale.en, 4) => 'Needs review',
_ => null,
};
}

extension PaymentPaymentTransferStatusUiText on hawkr_payment_v1alpha1_payment.PaymentTransferStatus {
  /// payment.PaymentTransferStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentTransferStatus(l, value);
}

String? _paymentRequirementCollection(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '金流商收件（託管頁面）',
(UiLocale.en, 1) => 'Provider-hosted',
(UiLocale.zhTW, 2) => '平台收件（內嵌元件）',
(UiLocale.en, 2) => 'Embedded on platform',
_ => null,
};
}

extension PaymentRequirementCollectionUiText on hawkr_payment_v1alpha1_payment.RequirementCollection {
  /// payment.RequirementCollection 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentRequirementCollection(l, value);
}

String? _procurementRejectionReason(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '內容是空的',
(UiLocale.en, 1) => 'The content is empty',
(UiLocale.zhTW, 2) => '超過長度上限，已截斷',
(UiLocale.en, 2) => 'Over the length limit, shortened',
(UiLocale.zhTW, 3) => '數量不在可接受的範圍內',
(UiLocale.en, 3) => 'The quantity is out of range',
(UiLocale.zhTW, 4) => '單價預算不在可接受的範圍內',
(UiLocale.en, 4) => 'The budget per unit is out of range',
(UiLocale.zhTW, 5) => '這個單位不支援',
(UiLocale.en, 5) => 'This unit is not supported',
(UiLocale.zhTW, 6) => '無法辨識這個縣市',
(UiLocale.en, 6) => 'The city could not be recognized',
(UiLocale.zhTW, 7) => '日期格式不正確',
(UiLocale.en, 7) => 'The date format is not valid',
(UiLocale.zhTW, 8) => '日期缺少西元年份',
(UiLocale.en, 8) => 'The date is missing the year',
(UiLocale.zhTW, 9) => '日期已經過了',
(UiLocale.en, 9) => 'The date has already passed',
(UiLocale.zhTW, 10) => '日期太久以後',
(UiLocale.en, 10) => 'The date is too far ahead',
(UiLocale.zhTW, 11) => '品項未確定，無法指定品項分類',
(UiLocale.en, 11) => 'The item is missing or was rejected, so no item category can be set',
(UiLocale.zhTW, 12) => '品項分類沒有名稱',
(UiLocale.en, 12) => 'The item category has no name',
(UiLocale.zhTW, 13) => '品項分類的判定方式不明',
(UiLocale.en, 13) => 'The way the item category was decided is unknown',
_ => null,
};
}

extension ProcurementRejectionReasonUiText on hawkr_procurement_v1alpha1_procurement.RejectionReason {
  /// procurement.RejectionReason 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _procurementRejectionReason(l, value);
}

String? _sellerContentKind(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '貼文',
(UiLocale.zhTW || UiLocale.en, 2) => '留言',
(UiLocale.zhTW || UiLocale.en, 3) => '討論區留言',
(UiLocale.zhTW || UiLocale.en, 4) => '聊天訊息',
(UiLocale.zhTW || UiLocale.en, 5) => '評價回覆',
_ => null,
};
}

extension SellerContentKindUiText on hawkr_seller_v1alpha1_seller.ContentKind {
  /// seller.ContentKind 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerContentKind(l, value);
}

String? _sellerInquiryKind(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '提供紀錄',
(UiLocale.en, 1) => 'Provide records',
(UiLocale.zhTW, 2) => '請說明',
(UiLocale.en, 2) => 'Provide an explanation',
_ => null,
};
}

extension SellerInquiryKindUiText on hawkr_seller_v1alpha1_seller.InquiryKind {
  /// seller.InquiryKind 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerInquiryKind(l, value);
}

String? _sellerInquiryReplyChannel(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '電話',
(UiLocale.en, 1) => 'Phone',
(UiLocale.zhTW, 2) => '電子信箱',
(UiLocale.en, 2) => 'Email',
(UiLocale.zhTW, 3) => '通訊軟體',
(UiLocale.en, 3) => 'Messaging app',
(UiLocale.zhTW, 4) => '其他管道',
(UiLocale.en, 4) => 'Other',
(UiLocale.zhTW, 5) => '賣家中心',
(UiLocale.en, 5) => 'Seller Center',
_ => null,
};
}

extension SellerInquiryReplyChannelUiText on hawkr_seller_v1alpha1_seller.InquiryReplyChannel {
  /// seller.InquiryReplyChannel 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerInquiryReplyChannel(l, value);
}

String? _sellerInquiryState(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '待回覆',
(UiLocale.en, 1) => 'Awaiting reply',
(UiLocale.zhTW, 2) => '已回覆',
(UiLocale.en, 2) => 'Replied',
(UiLocale.zhTW, 3) => '已撤回',
(UiLocale.en, 3) => 'Withdrawn',
_ => null,
};
}

extension SellerInquiryStateUiText on hawkr_seller_v1alpha1_seller.InquiryState {
  /// seller.InquiryState 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerInquiryState(l, value);
}

String? _sellerKYCField(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '主要商品類型',
(UiLocale.zhTW || UiLocale.en, 2) => '官方網站',
(UiLocale.zhTW || UiLocale.en, 3) => '公司名稱',
(UiLocale.zhTW || UiLocale.en, 4) => '統一編號',
(UiLocale.zhTW || UiLocale.en, 5) => '設立日期',
(UiLocale.zhTW || UiLocale.en, 6) => '公司電話',
(UiLocale.zhTW || UiLocale.en, 7) => '補充文件',
(UiLocale.zhTW || UiLocale.en, 8) => '公司地址',
_ => null,
};
}

extension SellerKYCFieldUiText on hawkr_seller_v1alpha1_seller.KYCField {
  /// seller.KYCField 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerKYCField(l, value);
}

String? _sellerKYCStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '草稿',
(UiLocale.en, 1) => 'Draft',
(UiLocale.zhTW, 2) => '審核中',
(UiLocale.en, 2) => 'Under review',
(UiLocale.zhTW, 3) => '已通過',
(UiLocale.en, 3) => 'Approved',
(UiLocale.zhTW, 4) => '需修改',
(UiLocale.en, 4) => 'Needs changes',
(UiLocale.zhTW, 5) => '已取消',
(UiLocale.en, 5) => 'Cancelled',
_ => null,
};
}

extension SellerKYCStatusUiText on hawkr_seller_v1alpha1_seller.KYCStatus {
  /// seller.KYCStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerKYCStatus(l, value);
}

String? _sellerMeasureBasis(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '違規認定',
(UiLocale.zhTW || UiLocale.en, 2) => '風險',
(UiLocale.zhTW || UiLocale.en, 3) => '契約條款',
_ => null,
};
}

extension SellerMeasureBasisUiText on hawkr_seller_v1alpha1_seller.MeasureBasis {
  /// seller.MeasureBasis 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerMeasureBasis(l, value);
}

String? _sellerMeasureKind(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '警告',
(UiLocale.zhTW || UiLocale.en, 2) => '停權',
(UiLocale.zhTW || UiLocale.en, 3) => '終止合作',
(UiLocale.zhTW, 4) => '內容下架',
(UiLocale.en, 4) => 'Content removal',
_ => null,
};
}

extension SellerMeasureKindUiText on hawkr_seller_v1alpha1_seller.MeasureKind {
  /// seller.MeasureKind 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerMeasureKind(l, value);
}

String? _sellerMeasureRequestKind(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '申覆',
(UiLocale.zhTW || UiLocale.en, 2) => '解除請求',
_ => null,
};
}

String? _sellerMeasureRequestKindSubmittedNote(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '已提交申覆，請等候平台複核。',
(UiLocale.zhTW || UiLocale.en, 2) => '已提交解除請求，請等候平台複核。',
_ => null,
};
}

extension SellerMeasureRequestKindUiText on hawkr_seller_v1alpha1_seller.MeasureRequestKind {
  /// seller.MeasureRequestKind 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerMeasureRequestKind(l, value);

  /// seller.MeasureRequestKind.submitted_note；沒有文字的值回 null。
  String? submittedNote(UiLocale l) => _sellerMeasureRequestKindSubmittedNote(l, value);
}

String? _sellerMeasureRequestState(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '審核中',
(UiLocale.zhTW || UiLocale.en, 2) => '已通過',
(UiLocale.zhTW || UiLocale.en, 3) => '已駁回',
(UiLocale.zhTW || UiLocale.en, 4) => '已結案',
_ => null,
};
}

String? _sellerMeasureRequestStateWithKind(UiLocale l, int value, hawkr_seller_v1alpha1_seller.MeasureRequestKind kind) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => (switch (kind.value) {1 => '申覆審核中',2 => '解除請求審核中',_ => '請求審核中',}),
(UiLocale.zhTW || UiLocale.en, 2) => (switch (kind.value) {1 => '申覆已通過',2 => '解除請求已通過',_ => '請求已通過',}),
(UiLocale.zhTW || UiLocale.en, 3) => (switch (kind.value) {1 => '申覆已駁回',2 => '解除請求已駁回',_ => '請求已駁回',}),
_ => null,
};
}

extension SellerMeasureRequestStateUiText on hawkr_seller_v1alpha1_seller.MeasureRequestState {
  /// seller.MeasureRequestState 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerMeasureRequestState(l, value);

  /// seller.MeasureRequestState.with_kind；沒有文字的值回 null。
  String? withKind(UiLocale l, {required hawkr_seller_v1alpha1_seller.MeasureRequestKind kind}) => _sellerMeasureRequestStateWithKind(l, value, kind);
}

String? _sellerMeasureState(UiLocale l, int value, hawkr_seller_v1alpha1_seller.MeasureKind kind) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '已發出',
(UiLocale.zhTW || UiLocale.en, 2) => (switch (kind.value) {2 => '停權中',3 => '合作已終止',_ => '生效中',}),
(UiLocale.zhTW || UiLocale.en, 3) => '已結束',
(UiLocale.zhTW || UiLocale.en, 4) => '已撤銷',
(UiLocale.zhTW || UiLocale.en, 5) => '預定生效',
_ => null,
};
}

extension SellerMeasureStateUiText on hawkr_seller_v1alpha1_seller.MeasureState {
  /// seller.MeasureState 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l, {required hawkr_seller_v1alpha1_seller.MeasureKind kind}) => _sellerMeasureState(l, value, kind);
}

String? _sellerOnboardingActionButton(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 2) => '去設定',
(UiLocale.en, 2) => 'Go to settings',
(UiLocale.zhTW, 3) => '前往商家認證',
(UiLocale.en, 3) => 'Go to seller verification',
(UiLocale.zhTW, 4) => '前往設定收款帳戶',
(UiLocale.en, 4) => 'Go to payout account setup',
(UiLocale.zhTW, 5) => '聯絡客服',
(UiLocale.en, 5) => 'Contact support',
_ => null,
};
}

extension SellerOnboardingActionUiText on hawkr_seller_v1alpha1_seller.OnboardingAction {
  /// seller.OnboardingAction.button；沒有文字的值回 null。
  String? button(UiLocale l) => _sellerOnboardingActionButton(l, value);
}

String? _sellerOnboardingTaskIdDescription(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '店名、logo 與所在地。',
(UiLocale.en, 1) => 'Store name, logo and location.',
(UiLocale.zhTW, 2) => '商家資料與統編的平台審核。',
(UiLocale.en, 2) => 'Platform review of your business details and tax ID.',
(UiLocale.zhTW, 3) => '向金流商設定身分與銀行資料。',
(UiLocale.en, 3) => 'Set up your identity and bank details with the payment provider.',
_ => null,
};
}

String? _sellerOnboardingTaskIdPanelDescription(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '買家要看得到你的店，需要有店名、logo 與所在地。',
(UiLocale.en, 1) => 'To be found by buyers, your store needs a name, a logo and a location.',
(UiLocale.zhTW, 2) => '平台審核通過後，店面才能公開，並開放設定收款帳戶。',
(UiLocale.en, 2) => 'Once the platform review is approved, your store can go public and you can set up a payout account.',
(UiLocale.zhTW, 3) => '身分與銀行資料由金流商收集，平台不經手；收款帳戶啟用後才能正式收款。',
(UiLocale.en, 3) => 'Your identity and bank details are collected by the payment provider, not the platform; you can only receive payments once the payout account is enabled.',
_ => null,
};
}

String? _sellerOnboardingTaskIdPanelTitle(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '完成商店資料',
(UiLocale.en, 1) => 'Complete your store profile',
(UiLocale.zhTW, 2) => '完成平台審核',
(UiLocale.en, 2) => 'Complete the platform review',
(UiLocale.zhTW, 3) => '設定收款帳戶',
(UiLocale.en, 3) => 'Set up your payout account',
_ => null,
};
}

extension SellerOnboardingTaskIdUiText on hawkr_seller_v1alpha1_seller.OnboardingTaskId {
  /// seller.OnboardingTaskId.description；沒有文字的值回 null。
  String? description(UiLocale l) => _sellerOnboardingTaskIdDescription(l, value);

  /// seller.OnboardingTaskId.panel_description；沒有文字的值回 null。
  String? panelDescription(UiLocale l) => _sellerOnboardingTaskIdPanelDescription(l, value);

  /// seller.OnboardingTaskId.panel_title；沒有文字的值回 null。
  String? panelTitle(UiLocale l) => _sellerOnboardingTaskIdPanelTitle(l, value);
}

String? _sellerReportOutcome(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '已處理（已採取措施）',
(UiLocale.en, 1) => 'Resolved (action taken)',
(UiLocale.zhTW, 2) => '已處理（不違規）',
(UiLocale.en, 2) => 'Resolved (no violation)',
(UiLocale.zhTW, 3) => '已處理（資料不足）',
(UiLocale.en, 3) => 'Resolved (insufficient information)',
(UiLocale.zhTW, 4) => '已處理（重複檢舉）',
(UiLocale.en, 4) => 'Resolved (duplicate report)',
(UiLocale.zhTW, 5) => '已處理（不受理）',
(UiLocale.en, 5) => 'Resolved (not accepted)',
_ => null,
};
}

extension SellerReportOutcomeUiText on hawkr_seller_v1alpha1_report.ReportOutcome {
  /// seller.ReportOutcome 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerReportOutcome(l, value);
}

String? _sellerRevokeReason(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '申覆成立',
(UiLocale.zhTW || UiLocale.en, 2) => '平台更正',
(UiLocale.zhTW || UiLocale.en, 3) => '主管機關或法院的決定',
(UiLocale.zhTW || UiLocale.en, 4) => '其他',
(UiLocale.zhTW || UiLocale.en, 5) => '依據的認定已撤銷',
(UiLocale.zhTW || UiLocale.en, 6) => '關聯的終止已撤銷',
_ => null,
};
}

extension SellerRevokeReasonUiText on hawkr_seller_v1alpha1_seller.RevokeReason {
  /// seller.RevokeReason 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerRevokeReason(l, value);
}

String? _sellerRiskGround(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '交易風險',
(UiLocale.zhTW || UiLocale.en, 2) => '食品安全',
(UiLocale.zhTW || UiLocale.en, 3) => '詐欺',
(UiLocale.zhTW || UiLocale.en, 4) => '個人資料',
(UiLocale.zhTW || UiLocale.en, 5) => '侵權',
(UiLocale.zhTW || UiLocale.en, 6) => '資安',
(UiLocale.zhTW || UiLocale.en, 7) => '履約',
(UiLocale.zhTW || UiLocale.en, 8) => '主管機關或司法機關的要求',
_ => null,
};
}

extension SellerRiskGroundUiText on hawkr_seller_v1alpha1_seller.RiskGround {
  /// seller.RiskGround 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerRiskGround(l, value);
}

String? _sellerSellerRequirement(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '店家未關閉',
(UiLocale.zhTW || UiLocale.en, 2) => '帳號未停權',
(UiLocale.zhTW, 3) => '平台審核與收款帳戶都完成',
(UiLocale.en, 3) => 'Platform review and payout account both completed',
(UiLocale.zhTW || UiLocale.en, 4) => '店名',
(UiLocale.zhTW || UiLocale.en, 5) => 'logo',
(UiLocale.zhTW || UiLocale.en, 6) => '所在地',
(UiLocale.zhTW || UiLocale.en, 7) => '店家未在刪除中',
_ => null,
};
}

extension SellerSellerRequirementUiText on hawkr_seller_v1alpha1_seller.SellerRequirement {
  /// seller.SellerRequirement 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerSellerRequirement(l, value);
}

String? _sellerStoreField(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '店家地址',
(UiLocale.zhTW || UiLocale.en, 2) => '聯絡人姓名',
(UiLocale.zhTW || UiLocale.en, 3) => '聯絡人手機',
(UiLocale.zhTW || UiLocale.en, 4) => '客服電子信箱',
(UiLocale.zhTW || UiLocale.en, 5) => '商店電話',
_ => null,
};
}

extension SellerStoreFieldUiText on hawkr_seller_v1alpha1_seller.StoreField {
  /// seller.StoreField 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerStoreField(l, value);
}

String? _sellerTerminationGround(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '預告終止（第 21.2 條）',
(UiLocale.zhTW || UiLocale.en, 2) => '重大違約未改善',
(UiLocale.zhTW || UiLocale.en, 3) => '停業',
(UiLocale.zhTW || UiLocale.en, 4) => '解散',
(UiLocale.zhTW || UiLocale.en, 5) => '破產',
(UiLocale.zhTW || UiLocale.en, 6) => '必要許可遭撤銷',
(UiLocale.zhTW || UiLocale.en, 7) => '重大食安事件',
(UiLocale.zhTW || UiLocale.en, 8) => '詐欺事件',
(UiLocale.zhTW || UiLocale.en, 9) => '危害平台安全',
(UiLocale.zhTW || UiLocale.en, 10) => '其他嚴重影響履約的情形',
(UiLocale.zhTW || UiLocale.en, 11) => '關聯帳號',
_ => null,
};
}

extension SellerTerminationGroundUiText on hawkr_seller_v1alpha1_seller.TerminationGround {
  /// seller.TerminationGround 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerTerminationGround(l, value);
}

String? _sellerViolationSeverity(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '低',
(UiLocale.zhTW || UiLocale.en, 2) => '中',
(UiLocale.zhTW || UiLocale.en, 3) => '高',
(UiLocale.zhTW || UiLocale.en, 4) => '嚴重',
_ => null,
};
}

extension SellerViolationSeverityUiText on hawkr_seller_v1alpha1_seller.ViolationSeverity {
  /// seller.ViolationSeverity 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerViolationSeverity(l, value);
}

String? _socialCommentStatePlaceholder(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 2) => '此留言已刪除',
(UiLocale.en, 2) => 'This comment was deleted',
(UiLocale.zhTW, 3) => '此留言已被隱藏',
(UiLocale.en, 3) => 'This comment was hidden',
_ => null,
};
}

extension SocialCommentStateUiText on hawkr_social_v1alpha1_social.CommentState {
  /// social.CommentState.placeholder；沒有文字的值回 null。
  String? placeholder(UiLocale l) => _socialCommentStatePlaceholder(l, value);
}

/// admin.moderation.CaseReport.outcome_unset
String adminModerationCaseReportOutcomeUnset(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '審核中',
UiLocale.en => 'Under review',
};
}

/// buyer.Profile.deleted_name
String buyerProfileDeletedName(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '已刪除帳號',
UiLocale.en => 'Deleted account',
};
}

/// buyer.Profile.gender_unset
String buyerProfileGenderUnset(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '尚未設定',
UiLocale.en => 'Not set',
};
}

/// chat.Message.unsupported_kind
String chatMessageUnsupportedKind(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '不支援的訊息',
UiLocale.en => 'Unsupported message',
};
}

/// chat.Preview.erased
String chatPreviewErased(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '此訊息已被移除',
UiLocale.en => 'This message has been removed',
};
}

/// chat.Preview.hidden
String chatPreviewHidden(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '此訊息已被隱藏',
UiLocale.en => 'This message has been hidden',
};
}

/// chat.Preview.image
String chatPreviewImage(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '傳送了一張圖片',
UiLocale.en => 'Sent a photo',
};
}

/// common.StoreCard.closed_store
String commonStoreCardClosedStore(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '已關閉的店家',
UiLocale.en => 'Closed store',
};
}

/// Connect code 的預設文字；canceled 不顯示，回 null。
String? commonErrorText(Code code, UiLocale l) => switch ((l, code)) {
(UiLocale.zhTW || UiLocale.en, Code.aborted) => '資源衝突',
(UiLocale.zhTW || UiLocale.en, Code.alreadyExists) => '資源衝突',
(UiLocale.zhTW || UiLocale.en, Code.dataLoss) => '系統錯誤',
(UiLocale.zhTW || UiLocale.en, Code.deadlineExceeded) => '服務暫時不可用',
(UiLocale.zhTW || UiLocale.en, Code.failedPrecondition) => '資源衝突',
(UiLocale.zhTW || UiLocale.en, Code.internal) => '系統錯誤',
(UiLocale.zhTW || UiLocale.en, Code.invalidArgument) => '參數錯誤',
(UiLocale.zhTW || UiLocale.en, Code.notFound) => '資源不存在',
(UiLocale.zhTW || UiLocale.en, Code.outOfRange) => '參數錯誤',
(UiLocale.zhTW || UiLocale.en, Code.permissionDenied) => '無權操作',
(UiLocale.zhTW || UiLocale.en, Code.resourceExhausted) => '操作過於頻繁，請稍後再試',
(UiLocale.zhTW || UiLocale.en, Code.unauthenticated) => '未登入',
(UiLocale.zhTW || UiLocale.en, Code.unavailable) => '服務暫時不可用',
(UiLocale.zhTW || UiLocale.en, Code.unimplemented) => '功能暫不支援',
(UiLocale.zhTW || UiLocale.en, Code.unknown) => '系統錯誤',
_ => null,
};

/// 不認得的值與 UNSPECIFIED 一律顯示的文字。
String commonUnknown(UiLocale l) => switch (l) {
UiLocale.zhTW || UiLocale.en => '未知',
};

/// moderation.CommunityReport.outcome_unset
String moderationCommunityReportOutcomeUnset(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '審核中',
UiLocale.en => 'Under review',
};
}

/// review.Review.rating_label_1
String reviewReviewRatingLabel1(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '非常不滿意',
UiLocale.en => 'Very dissatisfied',
};
}

/// review.Review.rating_label_2
String reviewReviewRatingLabel2(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '不滿意',
UiLocale.en => 'Dissatisfied',
};
}

/// review.Review.rating_label_3
String reviewReviewRatingLabel3(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '一般',
UiLocale.en => 'Okay',
};
}

/// review.Review.rating_label_4
String reviewReviewRatingLabel4(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '滿意',
UiLocale.en => 'Satisfied',
};
}

/// review.Review.rating_label_5
String reviewReviewRatingLabel5(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '非常滿意',
UiLocale.en => 'Very satisfied',
};
}

/// review.ReviewAuthor.anonymous_name
String reviewReviewAuthorAnonymousName(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '匿名會員',
UiLocale.en => 'Anonymous member',
};
}

/// seller.KYCApplication.decide_time
String sellerKYCApplicationDecideTime(UiLocale l, {required String time}) {
return switch (l) {
UiLocale.zhTW => '核准時間：' + time,
UiLocale.en => 'Approved at: ' + time,
};
}

/// seller.KYCApplication.submit_time
String sellerKYCApplicationSubmitTime(UiLocale l, {required String time}) {
return switch (l) {
UiLocale.zhTW => '送出時間：' + time,
UiLocale.en => 'Submitted at: ' + time,
};
}

/// seller.Measure.appeal_deadline
String sellerMeasureAppealDeadline(UiLocale l, {required String deadline}) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '申覆期限：' + deadline,
};
}

/// seller.Measure.meta_kind
String? sellerMeasureMetaKind(UiLocale l, {required hawkr_seller_v1alpha1_seller.MeasureKind kind}) {
final kindText = _sellerMeasureKind(l, kind.value);
if (kindText == null) return null;
return switch (l) {
UiLocale.zhTW => '措施：' + kindText,
UiLocale.en => 'Measure: ' + kindText,
};
}

/// seller.Measure.meta_severity
String? sellerMeasureMetaSeverity(UiLocale l, {required hawkr_seller_v1alpha1_seller.ViolationSeverity severity}) {
final severityText = _sellerViolationSeverity(l, severity.value);
if (severityText == null) return null;
return switch (l) {
UiLocale.zhTW || UiLocale.en => '嚴重程度：' + severityText,
};
}

/// seller.Measure.meta_termination_ground
String? sellerMeasureMetaTerminationGround(UiLocale l, {required hawkr_seller_v1alpha1_seller.TerminationGround ground}) {
final groundText = _sellerTerminationGround(l, ground.value);
if (groundText == null) return null;
return switch (l) {
UiLocale.zhTW || UiLocale.en => '事由：' + groundText,
};
}

/// seller.Measure.rejection_cooldown_warning
String sellerMeasureRejectionCooldownWarning(UiLocale l, {required int days}) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '若本次經駁回，將於 ' + days.toString() + ' 日內不得再提出解除請求',
};
}

/// seller.Measure.removed_contents_cleared
String sellerMeasureRemovedContentsCleared(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '內容副本已超過保留期限並刪除。',
UiLocale.en => 'The copy of this content was deleted after the retention period.',
};
}

/// seller.Measure.removed_contents_heading
String sellerMeasureRemovedContentsHeading(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '下架的內容',
UiLocale.en => 'Removed content',
};
}

/// seller.Measure.suspension_active_open
String sellerMeasureSuspensionActiveOpen(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '停權中（未定結束時間，平台定期複審）',
UiLocale.en => 'Suspended (no end date; under periodic review by the platform)',
};
}

/// seller.Measure.suspension_active_until
String sellerMeasureSuspensionActiveUntil(UiLocale l, {required String end}) {
return switch (l) {
UiLocale.zhTW => '停權中（至 ' + end + '）',
UiLocale.en => 'Suspended until ' + end,
};
}

/// seller.Measure.suspension_ended
String sellerMeasureSuspensionEnded(UiLocale l, {required String end}) {
return switch (l) {
UiLocale.zhTW => '已於 ' + end + ' 結束',
UiLocale.en => 'Ended on ' + end,
};
}

/// seller.Measure.termination_effective
String sellerMeasureTerminationEffective(UiLocale l, {required String start}) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '終止生效：' + start,
};
}

/// seller.Measure.termination_effective_ended
String sellerMeasureTerminationEffectiveEnded(UiLocale l, {required String start, required String end}) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '終止生效：' + start + '，合作已於 ' + end + ' 結束',
};
}

/// seller.Measure.termination_notice
String sellerMeasureTerminationNotice(UiLocale l, {required String start}) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '平台已通知將於 ' + start + ' 終止合作。在此之前店家照常營業；如有異議，可提出申覆。',
};
}

/// seller.Measure.title_fallback
String sellerMeasureTitleFallback(UiLocale l) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '措施紀錄',
};
}

/// seller.Measure.title_risk
String? sellerMeasureTitleRisk(UiLocale l, {required hawkr_seller_v1alpha1_seller.RiskGround ground}) {
final groundText = _sellerRiskGround(l, ground.value);
if (groundText == null) return null;
return switch (l) {
UiLocale.zhTW || UiLocale.en => '風險：' + groundText,
};
}

/// seller.Measure.title_risk_fallback
String sellerMeasureTitleRiskFallback(UiLocale l) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '風險措施',
};
}

/// seller.Measure.view_content_link
String sellerMeasureViewContentLink(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '查看貼文',
UiLocale.en => 'View post',
};
}

/// seller.MeasureRequest.reply_note
String sellerMeasureRequestReplyNote(UiLocale l, {required String reason}) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '平台回覆：' + reason,
};
}

/// seller.PaymentAccount.payouts_inactive_note
String sellerPaymentAccountPayoutsInactiveNote(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '撥款到銀行帳戶尚未啟用，金流商可能會通知你補件。',
UiLocale.en => 'Payouts to your bank account are not enabled yet. The payment provider may ask you for more information.',
};
}

/// seller.StoreReport.outcome_unset
String sellerStoreReportOutcomeUnset(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '審核中',
UiLocale.en => 'Under review',
};
}

