import 'package:freezed_annotation/freezed_annotation.dart';

part 'ai_style_model.freezed.dart';
part 'ai_style_model.g.dart';

/// `GET /app/ai_styles` 的一筆風格。
///
/// 欄名以 gateway `AiStyleItem`（backend-payuni `gateway/internal/handler/ai.go:22-32`）
/// 為準，縮圖是 49 §6.1 成對的 `thumbnailKey`（裸 objectKey，SSOT）+ `thumbnailUrl`
/// （Gateway 以 MEDIA_BASE_URL 組的 `/i/original/` 顯示 URL）。
///
/// **只讀 key**：顯示端帶 `scene` 過 `buildImageUrl` 才會套上縮圖 preset；
/// 直接用 `thumbnailUrl` 等於每張 100×100 的風格縮圖都去拉原圖。
@freezed
abstract class AiStyle with _$AiStyle {
  const factory AiStyle({
    required String id,
    required String name,
    required String label,
    required String prompt,
    @Default('') String thumbnailKey,
    @Default('0') String sortOrder,
  }) = _AiStyle;

  factory AiStyle.fromJson(Map<String, dynamic> json) =>
      _$AiStyleFromJson(json);
}

@freezed
abstract class AiStylesResponse with _$AiStylesResponse {
  const factory AiStylesResponse({
    @Default([]) List<AiStyle> styles,
    @Default('') String defaultPrompt,
    @Default('0') String weeklyUsage,
    @Default('10') String weeklyLimit,
  }) = _AiStylesResponse;

  factory AiStylesResponse.fromJson(Map<String, dynamic> json) =>
      _$AiStylesResponseFromJson(json);
}
