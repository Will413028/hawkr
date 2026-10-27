import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter/material.dart';

/// Read-only 版本的 hashtag highlight：把字串內 `#xxx` 切成獨立 TextSpan，
/// 上品牌深色 [AppColors.primaryDark] + `FontWeight.w700`，其餘維持 baseStyle。
///
/// 字元集與 seller `HashtagSpanBuilder` 一致（Unicode letter / digit / underscore），
/// 讓編輯態與顯示態視覺對齊。
final RegExp _kHashtagPattern = RegExp(r'#[\p{L}\p{N}_]+', unicode: true);

const Color _kHashtagColor = AppColors.brandPrimaryStrong;

List<InlineSpan> buildHashtagSpans(String content, TextStyle baseStyle) {
  final spans = <InlineSpan>[];
  final tagStyle = baseStyle.copyWith(
    color: _kHashtagColor,
    fontWeight: FontWeight.w700,
  );
  int last = 0;
  for (final m in _kHashtagPattern.allMatches(content)) {
    if (m.start > last) {
      spans.add(
        TextSpan(text: content.substring(last, m.start), style: baseStyle),
      );
    }
    spans.add(TextSpan(text: m.group(0)!, style: tagStyle));
    last = m.end;
  }
  if (last < content.length) {
    spans.add(TextSpan(text: content.substring(last), style: baseStyle));
  }
  return spans;
}
