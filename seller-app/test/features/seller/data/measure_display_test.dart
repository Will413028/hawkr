import 'package:hawkr_api/gen/hawkr/common/v1alpha1/policy_category.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/features/seller/data/measure_display.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../../support/measure_fixtures.dart';

void main() {
  group('measureEffect：措施目前的影響（讀 state）', () {
    final end = DateTime.utc(2026, 10, 19, 8, 30); // 臺北 2026/10/19 16:30

    test('已撤銷：一般寫「已撤銷」，依據的認定被撤銷時寫原因', () {
      final revoked = measureOf(
        state: MeasureState.MEASURE_STATE_REVOKED,
        canAppeal: false,
        period: measurePeriod(end: end),
      );
      expect(measureEffect(revoked, UiLocale.zhTW)?.text, '已撤銷');
      expect(measureEffect(revoked, UiLocale.zhTW)?.active, isFalse);
      final cascaded = measureOf(
        state: MeasureState.MEASURE_STATE_REVOKED,
        revokeReason: RevokeReason.REVOKE_REASON_FINDING_REVOKED,
        canAppeal: false,
      );
      expect(measureEffect(cascaded, UiLocale.zhTW)?.text, '依據的認定已撤銷');
    });

    test('警告：已發出，不是生效中的限制', () {
      final warning = measureOf(
        kind: MeasureKind.MEASURE_KIND_WARNING,
        state: MeasureState.MEASURE_STATE_ISSUED,
      );
      expect(measureEffect(warning, UiLocale.zhTW)?.text, '已發出');
      expect(measureEffect(warning, UiLocale.zhTW)?.active, isFalse);
    });

    test('停權生效中：至結束時間（臺北時間）', () {
      final m = measureOf(period: measurePeriod(end: end));
      expect(measureEffect(m, UiLocale.zhTW)?.text, '停權中（至 2026/10/19 16:30）');
      expect(measureEffect(m, UiLocale.zhTW)?.active, isTrue);
    });

    test('state 讀伺服器的值，不用現在時間與 end_time 比較', () {
      // end_time 在遠古的過去、伺服器仍說生效中：照 state。
      final m = measureOf(
        period: measurePeriod(end: DateTime.utc(2020, 1, 1, 8)),
      );
      expect(measureEffect(m, UiLocale.zhTW)?.text, '停權中（至 2020/01/01 16:00）');
      final ended = measureOf(
        state: MeasureState.MEASURE_STATE_ENDED,
        period: measurePeriod(end: DateTime.utc(2099, 1, 1, 8)),
      );
      expect(
        measureEffect(ended, UiLocale.zhTW)?.text,
        '已於 2099/01/01 16:00 結束',
      );
      expect(measureEffect(ended, UiLocale.zhTW)?.active, isFalse);
    });

    test('沒有結束時間的生效中停權：未定結束時間，平台定期複審；不出現 1970', () {
      for (final period in [measurePeriod(start: end), null]) {
        final m = measureOf(period: period);
        expect(measureEffect(m, UiLocale.zhTW)?.text, '停權中（未定結束時間，平台定期複審）');
        expect(measureEffect(m, UiLocale.zhTW)?.active, isTrue);
      }
    });

    test('停權效果句的英文來自目錄（三句）', () {
      final until = measureOf(period: measurePeriod(end: end));
      expect(
        measureEffect(until, UiLocale.en)?.text,
        'Suspended until 2026/10/19 16:30',
      );
      final open = measureOf(period: measurePeriod(start: end));
      expect(
        measureEffect(open, UiLocale.en)?.text,
        'Suspended (no end date; under periodic review by the platform)',
      );
      final ended = measureOf(
        state: MeasureState.MEASURE_STATE_ENDED,
        period: measurePeriod(end: end),
      );
      expect(
        measureEffect(ended, UiLocale.en)?.text,
        'Ended on 2026/10/19 16:30',
      );
    });

    test('已結束但沒有結束時間：只寫「已結束」', () {
      final m = measureOf(
        state: MeasureState.MEASURE_STATE_ENDED,
        period: measurePeriod(),
      );
      expect(measureEffect(m, UiLocale.zhTW)?.text, '已結束');
    });

    test('所有文字都不含 1970', () {
      for (final state in [
        MeasureState.MEASURE_STATE_ISSUED,
        MeasureState.MEASURE_STATE_ACTIVE,
        MeasureState.MEASURE_STATE_ENDED,
        MeasureState.MEASURE_STATE_REVOKED,
      ]) {
        expect(
          measureEffect(
            measureOf(state: state, period: measurePeriod()),
            UiLocale.zhTW,
          )?.text,
          isNot(contains('1970')),
        );
      }
    });

    test('UNSPECIFIED 的狀態：不顯示（不當成有效值）', () {
      expect(
        measureEffect(
          measureOf(
            state: MeasureState.MEASURE_STATE_UNSPECIFIED,
            period: measurePeriod(end: end),
          ),
          UiLocale.zhTW,
        ),
        isNull,
      );
    });
  });

  group('分組：需要處理 / 歷史', () {
    test('can_appeal 為真或停權生效中是需要處理，其餘是歷史', () {
      final canAppealOnly = measureOf(
        id: 'a',
        canAppeal: true,
        state: MeasureState.MEASURE_STATE_ENDED,
      );
      final activeOnly = measureOf(id: 'b', canAppeal: false);
      final ended = measureOf(
        id: 'c',
        canAppeal: false,
        state: MeasureState.MEASURE_STATE_ENDED,
      );
      final revoked = measureOf(
        id: 'd',
        canAppeal: false,
        state: MeasureState.MEASURE_STATE_REVOKED,
      );
      final warning = measureOf(
        id: 'e',
        canAppeal: false,
        kind: MeasureKind.MEASURE_KIND_WARNING,
        state: MeasureState.MEASURE_STATE_ISSUED,
      );

      final groups = groupMeasures([
        canAppealOnly,
        activeOnly,
        ended,
        revoked,
        warning,
      ]);

      expect([for (final m in groups.attention) m.id], ['a', 'b']);
      expect([for (final m in groups.history) m.id], ['c', 'd', 'e']);
    });

    test('can_appeal 由伺服器決定：已結束又沒申覆、但 can_appeal 為 false，不自己推導成需要處理', () {
      final m = measureOf(
        state: MeasureState.MEASURE_STATE_ENDED,
        canAppeal: false,
      );
      expect(m.requests, isEmpty);
      expect(measureNeedsAttention(m), isFalse);
    });
  });

  group('標籤與申覆說明', () {
    test('認定類別、嚴重程度、措施的對照', () {
      expect(measureTitle(measureOf(), UiLocale.zhTW), '食品安全');
      expect(measureMeta(measureOf(), UiLocale.zhTW), '嚴重程度：高 · 措施：停權');
      expect(
        measureMeta(
          measureOf(
            kind: MeasureKind.MEASURE_KIND_WARNING,
            findings: [
              finding(severity: ViolationSeverity.VIOLATION_SEVERITY_CRITICAL),
            ],
          ),
          UiLocale.zhTW,
        ),
        '嚴重程度：嚴重 · 措施：警告',
      );
      final labels = {
        PolicyCategory.POLICY_CATEGORY_PROHIBITED_GOODS: '違法或禁售商品',
        PolicyCategory.POLICY_CATEGORY_UNLICENSED_SALE: '未經許可販售',
        PolicyCategory.POLICY_CATEGORY_MISLEADING_INFO: '不實或誤導資訊',
        PolicyCategory.POLICY_CATEGORY_FAKE_CERTIFICATION: '假認證',
        PolicyCategory.POLICY_CATEGORY_COUNTERFEIT: '仿冒商品',
        PolicyCategory.POLICY_CATEGORY_IP_INFRINGEMENT: '侵害智慧財產權',
        PolicyCategory.POLICY_CATEGORY_FRAUD: '詐欺或操縱交易',
        PolicyCategory.POLICY_CATEGORY_OFF_PLATFORM_TRADE: '繞過平台交易',
        PolicyCategory.POLICY_CATEGORY_PRIVACY: '不當使用個人資料',
        PolicyCategory.POLICY_CATEGORY_IMPERSONATION: '冒用或誤導交易主體',
        PolicyCategory.POLICY_CATEGORY_PLATFORM_ABUSE: '干擾或濫用平台',
        PolicyCategory.POLICY_CATEGORY_OTHER: '其他',
      };
      labels.forEach((category, label) {
        expect(
          measureTitle(
            measureOf(findings: [finding(category: category)]),
            UiLocale.zhTW,
          ),
          label,
        );
      });
    });

    test('多筆認定：標題與嚴重程度都取主要依據（第一筆），與 website 相同', () {
      final m = measureOf(
        findings: [
          finding(
            id: 'f1',
            category: PolicyCategory.POLICY_CATEGORY_FRAUD,
            severity: ViolationSeverity.VIOLATION_SEVERITY_MEDIUM,
          ),
          finding(
            id: 'f2',
            category: PolicyCategory.POLICY_CATEGORY_FOOD_SAFETY,
          ),
          finding(id: 'f3', category: PolicyCategory.POLICY_CATEGORY_FRAUD),
        ],
      );
      expect(measureTitle(m, UiLocale.zhTW), '詐欺或操縱交易');
      expect(measureMeta(m, UiLocale.zhTW), '嚴重程度：中 · 措施：停權');
    });

    test('風險依據：標題是風險類型，沒有嚴重程度', () {
      final labels = {
        RiskGround.RISK_GROUND_TRANSACTION: '交易風險',
        RiskGround.RISK_GROUND_FOOD_SAFETY: '食品安全',
        RiskGround.RISK_GROUND_FRAUD: '詐欺',
        RiskGround.RISK_GROUND_PERSONAL_DATA: '個人資料',
        RiskGround.RISK_GROUND_INFRINGEMENT: '侵權',
        RiskGround.RISK_GROUND_SECURITY: '資安',
        RiskGround.RISK_GROUND_FULFILLMENT: '履約',
        RiskGround.RISK_GROUND_EXTERNAL_ORDER: '主管機關或司法機關的要求',
      };
      labels.forEach((ground, label) {
        expect(
          measureTitle(riskSuspension(ground: ground), UiLocale.zhTW),
          '風險：$label',
        );
      });
      expect(measureMeta(riskSuspension(), UiLocale.zhTW), '措施：停權');
    });

    test('UNSPECIFIED 的依據、類別、風險類型用通用文字，嚴重程度與措施不顯示', () {
      final noBasis = measureOf(
        basis: MeasureBasis.MEASURE_BASIS_UNSPECIFIED,
        kind: MeasureKind.MEASURE_KIND_UNSPECIFIED,
      );
      expect(measureTitle(noBasis, UiLocale.zhTW), '措施紀錄');
      expect(measureMeta(noBasis, UiLocale.zhTW), isNull);
      final unspecified = measureOf(
        kind: MeasureKind.MEASURE_KIND_UNSPECIFIED,
        findings: [
          finding(
            category: PolicyCategory.POLICY_CATEGORY_UNSPECIFIED,
            severity: ViolationSeverity.VIOLATION_SEVERITY_UNSPECIFIED,
          ),
        ],
      );
      expect(measureTitle(unspecified, UiLocale.zhTW), '措施紀錄');
      expect(measureMeta(unspecified, UiLocale.zhTW), isNull);
      expect(
        measureTitle(
          riskSuspension(ground: RiskGround.RISK_GROUND_UNSPECIFIED),
          UiLocale.zhTW,
        ),
        '風險措施',
      );
    });

    MeasureRequest request(MeasureRequestState state, [String reason = '']) =>
        MeasureRequest(state: state, decisionReason: reason);

    final pending = MeasureRequestState.MEASURE_REQUEST_STATE_PENDING;
    final accepted = MeasureRequestState.MEASURE_REQUEST_STATE_ACCEPTED;
    final rejected = MeasureRequestState.MEASURE_REQUEST_STATE_REJECTED;
    final closed = MeasureRequestState.MEASURE_REQUEST_STATE_CLOSED;

    test('請求 chip：沒有請求不顯示；申覆三種狀態；UNSPECIFIED 與結案不顯示', () {
      expect(requestChip(measureOf(), UiLocale.zhTW), isNull);
      expect(
        requestChip(measureOf(appeal: request(pending)), UiLocale.zhTW)?.label,
        '申覆審核中',
      );
      expect(
        requestChip(measureOf(appeal: request(accepted)), UiLocale.zhTW)?.label,
        '申覆已通過',
      );
      expect(
        requestChip(measureOf(appeal: request(rejected)), UiLocale.zhTW)?.label,
        '申覆已駁回',
      );
      for (final state in [
        closed,
        MeasureRequestState.MEASURE_REQUEST_STATE_UNSPECIFIED,
      ]) {
        expect(
          requestChip(measureOf(appeal: request(state)), UiLocale.zhTW),
          isNull,
        );
      }
    });

    test('結案（CLOSED）的請求不顯示 chip 也不顯示說明（含有結案原因時）', () {
      final byAppeal = measureOf(
        appeal: MeasureRequest(
          state: closed,
          closeCause:
              MeasureRequestCloseCause.MEASURE_REQUEST_CLOSE_CAUSE_REVOKED,
          decisionReason: '已結案',
        ),
      );
      expect(requestChip(byAppeal, UiLocale.zhTW), isNull);
      expect(requestNote(byAppeal, UiLocale.zhTW), isNull);
      final byRemediation = riskSuspension(
        state: MeasureState.MEASURE_STATE_ENDED,
        remediationRequests: [
          MeasureRequest(
            state: closed,
            closeCause:
                MeasureRequestCloseCause.MEASURE_REQUEST_CLOSE_CAUSE_ENDED,
          ),
        ],
      );
      expect(requestChip(byRemediation, UiLocale.zhTW), isNull);
      expect(requestNote(byRemediation, UiLocale.zhTW), isNull);
    });

    test('請求說明：待處理請等候複核，裁定帶出「平台回覆」（兩種請求同寫法），結案不顯示', () {
      expect(requestNote(measureOf(), UiLocale.zhTW), isNull);
      expect(
        requestNote(
          measureOf(appeal: request(rejected, '證據不足')),
          UiLocale.zhTW,
        ),
        '平台回覆：證據不足',
      );
      expect(
        requestNote(measureOf(appeal: request(pending)), UiLocale.zhTW),
        '已提交申覆，請等候平台複核。',
      );
      expect(
        requestNote(measureOf(appeal: request(closed)), UiLocale.zhTW),
        isNull,
      );
      expect(
        requestNote(
          riskSuspension(remediationRequests: [request(rejected, '尚未改善')]),
          UiLocale.zhTW,
        ),
        '平台回覆：尚未改善',
      );
    });

    test('優先序：待處理 > 最後裁定的請求（跨兩種）；冷卻期不影響標籤與說明', () {
      final byTime = riskSuspension(
        appeal: MeasureRequest(
          state: accepted,
          decideTime: ts(DateTime.utc(2026, 10, 7)),
          decisionReason: '申覆理由',
        ),
        remediationRequests: [
          MeasureRequest(
            state: rejected,
            decideTime: ts(DateTime.utc(2026, 10, 9)),
            decisionReason: '解除理由',
          ),
        ],
      );
      expect(requestChip(byTime, UiLocale.zhTW)?.label, '解除請求已駁回');
      expect(requestNote(byTime, UiLocale.zhTW), '平台回覆：解除理由');

      final withPending = riskSuspension(
        appeal: MeasureRequest(
          state: rejected,
          decideTime: ts(DateTime.utc(2026, 10, 9)),
        ),
        remediationRequests: [request(pending)],
        nextRemediationTime: DateTime.utc(2026, 11, 1),
      );
      expect(requestChip(withPending, UiLocale.zhTW)?.label, '解除請求審核中');
      expect(requestNote(withPending, UiLocale.zhTW), '已提交解除請求，請等候平台複核。');

      final cooling = riskSuspension(
        remediationRequests: [request(rejected, '尚未改善')],
        nextRemediationTime: DateTime.utc(2026, 11, 1),
      );
      expect(requestChip(cooling, UiLocale.zhTW)?.label, '解除請求已駁回');
      expect(requestNote(cooling, UiLocale.zhTW), '平台回覆：尚未改善');
    });
  });

  group('remediationAction：已改善，請求解除', () {
    test('可以按（can_request_remediation）時啟用，沒有原因', () {
      final action = remediationAction(
        riskSuspension(canRequestRemediation: true),
      );
      expect(action?.enabled, isTrue);
      expect(action?.reason, isNull);
    });

    test('NOT_RISK／NOT_ACTIVE 為第一項時不渲染按鈕', () {
      expect(
        remediationAction(
          measureOf(
            remediationUnavailableReasons: [
              RemediationUnavailableReason
                  .REMEDIATION_UNAVAILABLE_REASON_NOT_RISK,
            ],
          ),
        ),
        isNull,
      );
      expect(
        remediationAction(
          riskSuspension(
            state: MeasureState.MEASURE_STATE_ENDED,
            remediationUnavailableReasons: [
              RemediationUnavailableReason
                  .REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE,
            ],
          ),
        ),
        isNull,
      );
    });

    test('結構性的原因在前：違規措施或已結束的 RISK 措施上有待處理的申覆，也沒有解除請求入口', () {
      expect(
        remediationAction(
          measureOf(
            remediationUnavailableReasons: [
              RemediationUnavailableReason
                  .REMEDIATION_UNAVAILABLE_REASON_NOT_RISK,
              RemediationUnavailableReason
                  .REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING,
            ],
          ),
        ),
        isNull,
      );
      expect(
        remediationAction(
          riskSuspension(
            state: MeasureState.MEASURE_STATE_ENDED,
            remediationUnavailableReasons: [
              RemediationUnavailableReason
                  .REMEDIATION_UNAVAILABLE_REASON_NOT_ACTIVE,
              RemediationUnavailableReason
                  .REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING,
            ],
          ),
        ),
        isNull,
      );
    });

    test('原因取伺服器排好的第一筆的句子（不取最後一筆、不自己依優先序推導）', () {
      final next = DateTime.utc(2026, 11, 1, 6, 5); // 臺北 2026/11/01 14:05
      String? reasonOf(List<RemediationUnavailableReason> reasons) =>
          remediationAction(
            riskSuspension(
              remediationUnavailableReasons: reasons,
              nextRemediationTime: next,
            ),
          )?.reason;
      expect(
        reasonOf([
          RemediationUnavailableReason
              .REMEDIATION_UNAVAILABLE_REASON_REMEDIATION_PENDING,
          RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_COOLDOWN,
        ]),
        remediationPendingSentence,
      );
      expect(
        reasonOf([
          RemediationUnavailableReason
              .REMEDIATION_UNAVAILABLE_REASON_APPEAL_PENDING,
          RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_COOLDOWN,
        ]),
        remediationAppealPendingSentence,
      );
      expect(
        reasonOf([
          RemediationUnavailableReason.REMEDIATION_UNAVAILABLE_REASON_COOLDOWN,
        ]),
        remediationCooldownSentence,
      );
      final disabled = remediationAction(
        riskSuspension(
          remediationUnavailableReasons: [
            RemediationUnavailableReason
                .REMEDIATION_UNAVAILABLE_REASON_COOLDOWN,
          ],
          nextRemediationTime: next,
        ),
      );
      expect(disabled?.enabled, isFalse);
    });

    test('畫面不因為有待處理的請求就自己決定原因：原因清單空又不能按時沒有原因', () {
      final action = remediationAction(
        riskSuspension(
          remediationRequests: [
            MeasureRequest(
              state: MeasureRequestState.MEASURE_REQUEST_STATE_PENDING,
            ),
          ],
        ),
      );
      expect(action?.enabled, isFalse);
      expect(action?.reason, isNull);
    });
  });

  group('remediationAction：冷卻的時間由伺服器組進句子', () {
    test('句子逐字取自伺服器；nextRemediationTime 不被拿來組字', () {
      final action = remediationAction(
        riskSuspension(
          nextRemediationTime: DateTime.utc(2026, 11, 1, 6, 5, 1),
          remediationUnavailabilities: [
            remediationUnavailability(
              RemediationUnavailableReason
                  .REMEDIATION_UNAVAILABLE_REASON_COOLDOWN,
              message: '伺服器組好的任意一句',
            ),
          ],
        ),
      );
      expect(action?.enabled, isFalse);
      expect(action?.reason, '伺服器組好的任意一句');
    });

    test('伺服器沒有帶句子：反灰、沒有原因文字，不用 nextRemediationTime 自己組', () {
      final action = remediationAction(
        withoutUnavailableMessages(
          riskSuspension(
            nextRemediationTime: DateTime.utc(2026, 11, 1, 6, 5),
            remediationUnavailableReasons: [
              RemediationUnavailableReason
                  .REMEDIATION_UNAVAILABLE_REASON_COOLDOWN,
            ],
          ),
        ),
      );
      expect(action?.enabled, isFalse);
      expect(action?.reason, isNull);
    });
  });

  group('rejectionCooldownWarning', () {
    String? warningFor(Duration cooldown) => rejectionCooldownWarning(
      riskSuspension(rejectionCooldown: cooldown),
      UiLocale.zhTW,
    );

    test('沒有 rejection_cooldown 沒有警告；整日數照給', () {
      expect(rejectionCooldownWarning(riskSuspension(), UiLocale.zhTW), isNull);
      expect(warningFor(const Duration(days: 7)), '若本次經駁回，將於 7 日內不得再提出解除請求');
    });

    test('不足整日無條件進位（6 日 23 小時 -> 7、1 小時 -> 1、7 日 1 秒 -> 8）', () {
      expect(
        warningFor(const Duration(days: 6, hours: 23)),
        '若本次經駁回，將於 7 日內不得再提出解除請求',
      );
      expect(warningFor(const Duration(hours: 1)), '若本次經駁回，將於 1 日內不得再提出解除請求');
      expect(
        warningFor(const Duration(days: 7, seconds: 1)),
        '若本次經駁回，將於 8 日內不得再提出解除請求',
      );
    });
  });
}
