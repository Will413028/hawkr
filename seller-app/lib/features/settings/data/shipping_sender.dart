import 'package:hawkr_twaddr/hawkr_twaddr.dart' as twaddr;

import '../../../core/session/my_store.dart';
import '../../../shared/utils/text_length_validator.dart';

/// 物流寄件人的顯示長度上限（PayUni 計長：英文 1、中文 2）。
const taqbinSenderNameMaxDisplayLength = 30;
const cvsSenderNameMaxDisplayLength = 10;

/// 物流 onboarding 的 `payuniSubmitStatus`：4302 處理中、4303 成功時地址鎖定。
const payuniSubmitRequested = '4302';
const payuniSubmitSucceeded = '4303';
const payuniSubmitFailed = '4304';

/// 這筆 onboarding 的寄件資料已交給 PayUni（處理中或成功）：只有這兩種狀態鎖定。
bool isShippingRowLocked(Map<String, dynamic> row) {
  final status = row['payuniSubmitStatus']?.toString() ?? '';
  return status == payuniSubmitRequested || status == payuniSubmitSucceeded;
}

/// 這筆 onboarding 已建店且核准（有 onboardingId 且 4303），可用 toggle 增減溫層。
bool isShippingRowToggleable(Map<String, dynamic> row) {
  final id =
      row['onboardingId']?.toString() ??
      row['onboarding_id']?.toString() ??
      row['id']?.toString() ??
      '';
  return id.isNotEmpty &&
      (row['payuniSubmitStatus']?.toString() ?? '') == payuniSubmitSucceeded;
}

/// 表單用的寄件人。空字串表示沒有值；縣市與區是物流名稱表（`taiwan_cities_data.dart`，寫「台」）的寫法。
final class ShippingSender {
  const ShippingSender({
    this.name = '',
    this.phone = '',
    this.city = '',
    this.district = '',
    this.addressDetail = '',
  });

  final String name;
  final String phone;
  final String city;
  final String district;
  final String addressDetail;

  bool get isEmpty =>
      name.isEmpty &&
      phone.isEmpty &&
      city.isEmpty &&
      district.isEmpty &&
      addressDetail.isEmpty;
}

/// 物流名稱表寫「台」、內政部（twaddr）寫「臺」。
String toShippingName(String twaddrName) => twaddrName.replaceAll('臺', '台');

String _text(Map<String, dynamic> row, String key) =>
    (row[key]?.toString() ?? '').trim();

ShippingSender _homeSenderOf(Map<String, dynamic> row) => ShippingSender(
  name: _text(row, 'senderName'),
  phone: _text(row, 'senderPhone'),
  city: toShippingName(_text(row, 'senderCity')),
  district: toShippingName(_text(row, 'senderDistrict')),
  addressDetail: _text(row, 'senderAddressDetail'),
);

ShippingSender _cvsSenderOf(Map<String, dynamic> row) => ShippingSender(
  name: _text(row, 'senderName'),
  phone: _text(row, 'senderPhone'),
);

/// 黑貓寄件人：[row]（該物流商第一筆 onboarding，同 website 的 `findOnboarding`）有寄件人紀錄
/// （任何狀態，含失敗）就整組用紀錄，沒有才整組用店家。
///
/// 店家缺的值留空，不退回商店電話、帳號暱稱或店名。
ShippingSender homeSender(Map<String, dynamic>? row, MyStore? store) {
  if (row != null) {
    final sender = _homeSenderOf(row);
    if (!sender.isEmpty) return sender;
  }
  return _storeSender(store, withAddress: true);
}

/// 7-11 寄件人：[reference] 是常溫的 onboarding，沒有才用冷凍的（同 website）；紀錄與店家的取捨同 [homeSender]。
ShippingSender cvsSender(Map<String, dynamic>? reference, MyStore? store) {
  if (reference != null) {
    final sender = _cvsSenderOf(reference);
    if (!sender.isEmpty) return sender;
  }
  return _storeSender(store, withAddress: false);
}

ShippingSender _storeSender(MyStore? store, {required bool withAddress}) {
  if (store == null) return const ShippingSender();
  final name = store.contactName ?? '';
  final phone = store.contactMobile ?? '';
  if (!withAddress) return ShippingSender(name: name, phone: phone);
  final code = store.addressTownCode;
  final town = code == null ? null : twaddr.townByCode(code);
  final county = town == null ? null : twaddr.countyByCode(town.countyCode);
  return ShippingSender(
    name: name,
    phone: phone,
    city: county == null ? '' : toShippingName(county.name),
    district: town == null ? '' : toShippingName(town.name),
    addressDetail: store.addressDetail ?? '',
  );
}

/// 表單選的縣市、區對到的郵遞區號；區對不到代碼時是 null。
String? postalCodeOfSelection(String? city, String? district) {
  if (city == null || district == null) return null;
  final town = twaddr.townByName(city, district);
  if (town == null) return null;
  final postal = twaddr.postalCodeByTownCode(town.code);
  return postal.isEmpty ? null : postal;
}

/// 寄件人姓名顯示長度超過 [maxDisplayLength] 時的提示；空白或沒超過是 null（必填另外檢查）。
String? senderNameLengthError(
  String value, {
  required int maxDisplayLength,
  required String fieldLabel,
}) {
  if (value.trim().isEmpty) return null;
  return TextLengthValidator.validateName(
    value,
    maxDisplayLength: maxDisplayLength,
    fieldLabel: fieldLabel,
  );
}
