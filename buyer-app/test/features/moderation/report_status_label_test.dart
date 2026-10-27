import 'package:hawkr_buyer/features/moderation/data/report_target_type.dart';
import 'package:flutter_test/flutter_test.dart';

/// 檢舉狀態文案契約。
///
/// 守的是一個沒有任何 gate 攔得住的靜默失效：手抄的狀態對照表漂離後端值域
/// 之後，畫面不會崩、不會報錯、測試也不會紅——只是使用者看到一行
/// `RESOLVED_HIDDEN`。值域現在來自碼表，本檔負責證明每個值都真的翻得出字。
void main() {
  group('reportStatusLabel', () {
    test('碼表六個值全部翻得出中文，沒有一個掉進 fallback', () {
      for (final status in ReportStatus.values) {
        final label = reportStatusLabel(status.value);
        expect(
          label,
          isNot(status.value),
          reason: '${status.value} 掉進 fallback，使用者會看到裸英文',
        );
        expect(
          RegExp(r'[A-Z]{2,}').hasMatch(label),
          isFalse,
          reason: '${status.value} 的文案 "$label" 看起來仍是原始碼值',
        );
      }
    });

    test('文案維持既有用字（碼表 label 是治理後台視角，不直接拿來顯示）', () {
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

  test('「（請說明）」是表單提示，只掛在 OTHER 上且不進碼表', () {
    expect(reportReasonOptionLabel(ReportReasonCode.other), '其他（請說明）');
    expect(ReportReasonCode.other.label, '其他');
    for (final r in ReportReasonCode.values) {
      if (r == ReportReasonCode.other) continue;
      expect(reportReasonOptionLabel(r), r.label);
    }
  });
}
