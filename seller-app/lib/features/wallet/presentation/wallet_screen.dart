import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart' show commonUnknown;
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';

import '../../../config/flavor.dart';
import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/router/pending_wallet_focus.dart';
import '../../../core/session/my_store.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/use_on_became_visible.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/green_header_bar.dart';
import '../../../shared/widgets/logo_home_button.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../seller/data/current_kyc.dart';
import '../../seller/data/current_store.dart';
import '../../seller/data/kyc_flow.dart';
import '../../seller/data/payuni.dart';
import '../../seller/data/seller_status_refresh.dart';
import '../../seller/presentation/widgets/payuni_status_body.dart';
import 'wallet_account_section.dart';
import 'wallet_coupon_section.dart';

/// 錢包頁：依 `GetMyKYC` 的流程狀態與 `flow.payuni` 顯示認證閘門或 PayUni 狀態面板（和 website 的 `WalletClient`
/// 同一個判斷），核准後加上帳戶與認證、前往提領；優惠券由 `MARKETING` 能力決定，與認證狀態無關。
///
/// 錢包常駐在主導航的 IndexedStack 裡；它是否看得到由 widget 樹決定：被隱藏的分頁，或被不透明頁面（認證流程）蓋住時，
/// `TickerMode` 是關閉的（flutter_riverpod 在 `TickerMode` 關閉時暫停這個畫面的監看）。
/// 第一次開啟與從看不到變成看得到時重讀（30 秒內讀過就不讀），回前景只在看得到時重讀。重讀失敗時繼續顯示上一次成功的值
/// （同 website 的 `useCertGate`），只有完全沒有值才顯示錯誤面板。
/// 第一次顯示等 `GetMyKYC`（含 PayUni 狀態）與第一次 `GetMyStore`（店家決定帳戶與認證、前往提領與優惠券）。
///
/// 要展開加工食品列時，進度卡先設定 [pendingWalletFocusProvider] 再導覽到錢包；錢包套用後清掉，每次導覽只套用一次。
class WalletScreen extends HookConsumerWidget {
  const WalletScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final kyc = ref.watch(currentKycProvider);
    final store = ref.watch(currentStoreProvider);
    final my = ref.watch(myStoreSnapshotProvider);
    final pendingFocus = ref.watch(pendingWalletFocusProvider);

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
        child: _body(context, ref, kyc, store, my, pendingFocus),
      ),
    );
  }

  Widget _body(
    BuildContext context,
    WidgetRef ref,
    AsyncValue<MyKyc>? kyc,
    AsyncValue<Object?> store,
    MyStore? my,
    String? pendingFocus,
  ) {
    if (kyc == null || _isLoading(kyc) || _isLoading(store)) {
      return const TruckLoadingOverlay();
    }
    final marketing =
        my?.can(SellerCapability.SELLER_CAPABILITY_MARKETING) ?? false;
    if (_isFailed(kyc) || ref.watch(storeUnavailableProvider)) {
      _dropPendingFocus(ref, pendingFocus);
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

    final current = kyc.requireValue;
    final payuni = current.payuni;
    if (payuni == null) {
      return _WalletPage(
        showCoupons: marketing,
        children: [
          _CertificationGate(status: current.status),
          _foodSection(ref, pendingFocus, certified: false, kycApproved: false),
        ],
      );
    }

    final certified = my?.isCertified ?? false;
    return _WalletPage(
      showCoupons: marketing,
      children: [
        if (payuniHasState(payuni))
          _PayuniStatusPanel(payuni: payuni, canResubmit: current.canResubmit),
        if (walletCanWithdraw(my, current)) const _WithdrawButton(),
        _foodSection(
          ref,
          pendingFocus,
          certified: certified,
          kycApproved: kycInPayuniStage(current.status),
        ),
      ],
    );
  }

  Widget _foodSection(
    WidgetRef ref,
    String? pendingFocus, {
    required bool certified,
    required bool kycApproved,
  }) => WalletAccountSection(
    showCertification: certified,
    kycApproved: kycApproved,
    focusFoodRegistration: pendingFocus == WalletSections.foodRegistration,
    onFocusHandled: () => ref.read(pendingWalletFocusProvider.notifier).clear(),
  );
}

Future<void> _openPayuniLogin(BuildContext context) async {
  var launched = false;
  try {
    launched = await launchUrl(
      payuniLoginUri(F.appFlavor),
      mode: LaunchMode.inAppBrowserView,
    );
  } catch (_) {
    launched = false;
  }
  if (!context.mounted || launched) return;
  FloatingToast.error(context, '無法開啟 PayUni，請稍後再試');
}

/// 帳戶與認證區塊沒有顯示（錯誤、閘門、尚未認證）時，等待中的展開意圖在第一次穩定的畫面就丟掉：它只屬於那一次導覽，
/// 不會等到之後某次剛好顯示該區塊時才套用。
void _dropPendingFocus(WidgetRef ref, String? pendingFocus) {
  if (pendingFocus == null) return;
  WidgetsBinding.instance.addPostFrameCallback((_) {
    ref.read(pendingWalletFocusProvider.notifier).clear();
  });
}

/// 進認證流程；回來後伺服器可能剛改變了狀態，fresh 重讀。
Future<void> _openKycWizard(BuildContext context, WidgetRef ref) async {
  await context.push<bool>(SellerRoutes.certificationUpload);
  if (context.mounted) _reload(ref, fresh: true);
}

/// 重讀店家與認證流程（含 PayUni 狀態），統一由 [SellerStatusRefresh] 負責。[settledOnly]：只重讀已經有結果的項目
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

class _WithdrawButton extends StatelessWidget {
  const _WithdrawButton();

  @override
  Widget build(BuildContext context) {
    return FilledButton(
      onPressed: () => unawaited(_openPayuniLogin(context)),
      child: const HkButtonLabel('前往提領'),
    );
  }
}

/// 認證前的閘門：文案和 website 的 `SellerCertGate` 同一張表（`kycFlowCopy`）。按鈕進認證流程，流程自己選個人或企業；
/// 回來後重讀狀態。
class _CertificationGate extends ConsumerWidget {
  const _CertificationGate({required this.status});

  final KYCFlowStatus status;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final copy = kycFlowCopy(status, context.uiLocale);
    final action = copy.action;
    return HkContentCard(
      elevated: true,
      padding: const EdgeInsets.all(HkSpace.lg),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            copy.gateHeading ?? commonUnknown(context.uiLocale),
            style: AppTypography.bodyBold,
          ),
          const SizedBox(height: HkSpace.sm),
          Text(
            copy.gateMessage('啟用錢包與收款') ?? commonUnknown(context.uiLocale),
            style: AppTypography.body.copyWith(color: AppColors.textSecondary),
          ),
          if (action != null) ...[
            const SizedBox(height: HkSpace.lg),
            FilledButton(
              onPressed: () => _openKycWizard(context, ref),
              child: HkButtonLabel(action),
            ),
          ],
        ],
      ),
    );
  }
}

/// PayUni 的狀態面板（唯讀）：標題是目錄的 [PayUniState] 名詞，內文是伺服器的說明、審核意見與欄位清單（[PayuniStatusBody]）；
/// 伺服器說賣家能自己重送（[canResubmit]）時給補件入口，修正指出店家欄位時給店家設定入口。
class _PayuniStatusPanel extends ConsumerWidget {
  const _PayuniStatusPanel({required this.payuni, required this.canResubmit});

  final SellerPayUni payuni;
  final bool canResubmit;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final alert = payuniStateAlert(payuni.state);
    return HkContentCard(
      key: const ValueKey('wallet-payuni-panel'),
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
          Text(
            payuniStateTitle(payuni.state, context.uiLocale),
            style: AppTypography.bodyBold,
          ),
          const SizedBox(height: HkSpace.xs),
          PayuniStatusBody(payuni: payuni),
          if (payuniFixPointsAtStore(payuni)) ...[
            const SizedBox(height: HkSpace.md),
            OutlinedButton(
              onPressed: () => context.go(MainRoutes.settings),
              child: const HkButtonLabel('前往店家設定'),
            ),
          ],
          if (canResubmit) ...[
            const SizedBox(height: HkSpace.md),
            OutlinedButton(
              onPressed: () => _openKycWizard(context, ref),
              child: const HkButtonLabel('前往補件'),
            ),
          ],
        ],
      ),
    );
  }
}
