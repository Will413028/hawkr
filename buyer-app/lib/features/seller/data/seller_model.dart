import 'package:freezed_annotation/freezed_annotation.dart';

part 'seller_model.freezed.dart';
part 'seller_model.g.dart';

/// `GET /public/sellers/{id}/rating` 的 data（gateway `SellerRating`）。
///
/// 欄名以後端實際回傳為準：storeId / rating / reviewCount / {five..one}StarPct。
/// 舊宣告寫的是 average / total / fiveStar —— 後端一個都不吐，所以商家評分與
/// 評論數在這條路徑上一直是 null。
@freezed
abstract class RatingStats with _$RatingStats {
  const factory RatingStats({
    // 44 號身分分離：賣家業務鍵是 store surrogate。
    @JsonKey(name: 'storeId') String? storeId,
    @JsonKey(name: 'rating') double? average,
    @JsonKey(name: 'reviewCount') int? total,
    // 星級為百分比（omitempty：全 0 時不出現）。
    @JsonKey(name: 'fiveStarPct') double? fiveStarPct,
    @JsonKey(name: 'fourStarPct') double? fourStarPct,
    @JsonKey(name: 'threeStarPct') double? threeStarPct,
    @JsonKey(name: 'twoStarPct') double? twoStarPct,
    @JsonKey(name: 'oneStarPct') double? oneStarPct,
  }) = _RatingStats;

  factory RatingStats.fromJson(Map<String, dynamic> json) =>
      _$RatingStatsFromJson(json);
}
