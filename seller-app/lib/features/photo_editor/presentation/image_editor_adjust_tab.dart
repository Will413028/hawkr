import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// 調整標籤頁（亮度、對比度、飽和度等）
class ImageEditorAdjustTab extends StatelessWidget {
  const ImageEditorAdjustTab({
    super.key,
    required this.brightness,
    required this.contrast,
    required this.saturation,
    required this.temperature,
    required this.blur,
    required this.onBrightnessChanged,
    required this.onContrastChanged,
    required this.onSaturationChanged,
    required this.onTemperatureChanged,
    required this.onBlurChanged,
    required this.onReset,
    required this.onApply,
  });

  final double brightness;
  final double contrast;
  final double saturation;
  final double temperature;
  final double blur;
  final ValueChanged<double> onBrightnessChanged;
  final ValueChanged<double> onContrastChanged;
  final ValueChanged<double> onSaturationChanged;
  final ValueChanged<double> onTemperatureChanged;
  final ValueChanged<double> onBlurChanged;
  final VoidCallback onReset;
  final VoidCallback onApply;

  @override
  Widget build(BuildContext context) {
    return Container(
      color: AppColors.bgPrimary,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          // 標題列
          Padding(
            padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 12.h),
            child: Row(
              children: [
                Text(
                  '調整參數',
                  style: TextStyle(
                    fontSize: 16.sp,
                    fontWeight: FontWeight.w700,
                    color: AppColors.textEmphasis,
                  ),
                ),
              ],
            ),
          ),
          // 滑桿列表
          Padding(
            padding: EdgeInsets.symmetric(horizontal: 16.w),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                _buildSliderWithImage(
                  icon: AppIcons.lightModeOutlined,
                  label: '亮度',
                  value: brightness,
                  onChanged: onBrightnessChanged,
                ),
                _buildSliderWithImage(
                  icon: AppIcons.contrast,
                  label: '對比度',
                  value: contrast,
                  onChanged: onContrastChanged,
                ),
                _buildSliderWithImage(
                  icon: AppIcons.waterDropOutlined,
                  label: '飽和度',
                  value: saturation,
                  onChanged: onSaturationChanged,
                ),
                _buildSliderWithImage(
                  icon: AppIcons.thermostatOutlined,
                  label: '色溫',
                  value: temperature,
                  onChanged: onTemperatureChanged,
                ),
                SizedBox(height: 12.h),
                // 調整小技巧
                Container(
                  padding: EdgeInsets.all(12.w),
                  decoration: BoxDecoration(
                    color: AppColors.bgPrimary,
                    borderRadius: BorderRadius.circular(8.r),
                  ),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      const HkIcon(
                        HkIconName.info,
                        size: 20,
                        color: AppIconColors.active,
                        semanticLabel: '調整提示',
                      ),
                      SizedBox(width: 8.w),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              '調整小技巧',
                              style: TextStyle(
                                fontSize: 14.sp,
                                fontWeight: FontWeight.w700,
                                color: AppColors.primary,
                              ),
                            ),
                            SizedBox(height: 4.h),
                            Text(
                              '提高飽和度讓顏色更鮮豔，增加亮度讓產品更明亮誘人。適度的對比度能突顯產品細節。',
                              style: TextStyle(
                                fontSize: 12.sp,
                                color: AppColors.textSecondary,
                                height: 1.4,
                              ),
                            ),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildSliderWithImage({
    required IconData icon,
    required String label,
    required double value,
    required ValueChanged<double> onChanged,
    double min = -100,
    double max = 100,
  }) {
    return Padding(
      padding: EdgeInsets.symmetric(vertical: 8.h),
      child: Row(
        children: [
          ExcludeSemantics(
            child: Icon(icon, size: 20.w, color: AppIconColors.muted),
          ),
          SizedBox(width: 12.w),
          Text(
            label,
            style: TextStyle(fontSize: 14.sp, color: AppColors.textEmphasis),
          ),
          Expanded(
            child: SliderTheme(
              data: SliderThemeData(
                activeTrackColor: AppColors.primary,
                inactiveTrackColor: AppColors.dividerLight,
                thumbColor: AppColors.primary,
                overlayColor: AppColors.primary.withValues(alpha: 0.2),
                trackHeight: 4,
                thumbShape: const RoundSliderThumbShape(enabledThumbRadius: 8),
              ),
              child: Slider(
                value: value,
                min: min,
                max: max,
                onChanged: onChanged,
              ),
            ),
          ),
          SizedBox(
            width: 40.w,
            child: Text(
              value >= 0
                  ? '+${value.toStringAsFixed(0)}'
                  : value.toStringAsFixed(0),
              textAlign: TextAlign.right,
              style: TextStyle(
                fontSize: 14.sp,
                color: AppColors.primary,
                fontWeight: FontWeight.w500,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
