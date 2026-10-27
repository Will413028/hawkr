import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/features/seller/data/measure_display.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/measure_fixtures.dart';

void main() {
  group('名詞取自共用文案目錄（hawkr_ui_text）', () {
    test('TerminationGround：十一個事由的文字逐一對照（meta 的「事由：」），UNSPECIFIED 不顯示', () {
      final expected = {
        TerminationGround.TERMINATION_GROUND_NOTICE: '預告終止（第 21.2 條）',
        TerminationGround.TERMINATION_GROUND_MATERIAL_BREACH: '重大違約未改善',
        TerminationGround.TERMINATION_GROUND_CEASED_BUSINESS: '停業',
        TerminationGround.TERMINATION_GROUND_DISSOLUTION: '解散',
        TerminationGround.TERMINATION_GROUND_BANKRUPTCY: '破產',
        TerminationGround.TERMINATION_GROUND_LICENSE_REVOKED: '必要許可遭撤銷',
        TerminationGround.TERMINATION_GROUND_FOOD_SAFETY_INCIDENT: '重大食安事件',
        TerminationGround.TERMINATION_GROUND_FRAUD: '詐欺事件',
        TerminationGround.TERMINATION_GROUND_PLATFORM_SECURITY: '危害平台安全',
        TerminationGround.TERMINATION_GROUND_OTHER_SERIOUS: '其他嚴重影響履約的情形',
        TerminationGround.TERMINATION_GROUND_LINKED_ACCOUNT: '關聯帳號',
      };
      expect(
        TerminationGround.values.where(
          (g) => g != TerminationGround.TERMINATION_GROUND_UNSPECIFIED,
        ),
        unorderedEquals(expected.keys),
        reason: '新增事由時要同步補這張表與目錄',
      );
      for (final entry in expected.entries) {
        expect(
          measureMeta(terminationOf(ground: entry.key), UiLocale.zhTW),
          '措施：終止合作 · 事由：${entry.value}',
          reason: '${entry.key}',
        );
      }
    });

    test('MeasureKind 與 MeasureState：措施種類在 meta、合作已終止在狀態標示與效果', () {
      expect(measureMeta(measureOf(), UiLocale.zhTW), '嚴重程度：高 · 措施：停權');
      final active = terminationOf(state: MeasureState.MEASURE_STATE_ACTIVE);
      expect(stateChip(active, UiLocale.zhTW)?.label, '合作已終止');
      final scheduled = terminationOf(
        state: MeasureState.MEASURE_STATE_SCHEDULED,
      );
      expect(stateChip(scheduled, UiLocale.zhTW)?.label, '預定生效');
    });

    test('RevokeReason：連帶撤銷的兩個原因有說明，其餘（含 UNSPECIFIED）效果文字是「已撤銷」', () {
      String? effectOf(RevokeReason reason) => measureEffect(
        terminationOf(
          state: MeasureState.MEASURE_STATE_REVOKED,
          revokeReason: reason,
        ),
        UiLocale.zhTW,
      )?.text;
      expect(effectOf(RevokeReason.REVOKE_REASON_FINDING_REVOKED), '依據的認定已撤銷');
      expect(effectOf(RevokeReason.REVOKE_REASON_SOURCE_REVOKED), '關聯的終止已撤銷');
      for (final reason in [
        RevokeReason.REVOKE_REASON_UNSPECIFIED,
        RevokeReason.REVOKE_REASON_APPEAL_ACCEPTED,
        RevokeReason.REVOKE_REASON_ADMIN_CORRECTION,
        RevokeReason.REVOKE_REASON_EXTERNAL_DECISION,
        RevokeReason.REVOKE_REASON_OTHER,
      ]) {
        expect(effectOf(reason), '已撤銷', reason: '$reason');
      }
    });
  });

  group('終止合作的呈現', () {
    // 建立時間 10/15 10:00（臺北），預告 30 天後是 11/14；伺服器給的生效時間刻意是 11/20 12:30（臺北）。
    final created = DateTime.utc(2026, 10, 15, 2);
    final start = DateTime.utc(2026, 11, 20, 4, 30);

    test('標題與 meta：依據是契約條款，meta 帶措施與事由', () {
      final m = terminationOf(createTime: created);
      expect(measureTitle(m, UiLocale.zhTW), '契約條款');
      expect(measureMeta(m, UiLocale.zhTW), '措施：終止合作 · 事由：預告終止（第 21.2 條）');
      final linked = terminationOf(
        ground: TerminationGround.TERMINATION_GROUND_LINKED_ACCOUNT,
        state: MeasureState.MEASURE_STATE_ACTIVE,
      );
      expect(measureMeta(linked, UiLocale.zhTW), '措施：終止合作 · 事由：關聯帳號');
    });

    test('事由 UNSPECIFIED 不顯示；非終止的措施即使帶了事由也不顯示', () {
      expect(
        measureMeta(
          terminationOf(
            ground: TerminationGround.TERMINATION_GROUND_UNSPECIFIED,
          ),
          UiLocale.zhTW,
        ),
        '措施：終止合作',
      );
      final suspension = measureOf(
        terminationGround: TerminationGround.TERMINATION_GROUND_FRAUD,
      );
      expect(measureMeta(suspension, UiLocale.zhTW), '嚴重程度：高 · 措施：停權');
    });

    test('預定生效：「終止生效：{period.start_time}」，不是建立時間加 30 天', () {
      final m = terminationOf(start: start, createTime: created);
      final effect = measureEffect(m, UiLocale.zhTW);
      expect(effect?.text, '終止生效：2026/11/20 12:30');
      expect(effect?.active, isFalse);
      expect(effect!.text, isNot(contains('2026/11/14')));
    });

    test('生效中（含店家已關閉後仍是 ACTIVE）：終止生效，仍生效', () {
      final m = terminationOf(
        state: MeasureState.MEASURE_STATE_ACTIVE,
        start: start,
      );
      expect(measureEffect(m, UiLocale.zhTW)?.text, '終止生效：2026/11/20 12:30');
      expect(measureEffect(m, UiLocale.zhTW)?.active, isTrue);
    });

    test('預告終止因店家關閉而結束：加「合作已於 {end} 結束」；沒有結束時間只寫終止生效', () {
      final closed = terminationOf(
        state: MeasureState.MEASURE_STATE_ENDED,
        start: start,
        end: DateTime.utc(2026, 11, 1, 6, 15),
      );
      expect(
        measureEffect(closed, UiLocale.zhTW)?.text,
        '終止生效：2026/11/20 12:30，合作已於 2026/11/01 14:15 結束',
      );
      final noEnd = terminationOf(
        state: MeasureState.MEASURE_STATE_ENDED,
        start: start,
      );
      expect(
        measureEffect(noEnd, UiLocale.zhTW)?.text,
        '終止生效：2026/11/20 12:30',
      );
    });

    test('已撤銷：關聯的終止已撤銷', () {
      final m = terminationOf(
        state: MeasureState.MEASURE_STATE_REVOKED,
        revokeReason: RevokeReason.REVOKE_REASON_SOURCE_REVOKED,
        canAppeal: false,
      );
      expect(measureEffect(m, UiLocale.zhTW)?.text, '關聯的終止已撤銷');
    });

    test('終止合作沒有生效時間（違反合約）：不顯示影響文字，也不出現 1970', () {
      final m = measureOf(
        kind: MeasureKind.MEASURE_KIND_TERMINATION,
        basis: MeasureBasis.MEASURE_BASIS_CONTRACT,
        state: MeasureState.MEASURE_STATE_SCHEDULED,
      );
      expect(measureEffect(m, UiLocale.zhTW), isNull);
      expect(terminationNotices([m], UiLocale.zhTW), isEmpty);
    });

    test('生效中的終止合作 chip 是「合作已終止」（danger）；停權生效中沒有', () {
      final chip = stateChip(
        terminationOf(state: MeasureState.MEASURE_STATE_ACTIVE),
        UiLocale.zhTW,
      );
      expect(chip?.label, '合作已終止');
      expect(chip?.tone, MeasureChipTone.danger);
      expect(stateChip(activeSuspension(), UiLocale.zhTW), isNull);
      expect(
        stateChip(
          terminationOf(state: MeasureState.MEASURE_STATE_ENDED),
          UiLocale.zhTW,
        ),
        isNull,
      );
    });

    test('預定生效 chip：只有 SCHEDULED 有', () {
      expect(stateChip(terminationOf(), UiLocale.zhTW)?.label, '預定生效');
      expect(
        stateChip(terminationOf(), UiLocale.zhTW)?.tone,
        MeasureChipTone.warning,
      );
      for (final state in MeasureState.values.where(
        (s) => s != MeasureState.MEASURE_STATE_SCHEDULED,
      )) {
        expect(
          stateChip(measureOf(state: state), UiLocale.zhTW),
          isNull,
          reason: '$state',
        );
      }
    });

    test('申覆入口說明：原因與期限同一行以「 · 」相接，缺一個就只顯示另一個，都沒有是 null', () {
      final deadline = DateTime.utc(2027, 5, 22, 4, 30);
      expect(
        appealNoteText(
          terminationOf(
            canAppeal: false,
            appealDeadlineTime: deadline,
            appealUnavailableReasons: [
              AppealUnavailableReason
                  .APPEAL_UNAVAILABLE_REASON_ALREADY_APPEALED,
            ],
          ),
          UiLocale.zhTW,
        ),
        '$appealAlreadyAppealedSentence · 申覆期限：2027/05/22 12:30',
      );
      expect(
        appealNoteText(
          terminationOf(appealDeadlineTime: deadline),
          UiLocale.zhTW,
        ),
        '申覆期限：2027/05/22 12:30',
      );
      expect(appealNoteText(measureOf(), UiLocale.zhTW), isNull);
    });

    test('申覆期限：只有終止合作、而且伺服器給了才顯示（臺北時間）', () {
      final deadline = DateTime.utc(2027, 5, 22, 4, 30);
      expect(
        appealDeadlineText(
          terminationOf(appealDeadlineTime: deadline),
          UiLocale.zhTW,
        ),
        '申覆期限：2027/05/22 12:30',
      );
      expect(appealDeadlineText(terminationOf(), UiLocale.zhTW), isNull);
      expect(
        appealDeadlineText(
          measureOf(appealDeadlineTime: deadline),
          UiLocale.zhTW,
        ),
        isNull,
      );
    });
  });

  group('首頁預告文字（terminationNotices）', () {
    final created = DateTime.utc(2026, 10, 15, 2);
    final start = DateTime.utc(2026, 11, 20, 4, 30);

    test('日期等於 period.start_time，不是建立時間加 30 天', () {
      final notices = terminationNotices([
        terminationOf(start: start, createTime: created),
      ], UiLocale.zhTW);
      final text = notices.single.text;
      expect(text, '平台已通知將於 2026/11/20 12:30 終止合作。在此之前店家照常營業；如有異議，可提出申覆。');
      expect(text, isNot(contains('2026/11/14')));
    });

    test('沒有預定生效的終止：沒有預告；其他種類與狀態不算', () {
      expect(terminationNotices(const [], UiLocale.zhTW), isEmpty);
      expect(
        terminationNotices([
          terminationOf(state: MeasureState.MEASURE_STATE_ACTIVE),
          terminationOf(state: MeasureState.MEASURE_STATE_ENDED),
          measureOf(state: MeasureState.MEASURE_STATE_SCHEDULED),
        ], UiLocale.zhTW),
        isEmpty,
      );
    });

    test('多筆預定生效：每筆一則，順序照伺服器給的（不依生效時間重排）', () {
      final notices = terminationNotices([
        terminationOf(id: 'late', start: DateTime.utc(2026, 12, 1, 4, 30)),
        terminationOf(id: 'early', start: start),
      ], UiLocale.zhTW);
      expect([for (final n in notices) n.measureId], ['late', 'early']);
      expect(notices[0].text, contains('2026/12/01 12:30'));
      expect(notices[1].text, contains('2026/11/20 12:30'));
    });
  });

  group('申覆入口（appealAction）與分組', () {
    test('可以申覆：啟用，沒有原因', () {
      final a = appealAction(terminationOf());
      expect(a.enabled, isTrue);
      expect(a.reason, isNull);
    });

    test('不能申覆：原因取伺服器排好的第一筆的句子', () {
      final m = terminationOf(
        canAppeal: false,
        appealUnavailableReasons: [
          AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_APPEAL_WINDOW_ENDED,
          AppealUnavailableReason.APPEAL_UNAVAILABLE_REASON_REQUEST_PENDING,
        ],
      );
      final a = appealAction(m);
      expect(a.enabled, isFalse);
      expect(a.reason, appealWindowEndedSentence);
    });

    test('can_appeal 讀伺服器的值：原因清單不影響是否啟用，UNSPECIFIED 的原因不當成有效文字', () {
      final m = withUnrecognizedAppealReason(terminationOf());
      expect(appealAction(m).enabled, isFalse);
      expect(appealAction(m).reason, isNull);
    });

    test('預定生效的終止即使已申覆（can_appeal 為 false）仍是需要處理', () {
      final scheduled = terminationOf(canAppeal: false);
      expect(measureNeedsAttention(scheduled), isTrue);
      final ended = terminationOf(
        canAppeal: false,
        state: MeasureState.MEASURE_STATE_ENDED,
      );
      expect(measureNeedsAttention(ended), isFalse);
      final groups = groupMeasures([scheduled, ended]);
      expect([for (final m in groups.attention) m.id], ['t1']);
    });
  });
}
