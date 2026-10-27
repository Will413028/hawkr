import 'package:freezed_annotation/freezed_annotation.dart';

part 'hashtag_model.freezed.dart';
part 'hashtag_model.g.dart';

/// Hashtag 自動補全 / 商品 hashtag 顯示用 model。
/// 對齊 OpenAPI [handler.HashtagItem] schema。
@freezed
abstract class HashtagItem with _$HashtagItem {
  const factory HashtagItem({
    /// canonical 名稱（後端 normalize 後的 lowercase 字串）
    required String name,

    /// 顯示名稱（保留原大小寫）；auto-complete 場景 server 回空字串
    @Default('') String displayName,

    /// 引用此 hashtag 的商品總數（即時值）
    @Default(0) int productCount,
  }) = _HashtagItem;

  factory HashtagItem.fromJson(Map<String, dynamic> json) =>
      _$HashtagItemFromJson(json);
}
