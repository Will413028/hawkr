// dart format off
// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。

// ignore_for_file: unnecessary_parenthesis, unused_import, prefer_single_quotes, unnecessary_string_interpolations

import 'package:connectrpc/connect.dart' show Code;
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart' as hawkr_buyer_v1alpha1_buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart' as hawkr_common_v1alpha1_policy_category;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/store_card.pbenum.dart' as hawkr_common_v1alpha1_store_card;
import 'package:hawkr_api/gen/hawkr/payment/v1alpha1/payment.pbenum.dart' as hawkr_payment_v1alpha1_payment;
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart' as hawkr_seller_v1alpha1_seller;

import '../hawkr_ui_text.dart' show UiLocale;

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
(UiLocale.zhTW || UiLocale.en, 1) => '食品安全',
(UiLocale.zhTW || UiLocale.en, 2) => '違法或禁售商品',
(UiLocale.zhTW || UiLocale.en, 3) => '未經許可販售',
(UiLocale.zhTW || UiLocale.en, 4) => '不實或誤導資訊',
(UiLocale.zhTW || UiLocale.en, 5) => '假認證',
(UiLocale.zhTW || UiLocale.en, 6) => '仿冒商品',
(UiLocale.zhTW || UiLocale.en, 7) => '侵害智慧財產權',
(UiLocale.zhTW || UiLocale.en, 8) => '詐欺或操縱交易',
(UiLocale.zhTW || UiLocale.en, 9) => '繞過平台交易',
(UiLocale.zhTW || UiLocale.en, 10) => '不當使用個人資料',
(UiLocale.zhTW || UiLocale.en, 11) => '冒用或誤導交易主體',
(UiLocale.zhTW || UiLocale.en, 12) => '干擾或濫用平台',
(UiLocale.zhTW || UiLocale.en, 13) => '其他',
(UiLocale.zhTW, 14) => '垃圾訊息',
(UiLocale.en, 14) => 'Spam',
(UiLocale.zhTW, 15) => '騷擾或霸凌',
(UiLocale.en, 15) => 'Harassment or bullying',
(UiLocale.zhTW, 16) => '仇恨或歧視言論',
(UiLocale.en, 16) => 'Hate or discriminatory speech',
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

String? _paymentPaymentAccountStatusAction(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '重新整理',
(UiLocale.en, 1) => 'Refresh',
(UiLocale.zhTW, 2) => '重新整理',
(UiLocale.en, 2) => 'Refresh',
(UiLocale.zhTW, 3) => '前往設定收款帳戶',
(UiLocale.en, 3) => 'Go to payout account setup',
(UiLocale.zhTW, 5) => '聯絡客服',
(UiLocale.en, 5) => 'Contact support',
_ => null,
};
}

String? _paymentPaymentAccountStatusDescription(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW, 1) => '正在建立收款帳戶，通常幾分鐘內完成。',
(UiLocale.en, 1) => 'We are setting up your payout account. This usually takes a few minutes.',
(UiLocale.zhTW, 2) => '金流商正在審核你的資料，不需要額外操作。',
(UiLocale.en, 2) => 'The payment provider is reviewing your information. No action is needed.',
(UiLocale.zhTW, 3) => '提供身分與銀行資料後即可開始收款。資料由金流商收集，平台不經手。',
(UiLocale.en, 3) => 'Provide your identity and bank details to start getting paid. The payment provider collects this information; Hawkr does not handle it.',
(UiLocale.zhTW, 4) => '收款已啟用。',
(UiLocale.en, 4) => 'Payments are enabled.',
(UiLocale.zhTW, 5) => '金流商目前無法啟用你的收款帳戶，請聯絡客服。',
(UiLocale.en, 5) => 'The payment provider cannot enable your payout account right now. Please contact support.',
_ => null,
};
}

extension PaymentPaymentAccountStatusUiText on hawkr_payment_v1alpha1_payment.PaymentAccountStatus {
  /// payment.PaymentAccountStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _paymentPaymentAccountStatus(l, value);

  /// payment.PaymentAccountStatus.action；沒有文字的值回 null。
  String? action(UiLocale l) => _paymentPaymentAccountStatusAction(l, value);

  /// payment.PaymentAccountStatus.description；沒有文字的值回 null。
  String? description(UiLocale l) => _paymentPaymentAccountStatusDescription(l, value);
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

String? _sellerContentKind(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '貼文',
(UiLocale.zhTW || UiLocale.en, 2) => '留言',
(UiLocale.zhTW || UiLocale.en, 3) => '討論區留言',
_ => null,
};
}

extension SellerContentKindUiText on hawkr_seller_v1alpha1_seller.ContentKind {
  /// seller.ContentKind 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerContentKind(l, value);
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
(UiLocale.zhTW || UiLocale.en, 4) => '內容下架',
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
(UiLocale.zhTW, 5) => '重新整理',
(UiLocale.en, 5) => 'Refresh',
(UiLocale.zhTW, 6) => '聯絡客服',
(UiLocale.en, 6) => 'Contact support',
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

/// buyer.Profile.gender_unset
String buyerProfileGenderUnset(UiLocale l) {
return switch (l) {
UiLocale.zhTW => '尚未設定',
UiLocale.en => 'Not set',
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
UiLocale.zhTW || UiLocale.en => '措施：' + kindText,
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

