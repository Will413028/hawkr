import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';

import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'hk_empty_state.dart';
import 'hk_content_card.dart';
import 'hk_icon.dart';

enum HkAsyncStateKind { empty, filteredEmpty, offline, error, restricted }

/// Network-backed pages share the same recovery language and geometry.
class HkAsyncStatePanel extends StatelessWidget {
  const HkAsyncStatePanel({
    super.key,
    required this.kind,
    required this.title,
    required this.description,
    this.actionLabel,
    this.onAction,
    this.actionIcon,
    this.icon,
  });

  final HkAsyncStateKind kind;
  final String title;
  final String description;
  final String? actionLabel;
  final VoidCallback? onAction;

  /// 動作按鈕的圖示；沒給時依 [kind] 決定。
  final HkIconName? actionIcon;

  /// 狀態圖示；沒給時依 [kind] 決定。
  final HkIconName? icon;

  @override
  Widget build(BuildContext context) {
    final resolvedIcon =
        icon ??
        switch (kind) {
          HkAsyncStateKind.empty => HkIconName.search,
          HkAsyncStateKind.filteredEmpty => HkIconName.search,
          HkAsyncStateKind.offline => HkIconName.truck,
          HkAsyncStateKind.error => HkIconName.document,
          HkAsyncStateKind.restricted => HkIconName.document,
        };
    final resolvedActionIcon =
        actionIcon ??
        switch (kind) {
          HkAsyncStateKind.filteredEmpty => HkIconName.filter,
          HkAsyncStateKind.offline ||
          HkAsyncStateKind.error => HkIconName.refresh,
          HkAsyncStateKind.restricted => HkIconName.shield,
          HkAsyncStateKind.empty => HkIconName.add,
        };

    return HkContentCard(
      padding: const EdgeInsets.fromLTRB(
        HkSpace.xxl,
        HkSpace.lg,
        HkSpace.xxl,
        HkSpace.xxl,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          HkEmptyState(
            icon: resolvedIcon,
            message: title,
            description: description,
            topPadding: 0,
          ),
          if (actionLabel case final actionLabel?) ...[
            const SizedBox(height: HkSpace.lg),
            FilledButton.icon(
              onPressed: onAction,
              icon: HkIcon(
                resolvedActionIcon,
                color: AppIconColors.onBrand,
                semanticLabel: actionLabel,
              ),
              label: HkButtonLabel(actionLabel),
            ),
          ],
        ],
      ),
    );
  }
}
