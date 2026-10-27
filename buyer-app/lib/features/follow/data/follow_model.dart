import 'package:freezed_annotation/freezed_annotation.dart';

part 'follow_model.freezed.dart';
part 'follow_model.g.dart';

@freezed
abstract class FansCount with _$FansCount {
  const factory FansCount({
    @JsonKey(name: 'merchantId') int? merchantId,
    @Default(0) int count,
  }) = _FansCount;

  factory FansCount.fromJson(Map<String, dynamic> json) =>
      _$FansCountFromJson(json);
}
