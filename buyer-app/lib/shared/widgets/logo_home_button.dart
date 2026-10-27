import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../core/theme/hk_tokens.dart';
import 'tab_switch_provider.dart';

/// Shared logo button that navigates to home with a scale-down tap animation.
/// Used in top bars across all main screens (category, wallet, settings, etc.).
class LogoHomeButton extends ConsumerStatefulWidget {
  const LogoHomeButton({super.key, this.size, this.color = AppColors.bgCard});

  /// Icon size (width & height). Defaults to 32.w.
  final double? size;

  /// SVG color filter color. Defaults to white.
  final Color color;

  @override
  ConsumerState<LogoHomeButton> createState() => _LogoHomeButtonState();
}

/// 網頁 mobile header 使用的橫式 Hawkr 品牌標誌。
///
/// 維持 48dp 點擊熱區、保留原始 SVG 色票；窄螢幕只壓縮可用寬度，不縮小
/// Header 與 action 的標準高度。
class HawkrWordmarkButton extends ConsumerWidget {
  const HawkrWordmarkButton({super.key, this.compact = false});

  static const double wordmarkWidth = 112;
  static const double compactWidth = HkControl.minTap;

  final bool compact;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    void goHome() {
      ref.read(tabSwitchRequestProvider.notifier).state = 0;
    }

    return Semantics(
      label: 'Hawkr 首頁',
      button: true,
      onTap: goHome,
      child: ExcludeSemantics(
        child: GestureDetector(
          behavior: HitTestBehavior.opaque,
          onTap: goHome,
          child: SizedBox(
            key: ValueKey(compact ? 'hawkr-compact-brand' : 'hawkr-wordmark'),
            width: compact ? compactWidth : wordmarkWidth,
            height: HkControl.minTap,
            child: Align(
              alignment: Alignment.centerLeft,
              child: SvgPicture.asset(
                compact
                    ? 'assets/brand/logo-grass.svg'
                    : 'assets/brand/logo-horizontal.svg',
                width: compact ? 32 : 108,
                height: compact ? 32 : 28,
                fit: BoxFit.contain,
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _LogoHomeButtonState extends ConsumerState<LogoHomeButton>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  late final Animation<double> _scale;

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 120),
    );
    _scale = Tween<double>(
      begin: 1.0,
      end: 0.85,
    ).animate(CurvedAnimation(parent: _controller, curve: Curves.easeInOut));
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  Future<void> _onTap() async {
    await _controller.forward();
    await _controller.reverse();
    if (mounted) {
      ref.read(tabSwitchRequestProvider.notifier).state = 0;
    }
  }

  @override
  Widget build(BuildContext context) {
    final s = widget.size ?? 32.w;
    return GestureDetector(
      onTap: _onTap,
      behavior: HitTestBehavior.opaque,
      child: Padding(
        padding: EdgeInsets.all(8.w),
        child: ScaleTransition(
          scale: _scale,
          child: SvgPicture.asset(
            'assets/brand/logo-grass.svg',
            width: s,
            height: s,
            colorFilter: ColorFilter.mode(widget.color, BlendMode.srcIn),
          ),
        ),
      ),
    );
  }
}
