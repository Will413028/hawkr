import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../../../shared/widgets/hk_status_badge.dart';

/// 認證流程狀態的顯示文字，和 website 的 `lib/seller/kyc-flow.ts` 同取自共用文案目錄：以 [KYCFlowStatus] 為鍵。
final class KycFlowCopy {
  const KycFlowCopy({
    required this.label,
    required this.action,
    required this.tone,
    required this.bannerTitle,
    required this.bannerDescription,
    required this.gateHeading,
    required this.gateMessage,
  });

  /// 開店進度卡與認證入口的狀態文字。
  final String label;

  /// 引導賣家做哪一步；沒有下一步（已通過）時是 null。
  final String? action;

  final HkStatusTone tone;

  /// 認證頁與錢包入口的狀態橫幅標題；PayUni 階段沒有（那時畫面用 `flow.payuni`），是 null。
  final String? bannerTitle;

  /// 認證頁與錢包入口的狀態橫幅說明；PayUni 階段沒有，是 null。
  final String? bannerDescription;

  /// 認證前的閘門對話框標題；PayUni 階段沒有，是 null。
  final String? gateHeading;

  /// 認證前的閘門對話框內文；[subject] 是被擋住的事。PayUni 階段沒有文字，回 null。
  final String? Function(String subject) gateMessage;
}

/// 流程狀態的語氣（版面，留在 client）；key 同時是「有效的流程狀態」的集合：`UNSPECIFIED` 與不認得的值不在裡面。
const _kycFlowTone = <KYCFlowStatus, HkStatusTone>{
  KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED: HkStatusTone.neutral,
  KYCFlowStatus.KYC_FLOW_STATUS_DRAFT: HkStatusTone.neutral,
  KYCFlowStatus.KYC_FLOW_STATUS_SUBMITTED: HkStatusTone.info,
  KYCFlowStatus.KYC_FLOW_STATUS_REJECTED: HkStatusTone.danger,
  KYCFlowStatus.KYC_FLOW_STATUS_BINDING: HkStatusTone.info,
  KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED: HkStatusTone.info,
  KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED: HkStatusTone.info,
  KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW: HkStatusTone.info,
  KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO: HkStatusTone.warning,
  KYCFlowStatus.KYC_FLOW_STATUS_APPROVED: HkStatusTone.success,
  KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED: HkStatusTone.danger,
};

/// 所有有效的流程狀態（不含 `KYC_FLOW_STATUS_UNSPECIFIED`）。
Iterable<KYCFlowStatus> get validKycFlowStatuses => _kycFlowTone.keys;

/// [status] 是不是一個有效的流程狀態：`UNSPECIFIED` 與不認得的值都不是。
bool isValidKycFlowStatus(KYCFlowStatus status) =>
    _kycFlowTone.containsKey(status);

/// [status] 的顯示文字（共用文案目錄的 `KYCFlowStatus` 名詞與變體）。`UNSPECIFIED` 與不認得的狀態：狀態文字是目錄的「未知」，
/// 行動、橫幅與閘門與「尚未開始」相同（與 website 的 `kycFlowBanner` 一致）。PayUni 階段的橫幅與閘門沒有文字（null）。
KycFlowCopy kycFlowCopy(KYCFlowStatus status, UiLocale l) {
  final shown = isValidKycFlowStatus(status)
      ? status
      : KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED;
  return KycFlowCopy(
    label: status.uiText(l) ?? commonUnknown(l),
    action: shown.action(l),
    tone: _kycFlowTone[shown]!,
    bannerTitle: shown.bannerTitle(l),
    bannerDescription: shown.bannerDescription(l),
    gateHeading: shown.gateHeading(l),
    gateMessage: (subject) => shown.gateMessage(l, subject: subject),
  );
}

/// 申請已核准、進入 PayUni 階段（綁定、審核、補件、核准、被拒）：錢包顯示 PayUni 的狀態。
bool kycInPayuniStage(KYCFlowStatus status) => switch (status) {
  KYCFlowStatus.KYC_FLOW_STATUS_BINDING ||
  KYCFlowStatus.KYC_FLOW_STATUS_BIND_FAILED ||
  KYCFlowStatus.KYC_FLOW_STATUS_FORWARDED ||
  KYCFlowStatus.KYC_FLOW_STATUS_UNDER_REVIEW ||
  KYCFlowStatus.KYC_FLOW_STATUS_PENDING_INFO ||
  KYCFlowStatus.KYC_FLOW_STATUS_APPROVED ||
  KYCFlowStatus.KYC_FLOW_STATUS_PAYUNI_REJECTED => true,
  _ => false,
};
