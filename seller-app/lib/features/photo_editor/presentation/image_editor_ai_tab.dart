import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../config/image_config.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../data/ai_style_model.dart';

/// AI 風格編輯標籤頁
class ImageEditorAiTab extends StatefulWidget {
  const ImageEditorAiTab({
    super.key,
    this.selectedMode,
    this.selectedStyle,
    required this.onModeChanged,
    required this.onStyleChanged,
    required this.onApplyPrompt,
    this.aiHistory,
    this.aiUsageCount,
    this.aiMaxUsage,
    this.aiStyles,
    this.category = 'product',
  });

  final String? selectedMode;
  final String? selectedStyle;
  final ValueChanged<String?> onModeChanged;
  final ValueChanged<String?> onStyleChanged;
  final ValueChanged<String> onApplyPrompt;
  final List<Map<String, dynamic>>? aiHistory;
  final int? aiUsageCount;
  final int? aiMaxUsage;
  final List<AiStyle>? aiStyles;

  /// 'product' | 'post' — API 還沒回應時，用哪組本地 fallback 清單
  final String category;

  @override
  State<ImageEditorAiTab> createState() => ImageEditorAiTabState();
}

class ImageEditorAiTabState extends State<ImageEditorAiTab> {
  final TextEditingController _promptController = TextEditingController();
  String _selectedTab = 'preset'; // preset, custom, history
  String? _selectedStyle;
  bool _hasPromptText = false;

  // 使用外部傳入的歷史紀錄
  List<Map<String, dynamic>> get _aiHistory => widget.aiHistory ?? [];
  int get _aiUsageCount => widget.aiUsageCount ?? 0;
  int get _maxAiUsage => widget.aiMaxUsage ?? 10;
  // 額度用盡：停用預設風格 / 自訂套用；後端仍是最終把關者，這只是前端閘門。
  bool get _quotaReached => _aiUsageCount >= _maxAiUsage;

  // 靜態 fallback（API 失敗時使用），依分類拆兩組
  static final _fallbackStylesProduct = [
    const AiStyle(id: '1', name: 'natural', label: '新鮮自然', prompt: ''),
    const AiStyle(id: '2', name: 'warm', label: '陽光暖色', prompt: ''),
    const AiStyle(id: '3', name: 'crisp', label: '清脆鮮豔', prompt: ''),
    const AiStyle(id: '4', name: 'giftbox', label: '高級禮盒', prompt: ''),
    const AiStyle(id: '5', name: 'documentary', label: '自然寫真', prompt: ''),
    const AiStyle(id: '6', name: 'kitchen', label: '料理素材', prompt: ''),
    const AiStyle(id: '7', name: 'clean', label: '白底極淨', prompt: ''),
  ];

  static final _fallbackStylesPost = [
    const AiStyle(
      id: '9',
      name: 'post_japanese_poster',
      label: '日系排版海報風',
      prompt: '',
    ),
    const AiStyle(
      id: '10',
      name: 'post_mixed_collage',
      label: '混合媒體拼貼風',
      prompt: '',
    ),
    const AiStyle(
      id: '11',
      name: 'post_grid_minimal',
      label: '九宮格網格風',
      prompt: '',
    ),
    const AiStyle(
      id: '12',
      name: 'post_event_pixel',
      label: '活動海報風',
      prompt: '',
    ),
  ];

  List<AiStyle> get _fallbackStyles =>
      widget.category == 'post' ? _fallbackStylesPost : _fallbackStylesProduct;

  List<AiStyle> get _aiStyles => widget.aiStyles ?? _fallbackStyles;

  @override
  void initState() {
    super.initState();
    _selectedStyle = widget.selectedStyle;
    _promptController.addListener(_onPromptChanged);
  }

  void _onPromptChanged() {
    final hasText = _promptController.text.trim().isNotEmpty;
    if (hasText != _hasPromptText) {
      setState(() => _hasPromptText = hasText);
    }
  }

  @override
  void dispose() {
    _promptController.removeListener(_onPromptChanged);
    _promptController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppColors.bgPrimary,
      child: SingleChildScrollView(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            // AI 優化標題列
            Padding(
              padding: EdgeInsets.fromLTRB(16.w, 12.h, 16.w, 12.h),
              child: Row(
                children: [
                  Text(
                    'AI 優化',
                    style: TextStyle(
                      fontSize: 18.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  const Spacer(),
                  Text(
                    _quotaReached
                        ? 'AI修圖已用 $_aiUsageCount/$_maxAiUsage 已滿'
                        : 'AI修圖已用 $_aiUsageCount/$_maxAiUsage',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: _quotaReached
                          ? const Color(0xFFD32F2F)
                          : AppColors.textSecondary,
                      fontWeight: _quotaReached
                          ? FontWeight.w700
                          : FontWeight.w400,
                    ),
                  ),
                ],
              ),
            ),
            // Tab 選擇（預設風格、自訂提示詞、更改紀錄）
            Container(
              decoration: const BoxDecoration(
                border: Border(
                  bottom: BorderSide(color: AppColors.dividerLight, width: 1),
                ),
              ),
              child: Row(
                children: [
                  _buildTabButton('preset', '預設風格'),
                  _buildTabButton('custom', '自訂提示詞'),
                  _buildTabButton('history', '更改紀錄'),
                ],
              ),
            ),
            SizedBox(height: 16.h),
            // 內容區
            _selectedTab == 'custom'
                ? _buildCustomPromptContent()
                : _selectedTab == 'history'
                ? _buildHistoryContent()
                : _buildPresetContent(),
          ],
        ),
      ),
    );
  }

  Widget _buildTabButton(String tab, String label) {
    final isSelected = _selectedTab == tab;
    return Expanded(
      child: GestureDetector(
        onTap: () => setState(() => _selectedTab = tab),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Padding(
              padding: EdgeInsets.symmetric(vertical: 12.h),
              child: Text(
                label,
                style: TextStyle(
                  color: isSelected
                      ? AppColors.textEmphasis
                      : AppColors.textSecondary,
                  fontWeight: isSelected ? FontWeight.w700 : FontWeight.w400,
                  fontSize: 14.sp,
                ),
              ),
            ),
            Container(
              height: 3.h,
              width: 50.w,
              decoration: BoxDecoration(
                color: isSelected ? AppColors.primary : Colors.transparent,
                borderRadius: BorderRadius.circular(1.5.r),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPresetContent() {
    return SizedBox(
      height: 140.h,
      child: ListView.builder(
        scrollDirection: Axis.horizontal,
        padding: EdgeInsets.symmetric(horizontal: 16.w),
        itemCount: _aiStyles.length,
        itemBuilder: (context, index) {
          final style = _aiStyles[index];
          final isSelected = _selectedStyle == style.name;
          return Container(
            width: 100.w,
            margin: EdgeInsets.only(
              right: index < _aiStyles.length - 4 ? 12 : 0,
            ),
            child: GestureDetector(
              onTap: _quotaReached
                  ? null
                  : () {
                      setState(() => _selectedStyle = style.name);
                      widget.onStyleChanged(style.name);
                    },
              child: Column(
                children: [
                  Opacity(
                    opacity: _quotaReached ? 0.35 : (isSelected ? 1.0 : 0.5),
                    child: Container(
                      width: 100.w,
                      height: 100.h,
                      decoration: BoxDecoration(
                        borderRadius: BorderRadius.circular(12.r),
                        border: Border.all(
                          color: isSelected
                              ? AppColors.primary
                              : Colors.transparent,
                          width: 3.w,
                        ),
                      ),
                      child: ClipRRect(
                        borderRadius: BorderRadius.circular(8.r),
                        child: style.thumbnailKey.isNotEmpty
                            ? OptimizedNetworkImage(
                                // 裸 objectKey；OptimizedNetworkImage 帶 scene
                                // 會過 buildImageUrl（dual-mode）套縮圖 preset。
                                imageUrl: style.thumbnailKey,
                                scene: ImageScene.shareThumbnail,
                                fit: BoxFit.cover,
                                width: 100.w,
                                height: 100.h,
                                errorWidget: _buildPlaceholder(style),
                              )
                            : _buildPlaceholder(style),
                      ),
                    ),
                  ),
                  SizedBox(height: 8.h),
                  Text(
                    style.label,
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: isSelected
                          ? AppColors.primary
                          : AppColors.textSecondary,
                      fontWeight: isSelected
                          ? FontWeight.w700
                          : FontWeight.w400,
                    ),
                    textAlign: TextAlign.center,
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _buildPlaceholder(AiStyle style) {
    return Container(
      color: AppColors.ink100,
      alignment: Alignment.center,
      child: Text(
        style.label.isNotEmpty ? style.label[0] : '?',
        style: TextStyle(
          fontSize: 30.sp,
          fontWeight: FontWeight.w700,
          color: AppColors.primary,
        ),
      ),
    );
  }

  Widget _buildCustomPromptContent() {
    return Padding(
      padding: EdgeInsets.symmetric(horizontal: 16.w),
      child: Column(
        children: [
          TextField(
            controller: _promptController,
            maxLines: 3,
            style: TextStyle(color: AppColors.textEmphasis, fontSize: 14.sp),
            decoration: InputDecoration(
              hintText: '輸入自訂提示詞...',
              hintStyle: TextStyle(
                color: AppColors.textSecondary,
                fontSize: 14.sp,
              ),
              filled: true,
              fillColor: AppColors.cardBorder,
              border: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8.r),
                borderSide: BorderSide.none,
              ),
              enabledBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8.r),
                borderSide: BorderSide.none,
              ),
              focusedBorder: OutlineInputBorder(
                borderRadius: BorderRadius.circular(8.r),
                borderSide: const BorderSide(color: AppColors.primary),
              ),
              contentPadding: EdgeInsets.all(12.w),
            ),
          ),
          SizedBox(height: 12.h),
          SizedBox(
            width: double.infinity,
            height: 48.h,
            child: FilledButton(
              onPressed: (_hasPromptText && !_quotaReached)
                  ? () => widget.onApplyPrompt(_promptController.text.trim())
                  : null,
              child: const HkButtonLabel('送出提示詞'),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildHistoryContent() {
    if (_aiHistory.isEmpty) {
      return SizedBox(
        height: 120.h,
        child: Center(
          child: Text(
            '暫無更改紀錄',
            style: TextStyle(color: AppColors.textSecondary, fontSize: 14.sp),
          ),
        ),
      );
    }

    return SizedBox(
      height: 140.h,
      child: ListView.builder(
        padding: EdgeInsets.symmetric(horizontal: 16.w),
        itemCount: _aiHistory.length,
        itemBuilder: (context, index) {
          final history = _aiHistory[index];
          final action = history['action'] as String;
          final timestamp = history['timestamp'] as DateTime;
          final count = history['count'] as int;
          final timeAgo = _getTimeAgo(timestamp);

          return Padding(
            padding: EdgeInsets.symmetric(vertical: 8.h),
            child: Row(
              children: [
                Expanded(
                  child: Text(
                    '$count/$_maxAiUsage-$action',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                ),
                Text(
                  timeAgo,
                  style: TextStyle(
                    fontSize: 14.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ),
          );
        },
      ),
    );
  }

  String _getTimeAgo(DateTime timestamp) {
    final now = DateTime.now();
    final difference = now.difference(timestamp);

    if (difference.inSeconds < 60) {
      return '剛剛';
    } else if (difference.inMinutes < 60) {
      return '${difference.inMinutes} 分鐘前';
    } else if (difference.inHours < 24) {
      return '${difference.inHours} 小時前';
    } else {
      return '${difference.inDays} 天前';
    }
  }
}
