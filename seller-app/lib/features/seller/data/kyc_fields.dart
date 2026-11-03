import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

/// 認證表單的區塊，和 website 的 `CertificationSectionId` 同一組。
enum KycSection {
  company('公司資訊'),
  identity('身分證明'),
  bank('銀行帳戶'),
  other('其他資訊'),
  documents('補充文件');

  const KycSection(this.label);

  final String label;
}

/// 認證表單的一步。每一步負責的欄位由 [kycStepOfField] 決定，續填的位置由伺服器的 `missing_fields` 推導。
enum KycStep {
  company(KycSection.company),
  idUpload(KycSection.identity),
  idFront(KycSection.identity),
  idBack(KycSection.identity),
  bankUpload(KycSection.bank),
  bankInfo(KycSection.bank),
  other(KycSection.other),
  documents(KycSection.documents),
  confirm(null);

  const KycStep(this.section);

  final KycSection? section;

  /// 認證旅程列（`CertificationJourneyStepper`）上這一步屬於哪一格。
  int get journeyStep => switch (this) {
    company || idUpload || idFront || idBack => 1,
    _ => 2,
  };
}

/// 申請人填的是哪一種身分：個人或企業，對應 `KYCApplication.subject` 設定的是哪一支。
enum KycSubject {
  individual,
  business;

  /// 這種身分的步驟順序，最後一步一定是 [KycStep.confirm]。
  List<KycStep> get steps => switch (this) {
    individual => const [
      KycStep.idUpload,
      KycStep.idFront,
      KycStep.idBack,
      KycStep.bankUpload,
      KycStep.bankInfo,
      KycStep.other,
      KycStep.confirm,
    ],
    business => const [
      KycStep.company,
      KycStep.idUpload,
      KycStep.idFront,
      KycStep.idBack,
      KycStep.bankUpload,
      KycStep.bankInfo,
      KycStep.other,
      KycStep.documents,
      KycStep.confirm,
    ],
  };
}

/// 欄位所在的一步，與它在表單裡的控制項：同一個控制項的欄位在缺項清單與 PayUni 問題只列一次（見 [fieldsByControl]）。
typedef _FieldMeta = ({KycStep step, String control});

/// 伺服器的欄位（缺項、退回要補的欄位、PayUni 問題的欄位）所在的一步與表單控制項，和 website 的 `KYC_FIELDS` 同一張表；
/// 名稱取自共用文案目錄（[kycFieldLabel]）。銀行名稱與銀行代碼在這支 App 是兩個輸入框，各自一個控制項；個人與公司負責人的
/// 身分證字號是同一個輸入框。
const Map<KYCField, _FieldMeta> _kycFields = {
  KYCField.KYC_FIELD_REAL_NAME: (step: KycStep.idFront, control: 'realName'),
  KYCField.KYC_FIELD_BIRTH_DATE: (step: KycStep.idFront, control: 'birthDate'),
  KYCField.KYC_FIELD_ID_CARD_ISSUE_DATE: (
    step: KycStep.idBack,
    control: 'idCardIssueDate',
  ),
  KYCField.KYC_FIELD_ID_CARD_ISSUE_TYPE: (
    step: KycStep.idBack,
    control: 'idCardIssueType',
  ),
  KYCField.KYC_FIELD_ID_CARD_ISSUE_LOCATION: (
    step: KycStep.idBack,
    control: 'idCardIssueLocation',
  ),
  KYCField.KYC_FIELD_ENGLISH_NAME: (
    step: KycStep.other,
    control: 'englishName',
  ),
  KYCField.KYC_FIELD_BANK_NAME: (step: KycStep.bankInfo, control: 'bankName'),
  KYCField.KYC_FIELD_BANK_CODE: (step: KycStep.bankInfo, control: 'bankCode'),
  KYCField.KYC_FIELD_BANK_ACCOUNT: (
    step: KycStep.bankInfo,
    control: 'bankAccount',
  ),
  KYCField.KYC_FIELD_PRODUCT_TYPE: (
    step: KycStep.company,
    control: 'productType',
  ),
  KYCField.KYC_FIELD_WEBSITE_URL: (step: KycStep.other, control: 'websiteUrl'),
  KYCField.KYC_FIELD_ID_CARD_FRONT: (
    step: KycStep.idUpload,
    control: 'idCardFront',
  ),
  KYCField.KYC_FIELD_ID_CARD_BACK: (
    step: KycStep.idUpload,
    control: 'idCardBack',
  ),
  KYCField.KYC_FIELD_BANK_BOOK: (step: KycStep.bankUpload, control: 'bankBook'),
  KYCField.KYC_FIELD_INDIVIDUAL_ID_CARD_NO: (
    step: KycStep.idFront,
    control: 'idCardNo',
  ),
  KYCField.KYC_FIELD_BUSINESS_COMPANY_NAME: (
    step: KycStep.company,
    control: 'companyName',
  ),
  KYCField.KYC_FIELD_BUSINESS_TAX_ID: (step: KycStep.company, control: 'taxId'),
  KYCField.KYC_FIELD_BUSINESS_COMPANY_REGISTER_DATE: (
    step: KycStep.company,
    control: 'companyRegisterDate',
  ),
  KYCField.KYC_FIELD_BUSINESS_COMPANY_PHONE: (
    step: KycStep.company,
    control: 'companyPhone',
  ),
  KYCField.KYC_FIELD_BUSINESS_REP_ID_CARD_NO: (
    step: KycStep.idFront,
    control: 'idCardNo',
  ),
  KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS: (
    step: KycStep.documents,
    control: 'companyDocuments',
  ),
  KYCField.KYC_FIELD_BUSINESS_COMPANY_ADDRESS: (
    step: KycStep.company,
    control: 'companyAddress',
  ),
};

/// 欄位的呈現名稱（共用文案目錄）；不認得的欄位（含 `UNSPECIFIED`）回 null，不顯示空白或原始列舉名。
String? kycFieldLabel(KYCField field, UiLocale l) =>
    _kycFields.containsKey(field) ? field.uiText(l) : null;

/// 欄位所在的一步；不認得的欄位回 null。
KycStep? kycStepOfField(KYCField field) => _kycFields[field]?.step;

/// 欄位的區塊。
KycSection? kycSectionOfField(KYCField field) =>
    _kycFields[field]?.step.section;

/// 同一個表單控制項只留一個欄位：留 proto 順序（欄位號）最前面的那個，位置依該控制項第一次出現的順序。缺項清單與 PayUni
/// 問題的欄位都用這條規則，同一個控制項（個人與負責人共用的身分證字號輸入框）只列一次。不認得的欄位略過。
List<KYCField> fieldsByControl(Iterable<KYCField> fields) {
  final byControl = <String, KYCField>{};
  for (final field in fields) {
    final meta = _kycFields[field];
    if (meta == null) continue;
    final seen = byControl[meta.control];
    if (seen == null || field.value < seen.value) {
      byControl[meta.control] = field;
    }
  }
  return byControl.values.toList();
}

/// 缺項清單的一項：它在哪個區塊、叫什麼。
final class KycMissingItem {
  const KycMissingItem({required this.section, required this.label});

  final KycSection section;
  final String label;

  @override
  bool operator ==(Object other) =>
      other is KycMissingItem &&
      other.section == section &&
      other.label == label;

  @override
  int get hashCode => Object.hash(section, label);
}

/// 伺服器的 `missing_fields` 轉成畫面的缺項清單：依表單控制項去重（[fieldsByControl]）。
List<KycMissingItem> missingItemsOf(Iterable<KYCField> missing, UiLocale l) => [
  for (final field in fieldsByControl(missing))
    if (kycSectionOfField(field) case final section?)
      if (kycFieldLabel(field, l) case final label?)
        KycMissingItem(section: section, label: label),
];

/// 店家欄位的呈現名稱（共用文案目錄）；不認得的欄位回 null。
String? storeFieldLabel(StoreField field, UiLocale l) => field.uiText(l);

/// 目錄認得的店家欄位（`UNSPECIFIED` 與不認得的值不是）。目錄的必備語系 zh-TW 涵蓋每個值，所以不需要請求語系。
bool isKnownStoreField(StoreField field) =>
    storeFieldLabel(field, UiLocale.zhTW) != null;

/// 伺服器的 `missing_store_fields` 轉成缺項清單：缺什麼只由伺服器決定，不依本機輸入增減。
List<KycMissingItem> storeMissingItemsOf(
  Iterable<StoreField> missing,
  UiLocale l,
) => [
  for (final field in missing)
    if (storeFieldLabel(field, l) case final label?)
      KycMissingItem(section: KycSection.other, label: label),
];

/// 缺項的提示文字（SubmitKYC 之前或回 `invalid_argument` 之後）。
String missingItemsMessage(Iterable<KycMissingItem> items) =>
    '下列欄位尚未填寫：${items.map((item) => item.label).join('、')}';

/// SubmitKYC 回 failed_precondition 之後重讀申請：伺服器的 `missing_store_fields` 有缺項就列出缺哪些，沒有缺項時
/// （例如身分比對的固定訊息）原樣顯示伺服器的訊息。
String submitPreconditionMessage(
  Iterable<StoreField> missing,
  String serverMessage,
  UiLocale l,
) {
  final items = storeMissingItemsOf(missing, l);
  return items.isNotEmpty ? missingItemsMessage(items) : serverMessage;
}

/// BadRequest 的欄位路徑（`business.tax_id`）對應回 [KYCField]；路徑指到訊息欄位底下（`business.company_address.town_code`）時，
/// 取最長的前綴（`business.company_address`）；重複欄位裡的單一元素（`business.company_documents[2]`）指到整個欄位
/// （表單不分文件標示，標在那個欄位上）；不認得的路徑回 null。
KYCField? kycFieldOfPath(String path) {
  final segments = path.replaceAll(RegExp(r'\[\d+\]'), '').split('.');
  for (var length = segments.length; length > 0; length--) {
    final name = 'KYC_FIELD_${segments.take(length).join('_').toUpperCase()}';
    for (final field in KYCField.values) {
      if (field.name == name && field != KYCField.KYC_FIELD_UNSPECIFIED) {
        return field;
      }
    }
  }
  return null;
}

/// 證件欄位：值是上傳的圖片，不在表單的文字欄位裡（`_valueIn` 沒有它們），沒有行內的提示位置。
bool isKycDocumentField(KYCField field) => switch (field) {
  KYCField.KYC_FIELD_ID_CARD_FRONT ||
  KYCField.KYC_FIELD_ID_CARD_BACK ||
  KYCField.KYC_FIELD_BANK_BOOK ||
  KYCField.KYC_FIELD_BUSINESS_COMPANY_DOCUMENTS => true,
  _ => false,
};

/// 證件欄位的錯誤在提示裡的文字：證件的名稱加上伺服器的訊息（例如「身分證正面照：圖片無法使用，請重新上傳」）；補充文件的單一元素
/// （`business.company_documents[2]`）註明第幾張。不是證件欄位回 null。
String? kycDocumentNotice(
  String path,
  KYCField field,
  String message,
  UiLocale l,
) {
  if (!isKycDocumentField(field)) return null;
  final label = kycFieldLabel(field, l) ?? '證件';
  final index = RegExp(r'\[(\d+)\]').firstMatch(path)?.group(1);
  final where = index == null ? label : '$label（第 ${int.parse(index) + 1} 張）';
  return '$where：$message';
}

/// 續填的位置：第一個還有缺項的一步；沒有缺項回 null。
KycStep? resumeStepOf(
  KycSubject subject,
  Iterable<KYCField> missing,
  Iterable<StoreField> missingStore,
) {
  final steps = {
    for (final field in missing)
      if (kycStepOfField(field) case final step?) step,
    if (missingStore.any(isKnownStoreField)) KycStep.other,
  };
  for (final step in subject.steps) {
    if (steps.contains(step)) return step;
  }
  return null;
}
