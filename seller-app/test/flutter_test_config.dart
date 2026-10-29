import 'dart:async';

import 'package:hawkr_seller/shared/widgets/animated_orbs_background.dart';

/// 登入頁背景依時刻切換晝夜配色與文字墨色；固定成白天，覆蓋率與畫面斷言才不隨 CI 執行的時刻改變。
Future<void> testExecutable(FutureOr<void> Function() testMain) async {
  AnimatedOrbsBackground.debugNow = () => DateTime(2026, 1, 15, 12);
  await testMain();
}
