import 'package:riverpod_annotation/riverpod_annotation.dart';

part 'refresh_clock.g.dart';

/// 重讀伺服器狀態時用的現在時間（`CoalescingRefresher` 的保鮮期用它判斷）；測試換成可控的時鐘。
@Riverpod(keepAlive: true)
DateTime Function() refreshClock(Ref ref) => DateTime.now;
