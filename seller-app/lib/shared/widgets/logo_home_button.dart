import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:go_router/go_router.dart';

/// Hawkr logo 按鈕，點擊帶縮放動畫後導回首頁。
class LogoHomeButton extends StatefulWidget {
  const LogoHomeButton({
    super.key,
    this.size = 30,
    this.color,
    this.assetPath = defaultAssetPath,
  });

  static const defaultAssetPath = 'assets/brand/logo-four-color.svg';

  final double size;
  final String assetPath;

  /// 僅供需要單色反白的特殊情境；預設保留品牌四色。
  final Color? color;

  @override
  State<LogoHomeButton> createState() => _LogoHomeButtonState();
}

class _LogoHomeButtonState extends State<LogoHomeButton>
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
    final reduceMotion =
        MediaQuery.maybeOf(context)?.disableAnimations ?? false;
    if (!reduceMotion) {
      await _controller.forward();
      await _controller.reverse();
    }
    if (mounted) {
      context.go('/main/dashboard');
    }
  }

  @override
  Widget build(BuildContext context) {
    final mark = widget.assetPath.toLowerCase().endsWith('.svg')
        ? SvgPicture.asset(
            widget.assetPath,
            key: const ValueKey('hawkr_four_color_logo'),
            width: widget.size,
            height: widget.size,
            colorFilter: widget.color == null
                ? null
                : ColorFilter.mode(widget.color!, BlendMode.srcIn),
          )
        : Image.asset(
            widget.assetPath,
            key: const ValueKey('hawkr_four_color_logo'),
            width: widget.size,
            height: widget.size,
            fit: BoxFit.contain,
            filterQuality: FilterQuality.high,
            color: widget.color,
            colorBlendMode: widget.color == null ? null : BlendMode.srcIn,
          );

    return Semantics(
      button: true,
      label: '返回總覽',
      child: Tooltip(
        message: '返回總覽',
        child: Material(
          color: Colors.transparent,
          child: InkResponse(
            onTap: _onTap,
            radius: 24,
            customBorder: const CircleBorder(),
            child: SizedBox.square(
              dimension: 48,
              child: Center(
                child: ScaleTransition(scale: _scale, child: mark),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
