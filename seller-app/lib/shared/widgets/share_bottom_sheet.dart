import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../config/image_config.dart';
import '../utils/floating_toast.dart';
import 'optimized_image.dart';

/// 取不到伺服器分享資料時的失敗提示。
const String kShareFailedMessage = '分享失敗：請稍後再試';

/// 分享資料結構。
class ShareData {
  final String title;
  final String description;
  final String imageUrl;
  final String url;

  const ShareData({
    required this.title,
    required this.description,
    required this.imageUrl,
    required this.url,
  });
}

/// 顯示分享底部彈窗（貼緊螢幕邊緣 + 底部）。
void showShareSheet(BuildContext context, ShareData data) {
  showModalBottomSheet(
    context: context,
    useRootNavigator: true,
    backgroundColor: AppColors.bgCard,
    isScrollControlled: true,
    shape: RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(16.r)),
    ),
    builder: (_) => _ShareSheet(data: data),
  );
}

class _ShareSheet extends StatefulWidget {
  final ShareData data;
  const _ShareSheet({required this.data});

  @override
  State<_ShareSheet> createState() => _ShareSheetState();
}

class _ShareSheetState extends State<_ShareSheet>
    with SingleTickerProviderStateMixin {
  late AnimationController _copyCtrl;
  bool _copied = false;

  @override
  void initState() {
    super.initState();
    _copyCtrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 300),
    );
  }

  @override
  void dispose() {
    _copyCtrl.dispose();
    super.dispose();
  }

  void _handleCopy() {
    if (_copied) return;
    Clipboard.setData(ClipboardData(text: widget.data.url));
    setState(() => _copied = true);
    _copyCtrl.forward();
    FloatingToast.success(context, '連結已複製');

    Future.delayed(const Duration(milliseconds: 1200), () {
      if (!mounted) return;
      _copyCtrl.reverse().then((_) {
        if (mounted) setState(() => _copied = false);
      });
    });
  }

  @override
  Widget build(BuildContext context) {
    final bottomPadding = MediaQuery.of(context).padding.bottom;
    final hasImage =
        widget.data.imageUrl.isNotEmpty && widget.data.imageUrl != 'string';

    return Padding(
      padding: EdgeInsets.only(bottom: bottomPadding),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          // ── 拖曳指示條 ──────────────────────────────────
          Center(
            child: Container(
              margin: EdgeInsets.only(top: 12.h, bottom: 8.h),
              width: 36.w,
              height: 4.h,
              decoration: BoxDecoration(
                color: AppColors.dividerLight,
                borderRadius: BorderRadius.circular(2.r),
              ),
            ),
          ),

          // ── 標題列 ─────────────────────────────────────
          Padding(
            padding: EdgeInsets.fromLTRB(20.w, 4.h, 8.w, 8.h),
            child: Row(
              children: [
                Text(
                  '分享',
                  style: TextStyle(
                    fontSize: 18.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
                const Spacer(),
                IconButton(
                  onPressed: () => Navigator.of(context).pop(),
                  icon: Icon(
                    AppIcons.closeRounded,
                    size: 22.sp,
                    color: AppIconColors.muted,
                  ),
                ),
              ],
            ),
          ),

          // ── 圖片 + 文字 ────────────────────────────────
          Padding(
            padding: EdgeInsets.symmetric(horizontal: 20.w),
            child: Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                // 左側縮圖
                if (hasImage)
                  ClipRRect(
                    borderRadius: BorderRadius.circular(12.r),
                    child: SizedBox(
                      width: 80.w,
                      height: 80.w,
                      child: OptimizedNetworkImage(
                        imageUrl: widget.data.imageUrl,
                        scene: ImageScene.shareThumbnail,
                        width: 80.w,
                        height: 80.w,
                        fit: BoxFit.cover,
                        errorWidget: Container(
                          color: AppColors.gray100,
                          child: Icon(
                            AppIcons.imageOutlined,
                            size: 28.sp,
                            color: AppIconColors.disabled,
                          ),
                        ),
                      ),
                    ),
                  ),
                if (hasImage) SizedBox(width: 14.w),
                // 右側文字
                Expanded(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        widget.data.title,
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textEmphasis,
                          height: 1.3,
                        ),
                        maxLines: 2,
                        overflow: TextOverflow.ellipsis,
                      ),
                      if (widget.data.description.isNotEmpty) ...[
                        SizedBox(height: 4.h),
                        Text(
                          widget.data.description,
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.textSecondary,
                            height: 1.3,
                          ),
                          maxLines: 2,
                          overflow: TextOverflow.ellipsis,
                        ),
                      ],
                    ],
                  ),
                ),
              ],
            ),
          ),

          SizedBox(height: 16.h),

          // ── URL 複製列 ─────────────────────────────────
          Padding(
            padding: EdgeInsets.fromLTRB(20.w, 0, 20.w, 16.h),
            child: GestureDetector(
              onTap: _handleCopy,
              child: Container(
                padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 12.h),
                decoration: BoxDecoration(
                  color: AppColors.bgPrimary,
                  borderRadius: BorderRadius.circular(12.r),
                  border: Border.all(color: AppColors.surfacePanel),
                ),
                child: Row(
                  children: [
                    Icon(
                      AppIcons.linkRounded,
                      size: 18.sp,
                      color: AppIconColors.secondary,
                    ),
                    SizedBox(width: 8.w),
                    Expanded(
                      child: Text(
                        widget.data.url,
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.textSecondary,
                        ),
                        maxLines: 1,
                        overflow: TextOverflow.ellipsis,
                      ),
                    ),
                    SizedBox(width: 8.w),
                    AnimatedBuilder(
                      animation: _copyCtrl,
                      builder: (_, __) {
                        final t = _copyCtrl.value;
                        return Container(
                          padding: EdgeInsets.symmetric(
                            horizontal: 12.w,
                            vertical: 8.h,
                          ),
                          decoration: BoxDecoration(
                            color: Color.lerp(
                              AppColors.primary,
                              AppColors.success,
                              t,
                            ),
                            borderRadius: BorderRadius.circular(6.r),
                          ),
                          child: Row(
                            mainAxisSize: MainAxisSize.min,
                            children: [
                              Icon(
                                _copied
                                    ? AppIcons.checkRounded
                                    : AppIcons.copyRounded,
                                size: 14.sp,
                                color: AppIconColors.onBrand,
                              ),
                              SizedBox(width: 4.w),
                              Text(
                                _copied ? '已複製' : '複製',
                                style: TextStyle(
                                  fontSize: 12.sp,
                                  fontWeight: FontWeight.w700,
                                  color: AppColors.bgCard,
                                ),
                              ),
                            ],
                          ),
                        );
                      },
                    ),
                  ],
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }
}
