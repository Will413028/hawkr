import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:image_picker/image_picker.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/dialog_close_button.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../data/order_repository.dart';

/// Figma node 5618:51049 — 預約黑貓上門取件 dialog
///
/// 從 GET /app/shipping_business_days 取得可選日期，
/// 回傳 TcatReservationResult（shipDate + photo）供呼叫端使用。
class TcatReservationDialog extends ConsumerStatefulWidget {
  const TcatReservationDialog({super.key});

  @override
  ConsumerState<TcatReservationDialog> createState() =>
      _TcatReservationDialogState();
}

class _TcatReservationDialogState extends ConsumerState<TcatReservationDialog> {
  List<BusinessDay>? _businessDays;
  bool _loadingDays = true;
  String? _loadError;

  int _selectedDateIndex = 0;

  static const _timeSlots = ['13時前', '14-18時', '不指定'];
  int _selectedTimeIndex = 0;

  File? _photo;
  final _picker = ImagePicker();
  bool _submitting = false;

  static const _green = AppColors.primary;
  static const _gray100 = AppColors.textPrimary;
  static const _bgApp = AppColors.bgPrimary;
  static const _borderDefault = AppColors.cardBorder;

  @override
  void initState() {
    super.initState();
    _fetchBusinessDays();
  }

  Future<void> _fetchBusinessDays() async {
    try {
      final days = await ref
          .read(orderRepositoryProvider)
          .fetchShippingBusinessDays();
      if (!mounted) return;
      setState(() {
        _businessDays = days;
        _loadingDays = false;
        if (days.isEmpty) _loadError = '無可選出貨日期';
      });
    } catch (e) {
      if (!mounted) return;
      setState(() {
        _loadingDays = false;
        _loadError = '載入日期失敗';
      });
    }
  }

  Future<void> _pickPhoto() async {
    // iOS image_picker 對某些 iCloud 還沒下載完的 PNG/HEIC 會拋
    // PlatformException(invalid_image, Cannot load representation of type public.png)。
    // 包 try-catch + imageQuality 降低觸發機率 + requestFullMetadata=false 避免 PHAsset 全載。
    try {
      final picked = await _picker.pickImage(
        source: ImageSource.gallery,
        maxWidth: 1200,
        imageQuality: 85,
        requestFullMetadata: false,
      );
      if (picked != null && mounted) {
        setState(() => _photo = File(picked.path));
      }
    } on PlatformException catch (e) {
      if (!mounted) return;
      final friendly = e.code == 'invalid_image'
          ? '無法讀取此照片（可能還在 iCloud 上未下載），請改選其他張'
          : '選取照片失敗：${e.message ?? e.code}';
      FloatingToast.error(context, friendly);
    } catch (e) {
      if (!mounted) return;
      FloatingToast.error(context, '選取照片失敗：$e');
    }
  }

  void _submit() {
    if (_submitting) return;
    final days = _businessDays;
    if (days == null || days.isEmpty) return;

    setState(() => _submitting = true);

    final selectedDay = days[_selectedDateIndex];
    // 轉為 yyyyMMdd 格式
    final shipDate = selectedDay.date.replaceAll('-', '');

    Navigator.pop(
      context,
      TcatReservationResult(
        shipDate: shipDate,
        timeSlot: _timeSlots[_selectedTimeIndex],
        photo: _photo,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Material(
        color: Colors.transparent,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Container(
              width: 340.w,
              padding: EdgeInsets.all(20.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16.r),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  // 黑貓 logo
                  Container(
                    width: 45.w,
                    height: 45.w,
                    decoration: BoxDecoration(
                      color: AppColors.bgCard,
                      borderRadius: BorderRadius.circular(12.r),
                      border: Border.all(color: _gray100),
                    ),
                    child: Center(
                      child: HkIcon(
                        HkIconName.truck,
                        size: 28.sp,
                        color: AppIconColors.defaultColor,
                      ),
                    ),
                  ),
                  SizedBox(height: 12.h),

                  // Title
                  Text(
                    '預約黑貓上門取件',
                    style: TextStyle(
                      fontSize: 18.sp,
                      fontWeight: FontWeight.w700,
                      height: 1.23,
                      color: _gray100,
                    ),
                  ),
                  SizedBox(height: 12.h),
                  Text(
                    '請選擇期望的取件時間並上傳產品照片',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: _gray100.withValues(alpha: 0.8),
                    ),
                  ),
                  SizedBox(height: 12.h),

                  // ── 期望取件日期 ──
                  _sectionLabel(HkIconName.calendar, '期望取件日期'),
                  SizedBox(height: 10.h),
                  _buildDatePicker(),
                  SizedBox(height: 12.h),

                  // ── 期望取件時段 ──
                  _sectionLabel(HkIconName.clock, '期望取件時段'),
                  SizedBox(height: 10.h),
                  Row(
                    children: List.generate(_timeSlots.length, (i) {
                      final selected = _selectedTimeIndex == i;
                      return Padding(
                        padding: EdgeInsets.only(
                          right: i < _timeSlots.length - 4 ? 12.w : 0,
                        ),
                        child: GestureDetector(
                          onTap: () => setState(() => _selectedTimeIndex = i),
                          child: Container(
                            width: 80.w,
                            height: 36.h,
                            alignment: Alignment.center,
                            decoration: BoxDecoration(
                              color: selected ? _bgApp : AppColors.bgCard,
                              borderRadius: BorderRadius.circular(8.r),
                              border: Border.all(
                                color: selected ? _green : _borderDefault,
                                width: selected ? 2 : 1,
                              ),
                            ),
                            child: Text(
                              _timeSlots[i],
                              style: TextStyle(
                                fontSize: 14.sp,
                                fontWeight: selected
                                    ? FontWeight.w700
                                    : FontWeight.w500,
                                color: selected
                                    ? AppColors.textEmphasis
                                    : AppColors.textEmphasis,
                              ),
                            ),
                          ),
                        ),
                      );
                    }),
                  ),
                  SizedBox(height: 12.h),

                  // ── 上傳照片 ──
                  Semantics(
                    button: true,
                    label: '上傳產品或包裝照片',
                    child: GestureDetector(
                      onTap: _pickPhoto,
                      child: Container(
                        width: double.infinity,
                        height: 120.h,
                        decoration: BoxDecoration(
                          borderRadius: BorderRadius.circular(16.r),
                          border: Border.all(
                            color: _gray100.withValues(alpha: 0.5),
                          ),
                        ),
                        // BoxFit.contain — 高度 fit 父容器、寬度按比例縮，
                        // 寬螢幕橫圖左右會留白、不會裁切原圖。
                        child: ClipRRect(
                          borderRadius: BorderRadius.circular(16.r),
                          child: _photo != null
                              ? Center(
                                  child: Image.file(
                                    _photo!,
                                    fit: BoxFit.contain,
                                    height: double.infinity,
                                  ),
                                )
                              : Column(
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    HkIcon(
                                      HkIconName.photoAdd,
                                      size: 32.sp,
                                      color: _gray100.withValues(alpha: 0.4),
                                    ),
                                    SizedBox(height: 8.h),
                                    Text(
                                      '上傳產品或是包裝照片',
                                      style: TextStyle(
                                        fontSize: 14.sp,
                                        color: _gray100.withValues(alpha: 0.5),
                                      ),
                                    ),
                                  ],
                                ),
                        ),
                      ),
                    ),
                  ),
                  SizedBox(height: 20.h),

                  // ── 預約出貨按鈕 ──
                  Semantics(
                    button: true,
                    enabled:
                        _businessDays != null &&
                        _businessDays!.isNotEmpty &&
                        !_submitting,
                    label: '預約出貨',
                    child: GestureDetector(
                      onTap:
                          (_businessDays != null &&
                              _businessDays!.isNotEmpty &&
                              !_submitting)
                          ? _submit
                          : null,
                      child: Opacity(
                        opacity:
                            (_businessDays != null && _businessDays!.isNotEmpty)
                            ? 1.0
                            : 0.5,
                        child: Container(
                          width: double.infinity,
                          constraints: BoxConstraints(minHeight: 48.h),
                          alignment: Alignment.center,
                          decoration: BoxDecoration(
                            color: _green,
                            borderRadius: BorderRadius.circular(999),
                          ),
                          child: _submitting
                              ? Center(
                                  child: SizedBox(
                                    width: 20.w,
                                    height: 20.w,
                                    child: const CircularProgressIndicator(
                                      strokeWidth: 2,
                                      color: AppColors.bgCard,
                                    ),
                                  ),
                                )
                              : Text(
                                  '預約出貨',
                                  textAlign: TextAlign.center,
                                  style: TextStyle(
                                    fontSize: 16.sp,
                                    fontWeight: FontWeight.w700,
                                    color: AppColors.bgCard,
                                  ),
                                ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
            // X close button
            Positioned(
              top: -8.h,
              right: -8.w,
              child: const DialogCloseButton(
                visualAlignment: Alignment.topRight,
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildDatePicker() {
    if (_loadingDays) {
      return SizedBox(
        height: 60.h,
        child: const Center(
          child: CircularProgressIndicator(strokeWidth: 2, color: _green),
        ),
      );
    }
    if (_loadError != null || _businessDays == null || _businessDays!.isEmpty) {
      return SizedBox(
        height: 60.h,
        child: Center(
          child: Text(
            _loadError ?? '無可選日期',
            style: TextStyle(fontSize: 14.sp, color: _gray100),
          ),
        ),
      );
    }

    final days = _businessDays!;
    return SizedBox(
      height: 60.h,
      child: ListView.builder(
        scrollDirection: Axis.horizontal,
        itemCount: days.length,
        itemBuilder: (_, i) {
          final day = days[i];
          final dt = day.toDateTime();
          final selected = _selectedDateIndex == i;
          final dayLabel = '週${day.dayOfWeek}';
          final dateLabel = dt != null ? '${dt.month}/${dt.day}' : day.date;

          return GestureDetector(
            onTap: () => setState(() => _selectedDateIndex = i),
            child: Container(
              width: 70.w,
              margin: EdgeInsets.only(right: 8.w),
              decoration: BoxDecoration(
                color: selected ? _bgApp : AppColors.bgCard,
                borderRadius: BorderRadius.circular(8.r),
                border: Border.all(
                  color: selected ? _green : _borderDefault,
                  width: selected ? 2 : 1,
                ),
              ),
              child: Column(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  Text(
                    dayLabel,
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                      color: selected ? AppColors.textEmphasis : _gray100,
                    ),
                  ),
                  Text(
                    dateLabel,
                    style: TextStyle(
                      fontSize: 14.sp,
                      fontWeight: selected ? FontWeight.w700 : FontWeight.w500,
                      color: selected ? AppColors.textEmphasis : _gray100,
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }

  Widget _sectionLabel(HkIconName icon, String text) {
    return Align(
      alignment: Alignment.centerLeft,
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          HkIcon(icon, size: 20.sp, color: _gray100),
          SizedBox(width: 6.w),
          Text(
            text,
            style: TextStyle(fontSize: 14.sp, color: _gray100),
          ),
        ],
      ),
    );
  }
}

/// Dialog 回傳結果
class TcatReservationResult {
  const TcatReservationResult({
    required this.shipDate,
    required this.timeSlot,
    this.photo,
  });

  /// 出貨日期（yyyyMMdd）
  final String shipDate;

  /// 取件時段
  final String timeSlot;

  /// 產品照片（選填）
  final File? photo;
}
