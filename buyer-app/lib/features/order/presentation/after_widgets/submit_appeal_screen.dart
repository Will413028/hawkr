import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../../shared/widgets/common_snackbar.dart';
import '../../../../shared/widgets/common_top_bar.dart';
import '../../../../shared/widgets/truck_loading_overlay.dart';
import '../../../media/data/media_repository.dart';
import '../../data/after_sale_repository.dart';

/// PR #983：申訴表單頁
///
/// 入口：售後詳情頁（status=33 待寄回逾時自動關案 / status=60 賣家拒絕）。
/// 後端：`POST /app/after_sales/{caseNo}/appeal`
///
/// 表單樣式沿用 [AfterSaleScreen]（售後申請頁）— 同色系、
/// 同 radio / textarea / photo tile pattern。
class SubmitAppealScreen extends ConsumerStatefulWidget {
  const SubmitAppealScreen({super.key, required this.caseNo});

  final String caseNo;

  @override
  ConsumerState<SubmitAppealScreen> createState() => _SubmitAppealScreenState();
}

class _SubmitAppealScreenState extends ConsumerState<SubmitAppealScreen> {
  static const _reasons = [
    '賣家未公正處理',
    '商品品質問題未解決',
    '物流異常未處理',
    '期限內已寄回但被自動關案',
    '與賣家溝通無回應',
    '其他原因',
  ];

  int? _selectedReason;
  final _descController = TextEditingController();
  final List<File> _photos = [];
  bool _isSubmitting = false;

  bool get _canSubmit =>
      _selectedReason != null && !_isSubmitting && widget.caseNo.isNotEmpty;

  @override
  void dispose() {
    _descController.dispose();
    super.dispose();
  }

  Future<void> _pickPhoto() async {
    if (_photos.length >= 5) {
      showFloatingToast(context, '最多只能上傳 5 張照片');
      return;
    }
    final picker = ImagePicker();
    final picked = await picker.pickImage(
      source: ImageSource.gallery,
      maxWidth: 1024,
      imageQuality: 85,
    );
    if (picked != null && mounted) {
      setState(() => _photos.add(File(picked.path)));
    }
  }

  void _removePhoto(int index) {
    setState(() => _photos.removeAt(index));
  }

  Future<void> _submit() async {
    if (_isSubmitting || _selectedReason == null) return;
    setState(() => _isSubmitting = true);
    TruckLoadingOverlay.show(context, message: '提交中...');

    try {
      // 上傳照片取得 staged tokens（沿用售後申請頁的 media 上傳流程）
      final List<String> tokens = [];
      if (_photos.isNotEmpty) {
        final mediaRepo = ref.read(mediaRepositoryProvider);
        for (final file in _photos) {
          try {
            final result = await mediaRepo.uploadImage(file: file);
            tokens.add(result.stagedToken);
          } catch (_) {
            // 單張上傳失敗不阻擋提交（與售後申請頁一致）
          }
        }
      }

      final reasonLabel = _reasons[_selectedReason!];
      final desc = _descController.text.trim();

      await ref
          .read(afterSaleRepositoryProvider)
          .submitAppeal(
            caseNo: widget.caseNo,
            reason: reasonLabel,
            description: desc.isNotEmpty ? desc : null,
            evidencePhotoTokens: tokens.isNotEmpty ? tokens : null,
          );

      if (!mounted) return;
      TruckLoadingOverlay.hide(context);

      showFloatingToast(
        context,
        '申訴已提交，平台將盡快審核',
        icon: AppIcons.checkCircleOutline,
        duration: const Duration(seconds: 3),
      );

      await Future.delayed(const Duration(milliseconds: 800));
      if (mounted) context.pop();
    } catch (e) {
      if (!mounted) return;
      TruckLoadingOverlay.hide(context);
      setState(() => _isSubmitting = false);
      showFloatingToast(
        context,
        e.toString().replaceAll('Exception: ', ''),
        isSuccess: false,
      );
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          _buildAppBar(context),
          Expanded(
            child: SingleChildScrollView(
              padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 32.h),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // 標題
                  Text(
                    '提交申訴',
                    style: TextStyle(
                      fontSize: 20,
                      fontWeight: FontWeight.w700,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  SizedBox(height: 4.h),
                  Text(
                    '平台將於 3 個工作日內審核您的申訴',
                    style: TextStyle(
                      fontSize: 14,
                      color: AppColors.textPrimary.withValues(alpha: 0.5),
                    ),
                  ),
                  SizedBox(height: 20.h),

                  // 申訴原因
                  _buildSectionLabel('申訴原因', required: true),
                  SizedBox(height: 8.h),
                  ..._reasons.asMap().entries.map(
                    (e) => _buildRadioItem(e.key, e.value),
                  ),
                  SizedBox(height: 20.h),

                  // 詳細說明（選填）
                  _buildSectionLabel('詳細說明（選填）'),
                  SizedBox(height: 8.h),
                  Container(
                    decoration: BoxDecoration(
                      color: AppColors.textPrimary.withValues(alpha: 0.1),
                      borderRadius: BorderRadius.circular(8.r),
                    ),
                    child: TextField(
                      controller: _descController,
                      maxLines: 5,
                      maxLength: 1000,
                      onChanged: (_) => setState(() {}),
                      decoration: InputDecoration(
                        hintText: '請詳述申訴的事實依據與訴求...',
                        hintStyle: TextStyle(
                          fontSize: 14,
                          color: AppColors.textPrimary.withValues(alpha: 0.4),
                        ),
                        border: InputBorder.none,
                        contentPadding: EdgeInsets.all(12.w),
                      ),
                      style: TextStyle(
                        fontSize: 14,
                        color: AppColors.textPrimary,
                      ),
                    ),
                  ),
                  SizedBox(height: 20.h),

                  // 上傳照片（選填，最多 5 張）
                  _buildSectionLabel('上傳證據照片（選填）'),
                  SizedBox(height: 4.h),
                  Text(
                    '建議上傳爭議憑證或溝通紀錄，最多 5 張',
                    style: TextStyle(fontSize: 14, color: AppColors.primary),
                  ),
                  SizedBox(height: 12.h),
                  _buildPhotoRow(),
                  SizedBox(height: 32.h),

                  // 送出申訴
                  SizedBox(
                    width: double.infinity,
                    height: 48.h,
                    child: GestureDetector(
                      onTap: _canSubmit ? _submit : null,
                      child: Container(
                        decoration: BoxDecoration(
                          color: AppColors.primary,
                          borderRadius: BorderRadius.circular(12.r),
                        ),
                        child: Center(
                          child: Opacity(
                            opacity: _canSubmit ? 1.0 : 0.5,
                            child: Text(
                              '送出申訴',
                              style: TextStyle(
                                fontSize: 16,
                                fontWeight: FontWeight.w700,
                                color: AppColors.bgCard,
                              ),
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAppBar(BuildContext context) {
    return commonTitleTopBar(
      context,
      title: '提交申訴',
      backIcon: AppIcons.chevronLeft,
      onBack: () => context.pop(),
    );
  }

  Widget _buildSectionLabel(String text, {bool required = false}) {
    return RichText(
      text: TextSpan(
        style: TextStyle(
          fontSize: 16,
          fontWeight: FontWeight.w700,
          color: AppColors.textPrimary,
        ),
        children: [
          TextSpan(text: text),
          if (required)
            TextSpan(
              text: ' *',
              style: TextStyle(color: AppColors.danger, fontSize: 16),
            ),
        ],
      ),
    );
  }

  Widget _buildRadioItem(int index, String label) {
    final selected = _selectedReason == index;
    return GestureDetector(
      onTap: () => setState(() => _selectedReason = index),
      child: Padding(
        padding: EdgeInsets.symmetric(vertical: 8.h),
        child: Row(
          children: [
            Container(
              width: 20.w,
              height: 20.w,
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                border: Border.all(
                  color: selected
                      ? AppColors.primary
                      : AppColors.textPrimary.withValues(alpha: 0.3),
                  width: 1.5,
                ),
                color: selected ? AppColors.primary : Colors.transparent,
              ),
              child: selected
                  ? Center(
                      child: Container(
                        width: 8.w,
                        height: 8.w,
                        decoration: const BoxDecoration(
                          shape: BoxShape.circle,
                          color: AppColors.bgCard,
                        ),
                      ),
                    )
                  : null,
            ),
            SizedBox(width: 10.w),
            Text(
              label,
              style: TextStyle(fontSize: 16, color: AppColors.textPrimary),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildPhotoRow() {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          for (var i = 0; i < 5; i++) ...[
            if (i > 0) SizedBox(width: 10.w),
            if (i < _photos.length)
              _buildPhotoTile(i)
            else
              _buildAddPhotoTile(),
          ],
        ],
      ),
    );
  }

  Widget _buildPhotoTile(int index) {
    final tileSize = 80.w;
    final borderWidth = 1.5.w;
    final innerSize = tileSize - borderWidth * 2;
    final cacheSize = (innerSize * MediaQuery.devicePixelRatioOf(context))
        .round()
        .clamp(1, 2048);

    return SizedBox.square(
      dimension: tileSize,
      child: Stack(
        children: [
          Positioned.fill(
            child: DecoratedBox(
              decoration: BoxDecoration(
                color: AppColors.bgPrimary,
                borderRadius: BorderRadius.circular(16.r),
                border: Border.all(
                  color: AppColors.cardBorder,
                  width: borderWidth,
                ),
              ),
              child: Padding(
                padding: EdgeInsets.all(borderWidth),
                child: ClipRRect(
                  borderRadius: BorderRadius.circular(16.r),
                  child: SizedBox.square(
                    dimension: innerSize,
                    child: Image.file(
                      _photos[index],
                      key: ValueKey(_photos[index].path),
                      width: innerSize,
                      height: innerSize,
                      fit: BoxFit.cover,
                      cacheWidth: cacheSize,
                      cacheHeight: cacheSize,
                      filterQuality: FilterQuality.medium,
                      errorBuilder: (_, __, ___) => const ColoredBox(
                        color: AppColors.bgPrimary,
                        child: Center(
                          child: Icon(
                            AppIcons.imageNotSupportedOutlined,
                            color: AppIconColors.muted,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
          ),
          Positioned(
            top: 0,
            right: 0,
            child: GestureDetector(
              onTap: () => _removePhoto(index),
              child: Container(
                width: 20.w,
                height: 20.w,
                decoration: const BoxDecoration(
                  color: AppColors.primary,
                  shape: BoxShape.circle,
                ),
                child: Icon(
                  AppIcons.close,
                  size: 14.sp,
                  color: AppIconColors.onBrand,
                ),
              ),
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildAddPhotoTile() {
    final tileSize = 80.w;
    return GestureDetector(
      onTap: _photos.length < 5 ? _pickPhoto : null,
      child: SizedBox.square(
        dimension: tileSize,
        child: DecoratedBox(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(16.r),
            border: Border.all(
              color: AppColors.textPrimary.withValues(alpha: 0.3),
              width: 1.5.w,
            ),
          ),
          child: Column(
            mainAxisAlignment: MainAxisAlignment.center,
            children: [
              Icon(
                AppIcons.add,
                size: 24.sp,
                color: AppIconColors.defaultColor.withValues(alpha: 0.5),
              ),
              SizedBox(height: 4.h),
              Text(
                '照片',
                style: TextStyle(
                  fontSize: 14,
                  color: AppColors.textPrimary.withValues(alpha: 0.5),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
