import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_seller/features/seller/data/kyc_flow.dart';
import 'package:hawkr_seller/shared/widgets/hk_status_badge.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter_test/flutter_test.dart';

const _expected = <KYCFlowStatus, ({String label, String? action})>{
  KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED: (label: '待認證', action: '開始認證'),
  KYCFlowStatus.KYC_FLOW_STATUS_DRAFT: (label: '填寫中', action: '開始認證'),
  KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED: (label: '平台審核中', action: '查看進度'),
  KYCFlowStatus.KYC_FLOW_STATUS_REJECTED: (label: '未通過', action: '前往補件'),
  KYCFlowStatus.KYC_FLOW_STATUS_BINDING: (label: 'PayUni 審核中', action: '查看進度'),
  KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED: (
    label: 'PayUni 建檔失敗，處理中',
    action: '查看進度',
  ),
  KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED: (
    label: 'PayUni 審核中',
    action: '查看進度',
  ),
  KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW: (
    label: 'PayUni 審核中',
    action: '查看進度',
  ),
  KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO: (label: '需補件', action: '前往補件'),
  KYCFlowStatus.KYC_FLOW_STATUS_APPROVED: (label: '已通過', action: null),
  KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED: (
    label: 'PayUni 審核未通過',
    action: '前往補件',
  ),
};

/// 伺服器在這六個值一定設 `flow.payuni`，目錄沒有橫幅與閘門文字（omit）。
const _payuniPhase = {
  KYCFlowStatus.KYC_FLOW_STATUS_BINDING,
  KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED,
  KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED,
  KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW,
  KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO,
  KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED,
};

void main() {
  test('每個有效的流程狀態都有 website 同一份的標籤與動作', () {
    final valid = KYCFlowStatus.values.where(
      (s) => s != KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED,
    );
    expect(valid.toSet(), _expected.keys.toSet());
    expect(validKycFlowStatuses.toSet(), _expected.keys.toSet());
    for (final entry in _expected.entries) {
      final copy = kycFlowCopy(entry.key, UiLocale.zhTW);
      expect(copy.label, entry.value.label, reason: '${entry.key}');
      expect(copy.action, entry.value.action, reason: '${entry.key}');
    }
  });

  test('UNSPECIFIED 不是有效的流程狀態；文字是「未知」，行動、橫幅與閘門和尚未開始相同', () {
    expect(
      isValidKycFlowStatus(KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED),
      isFalse,
    );
    final unknown = kycFlowCopy(
      KYCFlowStatus.KYC_FLOW_STATUS_UNSPECIFIED,
      UiLocale.zhTW,
    );
    final notStarted = kycFlowCopy(
      KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
      UiLocale.zhTW,
    );
    expect(unknown.label, '未知');
    expect(unknown.action, notStarted.action);
    expect(unknown.tone, notStarted.tone);
    expect(unknown.bannerTitle, notStarted.bannerTitle);
    expect(unknown.gateHeading, notStarted.gateHeading);
    expect(unknown.gateMessage('x'), notStarted.gateMessage('x'));
  });

  test('語氣：已通過成功、被退回危險、補件警告、審核中資訊', () {
    HkStatusTone toneOf(KYCFlowStatus s) => kycFlowCopy(s, UiLocale.zhTW).tone;
    expect(
      toneOf(KYCFlowStatus.KYC_FLOW_STATUS_APPROVED),
      HkStatusTone.success,
    );
    expect(toneOf(KYCFlowStatus.KYC_FLOW_STATUS_REJECTED), HkStatusTone.danger);
    expect(
      toneOf(KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED),
      HkStatusTone.danger,
    );
    expect(
      toneOf(KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO),
      HkStatusTone.warning,
    );
    expect(
      toneOf(KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW),
      HkStatusTone.info,
    );
    expect(
      toneOf(KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED),
      HkStatusTone.neutral,
    );
  });

  test('閘門文字照 website 的認證閘門', () {
    const subject = '設定物流與退貨';
    final notStarted = kycFlowCopy(
      KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
      UiLocale.zhTW,
    );
    expect(notStarted.gateHeading, '尚未通過商家認證');
    expect(notStarted.gateMessage(subject), '請先完成商家認證，才能設定物流與退貨。');

    final submitted = kycFlowCopy(
      KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
      UiLocale.zhTW,
    );
    expect(submitted.gateHeading, '商家認證審核中');
    expect(
      submitted.gateMessage(subject),
      '你的商家認證平台審核中，通過後將自動送出至 PayUni 進行金流審核，完成後就能設定物流與退貨。',
    );

    for (final status in _payuniPhase) {
      final copy = kycFlowCopy(status, UiLocale.zhTW);
      expect(copy.gateHeading, isNull, reason: '$status');
      expect(copy.gateMessage(subject), isNull, reason: '$status');
    }

    final rejected = kycFlowCopy(
      KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
      UiLocale.zhTW,
    );
    expect(rejected.gateHeading, '商家認證需要補件');
    expect(rejected.gateMessage(subject), '認證未通過，請修改資料後重新送審。');
  });

  test('每個有效的流程狀態都有橫幅標題與說明（和 website 的 kyc-flow.ts 同一份）', () {
    for (final status in validKycFlowStatuses) {
      final copy = kycFlowCopy(status, UiLocale.zhTW);
      if (_payuniPhase.contains(status)) {
        expect(copy.bannerTitle, isNull, reason: '$status');
        expect(copy.bannerDescription, isNull, reason: '$status');
        continue;
      }
      expect(copy.bannerTitle, isNotEmpty, reason: '$status');
      expect(
        copy.bannerTitle,
        isNot(commonUnknown(UiLocale.zhTW)),
        reason: '$status',
      );
      expect(copy.bannerDescription, isNotEmpty, reason: '$status');
    }
    expect(
      kycFlowCopy(
        KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
        UiLocale.zhTW,
      ).bannerTitle,
      '認證未通過',
    );
    expect(
      kycFlowCopy(
        KYCFlowStatus.KYC_FLOW_STATUS_REJECTED,
        UiLocale.zhTW,
      ).bannerDescription,
      '請查看退件原因並編輯後重新送出。',
    );
    expect(
      kycFlowCopy(
        KYCFlowStatus.KYC_FLOW_STATUS_APPROVED,
        UiLocale.zhTW,
      ).bannerTitle,
      '商家認證已通過',
    );
    expect(
      kycFlowCopy(
        KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED,
        UiLocale.zhTW,
      ).bannerTitle,
      '平台審核中',
    );
  });
}
