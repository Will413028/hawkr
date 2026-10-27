import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart' show UiLocale, commonUnknown;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/session/my_store.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/app_confirm_dialog.dart';
import '../data/current_kyc.dart';
import '../data/current_store.dart';
import '../data/kyc_flow.dart';
import '../data/payuni.dart';
import '../data/version_conflict.dart' show storeStateUnavailableNotice;
import 'suspension_notice_screen.dart' show SuspensionNoticeContent;

typedef StoreLoader = Future<MyStore?> Function();
typedef KycLoader = Future<MyKyc?> Function();

const _shippingSubject = '設定物流與退貨';

/// 目前帳號重讀後的店家與認證流程，交給 [openShippingSettingsWithCertificationGate]。
Future<void> openShippingSettings(BuildContext context, WidgetRef ref) =>
    openShippingSettingsWithCertificationGate(
      context,
      loadStore: () async {
        await ref.read(currentStoreProvider.notifier).refresh();
        return ref.read(myStoreSnapshotProvider);
      },
      loadKyc: () => ref.read(kycReaderProvider.notifier).reload(),
      locale: ref.read(uiLocaleProvider),
    );

/// 配送設定的共用入口 gate：由店家的 [MyStore.shippingGate]（伺服器的 `SHIPPING_SETUP` 能力）決定，首頁與設定頁都從這裡進入。
///
/// 認證流程（`GetMyKYC`）只用來說明為什麼還不能設定；讀不到不影響閘門，文字退回通用的「尚未通過商家認證」。
Future<void> openShippingSettingsWithCertificationGate(
  BuildContext context, {
  required StoreLoader loadStore,
  required KycLoader loadKyc,
  required UiLocale locale,
}) async {
  MyStore? store;
  try {
    store = await loadStore();
  } catch (_) {
    store = null;
  }
  if (!context.mounted) return;

  final gate = store?.shippingGate ?? ShippingGate.unknown;
  if (gate == ShippingGate.open) {
    await context.push(SettingsRoutes.shipping);
    return;
  }

  if (gate == ShippingGate.unknown) {
    FloatingToast.error(context, storeStateUnavailableNotice);
    return;
  }

  if (gate == ShippingGate.suspended || gate == ShippingGate.pendingDeletion) {
    final content = SuspensionNoticeContent(
      store,
      locale,
      pendingOnly: gate == ShippingGate.pendingDeletion,
    );
    final shouldOpenNotice = await showAppConfirmDialog(
      context,
      title: content.title,
      message: content.reasonBody,
      cancelText: '稍後',
      confirmText: '查看說明',
      confirmTextColor: AppColors.primary,
      barrierColor: AppColors.bgOverlaySoft,
    );
    if (!shouldOpenNotice || !context.mounted) return;
    await context.push(SellerRoutes.suspensionNotice);
    return;
  }

  MyKyc? kyc;
  try {
    kyc = await loadKyc();
  } catch (_) {
    kyc = null;
  }
  if (!context.mounted) return;

  final copy = shippingCertificationGateCopy(kyc, locale);
  final shouldOpenCertification = await showAppConfirmDialog(
    context,
    title: copy.title,
    message: copy.message,
    cancelText: '稍後',
    confirmText: copy.confirmText,
    confirmTextColor: AppColors.primary,
    barrierColor: AppColors.bgOverlaySoft,
  );
  if (!shouldOpenCertification || !context.mounted) return;

  context.go(MainRoutes.wallet);
}

/// 認證閘門對話框的文字：有 PayUni 狀態（`flow.payuni`）時是 PayUni 的標題與伺服器的說明；否則依流程狀態照 website 的認證閘門。
/// 流程狀態讀不到（[kyc] 是 null）時用「尚未開始」的文字。
@visibleForTesting
ShippingCertificationGateCopy shippingCertificationGateCopy(
  MyKyc? kyc,
  UiLocale l,
) {
  final payuni = kyc?.payuni;
  if (payuniHasState(payuni)) {
    final explanation = payuni!.hasExplanation()
        ? payuni.explanation.message
        : '';
    return ShippingCertificationGateCopy(
      title: payuniStateTitle(payuni.state, l),
      message: explanation.isEmpty ? commonUnknown(l) : explanation,
      confirmText: kycFlowCopy(kyc!.status, l).action ?? '開始認證',
    );
  }
  final copy = kycFlowCopy(
    kyc?.status ?? KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
    l,
  );
  return ShippingCertificationGateCopy(
    title: copy.gateHeading ?? commonUnknown(l),
    message: copy.gateMessage(_shippingSubject) ?? commonUnknown(l),
    confirmText: copy.action ?? '開始認證',
  );
}

@visibleForTesting
class ShippingCertificationGateCopy {
  const ShippingCertificationGateCopy({
    required this.title,
    required this.message,
    required this.confirmText,
  });

  final String title;
  final String message;
  final String confirmText;
}
