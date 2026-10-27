import 'package:hawkr_api/gen/hawkr/common/v1alpha1/postal_address.pb.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';

/// 商店設定頁可以改的文字欄位：標籤、格式限制與錯誤文字和 website 的 `lib/seller/store-contact.ts`、`validators.ts`
/// 同一套語意（D1）；標籤的「商店電話」「店家地址」是兩端一起改名後的寫法。
enum StoreTextField {
  storeName(
    path: 'store_name',
    label: '商店名稱',
    maxLength: 40,
    placeholder: '例如：範例果園',
  ),
  description(
    path: 'description',
    label: '商店介紹',
    maxLength: 500,
    maxLines: 6,
    placeholder: '一段話告訴買家你是誰、種了什麼、為什麼買你的',
  ),
  contactPhone(path: 'contact_phone', label: '商店電話', maxLength: 20),
  contactName(path: 'contact_name', label: '聯絡人姓名', maxLength: 20),
  contactMobile(
    path: 'contact_mobile',
    label: '聯絡人手機',
    maxLength: 10,
    hint: '09 開頭的 10 碼手機號碼',
  ),
  supportEmail(
    path: 'support_email',
    label: '客服電子信箱',
    maxLength: supportEmailMaxLength,
    hint: 'PayUni 會寄金流相關通知到這個信箱',
  ),
  websiteUrl(
    path: 'website_url',
    label: '官方網站',
    maxLength: 240,
    placeholder: 'https://example.com',
    hint: '可填寫品牌官網、Facebook 粉絲專頁或其他公開網站',
  ),
  prize(
    path: 'prize',
    label: '得獎紀錄 / 店鋪亮點',
    maxLength: 120,
    maxLines: 2,
    placeholder: '請填寫得獎獎項',
  );

  const StoreTextField({
    required this.path,
    required this.label,
    required this.maxLength,
    this.maxLines = 1,
    this.placeholder,
    this.hint,
  });

  /// `Store` 的欄位名（update_mask 的路徑）。
  final String path;
  final String label;
  final int maxLength;
  final int maxLines;
  final String? placeholder;
  final String? hint;

  /// UpdateMyStore 的欄位錯誤路徑（`store.` 加 `Store` 的欄位名）。
  String get errorPath => 'store.$path';

  /// 店名與商店介紹在 PayUni 綁定後鎖定（`GetMyKYC.flow.identity_locked`）。
  bool get lockedByPayuni =>
      this == StoreTextField.storeName || this == StoreTextField.description;

  /// 店家目前的值。
  String valueOf(Store? store) {
    if (store == null) return '';
    return switch (this) {
      storeName => store.storeName,
      description => store.description,
      contactPhone => store.contactPhone,
      contactName => store.contactName,
      contactMobile => store.contactMobile,
      supportEmail => store.supportEmail,
      websiteUrl => store.websiteUrl,
      prize => store.prize,
    };
  }

  /// 送出前整理使用者輸入：去頭尾空白；官方網站沒有 scheme 時補 `https://`；手機去掉空白、橫線與括號。
  String normalize(String raw) {
    final value = raw.trim();
    return switch (this) {
      websiteUrl => normalizeWebsiteUrl(value),
      contactMobile => _stripPhoneSeparators(value),
      _ => value,
    };
  }

  /// 已整理過的 [value] 的格式錯誤；空白表示清掉，不在這裡報（是否必填由伺服器決定）。
  String? validate(String value) {
    if (value.isEmpty) return null;
    return switch (this) {
      contactName => value.runes.length > 20 ? '聯絡人姓名最多 20 字' : null,
      contactMobile =>
        RegExp(r'^09\d{8}$').hasMatch(value) ? null : '請輸入 09 開頭的 10 碼手機號碼',
      supportEmail => _validateSupportEmail(value),
      websiteUrl =>
        isValidWebsiteUrl(value) ? null : '請輸入有效的網站網址，例如 https://example.com',
      _ => null,
    };
  }

  /// 只改這個欄位的寫入。
  StoreChanges changes(String value) => StoreChanges(text: {this: value});
}

const supportEmailMaxLength = 50;

final _emailPattern = RegExp(r'^[^\s@]+@[^\s@]+\.[^\s@]+$');

String? _validateSupportEmail(String value) {
  if (!_emailPattern.hasMatch(value)) return 'Email 格式不正確';
  if (value.runes.length > supportEmailMaxLength) {
    return '客服電子信箱最多 $supportEmailMaxLength 字';
  }
  return null;
}

String _stripPhoneSeparators(String value) =>
    value.replaceAll(RegExp(r'[\s\-()]'), '');

/// 沒有 scheme 的網址補 `https://`。
String normalizeWebsiteUrl(String raw) {
  final value = raw.trim();
  if (value.isEmpty) return '';
  return RegExp(r'^https?://', caseSensitive: false).hasMatch(value)
      ? value
      : 'https://$value';
}

final _hostPattern = RegExp(
  r'^(?:[a-z0-9_-]+(?:\.[a-z0-9_-]+)*\.?|\[[0-9a-f:.]+\])$',
  caseSensitive: false,
);

/// 空字串（清掉）或有主機名稱的 http(s) 絕對網址。
bool isValidWebsiteUrl(String raw) {
  if (raw.isEmpty) return true;
  final uri = Uri.tryParse(raw);
  if (uri == null || !uri.hasScheme) return false;
  if (uri.scheme != 'http' && uri.scheme != 'https') return false;
  return uri.host.isNotEmpty && _hostPattern.hasMatch(uri.host);
}

/// 一次 UpdateMyStore 要寫的欄位；沒列的欄位不動（不進 update_mask）。
final class StoreChanges {
  const StoreChanges({
    this.text = const {},
    this.address,
    this.clearAddress = false,
    this.logoStagedToken,
    this.coverStagedToken,
  });

  final Map<StoreTextField, String> text;

  /// 要換成的地址（update_mask 的 `address` 整個換掉）。
  final PostalAddress? address;

  /// 清掉地址（列進 update_mask 但請求不帶 address）。
  final bool clearAddress;

  final String? logoStagedToken;
  final String? coverStagedToken;

  bool get isEmpty => paths.isEmpty;

  /// update_mask 的路徑，順序固定。
  List<String> get paths => [
    for (final field in StoreTextField.values)
      if (text.containsKey(field)) field.path,
    if (address != null || clearAddress) 'address',
    if (logoStagedToken != null) 'logo_staged_token',
    if (coverStagedToken != null) 'cover_staged_token',
  ];

  /// 帶上讀到的 [etag] 的請求店家。
  Store toStore(String etag) {
    final store = Store(etag: etag);
    for (final entry in text.entries) {
      final value = entry.value;
      switch (entry.key) {
        case StoreTextField.storeName:
          store.storeName = value;
        case StoreTextField.description:
          store.description = value;
        case StoreTextField.contactPhone:
          store.contactPhone = value;
        case StoreTextField.contactName:
          store.contactName = value;
        case StoreTextField.contactMobile:
          store.contactMobile = value;
        case StoreTextField.supportEmail:
          store.supportEmail = value;
        case StoreTextField.websiteUrl:
          store.websiteUrl = value;
        case StoreTextField.prize:
          store.prize = value;
      }
    }
    if (address != null) store.address = address!;
    if (logoStagedToken != null) store.logoStagedToken = logoStagedToken!;
    if (coverStagedToken != null) store.coverStagedToken = coverStagedToken!;
    return store;
  }

  StoreChanges withAddress({
    required String townCode,
    required String addressDetail,
  }) => StoreChanges(
    text: text,
    address: PostalAddress(townCode: townCode, addressDetail: addressDetail),
    logoStagedToken: logoStagedToken,
    coverStagedToken: coverStagedToken,
  );
}

/// 地址欄位的輸入檢查：有地址明細就一定要有鄉鎮市區（契約不接受只有明細的地址）。
String? validateAddressInput({
  required String townCode,
  required String addressDetail,
}) {
  if (addressDetail.trim().isNotEmpty && townCode.isEmpty) {
    return '請選擇縣市與鄉鎮市區';
  }
  return null;
}
