import 'dart:io';

import 'package:hawkr_seller/features/report/data/report_model.dart';
import 'package:flutter_test/flutter_test.dart';

/// 檢舉狀態文案契約。
///
/// 這裡守的是一個**沒有任何 gate 攔得住**的靜默失效：手抄的狀態對照表漂離
/// 後端值域之後，畫面不會崩、不會報錯、測試也不會紅——只是賣家看到一行
/// `RESOLVED_HIDDEN`。本檔曾經真的長這樣：對照表寫的是 `REVIEWING` /
/// `RESOLVED` / `ACTION_TAKEN` / `REJECTED`（DB CHECK `chk_reports_status`
/// 從不接受的四個值），而後端真正會送的 `RESOLVED_HIDDEN` /
/// `RESOLVED_WARNED` / `RESOLVED_BANNED` / `APPEALED` 全數掉進 fallback。
void main() {
  group('reportStatusLabel', () {
    test('碼表六個值全部翻得出中文，沒有一個掉進 fallback', () {
      for (final status in ReportStatus.values) {
        final label = reportStatusLabel(status.value);
        expect(
          label,
          isNot(status.value),
          reason: '${status.value} 掉進 fallback，賣家會看到裸英文',
        );
        expect(
          RegExp(r'[A-Z]{2,}').hasMatch(label),
          isFalse,
          reason: '${status.value} 的文案 "$label" 看起來仍是原始碼值',
        );
      }
    });

    test('狀態文案與買家 App 我的檢舉一致', () {
      expect(reportStatusLabel('PENDING'), '審核中');
      expect(reportStatusLabel('APPEALED'), '申訴中');
      expect(reportStatusLabel('RESOLVED_HIDDEN'), '已處理（已隱藏）');
      expect(reportStatusLabel('RESOLVED_WARNED'), '已處理（已警告）');
      expect(reportStatusLabel('RESOLVED_BANNED'), '已處理（已停權）');
      expect(reportStatusLabel('DISMISSED'), '已駁回');
    });

    test('空字串當審核中，未知值原樣回傳（畫面不留白）', () {
      expect(reportStatusLabel(''), '審核中');
      expect(reportStatusLabel('SOME_FUTURE_STATUS'), 'SOME_FUTURE_STATUS');
    });
  });

  test('已退役的四個狀態值不得再出現在 lib/', () {
    // 這四個從未存在於 `chk_reports_status`，也沒有任何 commit 讓 social-service
    // 寫出過它們；留著只會讓下一個人以為後端真的會送。
    const retired = ['REVIEWING', 'ACTION_TAKEN'];
    final offenders = <String>[];

    for (final entity in Directory('lib').listSync(recursive: true)) {
      if (entity is! File || !entity.path.endsWith('.dart')) continue;
      final lines = entity.readAsLinesSync();
      for (var i = 0; i < lines.length; i++) {
        for (final value in retired) {
          if (RegExp('(?<![A-Z_])$value').hasMatch(lines[i])) {
            offenders.add('${entity.path}:${i + 1} → ${lines[i].trim()}');
          }
        }
      }
    }

    expect(offenders, isEmpty, reason: '後端從不送這些值，對照它們等於什麼都沒對照');
  });

  group('reportReasonsFor', () {
    test('USER 不含 FOOD_SAFETY', () {
      final reasons = reportReasonsFor(ReportTargetType.user);
      expect(reasons.contains(ReportReasonCode.foodSafety), isFalse);
      expect(reasons.length, ReportReasonCode.values.length - 1);
    });

    test('其餘對象含全部原因', () {
      for (final t in ReportTargetType.values) {
        if (t == ReportTargetType.user) continue;
        expect(reportReasonsFor(t).length, ReportReasonCode.values.length);
      }
    });
  });

  test('reportReasonLabel / reportTargetTypeLabel 以碼表為唯一來源', () {
    for (final r in ReportReasonCode.values) {
      expect(reportReasonLabel(r.code), r.label);
    }
    for (final t in ReportTargetType.values) {
      expect(reportTargetTypeLabel(t.value), t.label);
    }
    expect(reportReasonLabel('SOMETHING_NEW'), 'SOMETHING_NEW');
    expect(reportTargetTypeLabel('SOMETHING_NEW'), 'SOMETHING_NEW');
  });
}
