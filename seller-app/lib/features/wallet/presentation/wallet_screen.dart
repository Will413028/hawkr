import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/session/my_store.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/utils/use_on_became_visible.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../../../shared/widgets/green_header_bar.dart';
import '../../../shared/widgets/logo_home_button.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../seller/data/current_kyc.dart';
import '../../seller/data/current_store.dart';
import '../../seller/data/payment_account.dart';
import '../../seller/data/seller_status_refresh.dart';
import '../../seller/presentation/payment_account_action.dart';
import '../../seller/presentation/seller_certification_gate.dart'
    show
        certificationGateConfirmText,
        certificationGateMessage,
        certificationGateTitle;
import 'wallet_account_section.dart';
import 'wallet_coupon_section.dart';

/// 錢包頁：依收款帳戶（`GetMyPaymentAccount`）顯示認證閘門或收款帳戶狀態面板（和 website 的 `WalletClient` 同一個判斷）：
/// 平台審核核准之前沒有收款帳戶，顯示認證閘門；收款帳戶還沒啟用顯示狀態面板；店家通過開通閘門後加上帳戶與認證。
/// 優惠券由 `MARKETING` 能力決定，與認證狀態無關。
///
/// 錢包常駐在主導航的 IndexedStack 裡；它是否看得到由 widget 樹決定：被隱藏的分頁，或被不透明頁面（認證流程）蓋住時，
/// `TickerMode` 是關閉的（flutter_riverpod 在 `TickerMode` 關閉時暫停這個畫面的監看）。
/// 第一次開啟與從看不到變成看得到時重讀（30 秒內讀過就不讀），回前景只在看得到時重讀。重讀失敗時繼續顯示上一次成功的值
/// （同 website 的 `useCertGate`），只有完全沒有值才顯示錯誤面板。
/// 第一次顯示等 `GetMyKYC`、`GetMyPaymentAccount` 與第一次 `GetMyStore`（店家決定帳戶與認證和優惠券）。
class WalletScreen extends HookConsumerWidget {
  const WalletScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final kyc = ref.watch(currentKycProvider);
    final payment = ref.watch(currentPaymentAccountProvider);
    final store = ref.watch(currentStoreProvider);
    final my = ref.watch(myStoreSnapshotProvider);

    final visible = useOnBecameVisible(
      () => _reload(ref, settledOnly: true, maxAge: statusStaleTime),
      includeInitial: true,
    );
    final visibleNow = useRef(visible)..value = visible;
    useOnAppLifecycleStateChange((previous, current) {
      if (current == AppLifecycleState.resumed && visibleNow.value) {
        _reload(ref);
      }
    });

    const statusBarStyle = SystemUiOverlayStyle.dark;
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: AnnotatedRegion<SystemUiOverlayStyle>(
        value: statusBarStyle,
        child: _body(context, ref, kyc, payment, store, my),
      ),
    );
  }

  Widget _body(
    BuildContext context,
    WidgetRef ref,
    AsyncValue<MyKyc>? kyc,
    AsyncValue<PaymentAccount?>? payment,
    AsyncValue<Object?> store,
    MyStore? my,
  ) {
    if (kyc == null ||
        payment == null ||
        _isLoading(kyc) ||
        _isLoading(payment) ||
        _isLoading(store)) {
      return const TruckLoadingOverlay();
    }
    final marketing =
        my?.can(SellerCapability.SELLER_CAPABILITY_MARKETING) ?? false;
    // 收款帳戶的讀取失敗只在平台審核通過之後才有意義（之前沒有收款帳戶，畫面是認證閘門）。
    final reviewStatus = kyc.value?.reviewStatus;
    final approved = platformApproved(reviewStatus);
    if (_isFailed(kyc) ||
        (approved && _isFailed(payment)) ||
        ref.watch(storeUnavailableProvider)) {
      return _WalletPage(
        showCoupons: marketing,
        children: [
          HkAsyncStatePanel(
            kind: HkAsyncStateKind.error,
            title: '無法載入認證狀態',
            description: '請檢查網路後再試一次。',
            actionLabel: '重試',
            onAction: () => _reload(ref),
          ),
        ],
      );
    }

    final account = effectivePaymentAccount(payment.value, approved: approved);
    final certified = my?.isCertified ?? false;
    return _WalletPage(
      showCoupons: marketing,
      children: [
        if (account == null)
          _CertificationGate(reviewStatus: reviewStatus)
        else if (!paymentAccountEnabled(account))
          _PaymentAccountPanel(account: account),
        if (certified) const WalletAccountSection(),
      ],
    );
  }
}

/// 進認證流程；回來後伺服器可能剛改變了狀態，fresh 重讀。
Future<void> _openKycWizard(BuildContext context, WidgetRef ref) async {
  await context.push<bool>(SellerRoutes.certificationUpload);
  if (context.mounted) _reload(ref, fresh: true);
}

/// 重讀店家、認證流程與收款帳戶，統一由 [SellerStatusRefresh] 負責。[settledOnly]：只重讀已經有結果的項目
/// （已經在讀的項目不重複讀）。[fresh]：伺服器剛改變了狀態。[maxAge]：這段時間內讀過就不讀。
void _reload(
  WidgetRef ref, {
  bool fresh = false,
  bool settledOnly = false,
  Duration? maxAge,
}) {
  unawaited(
    ref
        .read(sellerStatusRefreshProvider.notifier)
        .refresh(fresh: fresh, settledOnly: settledOnly, maxAge: maxAge),
  );
}

/// 讀取中：第一次讀取，或失敗後重試。重讀（已有值）不算。
bool _isLoading(AsyncValue<Object?> value) =>
    value.isLoading && !value.hasValue;

/// 讀取失敗而且沒有任何值：重讀失敗但還留著上一次成功的值時不算（繼續顯示那個值），也不當成未註冊。
bool _isFailed(AsyncValue<Object?> value) =>
    !value.isLoading && value.hasError && !value.hasValue;

class _WalletPage extends StatelessWidget {
  const _WalletPage({required this.children, required this.showCoupons});

  final List<Widget> children;
  final bool showCoupons;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: Column(
        children: [
          const GreenHeaderBar(
            includeTopSafeArea: true,
            title: '營運錢包',
            showBackButton: false,
            leading: LogoHomeButton(),
          ),
          Expanded(
            child: LayoutBuilder(
              builder: (context, constraints) {
                final gutter = HkBreakpoints.pageGutter(constraints.maxWidth);
                return SingleChildScrollView(
                  padding: EdgeInsets.fromLTRB(gutter, HkSpace.lg, gutter, 120),
                  child: Center(
                    child: ConstrainedBox(
                      constraints: const BoxConstraints(
                        maxWidth: HkControl.chromeMaxWidth,
                      ),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.stretch,
                        children: [
                          for (final child in children) ...[
                            child,
                            const SizedBox(height: HkSpace.xl),
                          ],
                          if (showCoupons) const WalletCouponSection(),
                        ],
                      ),
                    ),
                  ),
                );
              },
            ),
          ),
        ],
      ),
    );
  }
}

/// 認證前的閘門（平台審核還沒核准，沒有收款帳戶）：文字與 website 的 `SellerCertGate` 相同，目前狀態是平台審核的目錄名詞
/// （`seller.KYCStatus`），沒有申請是「未送出」。按鈕進認證流程，流程自己選個人或企業；回來後重讀狀態。
class _CertificationGate extends ConsumerWidget {
  const _CertificationGate({required this.reviewStatus});

  final KYCStatus? reviewStatus;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = context.uiLocale;
    final status = reviewStatus == null
        ? '未送出'
        : reviewStatus!.uiText(l) ?? commonUnknown(l);
    return HkContentCard(
      elevated: true,
      padding: const EdgeInsets.all(HkSpace.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(certificationGateTitle, style: AppTypography.bodyBold),
          const SizedBox(height: HkSpace.sm),
          Text(
            certificationGateMessage('啟用錢包與收款'),
            style: AppTypography.body.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: HkSpace.xs),
          Text(
            '目前狀態：$status',
            key: const Key('wallet-gate-status'),
            style: AppTypography.body.copyWith(color: AppColors.textSecondary),
          ),
          const SizedBox(height: HkSpace.lg),
          FilledButton(
            onPressed: () => _openKycWizard(context, ref),
            child: const HkButtonLabel(certificationGateConfirmText),
          ),
        ],
      ),
    );
  }
}

/// 收款帳戶的狀態面板（唯讀）：標題、說明與動作文字都是共用文案目錄（`seller.PaymentAccountStatus`）的文字；不顯示金流商的
/// 待補條件細項（金流商自己的頁面會列出要補什麼）。動作與認證狀態畫面共用 [paymentAccountActionKind]：無法啟用去聯絡客服，
/// 需要開通開始開通，其他重新讀取。
class _PaymentAccountPanel extends HookConsumerWidget {
  const _PaymentAccountPanel({required this.account});

  final PaymentAccount account;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    // hook 要在任何提早 return 之前呼叫，狀態在認得與不認得之間變動時進行中的動作才不會被重置。
    final actions = usePaymentAccountActions(context, ref);
    final l = context.uiLocale;
    final tone = paymentAccountTone(account.status);
    final title = account.status.uiText(l);
    if (tone == null || title == null) {
      return HkAsyncStatePanel(
        kind: HkAsyncStateKind.error,
        title: '無法載入認證狀態',
        description: '請檢查網路後再試一次。',
        actionLabel: '重試',
        onAction: () => _reload(ref),
      );
    }
    final alert = tone == HkStatusTone.warning || tone == HkStatusTone.danger;
    final description = account.status.description(l);
    final action = account.status.action(l);
    final kind = action == null
        ? PaymentAccountActionKind.none
        : paymentAccountActionKind(account);
    final embedded =
        paymentAccountCanOnboard(account) &&
        account.onboardingSurface ==
            OnboardingSurface.ONBOARDING_SURFACE_EMBEDDED;
    return HkContentCard(
      key: const ValueKey('wallet-payment-panel'),
      elevated: true,
      backgroundColor: alert ? AppColors.warningSoft : AppColors.surfaceCard,
      padding: const EdgeInsets.all(HkSpace.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          HkIcon(
            alert ? HkIconName.warning : HkIconName.clock,
            size: 28,
            color: alert ? AppIconColors.warning : AppIconColors.secondary,
          ),
          const SizedBox(height: HkSpace.sm),
          Text(title, style: AppTypography.bodyBold),
          if (description != null) ...[
            const SizedBox(height: HkSpace.xs),
            Text(
              description,
              style: AppTypography.body.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
          ],
          if (embedded) ...[
            const SizedBox(height: HkSpace.xs),
            Text(
              paymentAccountEmbeddedNotice,
              style: AppTypography.body.copyWith(
                color: AppColors.textSecondary,
              ),
            ),
          ],
          if (action != null && kind != PaymentAccountActionKind.none) ...[
            const SizedBox(height: HkSpace.md),
            OutlinedButton(
              onPressed: actions.busy ? null : () => actions.run(account, kind),
              child: HkButtonLabel(action),
            ),
          ],
        ],
      ),
    );
  }
}
