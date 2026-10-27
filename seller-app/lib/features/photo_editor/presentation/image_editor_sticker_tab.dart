import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'image_editor_models.dart';
import 'image_editor_sticker_badge.dart';

/// 貼圖標籤頁
class ImageEditorStickerTab extends StatefulWidget {
  const ImageEditorStickerTab({
    super.key,
    required this.stickerLayers,
    required this.onAddSticker,
    required this.onUpdateLayer,
    required this.onDeleteLayer,
    required this.onToggleVisibility,
  });

  final List<StickerLayer> stickerLayers;
  final ValueChanged<StickerLayer> onAddSticker;
  final void Function(int index, StickerLayer layer) onUpdateLayer;
  final ValueChanged<int> onDeleteLayer;
  final ValueChanged<int> onToggleVisibility;

  @override
  State<ImageEditorStickerTab> createState() => _ImageEditorStickerTabState();
}

class _ImageEditorStickerTabState extends State<ImageEditorStickerTab> {
  static const List<StickerBadge> _freshStickers = [
    StickerBadge.organic,
    StickerBadge.naturalDrop,
    StickerBadge.naturalWave,
    StickerBadge.freshPicked,
    StickerBadge.localDelivery,
    StickerBadge.fresh,
  ];

  static const List<StickerBadge> _qualityStickers = [
    StickerBadge.quality,
    StickerBadge.premium,
    StickerBadge.limited,
  ];

  // 認證標籤後面用 emoji 補滿
  static const List<String> _certEmojis = ['🏆', '✅', '🎖️'];

  // 裝飾元素 - 使用 emoji
  final List<String> _decoStickers = ['🌸', '🌺', '🌼', '🌻', '🌷', '🪻'];

  // 促銷標記 - 使用 emoji
  final List<String> _promoStickers = ['💰', '🎁', '🎊', '🏷️', '💝', '🎫'];

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppColors.bgPrimary,
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // 標題列
            Padding(
              padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 12.h),
              child: Row(
                children: [
                  Text(
                    '標籤貼圖',
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                ],
              ),
            ),
            // 新鮮標籤 (徽章)
            _buildBadgeStickerSection('新鮮標籤', _freshStickers),
            // 認證標籤 (混合: 徽章 + emoji)
            _buildMixedStickerSection('認證標籤', _qualityStickers, _certEmojis),
            // 裝飾元素 (emoji)
            _buildEmojiStickerSection('裝飾元素', _decoStickers),
            // 促銷標記 (emoji)
            _buildEmojiStickerSection('促銷標記', _promoStickers),
            SizedBox(height: 12.h),
            // 底部提示
            Center(
              child: Text(
                '選擇標籤貼圖來裝飾您的農產品圖片',
                style: TextStyle(
                  fontSize: 12.sp,
                  color: AppColors.textSecondary,
                ),
              ),
            ),
            SizedBox(height: 16.h),
          ],
        ),
      ),
    );
  }

  /// 徽章類型的貼圖區塊
  Widget _buildBadgeStickerSection(String title, List<StickerBadge> stickers) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 8.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
          SizedBox(height: 8.h),
          Wrap(
            spacing: 12,
            runSpacing: 12,
            children: stickers.map((badge) {
              return GestureDetector(
                onTap: () {
                  widget.onAddSticker(StickerLayer(badge: badge));
                },
                child: Container(
                  width: 52.w,
                  height: 52.h,
                  decoration: BoxDecoration(
                    color: AppColors.gray100,
                    borderRadius: BorderRadius.circular(8.r),
                  ),
                  child: Padding(
                    padding: EdgeInsets.all(4.w),
                    child: StickerBadgeView(badge, size: 44.w),
                  ),
                ),
              );
            }).toList(),
          ),
        ],
      ),
    );
  }

  /// 混合類型的貼圖區塊（徽章 + emoji）
  Widget _buildMixedStickerSection(
    String title,
    List<StickerBadge> badges,
    List<String> emojis,
  ) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 8.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
          SizedBox(height: 8.h),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: [
              for (final badge in badges)
                GestureDetector(
                  onTap: () => widget.onAddSticker(StickerLayer(badge: badge)),
                  child: Container(
                    width: 48.w,
                    height: 48.h,
                    decoration: BoxDecoration(
                      color: AppColors.gray100,
                      borderRadius: BorderRadius.circular(8.r),
                    ),
                    child: Padding(
                      padding: EdgeInsets.all(4.w),
                      child: StickerBadgeView(badge, size: 40.w),
                    ),
                  ),
                ),
              for (final emoji in emojis)
                GestureDetector(
                  onTap: () => widget.onAddSticker(StickerLayer(emoji: emoji)),
                  child: Container(
                    width: 48.w,
                    height: 48.h,
                    decoration: BoxDecoration(
                      color: AppColors.gray100,
                      borderRadius: BorderRadius.circular(8.r),
                    ),
                    child: Center(
                      child: Text(emoji, style: TextStyle(fontSize: 24.sp)),
                    ),
                  ),
                ),
            ],
          ),
        ],
      ),
    );
  }

  /// Emoji 類型的貼圖區塊
  Widget _buildEmojiStickerSection(String title, List<String> stickers) {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 16.w, vertical: 8.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            title,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textSecondary),
          ),
          SizedBox(height: 8.h),
          Row(
            mainAxisAlignment: MainAxisAlignment.spaceBetween,
            children: stickers.map((emoji) {
              return GestureDetector(
                onTap: () {
                  widget.onAddSticker(StickerLayer(emoji: emoji));
                },
                child: Container(
                  width: 48.w,
                  height: 48.h,
                  decoration: BoxDecoration(
                    color: AppColors.gray100,
                    borderRadius: BorderRadius.circular(8.r),
                  ),
                  child: Center(
                    child: Text(emoji, style: TextStyle(fontSize: 24.sp)),
                  ),
                ),
              );
            }).toList(),
          ),
        ],
      ),
    );
  }
}
