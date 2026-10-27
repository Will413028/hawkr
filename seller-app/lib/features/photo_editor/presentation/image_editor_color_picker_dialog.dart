import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import '../../../shared/widgets/dialog_close_button.dart';

/// HSV 色彩選擇器對話框
class ColorPickerDialog extends StatefulWidget {
  const ColorPickerDialog({super.key, required this.initialColor});

  final Color initialColor;

  @override
  State<ColorPickerDialog> createState() => _ColorPickerDialogState();
}

class _ColorPickerDialogState extends State<ColorPickerDialog> {
  late double _hue;
  late double _saturation;
  late double _brightness;

  @override
  void initState() {
    super.initState();
    final hsv = HSVColor.fromColor(widget.initialColor);
    _hue = hsv.hue;
    _saturation = hsv.saturation;
    _brightness = hsv.value;
  }

  Color get _currentColor =>
      HSVColor.fromAHSV(1.0, _hue, _saturation, _brightness).toColor();

  @override
  Widget build(BuildContext context) {
    return Dialog(
      backgroundColor: Colors.transparent,
      insetPadding: EdgeInsets.symmetric(horizontal: 24.w, vertical: 40.h),
      child: Container(
        decoration: BoxDecoration(
          color: Colors.white,
          borderRadius: BorderRadius.circular(16.r),
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            // 標題列
            Padding(
              padding: EdgeInsets.fromLTRB(20.w, 16.h, 12.w, 12.h),
              child: Row(
                children: [
                  Text(
                    '選擇顏色',
                    style: TextStyle(
                      fontSize: 18.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  const Spacer(),
                  const DialogCloseButton(
                    size: 28,
                    iconSize: 18,
                    visualAlignment: Alignment.centerRight,
                  ),
                ],
              ),
            ),
            // 色板標籤（單色）
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 20.w),
              child: Row(
                children: [
                  Container(
                    padding: EdgeInsets.symmetric(
                      horizontal: 16.w,
                      vertical: 8.h,
                    ),
                    decoration: BoxDecoration(
                      color: AppColors.primary,
                      borderRadius: BorderRadius.circular(24.r),
                    ),
                    child: Text(
                      '單色',
                      style: TextStyle(
                        color: Colors.white,
                        fontSize: 14.sp,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: 16.h),
            // 飽和度/亮度選擇區域
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 20.w),
              child: _SaturationBrightnessPicker(
                hue: _hue,
                saturation: _saturation,
                brightness: _brightness,
                onChanged: (sat, bright) {
                  setState(() {
                    _saturation = sat;
                    _brightness = bright;
                  });
                },
              ),
            ),
            SizedBox(height: 16.h),
            // 色相滑桿
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 20.w),
              child: _HueSlider(
                hue: _hue,
                onChanged: (h) => setState(() => _hue = h),
              ),
            ),
            SizedBox(height: 16.h),
            // 當前顏色預覽
            Padding(
              padding: EdgeInsets.symmetric(horizontal: 20.w),
              child: Row(
                children: [
                  Text(
                    '選擇的顏色',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                  const Spacer(),
                  Container(
                    width: 48.w,
                    height: 48.h,
                    decoration: BoxDecoration(
                      color: _currentColor,
                      borderRadius: BorderRadius.circular(8.r),
                      border: Border.all(
                        color: AppColors.dividerLight,
                        width: 2.w,
                      ),
                    ),
                  ),
                ],
              ),
            ),
            SizedBox(height: 20.h),
            // 確認按鈕
            Padding(
              padding: EdgeInsets.fromLTRB(20.w, 0, 20.w, 20.h),
              child: SizedBox(
                width: double.infinity,
                height: 48.h,
                child: ElevatedButton(
                  onPressed: () => Navigator.pop(context, _currentColor),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.primary,
                    foregroundColor: Colors.white,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(24.r),
                    ),
                    elevation: 0,
                  ),
                  child: HkButtonLabel(
                    '確認',
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// 飽和度/亮度選擇器（方形色板）
class _SaturationBrightnessPicker extends StatelessWidget {
  const _SaturationBrightnessPicker({
    required this.hue,
    required this.saturation,
    required this.brightness,
    required this.onChanged,
  });

  final double hue;
  final double saturation;
  final double brightness;
  final void Function(double saturation, double brightness) onChanged;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final size = constraints.maxWidth;
        return GestureDetector(
          onPanDown: (details) =>
              _updateFromPosition(details.localPosition, size),
          onPanUpdate: (details) =>
              _updateFromPosition(details.localPosition, size),
          child: SizedBox(
            width: size,
            height: size,
            child: CustomPaint(
              painter: _SaturationBrightnessPainter(hue: hue),
              child: Stack(
                children: [
                  Positioned(
                    left: saturation * size - 12,
                    top: (1 - brightness) * size - 12,
                    child: Container(
                      width: 24.w,
                      height: 24.h,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        border: Border.all(color: Colors.white, width: 3.w),
                        boxShadow: [
                          BoxShadow(
                            color: Colors.black.withValues(alpha: 0.3),
                            blurRadius: 4,
                            offset: Offset(0, 2.h),
                          ),
                        ],
                      ),
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

  void _updateFromPosition(Offset position, double size) {
    final sat = (position.dx / size).clamp(0.0, 1.0);
    final bright = (1 - position.dy / size).clamp(0.0, 1.0);
    onChanged(sat, bright);
  }
}

class _SaturationBrightnessPainter extends CustomPainter {
  _SaturationBrightnessPainter({required this.hue});

  final double hue;

  @override
  void paint(Canvas canvas, Size size) {
    final rect = Rect.fromLTWH(0, 0, size.width, size.height);
    final rrect = RRect.fromRectAndRadius(rect, Radius.circular(12.r));

    // 裁剪為圓角矩形
    canvas.save();
    canvas.clipRRect(rrect);

    // 繪製純色背景
    final baseColor = HSVColor.fromAHSV(1.0, hue, 1.0, 1.0).toColor();
    canvas.drawRect(rect, Paint()..color = baseColor);

    // 繪製水平白色漸層（從白到透明）
    canvas.drawRect(
      rect,
      Paint()
        ..shader = const LinearGradient(
          colors: [Colors.white, Colors.transparent],
          begin: Alignment.centerLeft,
          end: Alignment.centerRight,
        ).createShader(rect),
    );

    // 繪製垂直黑色漸層（從透明到黑）
    canvas.drawRect(
      rect,
      Paint()
        ..shader = const LinearGradient(
          colors: [Colors.transparent, Colors.black],
          begin: Alignment.topCenter,
          end: Alignment.bottomCenter,
        ).createShader(rect),
    );

    canvas.restore();

    // 繪製邊框
    canvas.drawRRect(
      rrect,
      Paint()
        ..style = PaintingStyle.stroke
        ..color = AppColors.dividerLight
        ..strokeWidth = 1,
    );
  }

  @override
  bool shouldRepaint(covariant _SaturationBrightnessPainter oldDelegate) {
    return oldDelegate.hue != hue;
  }
}

/// 色相滑桿
class _HueSlider extends StatelessWidget {
  const _HueSlider({required this.hue, required this.onChanged});

  final double hue;
  final ValueChanged<double> onChanged;

  @override
  Widget build(BuildContext context) {
    return LayoutBuilder(
      builder: (context, constraints) {
        final width = constraints.maxWidth;
        return GestureDetector(
          onPanDown: (details) =>
              _updateFromPosition(details.localPosition.dx, width),
          onPanUpdate: (details) =>
              _updateFromPosition(details.localPosition.dx, width),
          child: SizedBox(
            width: width,
            height: 28.h,
            child: CustomPaint(
              painter: _HueSliderPainter(),
              child: Stack(
                children: [
                  Positioned(
                    left: (hue / 360) * width - 10,
                    top: 0,
                    child: Container(
                      width: 20.w,
                      height: 28.h,
                      decoration: BoxDecoration(
                        borderRadius: BorderRadius.circular(6.r),
                        border: Border.all(color: Colors.white, width: 3.w),
                        boxShadow: [
                          BoxShadow(
                            color: Colors.black.withValues(alpha: 0.3),
                            blurRadius: 4,
                            offset: Offset(0, 2.h),
                          ),
                        ],
                      ),
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

  void _updateFromPosition(double x, double width) {
    final h = ((x / width) * 360).clamp(0.0, 360.0);
    onChanged(h);
  }
}

class _HueSliderPainter extends CustomPainter {
  @override
  void paint(Canvas canvas, Size size) {
    final rect = Rect.fromLTWH(0, 0, size.width, size.height);
    final rrect = RRect.fromRectAndRadius(rect, Radius.circular(16.r));

    // 繪製彩虹漸層
    final colors = List.generate(
      7,
      (i) => HSVColor.fromAHSV(1.0, i * 60.0, 1.0, 1.0).toColor(),
    );

    canvas.drawRRect(
      rrect,
      Paint()..shader = LinearGradient(colors: colors).createShader(rect),
    );

    // 繪製邊框
    canvas.drawRRect(
      rrect,
      Paint()
        ..style = PaintingStyle.stroke
        ..color = AppColors.dividerLight
        ..strokeWidth = 1,
    );
  }

  @override
  bool shouldRepaint(covariant CustomPainter oldDelegate) => false;
}
