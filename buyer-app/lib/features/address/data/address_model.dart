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
    @Default('') String townCode,
    @Default('') String addressDetail,
    @Default(false) bool isDefault,
  }) = _Address;
}

/// 地址的完整顯示文字（縣市 + 鄉鎮市區 + 明細，內政部寫法）。
extension AddressDisplay on Address {
  String get fullAddress => formatAddress(townCode, addressDetail);
}
