import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';

import 'store_form.dart';

/// 認證頁編輯、寫進店家（`UpdateMyStore`）的資料：聯絡人姓名、聯絡人手機、客服電子信箱與店家地址。PayUni 綁定與同步都
/// 讀店家的值，KYC 申請上沒有它們的副本。
final class StoreContactDraft {
  const StoreContactDraft({
    this.contactName = '',
    this.contactMobile = '',
    this.supportEmail = '',
    this.townCode = '',
    this.addressDetail = '',
  });

  final String contactName;
  final String contactMobile;
  final String supportEmail;
  final String townCode;
  final String addressDetail;

  StoreContactDraft copyWith({
    String? contactName,
    String? contactMobile,
    String? supportEmail,
    String? townCode,
    String? addressDetail,
  }) => StoreContactDraft(
    contactName: contactName ?? this.contactName,
    contactMobile: contactMobile ?? this.contactMobile,
    supportEmail: supportEmail ?? this.supportEmail,
    townCode: townCode ?? this.townCode,
    addressDetail: addressDetail ?? this.addressDetail,
  );

  @override
  bool operator ==(Object other) =>
      other is StoreContactDraft &&
      other.contactName == contactName &&
      other.contactMobile == contactMobile &&
      other.supportEmail == supportEmail &&
      other.townCode == townCode &&
      other.addressDetail == addressDetail;

  @override
  int get hashCode => Object.hash(
    contactName,
    contactMobile,
    supportEmail,
    townCode,
    addressDetail,
  );
}

/// 伺服器上的店家在認證頁的輸入形狀。
StoreContactDraft storeContactDraftOf(Store? store) {
  if (store == null) return const StoreContactDraft();
  return StoreContactDraft(
    contactName: store.contactName,
    contactMobile: store.contactMobile,
    supportEmail: store.supportEmail,
    townCode: store.hasAddress() ? store.address.townCode : '',
    addressDetail: store.hasAddress() ? store.address.addressDetail.trim() : '',
  );
}

/// 版本衝突之後採用最新的店家：使用者自上次儲存以來沒有改過的欄位取伺服器的值（別處改過的不會被當成使用者的修改寫回去），
/// 使用者自己改過、還沒寫進去的欄位保留。
StoreContactDraft rebaseStoreContact({
  required StoreContactDraft draft,
  required StoreContactDraft was,
  required StoreContactDraft latest,
}) {
  String pick(String mine, String base, String theirs) =>
      mine.trim() == base.trim() ? theirs : mine;
  final addressEdited =
      draft.townCode != was.townCode ||
      draft.addressDetail.trim() != was.addressDetail.trim();
  return StoreContactDraft(
    contactName: pick(draft.contactName, was.contactName, latest.contactName),
    contactMobile: pick(
      draft.contactMobile,
      was.contactMobile,
      latest.contactMobile,
    ),
    supportEmail: pick(
      draft.supportEmail,
      was.supportEmail,
      latest.supportEmail,
    ),
    townCode: addressEdited ? draft.townCode : latest.townCode,
    addressDetail: addressEdited ? draft.addressDetail : latest.addressDetail,
  );
}

/// [draft] 相對於 [baseline]（伺服器上的店家）要寫的欄位；沒有要寫的欄位時是 null。
StoreChanges? pendingStoreChanges(
  StoreContactDraft draft,
  StoreContactDraft baseline,
) {
  String normalized(StoreTextField field, String value) =>
      field.normalize(value);
  final text = <StoreTextField, String>{};
  void diff(StoreTextField field, String value, String base) {
    final next = normalized(field, value);
    if (next != normalized(field, base)) text[field] = next;
  }

  diff(StoreTextField.contactName, draft.contactName, baseline.contactName);
  diff(
    StoreTextField.contactMobile,
    draft.contactMobile,
    baseline.contactMobile,
  );
  diff(StoreTextField.supportEmail, draft.supportEmail, baseline.supportEmail);

  final detail = draft.addressDetail.trim();
  final addressChanged =
      draft.townCode != baseline.townCode ||
      detail != baseline.addressDetail.trim();
  StoreChanges changes = StoreChanges(text: text);
  if (addressChanged) {
    changes = draft.townCode.isEmpty && detail.isEmpty
        ? StoreChanges(text: text, clearAddress: true)
        : changes.withAddress(townCode: draft.townCode, addressDetail: detail);
  }
  return changes.isEmpty ? null : changes;
}

/// 聯絡資料已填欄位的格式錯誤；空白不報（是否必填由伺服器的缺項決定）。
final class StoreContactErrors {
  const StoreContactErrors({this.name, this.mobile, this.email});

  final String? name;
  final String? mobile;
  final String? email;

  bool get any => name != null || mobile != null || email != null;
}

StoreContactErrors storeContactErrors(StoreContactDraft draft) =>
    StoreContactErrors(
      name: StoreTextField.contactName.validate(
        StoreTextField.contactName.normalize(draft.contactName),
      ),
      mobile: StoreTextField.contactMobile.validate(
        StoreTextField.contactMobile.normalize(draft.contactMobile),
      ),
      email: StoreTextField.supportEmail.validate(
        StoreTextField.supportEmail.normalize(draft.supportEmail),
      ),
    );

/// UpdateMyStore 的欄位錯誤（`store.` 加 Store 的欄位名）對應到聯絡欄位的錯誤文字；其他欄位忽略。
StoreContactErrors storeContactServerErrors(Map<String, String> fields) =>
    StoreContactErrors(
      name: fields[StoreTextField.contactName.errorPath],
      mobile: fields[StoreTextField.contactMobile.errorPath],
      email: fields[StoreTextField.supportEmail.errorPath],
    );

/// 地址輸入還沒完成（有明細卻沒有鄉鎮市區）的錯誤文字；完整或整個空白是 null。
String? storeAddressInputError(StoreContactDraft draft) => validateAddressInput(
  townCode: draft.townCode,
  addressDetail: draft.addressDetail,
);
