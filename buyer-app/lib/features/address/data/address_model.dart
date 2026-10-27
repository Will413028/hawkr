import 'package:hawkr_twaddr/hawkr_twaddr.dart';
import 'package:freezed_annotation/freezed_annotation.dart';

part 'address_model.freezed.dart';

@freezed
abstract class Address with _$Address {
  const factory Address({
    required int id,
    required String etag,
    required String receiverName,
    required String receiverPhone,
    // 內政部 8 碼鄉鎮市區代碼與其後的明細（postal_address）；縣市、郵遞區號、名稱由 hawkr_twaddr 依代碼取得。
    @Default('') String townCode,
    @Default('') String addressDetail,
    @Default(false) bool isDefault,
  }) = _Address;
}

/// 地址的完整顯示文字（縣市 + 鄉鎮市區 + 明細，內政部寫法）。
extension AddressDisplay on Address {
  String get fullAddress => formatAddress(townCode, addressDetail);
}
