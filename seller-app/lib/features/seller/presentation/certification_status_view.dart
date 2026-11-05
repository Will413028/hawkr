import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/taipei_time.dart';
import 'package:hawkr_api/transport.dart' show Code;
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_frosted_card_surface.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../data/payment_account.dart';
import '../data/payment_onboarding.dart';
import 'payment_account_action.dart';
import 'widgets/certification_journey_stepper.dart';

/// 平台審核的色調（版面，留在 client）；文字取自共用文案目錄（`seller.KYCStatus`）。`UNSPECIFIED` 與不認得的值沒有色調。
HkStatusTone? kycStatusTone(KYCStatus status) => switch (status) {
  KYCStatus.KYC_STATUS_DRAFT => HkStatusTone.neutral,
  KYCStatus.KYC_STATUS_SUBMITTED => HkStatusTone.info,
  KYCStatus.KYC_STATUS_APPROVED => HkStatusTone.success,
  KYCStatus.KYC_STATUS_REJECTED => HkStatusTone.danger,
  _ => null,
};

/// 平台審核區顯示哪一個時間：已通過看核准時間，審核中看送出時間。
enum KycStampKind { decide, submit }

/// 這個狀態要顯示的時間（已格式化成台北時間）；其他狀態、或伺服器沒有給時間時是 null。只決定「哪一個時間」，不產生文字（文字取決於 [KycStampKind]，
/// 狀態名詞在共用目錄）。
({KycStampKind kind, String time})? kycStampOf(
  KYCApplication application,
  KYCStatus status,
) {
  if (status == KYCStatus.KYC_STATUS_APPROVED && application.hasDecideTime()) {
    return (
      kind: KycStampKind.decide,
      time: formatTaipeiTimestamp(application.decideTime),
    );
  }
  if (status == KYCStatus.KYC_STATUS_SUBMITTED && application.hasSubmitTime()) {
    return (
      kind: KycStampKind.submit,
      time: formatTaipeiTimestamp(application.submitTime),
    );
  }
  return null;
}

/// 已送出審核 / 已完成審核的唯讀狀態畫面（D15）：兩區，平台審核讀 `KYCApplication.status`，收款帳戶讀
/// `GetMyPaymentAccount`；不看 `KYCFlowStatus`。
///
/// 從外部瀏覽器（金流商的託管頁面）切回 App 時重讀狀態：[PaymentOnboarding.onResumed]，從瀏覽器回來帶 `fresh: true`。
class CertificationStatusView extends HookConsumerWidget {
  const CertificationStatusView({super.key, required this.application});

  final KYCApplication application;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final onboarding = ref.read(paymentOnboardingProvider.notifier);
    useOnAppLifecycleStateChange((previous, current) {
      if (current == AppLifecycleState.resumed) {
        unawaited(onboarding.onResumed());
      }
    });

    final status = application.status;
    return Center(
      child: SingleChildScrollView(
        key: const Key('kyc_locked'),
        padding: const EdgeInsets.all(24),
        child: ConstrainedBox(
          constraints: const BoxConstraints(maxWidth: 520),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const CertificationJourneyStepper(
                currentStep: CertificationJourneyStepper.lastStep,
              ),
              const SizedBox(height: HkSpace.lg),
              _PlatformReviewSection(application: application, status: status),
              const SizedBox(height: HkSpace.lg),
              _PaymentAccountSection(approved: platformApproved(status)),
            ],
          ),
        ),
      ),
    );
  }
}

class _SectionCard extends StatelessWidget {
  const _SectionCard({
    required this.cardKey,
    required this.title,
    required this.children,
  });

  final Key cardKey;
  final String title;
  final List<Widget> children;

  @override
  Widget build(BuildContext context) {
    return SizedBox(
      width: double.infinity,
      child: HkFrostedCardSurface(
        radius: HkRadius.xl,
        shadows: HkShadows.lg,
        child: Padding(
          key: cardKey,
          padding: const EdgeInsets.all(HkSpace.xl),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: const TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textEmphasis,
                ),
              ),
              const SizedBox(height: HkSpace.md),
              ...children,
            ],
          ),
        ),
      ),
    );
  }
}

const _bodyStyle = TextStyle(fontSize: 14, color: AppColors.gray1, height: 1.5);

/// 「平台審核」區：徽章文字是目錄的 `KYCStatus` 名詞；審核中顯示送出時間，已通過顯示核准時間。
class _PlatformReviewSection extends StatelessWidget {
  const _PlatformReviewSection({
    required this.application,
    required this.status,
  });

  final KYCApplication application;
  final KYCStatus status;

  @override
  Widget build(BuildContext context) {
    final l = context.uiLocale;
    final tone = kycStatusTone(status);
    final stamp = kycStampOf(application, status);
    return _SectionCard(
      cardKey: const Key('kyc_platform_review'),
      title: '平台審核',
      children: [
        HkStatusBadge(
          label: status.uiText(l) ?? commonUnknown(l),
          tone: tone ?? HkStatusTone.neutral,
        ),
        if (stamp != null) ...[
          const SizedBox(height: HkSpace.sm),
          Text(switch (stamp.kind) {
            KycStampKind.decide => '核准時間：${stamp.time}',
            KycStampKind.submit => '送出時間：${stamp.time}',
          }, style: _bodyStyle),
        ],
      ],
    );
  }
}

/// 「收款帳戶」區：平台審核核准之前是灰色的提示；核准之後依 `GetMyPaymentAccount` 顯示目錄的狀態、說明與動作。
/// 不顯示金流商的待補條件細項（金流商自己的頁面會列出要補什麼）。
class _PaymentAccountSection extends HookConsumerWidget {
  const _PaymentAccountSection({required this.approved});

  final bool approved;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final l = context.uiLocale;
    final actions = usePaymentAccountActions(context, ref);

    if (!approved) {
      return const _SectionCard(
        cardKey: Key('kyc_payment_account'),
        title: '收款帳戶',
        children: [Text(paymentAccountNotYetOpenNotice, style: _bodyStyle)],
      );
    }

    final read = ref.watch(currentPaymentAccountProvider);
    if (read == null || (read.isLoading && !read.hasValue)) {
      return const _SectionCard(
        cardKey: Key('kyc_payment_account'),
        title: '收款帳戶',
        children: [Text('載入中…', style: _bodyStyle)],
      );
    }
    final account = read.hasValue
        ? effectivePaymentAccount(read.requireValue, approved: true)
        : null;
    final tone = account == null ? null : paymentAccountTone(account.status);
    final badge = account?.status.uiText(l);
    if (account == null || tone == null || badge == null) {
      return _SectionCard(
        cardKey: const Key('kyc_payment_account'),
        title: '收款帳戶',
        children: [
          Text(
            ref.watch(defaultErrorTextProvider)(Code.internal),
            style: _bodyStyle,
          ),
          const SizedBox(height: HkSpace.md),
          OutlinedButton(
            onPressed: actions.busy
                ? null
                : () => actions.run(
                    PaymentAccount(),
                    PaymentAccountActionKind.reload,
                  ),
            child: const HkButtonLabel('重新整理'),
          ),
        ],
      );
    }

    final description = account.status.description(l);
    final action = account.status.action(l);
    final enabled =
        account.status == PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED;
    final fetched = account.hasFetchTime()
        ? formatTaipeiTimestamp(account.fetchTime)
        : null;
    final embedded =
        account.onboardingSurface ==
        OnboardingSurface.ONBOARDING_SURFACE_EMBEDDED;
    final kind = action == null
        ? PaymentAccountActionKind.none
        : paymentAccountActionKind(account);
    VoidCallback? onPressed() =>
        actions.busy ? null : () => actions.run(account, kind);
    final Widget? button = switch (kind) {
      PaymentAccountActionKind.none => null,
      PaymentAccountActionKind.contactSupport => OutlinedButton(
        onPressed: () => actions.run(account, kind),
        child: HkButtonLabel(action!),
      ),
      PaymentAccountActionKind.onboard => FilledButton(
        key: const Key('payment_account_onboard'),
        onPressed: onPressed(),
        child: HkButtonLabel(action!),
      ),
      PaymentAccountActionKind.reload => OutlinedButton(
        key: const Key('payment_account_refresh'),
        onPressed: onPressed(),
        child: HkButtonLabel(action!),
      ),
    };
    return _SectionCard(
      cardKey: const Key('kyc_payment_account'),
      title: '收款帳戶',
      children: [
        HkStatusBadge(label: badge, tone: tone),
        if (description != null) ...[
          const SizedBox(height: HkSpace.sm),
          Text(description, style: _bodyStyle),
        ],
        if (enabled && !account.payoutsActive) ...[
          const SizedBox(height: HkSpace.xs),
          Text(sellerPaymentAccountPayoutsInactiveNote(l), style: _bodyStyle),
        ],
        if (paymentAccountCanOnboard(account) && embedded) ...[
          const SizedBox(height: HkSpace.sm),
          const Text(paymentAccountEmbeddedNotice, style: _bodyStyle),
        ],
        if (button != null) ...[const SizedBox(height: HkSpace.md), button],
        if (fetched != null) ...[
          const SizedBox(height: HkSpace.sm),
          Text(
            '上次更新 $fetched',
            style: const TextStyle(
              fontSize: 12,
              color: AppColors.textSecondary,
            ),
          ),
        ],
      ],
    );
  }
}
