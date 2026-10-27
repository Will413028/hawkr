import 'dart:async';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';

class SwipeToPay extends StatefulWidget {
  const SwipeToPay({super.key, this.onComplete});
  final FutureOr<void> Function()? onComplete;
  @override
  State<SwipeToPay> createState() => _SwipeToPayState();
}

class _SwipeToPayState extends State<SwipeToPay> {
  double _dx = 0; // current drag x in px
  bool _done = false;

  Future<void> _complete(double maxDx) async {
    if (_done || widget.onComplete == null) return;
    setState(() {
      _dx = maxDx;
      _done = true;
    });
    try {
      await widget.onComplete!();
    } finally {
      // Successful checkout normally closes this surface. If validation or a
      // request fails and the sheet stays open, restore the control so the
      // buyer can correct the issue and retry.
      if (mounted) {
        setState(() {
          _dx = 0;
          _done = false;
        });
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    const barHeight = HkControl.minTap;
    const radius = HkRadius.md;
    const sidePadding = HkSpace.xl;
    const arrowSize = HkControl.icon;

    return LayoutBuilder(
      builder: (context, constraints) {
        final double width = constraints.maxWidth;
        final maxDx = (width - sidePadding * 2 - arrowSize)
            .clamp(0.0, double.infinity)
            .toDouble();
        final progress = maxDx == 0
            ? 0.0
            : (_dx / maxDx).clamp(0.0, 1.0).toDouble();
        final enabled = widget.onComplete != null && !_done;

        return Semantics(
          container: true,
          button: true,
          enabled: enabled,
          label: _done ? '正在建立訂單' : '向右滑結帳',
          hint: enabled ? '向右滑動至底；使用輔助功能時點兩下確認' : null,
          onTap: enabled
              ? () {
                  unawaited(_complete(maxDx));
                }
              : null,
          excludeSemantics: true,
          child: GestureDetector(
            behavior: HitTestBehavior.opaque,
            onHorizontalDragUpdate: enabled
                ? (details) {
                    setState(() {
                      _dx = (_dx + details.delta.dx).clamp(0.0, maxDx);
                    });
                  }
                : null,
            onHorizontalDragEnd: enabled
                ? (_) {
                    final releaseProgress = maxDx == 0
                        ? 0.0
                        : (_dx / maxDx).clamp(0.0, 1.0);
                    if (releaseProgress > 0.9) {
                      unawaited(_complete(maxDx));
                    } else {
                      setState(() {
                        _dx = 0;
                      });
                    }
                  }
                : null,
            child: SizedBox(
              height: barHeight,
              child: Stack(
                children: [
                  // Base green track
                  Positioned.fill(
                    child: Container(
                      decoration: BoxDecoration(
                        color: AppColors.primary,
                        borderRadius: BorderRadius.circular(radius),
                      ),
                    ),
                  ),

                  // Progress sheen
                  Positioned.fill(
                    child: Align(
                      alignment: Alignment.centerLeft,
                      child: FractionallySizedBox(
                        widthFactor: progress,
                        child: Container(
                          decoration: BoxDecoration(
                            color: AppColors.bgCard.withValues(alpha: 0.12),
                            borderRadius: BorderRadius.circular(radius),
                          ),
                        ),
                      ),
                    ),
                  ),

                  // Reserve room for both icons so large text never overlaps.
                  Positioned.fill(
                    child: Padding(
                      padding: const EdgeInsets.symmetric(
                        horizontal: sidePadding + arrowSize + HkSpace.sm,
                      ),
                      child: Center(
                        child: HkButtonLabel(
                          _done ? '處理中…' : '向右滑結帳',
                          style: AppTypography.sectionTitle.copyWith(
                            color: AppColors.bgCard,
                          ),
                        ),
                      ),
                    ),
                  ),

                  // Left chevron-right icon (moves with drag)
                  if (!_done)
                    Positioned(
                      left: sidePadding + progress * maxDx,
                      top: 0,
                      bottom: 0,
                      child: Icon(
                        AppIcons.chevronRight,
                        color: AppIconColors.onBrand,
                        size: arrowSize,
                      ),
                    ),

                  // Right lock icon
                  Positioned(
                    right: sidePadding,
                    top: 0,
                    bottom: 0,
                    child: Icon(
                      AppIcons.lockOutline,
                      color: AppIconColors.onBrand.withValues(alpha: 0.6),
                      size: arrowSize,
                    ),
                  ),
                ],
              ),
            ),
          ),
        );
      },
    );
  }
}
