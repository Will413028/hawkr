import 'package:flutter/material.dart';

import '../../core/theme/hk_tokens.dart';

/// 停用的控制項（分享、發布等）：保留控制項名稱供讀屏，說明放在 hint 與點擊提示（同 `HkTapTarget` 的分法）。
///
/// 點擊不會落到底下的卡片或頁面。[hint] 為 null 表示不知道為什麼停用，不顯示說明。圖示與文字的顏色由呼叫端用 disabled token 給。
class HkDisabledWithHint extends StatelessWidget {
  const HkDisabledWithHint({
    super.key,
    required this.semanticLabel,
    required this.child,
    this.hint,
  });

  final String semanticLabel;
  final Widget child;
  final String? hint;

  @override
  Widget build(BuildContext context) {
    final hint = this.hint;
    final box = ConstrainedBox(
      constraints: const BoxConstraints(
        minWidth: HkControl.minTap,
        minHeight: HkControl.minTap,
      ),
      child: Center(child: child),
    );
    return Semantics(
      container: true,
      button: true,
      enabled: false,
      label: semanticLabel,
      hint: hint,
      child: ExcludeSemantics(
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: () {},
          child: hint == null
              ? box
              : Tooltip(
                  message: hint,
                  triggerMode: TooltipTriggerMode.tap,
                  child: box,
                ),
        ),
      ),
    );
  }
}
