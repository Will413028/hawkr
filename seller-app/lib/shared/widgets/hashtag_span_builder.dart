import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:extended_text_field/extended_text_field.dart';
import 'package:flutter/material.dart';

/// rich text editor 內 `#xxx` hashtag 的 inline 高亮樣式。
///
/// 設計決定：編輯中用 TextSpan 上色（品牌深綠 + 粗體），不做 WidgetSpan chip。
/// 理由：
/// - WidgetSpan 在 TextField 內 cursor / selection 行為不穩，編輯體驗差
/// - X / Twitter / Notion 等業界做法都是「色彩高亮」，非真實 chip
/// - 真正的 chip 形狀留給 preview / 商品詳情頁的 read-only 渲染
///
/// 使用方式：
/// ```dart
/// ExtendedTextField(
///   controller: ...,
///   specialTextSpanBuilder: const HashtagSpanBuilder(),
///   ...
/// )
/// ```
class HashtagSpanBuilder extends SpecialTextSpanBuilder {
  HashtagSpanBuilder({this.tagColor, this.tagWeight});

  /// hashtag 顯示色（預設 AppColors.primaryDark）
  final Color? tagColor;

  /// hashtag 字重（預設 w700）
  final FontWeight? tagWeight;

  @override
  SpecialText? createSpecialText(
    String flag, {
    TextStyle? textStyle,
    SpecialTextGestureTapCallback? onTap,
    int? index,
  }) {
    if (flag != _HashtagSpecialText.flag) return null;
    return _HashtagSpecialText(
      textStyle: textStyle,
      onTap: onTap,
      startIndex: index ?? 0,
      tagColor: tagColor ?? AppColors.brandPrimaryStrong,
      tagWeight: tagWeight ?? FontWeight.w700,
    );
  }
}

class _HashtagSpecialText extends SpecialText {
  _HashtagSpecialText({
    TextStyle? textStyle,
    SpecialTextGestureTapCallback? onTap,
    required this.startIndex,
    required this.tagColor,
    required this.tagWeight,
  }) : super(flag, '', textStyle, onTap: onTap);

  static const String flag = '#';
  final int startIndex;
  final Color tagColor;
  final FontWeight tagWeight;

  static final RegExp _bodyChar = RegExp(r'[\p{L}\p{N}_]', unicode: true);

  /// `#` 後遇到第一個「非合法 hashtag 字元」就視為結束。
  /// 空白、標點、emoji 都會結束 hashtag。
  @override
  bool isEnd(String value) {
    if (value.length <= 1) return false; // 還沒收到 hashtag body
    final last = value.substring(value.length - 1);
    return !_bodyChar.hasMatch(last);
  }

  @override
  InlineSpan finishText() {
    final raw = toString(); // 包含 # 加 body 加結束 char (若有)
    final m = RegExp(r'^#([\p{L}\p{N}_]+)', unicode: true).firstMatch(raw);
    if (m == null || m.group(1) == null || m.group(1)!.isEmpty) {
      // 只有 `#` 沒有 body → 不上色，當普通文字
      return TextSpan(text: raw, style: textStyle);
    }
    final tagPart = '#${m.group(1)}'; // 純 hashtag (含 #)
    final rest = raw.substring(tagPart.length); // 結束字元（空白 / 標點等）

    final tagStyle = (textStyle ?? const TextStyle()).copyWith(
      color: tagColor,
      fontWeight: tagWeight,
    );

    if (rest.isEmpty) {
      return TextSpan(text: tagPart, style: tagStyle);
    }
    // tag 本體上色，結束字元維持原樣
    return TextSpan(
      children: [
        TextSpan(text: tagPart, style: tagStyle),
        TextSpan(text: rest, style: textStyle),
      ],
    );
  }
}
