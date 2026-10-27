import 'package:freezed_annotation/freezed_annotation.dart';

part 'review_model.freezed.dart';
part 'review_model.g.dart';

@freezed
abstract class Review with _$Review {
  const factory Review({
    required int id,
    @JsonKey(name: 'skuId') required int skuId,
    @JsonKey(name: 'orderItemId') int? orderItemId,
    required int rating,
    required String content,
    @Default([]) List<String> images,
    @JsonKey(name: 'userId') int? userId,
    @JsonKey(name: 'username') String? username,
    // 49 §6.1 成對契約：avatarKey 是裸 objectKey（SSOT），顯示前過 buildImageUrl。
    @JsonKey(name: 'avatarKey') String? avatarKey,
    @JsonKey(name: 'createdAt') String? createdAt,
  }) = _Review;

  factory Review.fromJson(Map<String, dynamic> json) => _$ReviewFromJson(json);
}
