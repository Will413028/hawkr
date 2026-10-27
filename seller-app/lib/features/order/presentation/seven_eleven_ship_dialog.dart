import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'dart:io';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_icon.dart';
import 'package:image_picker/image_picker.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/dialog_close_button.dart';

/// Figma node 4820:42202 — 711 預約出貨 dialog
/// 顯示交貨便代碼（odno + validationNo）+ 上傳照片
class SevenElevenShipDialog extends StatefulWidget {
  const SevenElevenShipDialog({
    super.key,
    required this.odno,
    required this.validationNo,
  });

  /// 出貨單編號（8 碼）
  final String odno;

  /// C2C 驗證碼（4 碼）
  final String validationNo;

  @override
  State<SevenElevenShipDialog> createState() => _SevenElevenShipDialogState();
}

class _SevenElevenShipDialogState extends State<SevenElevenShipDialog> {
  File? _photo;
  final _picker = ImagePicker();

  static const _green = AppColors.primary;
  static const _gray100 = AppColors.textPrimary;

  Future<void> _pickPhoto() async {
    // iOS image_picker 對某些 iCloud 還沒下載完的 PNG/HEIC 會拋
    // PlatformException(invalid_image, ...) — 包 try-catch 給友善提示。
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

  @override
  Widget build(BuildContext context) {
    final displayCode = '${widget.odno}${widget.validationNo}';

    return Center(
      child: Material(
        color: Colors.transparent,
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Container(
              width: 304.w,
              padding: EdgeInsets.all(20.w),
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16.r),
              ),
              child: Column(
                mainAxisSize: MainAxisSize.min,
                children: [
                  SizedBox.square(
                    dimension: 45.w,
                    child: Center(
                      child: HkIcon(
                        HkIconName.store,
                        size: 28.sp,
                        color: AppIconColors.defaultColor,
                      ),
                    ),
                  ),
                  SizedBox(height: 12.h),

                  // 交貨便代碼
                  Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      HkIcon(HkIconName.document, size: 20.sp, color: _gray100),
                      SizedBox(width: 6.w),
                      Text(
                        '交貨便代碼：$displayCode',
                        style: TextStyle(fontSize: 14.sp, color: _gray100),
                      ),
                    ],
                  ),
                  SizedBox(height: 12.h),

                  // 分隔線
                  Divider(color: _gray100.withValues(alpha: 0.2), height: 1),
                  SizedBox(height: 12.h),

                  // 標題
                  Text(
                    '上傳產品的照片且出貨',
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                  SizedBox(height: 12.h),

                  // 照片上傳區
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

                  // 兩個按鈕
                  Row(
                    children: [
                      // 送出照片 (outline green, enabled only when photo selected)
                      Expanded(
                        child: GestureDetector(
                          onTap: _photo != null
                              ? () => Navigator.pop(
                                  context,
                                  SevenElevenShipResult(photo: _photo),
                                )
                              : null,
                          child: Opacity(
                            opacity: _photo != null ? 1.0 : 0.5,
                            child: Container(
                              padding: EdgeInsets.symmetric(vertical: 8.h),
                              decoration: BoxDecoration(
                                borderRadius: BorderRadius.circular(999),
                                border: Border.all(color: _green, width: 2),
                              ),
                              child: Text(
                                '送出照片',
                                textAlign: TextAlign.center,
                                style: TextStyle(
                                  fontSize: 16.sp,
                                  fontWeight: FontWeight.w700,
                                  color: _green,
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                      SizedBox(width: 12.w),
                      // 出貨且略過 (filled green) — 確認出貨但不上傳照片
                      Expanded(
                        child: GestureDetector(
                          onTap: () => Navigator.pop(
                            context,
                            const SevenElevenShipResult(photo: null),
                          ),
                          child: Container(
                            padding: EdgeInsets.symmetric(vertical: 8.h),
                            decoration: BoxDecoration(
                              color: _green,
                              borderRadius: BorderRadius.circular(999),
                            ),
                            child: Text(
                              '出貨且略過',
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
                    ],
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
}

/// Dialog 回傳結果
/// null = X 關閉（不做任何事）
/// non-null = 確認出貨（photo 可能為 null 表示略過照片）
class SevenElevenShipResult {
  const SevenElevenShipResult({required this.photo});
  final File? photo;
}
