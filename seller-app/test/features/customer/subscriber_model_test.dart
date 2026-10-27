import 'package:hawkr_seller/features/customer/data/subscriber_model.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  test('已刪除的買家顯示墓碑字串，不顯示暱稱或 id', () {
    const s = Subscriber(
      buyerId: '7',
      subscribedAt: '2026-10-01T00:00:00Z',
      nickname: '舊暱稱',
      deleted: true,
    );
    expect(s.displayName, '已刪除帳號');
  });

  test('沒有暱稱的買家以 id 顯示', () {
    const s = Subscriber(buyerId: '7', subscribedAt: '2026-10-01T00:00:00Z');
    expect(s.displayName, '用戶 7');
  });
}
