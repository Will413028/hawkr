import 'package:hawkr_ui_text/hawkr_ui_text.dart' show UiLocale;
import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/session/my_store.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/app_confirm_dialog.dart';
import '../data/current_store.dart';
import '../data/version_conflict.dart' show storeStateUnavailableNotice;
import 'suspension_notice_screen.dart' show SuspensionNoticeContent;

typedef StoreLoader = Future<MyStore?> Function();

const _shippingSubject = '設定物流與退貨';

/// 認證閘門（物流對話框與錢包卡）共用的文字；錢包與物流只差「通過之後能做什麼」。
const certificationGateTitle = '尚未通過商家認證';
const certificationGateConfirmText = '前往商家認證';
String certificationGateMessage(String subject) => '通過平台審核並啟用收款帳戶後，就能$subject。';

/// 目前帳號重讀後的店家，交給 [openShippingSettingsWithCertificationGate]。
Future<void> openShippingSettings(BuildContext context, WidgetRef ref) =>
    openShippingSettingsWithCertificationGate(
      context,
      loadStore: () async {
        await ref.read(currentStoreProvider.notifier).refresh();
        return ref.read(myStoreSnapshotProvider);
      },
      locale: ref.read(uiLocaleProvider),
    );

/// 配送設定的共用入口 gate：由店家的 [MyStore.shippingGate]（伺服器的 `SHIPPING_SETUP` 能力）決定，首頁與設定頁都從這裡進入。
/// 缺認證時的說明是固定文字（與 website 的認證閘門相同），細分的平台審核與收款帳戶狀態在錢包頁。
Future<void> openShippingSettingsWithCertificationGate(
  BuildContext context, {
  required StoreLoader loadStore,
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

  final copy = shippingCertificationGateCopy();
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

/// 認證閘門對話框的文字：website 的認證閘門同一句（通過平台審核並啟用收款帳戶後才能設定物流）。
@visibleForTesting
ShippingCertificationGateCopy shippingCertificationGateCopy() =>
    ShippingCertificationGateCopy(
      title: certificationGateTitle,
      message: certificationGateMessage(_shippingSubject),
      confirmText: certificationGateConfirmText,
    );

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
