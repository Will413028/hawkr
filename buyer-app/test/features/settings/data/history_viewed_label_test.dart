import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/features/settings/presentation/my_subscription_screen.dart';

void main() {
  test('yesterday is decided by the Taipei calendar day', () {
    // 2026-07-31 00:10 Taipei.
    final now = DateTime.utc(2026, 7, 30, 16, 10);
    // 2026-07-30 00:05 Taipei: 24h05 earlier but 1 calendar day ago.
    expect(
      historyViewedLabel(DateTime.utc(2026, 7, 29, 16, 5), now: now),
      '昨天看過',
    );
    // 2026-07-29 23:30 Taipei: 24h40 earlier, 2 calendar days ago.
    expect(
      historyViewedLabel(DateTime.utc(2026, 7, 29, 15, 30), now: now),
      '2 天前看過',
    );
  });

  test('older entries show days or Taipei date', () {
    final now = DateTime.utc(2026, 8, 5, 2);
    expect(historyViewedLabel(DateTime.utc(2026, 8, 2, 2), now: now), '3 天前看過');
    expect(
      historyViewedLabel(DateTime.utc(2026, 7, 20, 2), now: now),
      '7/20 看過',
    );
    expect(
      historyViewedLabel(now.subtract(const Duration(seconds: 5)), now: now),
      '剛剛看過',
    );
  });
}
