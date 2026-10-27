import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import 'image_editor_color_picker_dialog.dart';
import 'image_editor_models.dart';

/// 文字標籤頁
class ImageEditorTextTab extends StatefulWidget {
  const ImageEditorTextTab({
    super.key,
    required this.textLayers,
    required this.onAddText,
    required this.onUpdateLayer,
    required this.onDeleteLayer,
    required this.onToggleVisibility,
  });

  final List<TextLayer> textLayers;
  final ValueChanged<TextLayer> onAddText;
  final void Function(int index, TextLayer layer) onUpdateLayer;
  final ValueChanged<int> onDeleteLayer;
  final ValueChanged<int> onToggleVisibility;

  @override
  State<ImageEditorTextTab> createState() => _ImageEditorTextTabState();
}

class _ImageEditorTextTabState extends State<ImageEditorTextTab> {
  final TextEditingController _textController = TextEditingController();
  double _fontSize = 36;
  Color _selectedColor = const Color(0x80B8441F);
  TextAlign _textAlign = TextAlign.center;
  String? _selectedPreset;
  bool _hasText = false;
  String _selectedFontFamily = 'notoSansTC';

  // 字型選項（使用系統內建中文字型，不需下載）
  final List<Map<String, String>> _fontOptions = [
    {'id': 'notoSansTC', 'label': '黑體'},
    {'id': 'notoSerifTC', 'label': '宋體'},
    {'id': 'maShanZheng', 'label': '楷書'},
    {'id': 'zhiMangXing', 'label': '行書'},
    {'id': 'longCang', 'label': '報隸'},
  ];

  /// 根據字型 ID 取得對應的 TextStyle（使用系統內建字型）
  TextStyle _getFontStyle(
    String fontId, {
    double fontSize = 14,
    Color? color,
    FontWeight? fontWeight,
  }) {
    String fontFamily;
    switch (fontId) {
      case 'notoSerifTC':
        fontFamily = 'Songti TC';
        break;
      case 'maShanZheng':
        fontFamily = 'Kaiti TC';
        break;
      case 'zhiMangXing':
        fontFamily = 'STXingkai';
        break;
      case 'longCang':
        fontFamily = 'Baoli TC';
        break;
      case 'notoSansTC':
      default:
        fontFamily = 'PingFang TC';
        break;
    }
    return TextStyle(
      fontFamily: fontFamily,
      fontFamilyFallback: const [
        'PingFang TC',
        '.AppleSystemUIFont',
        'Noto Sans CJK TC',
        'Roboto',
      ],
      fontSize: fontSize,
      color: color,
      fontWeight: fontWeight,
    );
  }

  // 快速樣式
  final List<Map<String, dynamic>> _presets = [
    {'id': 'title', 'label': '標題', 'size': 36.0, 'color': AppColors.primary},
    {
      'id': 'big_title',
      'label': '大標題',
      'size': 48.0,
      'color': AppColors.warning,
    },
    {
      'id': 'small_title',
      'label': '小標題',
      'size': 24.0,
      'color': AppColors.textPrimary,
    },
    {'id': 'desc', 'label': '說明', 'size': 18.0, 'color': AppColors.textPrimary},
  ];

  static const List<Color> _colorOptions = [
    Color(0x80B8441F),
    AppColors.warning,
    AppColors.textPrimary,
    Color(0xFFFFFFFF),
    Color(0xFF000000),
    Color(0xFFE74C3C),
  ];

  @override
  void initState() {
    super.initState();
    _textController.addListener(_onTextChanged);
  }

  void _onTextChanged() {
    final hasText = _textController.text.trim().isNotEmpty;
    if (hasText != _hasText) {
      setState(() => _hasText = hasText);
    }
  }

  @override
  void dispose() {
    _textController.removeListener(_onTextChanged);
    _textController.dispose();
    super.dispose();
  }

  void _applyPreset(Map<String, dynamic> preset) {
    setState(() {
      _selectedPreset = preset['id'];
      _fontSize = preset['size'];
      _selectedColor = preset['color'];
    });
  }

  void _addTextLayer() {
    if (_textController.text.trim().isEmpty) return;

    widget.onAddText(
      TextLayer(
        text: _textController.text.trim(),
        fontSize: _fontSize,
        color: _selectedColor,
        textAlign: _textAlign,
        fontFamily: _selectedFontFamily,
      ),
    );

    _textController.clear();
  }

  Future<void> _showColorPicker() async {
    final pickedColor = await showDialog<Color>(
      context: context,
      builder: (context) => ColorPickerDialog(initialColor: _selectedColor),
    );
    if (pickedColor != null) {
      setState(() => _selectedColor = pickedColor);
    }
  }

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
                    '新增文字',
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                ],
              ),
            ),
            // 文字輸入框
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              child: Container(
                decoration: BoxDecoration(
                  color: AppColors.cardBorder,
                  borderRadius: BorderRadius.circular(8.r),
                ),
                child: TextField(
                  controller: _textController,
                  maxLines: 3,
                  decoration: InputDecoration(
                    hintText: '輸入文字...',
                    hintStyle: TextStyle(
                      color: AppColors.textSecondary,
                      fontSize: 16.sp,
                    ),
                    border: InputBorder.none,
                    contentPadding: EdgeInsets.all(16.w),
                  ),
                ),
              ),
            ),
            SizedBox(height: 12.h),
            // 新增到畫面按鈕
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              child: SizedBox(
                width: double.infinity,
                height: 48.h,
                child: FilledButton(
                  onPressed: _hasText ? _addTextLayer : null,
                  child: const HkButtonLabel('新增到畫面'),
                ),
              ),
            ),
            SizedBox(height: 16.h),
            // 快速樣式
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              child: Text(
                '快速樣式',
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textSecondary,
                ),
              ),
            ),
            SizedBox(height: 8.h),
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              child: Column(
                children: [
                  // 第一行：標題、大標題
                  Row(
                    children: [
                      Expanded(
                        child: _buildPresetButton(_presets[0], height: 50.h),
                      ),
                      SizedBox(width: 12.w),
                      Expanded(
                        child: _buildPresetButton(_presets[1], height: 60.h),
                      ),
                    ],
                  ),
                  SizedBox(height: 12.h),
                  // 第二行：小標題、說明
                  Row(
                    children: [
                      Expanded(
                        child: _buildPresetButton(_presets[2], height: 40.h),
                      ),
                      SizedBox(width: 12.w),
                      Expanded(
                        child: _buildPresetButton(_presets[3], height: 36.h),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            SizedBox(height: 16.h),
            // 字型選擇
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '字型',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                  SizedBox(height: 8.h),
                  SizedBox(
                    height: 36.h,
                    child: ListView.separated(
                      scrollDirection: Axis.horizontal,
                      itemCount: _fontOptions.length,
                      separatorBuilder: (_, __) => SizedBox(width: 8.w),
                      itemBuilder: (context, index) {
                        final font = _fontOptions[index];
                        final isSelected = _selectedFontFamily == font['id'];
                        return GestureDetector(
                          onTap: () =>
                              setState(() => _selectedFontFamily = font['id']!),
                          child: Container(
                            padding: EdgeInsets.symmetric(horizontal: 16.w),
                            decoration: BoxDecoration(
                              color: isSelected
                                  ? AppColors.primary
                                  : AppColors.gray100,
                              borderRadius: BorderRadius.circular(16.r),
                              border: Border.all(
                                color: isSelected
                                    ? AppColors.primary
                                    : AppColors.dividerLight,
                                width: 1,
                              ),
                            ),
                            child: Center(
                              child: Text(
                                font['label']!,
                                style: _getFontStyle(
                                  font['id']!,
                                  fontSize: 14.sp,
                                  color: isSelected
                                      ? Colors.white
                                      : AppColors.textSecondary,
                                  fontWeight: isSelected
                                      ? FontWeight.w700
                                      : FontWeight.normal,
                                ),
                              ),
                            ),
                          ),
                        );
                      },
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: 12.h),
            // 字體大小
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              child: Row(
                children: [
                  Text(
                    '字體大小',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                  Expanded(
                    child: SliderTheme(
                      data: SliderThemeData(
                        activeTrackColor: AppColors.primary,
                        inactiveTrackColor: AppColors.dividerLight,
                        thumbColor: AppColors.primary,
                        overlayColor: AppColors.primary.withValues(alpha: 0.2),
                        trackHeight: 4,
                        thumbShape: const RoundSliderThumbShape(
                          enabledThumbRadius: 8,
                        ),
                      ),
                      child: Slider(
                        value: _fontSize,
                        min: 12,
                        max: 72,
                        onChanged: (v) => setState(() => _fontSize = v),
                      ),
                    ),
                  ),
                  Text(
                    '${_fontSize.toInt()}px',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.primary,
                      fontWeight: FontWeight.w500,
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: 12.h),
            // 顏色
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '顏色',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                  SizedBox(height: 8.h),
                  Row(
                    children: [
                      ..._colorOptions.map(
                        (color) => _ColorSwatch(
                          color: color,
                          isSelected: _selectedColor == color,
                          onTap: () => setState(() => _selectedColor = color),
                        ),
                      ),
                      // 彩虹圖示 - 自訂顏色選擇器
                      GestureDetector(
                        onTap: _showColorPicker,
                        child: Container(
                          width: 36.w,
                          height: 36.h,
                          decoration: BoxDecoration(
                            shape: BoxShape.circle,
                            gradient: const SweepGradient(
                              colors: [
                                Color(0xFFFF0000),
                                Color(0xFFFF7F00),
                                Color(0xFFFFFF00),
                                Color(0xFF00FF00),
                                Color(0xFF0000FF),
                                Color(0xFF4B0082),
                                Color(0xFF9400D3),
                                Color(0xFFFF0000),
                              ],
                            ),
                            border: Border.all(
                              color: AppColors.dividerLight,
                              width: 2.w,
                            ),
                          ),
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
            SizedBox(height: 12.h),
            // 對齊方式
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 16.w),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    '對齊方式',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                  SizedBox(height: 8.h),
                  Row(
                    children: [
                      _buildAlignButton(
                        TextAlign.left,
                        AppIcons.formatAlignLeft,
                      ),
                      SizedBox(width: 12.w),
                      _buildAlignButton(
                        TextAlign.center,
                        AppIcons.formatAlignCenter,
                      ),
                      SizedBox(width: 12.w),
                      _buildAlignButton(
                        TextAlign.right,
                        AppIcons.formatAlignRight,
                      ),
                    ],
                  ),
                ],
              ),
            ),
            SizedBox(height: 16.h),
          ],
        ),
      ),
    );
  }

  Widget _buildPresetButton(
    Map<String, dynamic> preset, {
    required double height,
  }) {
    final isSelected = _selectedPreset == preset['id'];
    final double displayFontSize = (preset['size'] as double) * 0.5; // 縮小顯示
    final Color presetColor = preset['color'] as Color;
    return GestureDetector(
      onTap: () => _applyPreset(preset),
      child: Container(
        height: height,
        decoration: BoxDecoration(
          color: isSelected ? Colors.white : AppColors.gray100,
          borderRadius: BorderRadius.circular(8.r),
          border: Border.all(
            color: isSelected ? AppColors.primary : Colors.transparent,
            width: 2.w,
          ),
        ),
        child: Center(
          child: Text(
            preset['label'],
            style: TextStyle(
              fontSize: displayFontSize,
              fontWeight: FontWeight.w500,
              color: presetColor,
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildAlignButton(TextAlign align, IconData icon) {
    final isSelected = _textAlign == align;
    return Expanded(
      child: GestureDetector(
        onTap: () => setState(() => _textAlign = align),
        child: Container(
          padding: EdgeInsets.symmetric(vertical: 12.h),
          decoration: BoxDecoration(
            color: isSelected ? AppColors.primary : AppColors.gray100,
            borderRadius: BorderRadius.circular(8.r),
            border: Border.all(
              color: isSelected ? AppColors.primary : Colors.transparent,
              width: 2.w,
            ),
          ),
          child: Center(
            child: Icon(
              icon,
              size: 22.sp,
              color: isSelected ? AppIconColors.onBrand : AppIconColors.muted,
            ),
          ),
        ),
      ),
    );
  }
}

class _ColorSwatch extends StatelessWidget {
  const _ColorSwatch({
    required this.color,
    required this.isSelected,
    required this.onTap,
  });

  final Color color;
  final bool isSelected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final borderColor = Color.fromARGB(
      (color.a * 255).round() & 0xff,
      ((color.r * 255 * 0.7).round()).clamp(0, 255),
      ((color.g * 255 * 0.7).round()).clamp(0, 255),
      ((color.b * 255 * 0.7).round()).clamp(0, 255),
    );
    final checkColor =
        Color.alphaBlend(color, AppColors.neutralWhite).computeLuminance() > 0.5
        ? AppColors.brandInk
        : AppColors.neutralWhite;
    return Semantics(
      button: true,
      selected: isSelected,
      child: GestureDetector(
        onTap: onTap,
        child: Container(
          margin: EdgeInsets.only(right: 12.w),
          width: 36.w,
          height: 36.h,
          padding: EdgeInsets.all(isSelected ? 4.w : 0),
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: isSelected ? borderColor.withValues(alpha: 0.35) : null,
          ),
          child: DecoratedBox(
            decoration: BoxDecoration(
              color: color,
              shape: BoxShape.circle,
              border: Border.all(color: borderColor, width: 2.w),
            ),
            child: isSelected
                ? Center(
                    child: ExcludeSemantics(
                      child: Icon(
                        AppIcons.check,
                        size: 16.w,
                        color: checkColor,
                      ),
                    ),
                  )
                : null,
          ),
        ),
      ),
    );
  }
}
