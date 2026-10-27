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
import '../../data/after_sale_model.dart';
import '../../data/after_sale_repository.dart';
import '../../data/after_sale_order_lookup.dart';
import '../order_list_controller.dart';

/// 售後服務頁面 — 對齊 Figma node 2857:62676 / 2857:62677
class AfterSaleScreen extends ConsumerStatefulWidget {
  const AfterSaleScreen({
    super.key,
    required this.orderSn,
    this.items = const [],
  });

  final String orderSn;
  final List<Map<String, dynamic>> items;

  @override
  ConsumerState<AfterSaleScreen> createState() => _AfterSaleScreenState();
}

class _AfterSaleScreenState extends ConsumerState<AfterSaleScreen> {
  static const _reasons = ['商品有瑕疵', '收到錯誤商品', '與描述不符', '改變心意', '其他原因'];
  static const _reasonCodes = [
    '5001', // AfterSaleReason.defective
    '5002', // AfterSaleReason.wrongItem
    '5003', // AfterSaleReason.notAsDescribed
    '5004', // AfterSaleReason.changeOfMind
    '5005', // AfterSaleReason.other
  ];

  int? _selectedReason;
  final _descController = TextEditingController();
  final List<File> _photos = [];
  bool _isSubmitting = false;
  bool _isSubmitted = false;
  bool _checkingStatus = true;

  bool get _canSubmit =>
      _selectedReason != null &&
      _descController.text.trim().isNotEmpty &&
      !_isSubmitting &&
      !_isSubmitted;

  @override
  void initState() {
    super.initState();
    _checkRefundStatus();
  }

  Future<void> _checkRefundStatus() async {
    if (widget.orderSn.isEmpty) {
      if (mounted) setState(() => _checkingStatus = false);
      return;
    }
    try {
      // 用新的 after_sales API 檢查是否已有進行中的售後單
      final result = await ref
          .read(afterSaleRepositoryProvider)
          .getAfterSaleList();
      final hasActive = result.records.any(
        (c) =>
            c.orderNo == widget.orderSn &&
            c.status != AfterSaleStatus.closed &&
            c.status != AfterSaleStatus.cancelled,
      );
      if (mounted) {
        setState(() {
          _isSubmitted = hasActive;
          _checkingStatus = false;
        });
      }
    } catch (_) {
      if (mounted) setState(() => _checkingStatus = false);
    }
  }

  @override
  void dispose() {
    _descController.dispose();
    super.dispose();
  }

  Future<void> _pickPhoto() async {
    if (_photos.length >= 3) {
      showFloatingToast(context, '最多只能上傳 3 張照片');
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
    if (_isSubmitting ||
        _selectedReason == null ||
        _descController.text.trim().isEmpty) {
      return;
    }
    _isSubmitting = true;
    setState(() {});
    TruckLoadingOverlay.show(context, message: '提交中...');

    try {
      // 上傳照片取得 staged tokens
      final List<String> photoTokens = [];
      if (_photos.isNotEmpty) {
        try {
          final mediaRepo = ref.read(mediaRepositoryProvider);
          for (final file in _photos) {
            final result = await mediaRepo.uploadImage(file: file);
            photoTokens.add(result.stagedToken);
          }
        } catch (_) {
          // 照片上傳失敗不阻擋提交
        }
      }

      final reasonCode = _reasonCodes[_selectedReason!];
      final desc = _descController.text.trim();

      await ref
          .read(afterSaleRepositoryProvider)
          .createAfterSale(
            orderNo: widget.orderSn,
            reason: reasonCode,
            description: desc,
            photoTokens: photoTokens.isNotEmpty ? photoTokens : null,
          );

      if (!mounted) return;
      TruckLoadingOverlay.hide(context);

      setState(() => _isSubmitted = true);

      showFloatingToast(
        context,
        '售後服務已提交申請\n我們在1-2個工作天內處理您的申請',
        icon: AppIcons.checkCircleOutline,
        duration: const Duration(seconds: 3),
      );

      // 把這張訂單加進售後 lookup（card 馬上切換為售後卡）
      ref
          .read(afterSaleOrderLookupProvider.notifier)
          .addOrderNo(widget.orderSn, status: '0');
      // 刷新訂單列表，然後回到訂單卡片位置
      ref.read(orderListControllerProvider.notifier).refresh();
      await Future.delayed(const Duration(milliseconds: 800));
      if (mounted) context.pop();
    } catch (e) {
      if (!mounted) return;
      TruckLoadingOverlay.hide(context);
      setState(() => _isSubmitting = false);
      showFloatingToast(
        context,
        '申請失敗：${e.toString().replaceAll('Exception: ', '')}',
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
            child: _checkingStatus
                ? const TruckLoadingOverlay(message: '載入中...')
                : SingleChildScrollView(
                    padding: EdgeInsets.fromLTRB(20.w, 20.h, 20.w, 32.h),
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        // 標題
                        Text(
                          '填寫退貨資料',
                          style: TextStyle(
                            fontSize: 20,
                            fontWeight: FontWeight.w700,
                            color: AppColors.textPrimary,
                          ),
                        ),
                        SizedBox(height: 4.h),
                        Text(
                          '請詳細填寫退貨原因和相關資訊',
                          style: TextStyle(
                            fontSize: 14,
                            color: const Color(
                              0xFF42493F,
                            ).withValues(alpha: 0.5),
                          ),
                        ),
                        SizedBox(height: 20.h),

                        // 退貨原因
                        _buildSectionLabel('退貨原因', required: true),
                        SizedBox(height: 8.h),
                        ..._reasons.asMap().entries.map(
                          (e) => _buildRadioItem(e.key, e.value),
                        ),
                        SizedBox(height: 20.h),

                        // 詳細說明
                        _buildSectionLabel('詳細說明', required: true),
                        SizedBox(height: 8.h),
                        Container(
                          decoration: BoxDecoration(
                            color: const Color(
                              0xFF42493F,
                            ).withValues(alpha: 0.1),
                            borderRadius: BorderRadius.circular(8.r),
                          ),
                          child: TextField(
                            controller: _descController,
                            maxLines: 5,
                            onChanged: (_) => setState(() {}),
                            decoration: InputDecoration(
                              hintText: '請描述退貨原因的詳細情況...',
                              hintStyle: TextStyle(
                                fontSize: 14,
                                color: const Color(
                                  0xFF42493F,
                                ).withValues(alpha: 0.4),
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

                        // 上傳照片
                        _buildSectionLabel('上傳照片（選填）'),
                        SizedBox(height: 4.h),
                        Text(
                          '建議上傳商品瑕疵或問題照片，最多3張',
                          style: TextStyle(
                            fontSize: 14,
                            color: AppColors.primary,
                          ),
                        ),
                        SizedBox(height: 12.h),
                        _buildPhotoRow(),
                        SizedBox(height: 32.h),

                        // 送出申請 / 已提交
                        SizedBox(
                          width: double.infinity,
                          height: 48.h,
                          child: GestureDetector(
                            onTap: _canSubmit ? _submit : null,
                            child: Container(
                              decoration: BoxDecoration(
                                color: _isSubmitted
                                    ? AppColors.gray300
                                    : AppColors.primary,
                                borderRadius: BorderRadius.circular(12.r),
                              ),
                              child: Center(
                                child: Opacity(
                                  opacity: _isSubmitted
                                      ? 1.0
                                      : (_canSubmit ? 1.0 : 0.5),
                                  child: Text(
                                    _isSubmitted ? '已提交' : '送出申請',
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

  // ── AppBar ──
  Widget _buildAppBar(BuildContext context) {
    return commonTitleTopBar(
      context,
      title: '售後服務',
      backIcon: AppIcons.chevronLeft,
      onBack: () => context.pop(),
    );
  }

  // ── Section label ──
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

  // ── Radio item ──
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

  // ── Photo row ──
  Widget _buildPhotoRow() {
    return SingleChildScrollView(
      scrollDirection: Axis.horizontal,
      child: Row(
        children: [
          for (var i = 0; i < 3; i++) ...[
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
      onTap: _photos.length < 3 ? _pickPhoto : null,
      child: SizedBox.square(
        dimension: tileSize,
        child: DecoratedBox(
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(16.r),
            border: Border.all(
              color: AppColors.textPrimary.withValues(alpha: 0.3),
              width: 1.5.w,
              // TODO: dashed border 需要自訂 painter，暫用 solid
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
