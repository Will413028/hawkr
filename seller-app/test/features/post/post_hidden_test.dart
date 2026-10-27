import 'package:hawkr_seller/features/post/data/post_hidden.dart';
import 'package:flutter_test/flutter_test.dart';

/// 契約鍵（backend `PostListItemData`，PR backend-payuni#2235 起全面 camelCase）：
/// `hiddenAt`（unix int，`omitempty`）、`hiddenReason`（string，`omitempty`）。
/// **沒有 snake 別名** —— 下面的負向案例就是守這件事：舊鍵不得再被認得，
/// 否則等於偷偷留一條雙讀 fallback，將來後端形狀再變也不會轉紅。
void main() {
  test('isPostHidden true when hiddenAt > 0', () {
    expect(isPostHidden({'hiddenAt': 1720000000}), isTrue);
  });
  test('isPostHidden false when 0 / null / absent', () {
    expect(isPostHidden({'hiddenAt': 0}), isFalse);
    expect(isPostHidden({'hiddenAt': null}), isFalse);
    expect(isPostHidden(<String, dynamic>{}), isFalse);
  });
  test('只有舊 snake 鍵 hidden_at → 一律視為未下架（不得有 fallback）', () {
    expect(isPostHidden({'hidden_at': 1720000000}), isFalse);
  });

  test('hiddenReasonOf reads hiddenReason, null when empty', () {
    expect(hiddenReasonOf({'hiddenReason': '違規內容'}), '違規內容');
    expect(hiddenReasonOf({'hiddenReason': ''}), isNull);
    expect(hiddenReasonOf(<String, dynamic>{}), isNull);
  });
  test('只有舊 snake 鍵 hidden_reason → null（不得有 fallback）', () {
    expect(hiddenReasonOf({'hidden_reason': '違規內容'}), isNull);
  });
}
