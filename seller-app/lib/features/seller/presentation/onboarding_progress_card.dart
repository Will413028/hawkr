import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/router/app_router.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/utils/use_on_became_visible.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../data/current_kyc.dart';
import '../data/current_store.dart';
import '../data/onboarding_tasks.dart';
import '../data/seller_status_refresh.dart';

/// 開店進度卡（B8）：項目、分組與文字和 website 的 `OnboardingProgressCard` 同一套，資料來自
/// `GetMyStore.capabilities` 的 `PUBLIC_LISTING`、`GetMyKYC.flow`、物流開通與商品數。
///
/// 「公開前必須」全部完成時預設收合。[onOpenProfile] 是「商店資料」「商店介紹」的去處；沒給時回商店設定頁。
class OnboardingProgressCard extends HookConsumerWidget {
  const OnboardingProgressCard({super.key, this.onOpenProfile});

  final VoidCallback? onOpenProfile;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final my = ref.watch(myStoreSnapshotProvider);
    final kyc = ref.watch(currentKycProvider)?.value;
    final inputs =
        ref.watch(onboardingInputsProvider).value ?? const OnboardingInputs();
    final tasks = buildOnboardingTasks(
      l: context.uiLocale,
      my: my,
      flowStatus: kyc?.status,
      shipping: inputs.shipping,
      productCount: inputs.productCount,
    );
    final required = tasks
        .where((task) => task.group == OnboardingTaskGroup.beforePublish)
        .toList();
    final completedRequired = required.where((task) => task.completed).length;
    final allRequiredComplete =
        required.isNotEmpty && completedRequired == required.length;
    final collapsedOverride = useState<bool?>(null);
    final collapsed = collapsedOverride.value ?? allRequiredComplete;

    // 進度卡常駐在主導航的分頁裡：去別的頁面辦事回來（成為可見）才重讀，離開時不讀。物流與商品數是辦事的結果，每次回來都重讀；
    // 店家與認證流程 30 秒內讀過就不再讀。
    useOnBecameVisible(() {
      ref.invalidate(onboardingInputsProvider);
      unawaited(
        ref
            .read(sellerStatusRefreshProvider.notifier)
            .refresh(maxAge: statusStaleTime),
      );
    });

    void open(OnboardingTask task) {
      switch (task.id) {
        case OnboardingTaskId.profile:
        case OnboardingTaskId.description:
          _openProfile(context);
        case OnboardingTaskId.cert:
          context.go(MainRoutes.wallet);
        case OnboardingTaskId.notSuspended:
          unawaited(context.push<void>(SellerRoutes.suspensionNotice));
        case OnboardingTaskId.storeActive:
          unawaited(context.push<void>(SellerRoutes.suspensionNotice));
        case OnboardingTaskId.shipping:
          unawaited(context.push<void>(SettingsRoutes.shipping));
        case OnboardingTaskId.product:
          unawaited(context.push<void>(ProductRoutes.picker));
        case OnboardingTaskId.otherRequirement:
          break;
      }
    }

    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _Header(
            completedRequired: completedRequired,
            totalRequired: required.length,
            allRequiredComplete: allRequiredComplete,
            collapsed: collapsed,
            onToggle: () => collapsedOverride.value = !collapsed,
          ),
          if (!collapsed)
            for (final group in OnboardingTaskGroup.values)
              if (tasks.any((task) => task.group == group)) ...[
                const SizedBox(height: HkSpace.lg),
                Semantics(
                  header: true,
                  child: Text(
                    group.label,
                    style: const TextStyle(
                      color: AppColors.textSecondary,
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                for (final task in tasks.where((task) => task.group == group))
                  _TaskRow(task: task, onOpen: () => open(task)),
              ],
        ],
      ),
    );
  }

  void _openProfile(BuildContext context) {
    final callback = onOpenProfile;
    if (callback != null) {
      callback();
      return;
    }
    context.go(MainRoutes.settings);
  }
}

class _Header extends StatelessWidget {
  const _Header({
    required this.completedRequired,
    required this.totalRequired,
    required this.allRequiredComplete,
    required this.collapsed,
    required this.onToggle,
  });

  final int completedRequired;
  final int totalRequired;
  final bool allRequiredComplete;
  final bool collapsed;
  final VoidCallback onToggle;

  @override
  Widget build(BuildContext context) {
    final progress = totalRequired == 0
        ? 0.0
        : completedRequired / totalRequired;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  const Text(
                    '開店進度',
                    style: TextStyle(
                      color: AppColors.textSecondary,
                      fontSize: 12,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: HkSpace.xs),
                  Wrap(
                    crossAxisAlignment: WrapCrossAlignment.center,
                    spacing: HkSpace.sm,
                    children: [
                      const Text(
                        '賣家開店任務',
                        style: TextStyle(
                          color: AppColors.textEmphasis,
                          fontSize: 16,
                          fontWeight: FontWeight.w700,
                        ),
                      ),
                      if (allRequiredComplete)
                        const HkStatusBadge(
                          label: '任務完成',
                          tone: HkStatusTone.success,
                          icon: HkIconName.check,
                        ),
                    ],
                  ),
                ],
              ),
            ),
            if (!allRequiredComplete)
              Text(
                '$completedRequired/$totalRequired',
                style: const TextStyle(
                  color: AppColors.textEmphasis,
                  fontSize: 14,
                  fontWeight: FontWeight.w700,
                ),
              ),
          ],
        ),
        if (!allRequiredComplete) ...[
          const SizedBox(height: HkSpace.md),
          Semantics(
            label: '開店進度',
            value: '${(progress * 100).round()}%',
            child: ExcludeSemantics(
              child: ClipRRect(
                borderRadius: BorderRadius.circular(HkRadius.pill),
                child: LinearProgressIndicator(
                  value: progress,
                  minHeight: 6,
                  backgroundColor: AppColors.neutralPanel,
                  color: AppColors.actionPrimary,
                ),
              ),
            ),
          ),
        ],
        Align(
          alignment: Alignment.centerRight,
          child: TextButton(
            onPressed: onToggle,
            style: TextButton.styleFrom(
              minimumSize: const Size(HkControl.minTap, HkControl.minTap),
            ),
            child: HkButtonLabel(collapsed ? '展開任務' : '收合'),
          ),
        ),
      ],
    );
  }
}

class _TaskRow extends StatelessWidget {
  const _TaskRow({required this.task, required this.onOpen});

  final OnboardingTask task;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    final icon = task.completed
        ? HkIconName.check
        : task.locked
        ? HkIconName.shield
        : HkIconName.clock;
    final iconColor = task.completed
        ? AppIconColors.success
        : AppIconColors.muted;
    final cta = task.cta;
    return Semantics(
      container: true,
      label: '${task.title}，${task.statusLabel}',
      child: Padding(
        padding: const EdgeInsets.only(top: HkSpace.md),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: HkIcon(icon, size: 20, color: iconColor),
            ),
            const SizedBox(width: HkSpace.md),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    task.title,
                    style: const TextStyle(
                      color: AppColors.textEmphasis,
                      fontSize: 16,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                  const SizedBox(height: HkSpace.xs),
                  Text(
                    task.description,
                    style: const TextStyle(
                      color: AppColors.textSecondary,
                      fontSize: 12,
                    ),
                  ),
                  const SizedBox(height: HkSpace.sm),
                  HkStatusBadge(label: task.statusLabel, tone: task.tone),
                ],
              ),
            ),
            if (cta != null) ...[
              const SizedBox(width: HkSpace.sm),
              TextButton(
                onPressed: task.locked ? null : onOpen,
                style: TextButton.styleFrom(
                  minimumSize: const Size(HkControl.minTap, HkControl.minTap),
                ),
                child: HkButtonLabel(cta),
              ),
            ],
          ],
        ),
      ),
    );
  }
}
