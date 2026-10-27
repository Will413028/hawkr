import 'package:hooks_riverpod/legacy.dart'; // StateProvider 在 Riverpod 3.0 移到 legacy

/// A simple provider that signals a tab switch request to MainNavigationScreen.
/// Write a tab index to trigger navigation; MainNavigationScreen listens and resets to -1.
final tabSwitchRequestProvider = StateProvider<int>((ref) => -1);

/// 記住 MainNavigationScreen 當前選中的 tab index。
/// 當 pop() 回到 / 時，MainNavigationScreen 會讀取此值恢復 tab 狀態。
final currentTabProvider = StateProvider<int>((ref) => 0);
