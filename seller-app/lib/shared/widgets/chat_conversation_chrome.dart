import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../core/theme/app_colors.dart';
import '../../core/theme/app_typography.dart';
import '../../core/theme/hk_tokens.dart';
import '../icons/app_icon_colors.dart';
import 'hk_glass_chrome_surface.dart';
import 'hk_icon.dart';

/// 買賣家聊天室共用的 iOS 26 霧面玻璃頂部列。
class ChatConversationTopBar extends StatelessWidget {
  const ChatConversationTopBar({
    super.key,
    required this.avatar,
    required this.title,
    required this.statusLabel,
    required this.statusColor,
    required this.onBack,
  });

  final Widget avatar;
  final String title;
  final String statusLabel;
  final Color statusColor;
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    return AnnotatedRegion<SystemUiOverlayStyle>(
      value: SystemUiOverlayStyle.dark,
      child: HkGlassChromeSurface(
        surfaceKey: const ValueKey('chat-conversation-top-bar'),
        child: Padding(
          padding: EdgeInsets.only(top: MediaQuery.paddingOf(context).top),
          child: ChatConversationHeaderContent(
            avatar: avatar,
            title: title,
            statusLabel: statusLabel,
            statusColor: statusColor,
            onBack: onBack,
          ),
        ),
      ),
    );
  }
}

/// 與買家端一致的返回鍵、40px 頭像、標題與在線狀態幾何。
class ChatConversationHeaderContent extends StatelessWidget {
  const ChatConversationHeaderContent({
    super.key,
    required this.avatar,
    required this.title,
    required this.statusLabel,
    required this.statusColor,
    required this.onBack,
  });

  final Widget avatar;
  final String title;
  final String statusLabel;
  final Color statusColor;
  final VoidCallback onBack;

  @override
  Widget build(BuildContext context) {
    final textScaler = MediaQuery.textScalerOf(context);
    final titleStyle = AppTypography.sectionTitle.copyWith(
      color: AppColors.textEmphasis,
    );
    final statusStyle = AppTypography.caption.copyWith(
      color: AppColors.textHint,
    );
    final stackedTextHeight =
        _singleLineTextHeight(
          context: context,
          text: title,
          style: titleStyle,
          textScaler: textScaler,
        ) +
        2 +
        _singleLineTextHeight(
          context: context,
          text: statusLabel,
          style: statusStyle,
          textScaler: textScaler,
        );
    final usesInlineIdentity = stackedTextHeight > HkControl.topBarHeight;

    final status = Row(
      mainAxisSize: usesInlineIdentity ? MainAxisSize.min : MainAxisSize.max,
      children: [
        Container(
          width: 6,
          height: 6,
          decoration: BoxDecoration(color: statusColor, shape: BoxShape.circle),
        ),
        SizedBox(width: 4.w),
        Flexible(
          child: Text(
            statusLabel,
            style: statusStyle,
            maxLines: 1,
            softWrap: false,
            overflow: TextOverflow.ellipsis,
          ),
        ),
      ],
    );

    final identity = usesInlineIdentity
        ? Row(
            children: [
              Expanded(
                flex: 2,
                child: Text(
                  title,
                  style: titleStyle,
                  maxLines: 1,
                  softWrap: false,
                  overflow: TextOverflow.ellipsis,
                ),
              ),
              SizedBox(width: 8.w),
              Flexible(child: status),
            ],
          )
        : Column(
            mainAxisAlignment: MainAxisAlignment.center,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: titleStyle,
                maxLines: 1,
                softWrap: false,
                overflow: TextOverflow.ellipsis,
              ),
              SizedBox(height: 2.h),
              status,
            ],
          );

    return SizedBox(
      height: HkControl.topBarHeight,
      child: Row(
        children: [
          SizedBox(width: 8.w),
          Semantics(
            label: '返回',
            button: true,
            onTap: onBack,
            child: ExcludeSemantics(
              child: SizedBox.square(
                dimension: HkControl.minTap,
                child: IconButton(
                  onPressed: onBack,
                  tooltip: '返回',
                  icon: const HkIcon(
                    HkIconName.back,
                    color: AppIconColors.strong,
                    size: HkControl.icon,
                  ),
                ),
              ),
            ),
          ),
          SizedBox(width: 8.w),
          SizedBox.square(dimension: 40, child: avatar),
          SizedBox(width: 12.w),
          Expanded(child: identity),
          SizedBox(width: 16.w),
        ],
      ),
    );
  }
}

double _singleLineTextHeight({
  required BuildContext context,
  required String text,
  required TextStyle style,
  required TextScaler textScaler,
}) {
  final painter = TextPainter(
    text: TextSpan(text: text, style: style),
    maxLines: 1,
    textDirection: Directionality.of(context),
    textScaler: textScaler,
  )..layout();
  final height = painter.height;
  painter.dispose();
  return height;
}

/// 與買家端相同的 Hawkr 品牌點陣聊天背景。
class ChatConversationDotPattern extends StatelessWidget {
  const ChatConversationDotPattern({super.key});

  @override
  Widget build(BuildContext context) {
    return const RepaintBoundary(
      child: CustomPaint(painter: _ChatConversationDotPainter()),
    );
  }
}

class _ChatConversationDotPainter extends CustomPainter {
  const _ChatConversationDotPainter();

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = AppColors.primary.withValues(alpha: 0.07)
      ..style = PaintingStyle.fill;
    const spacing = 24.0;
    for (double x = 8; x < size.width; x += spacing) {
      for (double y = 8; y < size.height; y += spacing) {
        canvas.drawCircle(Offset(x, y), 1.5, paint);
      }
    }
  }

  @override
  bool shouldRepaint(_ChatConversationDotPainter oldDelegate) => false;
}
