// dart format off
// 由 backend/cmd/uitext-gen 生成，不得手改；重新生成：在 backend 目錄執行 make uitext。

// ignore_for_file: unnecessary_parenthesis, unused_import, prefer_single_quotes, unnecessary_string_interpolations

import 'package:connectrpc/connect.dart' show Code;
import 'package:hawkr_api/gen/hawkr/buyer/v1alpha1/buyer.pbenum.dart' as hawkr_buyer_v1alpha1_buyer;
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart' as hawkr_common_v1alpha1_policy_category;
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
(UiLocale.zhTW || UiLocale.en, 1) => '真實姓名',
(UiLocale.zhTW || UiLocale.en, 2) => '出生日期',
(UiLocale.zhTW || UiLocale.en, 3) => '發證日期',
(UiLocale.zhTW || UiLocale.en, 4) => '發證類型',
(UiLocale.zhTW || UiLocale.en, 5) => '發證地點',
(UiLocale.zhTW || UiLocale.en, 6) => '英文姓名',
(UiLocale.zhTW || UiLocale.en, 7) => '銀行名稱',
(UiLocale.zhTW || UiLocale.en, 8) => '銀行代碼',
(UiLocale.zhTW || UiLocale.en, 9) => '銀行帳號',
(UiLocale.zhTW || UiLocale.en, 10) => '主要商品類型',
(UiLocale.zhTW || UiLocale.en, 12) => '官方網站',
(UiLocale.zhTW || UiLocale.en, 15) => '身分證正面照',
(UiLocale.zhTW || UiLocale.en, 16) => '身分證反面照',
(UiLocale.zhTW || UiLocale.en, 17) => '存摺封面',
(UiLocale.zhTW || UiLocale.en, 18) => '身分證字號',
(UiLocale.zhTW || UiLocale.en, 20) => '公司名稱',
(UiLocale.zhTW || UiLocale.en, 21) => '統一編號',
(UiLocale.zhTW || UiLocale.en, 22) => '設立日期',
(UiLocale.zhTW || UiLocale.en, 25) => '公司電話',
(UiLocale.zhTW || UiLocale.en, 26) => '負責人身分證字號',
(UiLocale.zhTW || UiLocale.en, 27) => '補充文件',
(UiLocale.zhTW || UiLocale.en, 28) => '公司地址',
_ => null,
};
}

extension SellerKYCFieldUiText on hawkr_seller_v1alpha1_seller.KYCField {
  /// seller.KYCField 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerKYCField(l, value);
}

String? _sellerKYCFlowStatus(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '待認證',
(UiLocale.zhTW || UiLocale.en, 2) => '填寫中',
(UiLocale.zhTW || UiLocale.en, 3) => '平台審核中',
(UiLocale.zhTW || UiLocale.en, 4) => '未通過',
(UiLocale.zhTW || UiLocale.en, 5) => 'PayUni 審核中',
(UiLocale.zhTW || UiLocale.en, 6) => 'PayUni 建檔失敗，處理中',
(UiLocale.zhTW || UiLocale.en, 7) => 'PayUni 審核中',
(UiLocale.zhTW || UiLocale.en, 8) => 'PayUni 審核中',
(UiLocale.zhTW || UiLocale.en, 9) => '需補件',
(UiLocale.zhTW || UiLocale.en, 10) => '已通過',
(UiLocale.zhTW || UiLocale.en, 11) => 'PayUni 審核未通過',
_ => null,
};
}

String? _sellerKYCFlowStatusAction(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '開始認證',
(UiLocale.zhTW || UiLocale.en, 2) => '開始認證',
(UiLocale.zhTW || UiLocale.en, 3) => '查看進度',
(UiLocale.zhTW || UiLocale.en, 4) => '前往補件',
(UiLocale.zhTW || UiLocale.en, 5) => '查看進度',
(UiLocale.zhTW || UiLocale.en, 6) => '查看進度',
(UiLocale.zhTW || UiLocale.en, 7) => '查看進度',
(UiLocale.zhTW || UiLocale.en, 8) => '查看進度',
(UiLocale.zhTW || UiLocale.en, 9) => '前往補件',
(UiLocale.zhTW || UiLocale.en, 11) => '前往補件',
_ => null,
};
}

String? _sellerKYCFlowStatusBannerDescription(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '完成認證後才能正式收款、提領與配送設定。',
(UiLocale.zhTW || UiLocale.en, 2) => '完成認證後才能正式收款、提領與配送設定。',
(UiLocale.zhTW || UiLocale.en, 3) => '審核通過後將自動送出至 PayUni 進行金流審核。',
(UiLocale.zhTW || UiLocale.en, 4) => '請查看退件原因並編輯後重新送出。',
(UiLocale.zhTW || UiLocale.en, 10) => '可開始正式收款、提領與配送設定。',
_ => null,
};
}

String? _sellerKYCFlowStatusBannerTitle(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '尚未開始商家認證',
(UiLocale.zhTW || UiLocale.en, 2) => '尚未開始商家認證',
(UiLocale.zhTW || UiLocale.en, 3) => '平台審核中',
(UiLocale.zhTW || UiLocale.en, 4) => '認證未通過',
(UiLocale.zhTW || UiLocale.en, 10) => '商家認證已通過',
_ => null,
};
}

String? _sellerKYCFlowStatusGateHeading(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '尚未通過商家認證',
(UiLocale.zhTW || UiLocale.en, 2) => '尚未通過商家認證',
(UiLocale.zhTW || UiLocale.en, 3) => '商家認證審核中',
(UiLocale.zhTW || UiLocale.en, 4) => '商家認證需要補件',
(UiLocale.zhTW || UiLocale.en, 10) => '尚未通過商家認證',
_ => null,
};
}

String? _sellerKYCFlowStatusGateMessage(UiLocale l, int value, String subject) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => '請先完成商家認證，才能' + subject + '。',
(UiLocale.zhTW || UiLocale.en, 2) => '請先完成商家認證，才能' + subject + '。',
(UiLocale.zhTW || UiLocale.en, 3) => '你的商家認證平台審核中，通過後將自動送出至 PayUni 進行金流審核，完成後就能' + subject + '。',
(UiLocale.zhTW || UiLocale.en, 4) => '認證未通過，請修改資料後重新送審。',
(UiLocale.zhTW || UiLocale.en, 10) => '請先完成商家認證，才能' + subject + '。',
_ => null,
};
}

extension SellerKYCFlowStatusUiText on hawkr_seller_v1alpha1_seller.KYCFlowStatus {
  /// seller.KYCFlowStatus 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerKYCFlowStatus(l, value);

  /// seller.KYCFlowStatus.action；沒有文字的值回 null。
  String? action(UiLocale l) => _sellerKYCFlowStatusAction(l, value);

  /// seller.KYCFlowStatus.banner_description；沒有文字的值回 null。
  String? bannerDescription(UiLocale l) => _sellerKYCFlowStatusBannerDescription(l, value);

  /// seller.KYCFlowStatus.banner_title；沒有文字的值回 null。
  String? bannerTitle(UiLocale l) => _sellerKYCFlowStatusBannerTitle(l, value);

  /// seller.KYCFlowStatus.gate_heading；沒有文字的值回 null。
  String? gateHeading(UiLocale l) => _sellerKYCFlowStatusGateHeading(l, value);

  /// seller.KYCFlowStatus.gate_message；沒有文字的值回 null。
  String? gateMessage(UiLocale l, {required String subject}) => _sellerKYCFlowStatusGateMessage(l, value, subject);
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

String? _sellerPayUniState(UiLocale l, int value) {
return switch ((l, value)) {
(UiLocale.zhTW || UiLocale.en, 1) => 'PayUni 審核中',
(UiLocale.zhTW || UiLocale.en, 2) => 'PayUni 需要補件',
(UiLocale.zhTW || UiLocale.en, 3) => 'PayUni 需要補件',
(UiLocale.zhTW || UiLocale.en, 4) => 'PayUni 需要補件',
(UiLocale.zhTW || UiLocale.en, 5) => 'PayUni 申請未通過',
(UiLocale.zhTW || UiLocale.en, 6) => 'PayUni 暫時無法處理',
(UiLocale.zhTW || UiLocale.en, 7) => 'PayUni 收款帳戶已終止',
(UiLocale.zhTW || UiLocale.en, 8) => 'PayUni 收款已啟用',
_ => null,
};
}

extension SellerPayUniStateUiText on hawkr_seller_v1alpha1_seller.PayUniState {
  /// seller.PayUniState 的名詞；UNSPECIFIED 與不認得的值回 null，要顯示「未知」的呼叫端自己用 commonUnknown(l)。
  String? uiText(UiLocale l) => _sellerPayUniState(l, value);
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
(UiLocale.zhTW || UiLocale.en, 3) => '商家認證已通過',
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

/// seller.SellerPayUni.fix_fields_heading
String sellerSellerPayUniFixFieldsHeading(UiLocale l) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => '請修正以下資料',
};
}

/// seller.SellerPayUni.reason_heading
String sellerSellerPayUniReasonHeading(UiLocale l) {
return switch (l) {
UiLocale.zhTW || UiLocale.en => 'PayUni 的審核意見',
};
}

