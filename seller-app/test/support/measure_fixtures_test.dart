import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:flutter_test/flutter_test.dart';

import 'measure_fixtures.dart';

/// 測試假資料守住契約：`can_appeal` 為否時 `appeal_unavailabilities` 至少一筆、為是時為空，原因不含 UNSPECIFIED，每一筆都帶伺服器的句子。
void main() {
  test('can_appeal=false 又沒給原因：補一個有效原因，不留空清單', () {
    final m = measureOf(canAppeal: false);
    expect(m.appealUnavailabilities, isNotEmpty);
    expect(
      m.appealUnavailabilities.map((u) => u.reason),
      isNot(
        contains(AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_UNSPECIFIED),
      ),
    );
    for (final u in m.appealUnavailabilities) {
      expect(u.message.locale, 'zh-TW');
      expect(u.message.message, isNotEmpty);
    }
  });

  test('can_appeal=true：原因清單是空的', () {
    expect(measureOf().appealUnavailabilities, isEmpty);
  });

  test('明確給空清單卻 can_appeal=false：拒絕', () {
    expect(
      () => measureOf(canAppeal: false, appealUnavailableReasons: const []),
      throwsArgumentError,
    );
  });

  test('can_appeal=true 卻給原因：拒絕', () {
    expect(
      () => measureOf(
        appealUnavailableReasons: [
          AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REVOKED,
        ],
      ),
      throwsArgumentError,
    );
  });

  test('原因含 UNSPECIFIED：拒絕（只有 withUnrecognizedAppealReason 能造出來）', () {
    expect(
      () => measureOf(
        canAppeal: false,
        appealUnavailableReasons: [
          AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_UNSPECIFIED,
        ],
      ),
      throwsArgumentError,
    );
  });
}
