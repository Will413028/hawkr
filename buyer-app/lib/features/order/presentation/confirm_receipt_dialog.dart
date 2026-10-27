import 'dart:io';

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../shared/widgets/common_snackbar.dart';
import '../../media/data/media_repository.dart';
import '../data/order_repository.dart';
import 'receipt_photo_controller.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 確認收貨結果
class ConfirmReceiptResult {
  const ConfirmReceiptResult({this.photoUrl});

  /// 上傳後的照片 URL（null = 略過照片）
  final String? photoUrl;
}

/// 確認收貨 dialog — 對齊 Figma node 2594:24522
///
/// 顯示照片上傳區域，使用者可選擇：
/// - 上傳照片後「送出」
/// - 「收貨且略過」不上傳照片
/// - X 關閉取消
///
/// 回傳 [ConfirmReceiptResult] 或 null（取消）
class ConfirmReceiptDialog extends ConsumerStatefulWidget {
  const ConfirmReceiptDialog({super.key, required this.orderSn});

  final String orderSn;

  /// 顯示 dialog，回傳結果
  static Future<ConfirmReceiptResult?> show(
    BuildContext context, {
    required String orderSn,
  }) {
    return showDialog<ConfirmReceiptResult>(
      context: context,
      barrierColor: AppColors.textEmphasis.withValues(alpha: 0.5),
      barrierDismissible: false,
      builder: (_) => ConfirmReceiptDialog(orderSn: orderSn),
    );
  }

  @override
  ConsumerState<ConfirmReceiptDialog> createState() =>
      _ConfirmReceiptDialogState();
}

class _ConfirmReceiptDialogState extends ConsumerState<ConfirmReceiptDialog> {
  File? _selectedPhoto;
  bool _isUploading = false;

  Future<void> _pickPhoto() async {
    // iOS image_picker 對某些 iCloud 還沒下載完的 PNG/HEIC 會拋
    // PlatformException(invalid_image, ...) — 包 try-catch 給友善提示。
    try {
      final picker = ImagePicker();
      final picked = await picker.pickImage(
        source: ImageSource.gallery,
        maxWidth: 1024,
        imageQuality: 85,
        requestFullMetadata: false,
      );
      if (picked != null && mounted) {
        setState(() => _selectedPhoto = File(picked.path));
      }
    } on PlatformException catch (e) {
      if (!mounted) return;
      final friendly = e.code == 'invalid_image'
          ? '無法讀取此照片（可能還在 iCloud 上未下載），請改選其他張'
          : '選取照片失敗：${e.message ?? e.code}';
      showFloatingToast(context, friendly, isSuccess: false);
    } catch (e) {
      if (!mounted) return;
      showFloatingToast(context, '選取照片失敗：$e', isSuccess: false);
    }
  }

  /// 送出（上傳照片 + 確認收貨）
  Future<void> _submit() async {
    if (_selectedPhoto == null || _isUploading) return;
    setState(() => _isUploading = true);
    debugPrint(
      '[ConfirmReceipt] _submit start orderSn=${widget.orderSn} '
      'photoPath=${_selectedPhoto!.path}',
    );
    try {
      // Step 1: 上傳照片 → 取得 stagedToken + previewUrl
      final uploadResult = await ref
          .read(mediaRepositoryProvider)
          .uploadImage(file: _selectedPhoto!);
      debugPrint(
        '[ConfirmReceipt] uploadImage done '
        'stagedToken.len=${uploadResult.stagedToken.length} '
        'previewUrl.len=${uploadResult.previewUrl.length}',
      );

      // Step 2: 立刻把 previewUrl 交由 receipt-photo controller 暫存，
      // 不依賴 caller — 即使後續 confirmReceipt 或 caller 出錯，本機也已有暫存。
      if (uploadResult.previewUrl.trim().isNotEmpty) {
        final cached = await ref
            .read(
              receiptPhotoControllerProvider(
                orderSn: widget.orderSn,
                backendUrls: const [],
              ).notifier,
            )
            .cachePreview(uploadResult.previewUrl);
        debugPrint(
          '[ConfirmReceipt] receipt photo cache ${cached ? 'saved' : 'failed'} '
          'orderSn=${widget.orderSn}',
        );
      }

      // Step 3: 確認收貨 + 把 stagedToken 帶給後端轉正進 order.receiptPhotoUrls
      await ref
          .read(orderRepositoryProvider)
          .confirmReceipt(
            widget.orderSn,
            stagedTokens: uploadResult.stagedToken.isNotEmpty
                ? [uploadResult.stagedToken]
                : const [],
          );
      debugPrint('[ConfirmReceipt] confirmReceipt API done');

      if (!mounted) return;
      Navigator.of(
        context,
      ).pop(ConfirmReceiptResult(photoUrl: uploadResult.previewUrl));
    } catch (e, st) {
      debugPrint('[ConfirmReceipt] _submit error: $e\n$st');
      if (!mounted) return;
      setState(() => _isUploading = false);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('操作失敗：$e')));
    }
  }

  /// 收貨且略過（不上傳照片，直接確認收貨）
  Future<void> _skipAndConfirm() async {
    if (_isUploading) return;
    setState(() => _isUploading = true);
    debugPrint('[ConfirmReceipt] _skipAndConfirm orderSn=${widget.orderSn}');
    try {
      await ref.read(orderRepositoryProvider).confirmReceipt(widget.orderSn);
      if (!mounted) return;
      Navigator.of(context).pop(const ConfirmReceiptResult());
    } catch (e) {
      debugPrint('[ConfirmReceipt] _skipAndConfirm error: $e');
      if (!mounted) return;
      setState(() => _isUploading = false);
      ScaffoldMessenger.of(
        context,
      ).showSnackBar(SnackBar(content: Text('確認收貨失敗：$e')));
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: !_isUploading,
      child: SafeArea(
        minimum: const EdgeInsets.all(HkSpace.sm),
        child: Dialog(
          backgroundColor: Colors.transparent,
          insetPadding: const EdgeInsets.symmetric(
            horizontal: HkSpace.xxl,
            vertical: HkSpace.lg,
          ),
          child: ConstrainedBox(
            constraints: const BoxConstraints(maxWidth: 360),
            child: Stack(
              clipBehavior: Clip.none,
              children: [
                Container(
                  key: const ValueKey('confirm-receipt-dialog-surface'),
                  clipBehavior: Clip.antiAlias,
                  decoration: BoxDecoration(
                    color: AppColors.surfacePanel,
                    borderRadius: BorderRadius.circular(HkRadius.xxl),
                  ),
                  child: SingleChildScrollView(
                    key: const ValueKey('confirm-receipt-dialog-scroll'),
                    padding: const EdgeInsets.fromLTRB(
                      HkSpace.xl,
                      HkSpace.xxl,
                      HkSpace.xl,
                      HkSpace.xxl,
                    ),
                    child: Column(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(
                          '上傳收到貨品的照片',
                          style: AppTypography.h1,
                          textAlign: TextAlign.center,
                        ),
                        const SizedBox(height: HkSpace.xl),

                        // Upload box
                        Semantics(
                          button: true,
                          enabled: !_isUploading,
                          label: _selectedPhoto == null ? '上傳收貨照片' : '更換收貨照片',
                          child: GestureDetector(
                            onTap: _isUploading ? null : _pickPhoto,
                            behavior: HitTestBehavior.opaque,
                            child: Container(
                              key: const ValueKey(
                                'confirm-receipt-photo-picker',
                              ),
                              width: 120,
                              height: 120,
                              decoration: BoxDecoration(
                                color: AppColors.bgCard.withValues(alpha: 0.15),
                                borderRadius: BorderRadius.circular(
                                  HkRadius.xl,
                                ),
                                border: Border.all(
                                  color: AppColors.textPrimary.withValues(
                                    alpha: 0.5,
                                  ),
                                  width: 2,
                                ),
                              ),
                              clipBehavior: Clip.antiAlias,
                              child: _selectedPhoto != null
                                  ? Image.file(
                                      _selectedPhoto!,
                                      fit: BoxFit.cover,
                                      width: 120,
                                      height: 120,
                                    )
                                  : Column(
                                      mainAxisAlignment:
                                          MainAxisAlignment.center,
                                      children: [
                                        const Icon(
                                          AppIcons.add,
                                          size: 28,
                                          color: AppIconColors.defaultColor,
                                        ),
                                        const SizedBox(height: HkSpace.xs),
                                        Text('上傳照片', style: AppTypography.body),
                                      ],
                                    ),
                            ),
                          ),
                        ),
                        const SizedBox(height: HkSpace.xl),

                        if (_isUploading)
                          const SizedBox(
                            height: HkControl.minTap,
                            child: Center(
                              child: HkBrandLoader(size: HkBrandLoader.inline),
                            ),
                          )
                        else
                          LayoutBuilder(
                            builder: (context, constraints) {
                              final submit = _buildActionButton(
                                key: const ValueKey('confirm-receipt-submit'),
                                label: '送出',
                                onPressed: _selectedPhoto != null
                                    ? _submit
                                    : null,
                              );
                              final skip = _buildActionButton(
                                key: const ValueKey('confirm-receipt-skip'),
                                label: '收貨且略過',
                                onPressed: _skipAndConfirm,
                              );

                              if (constraints.maxWidth < 200) {
                                return Column(
                                  key: const ValueKey(
                                    'confirm-receipt-actions-stacked',
                                  ),
                                  crossAxisAlignment:
                                      CrossAxisAlignment.stretch,
                                  children: [
                                    submit,
                                    const SizedBox(height: HkSpace.sm),
                                    skip,
                                  ],
                                );
                              }

                              final compact = constraints.maxWidth < 240;
                              return Row(
                                key: const ValueKey(
                                  'confirm-receipt-actions-row',
                                ),
                                children: [
                                  Expanded(
                                    flex: compact ? 32 : 43,
                                    child: submit,
                                  ),
                                  const SizedBox(width: HkSpace.md),
                                  Expanded(
                                    flex: compact ? 68 : 57,
                                    child: skip,
                                  ),
                                ],
                              );
                            },
                          ),
                      ],
                    ),
                  ),
                ),
                Positioned(
                  top: -HkSpace.sm,
                  right: -HkSpace.sm,
                  child: IconButton(
                    key: const ValueKey('confirm-receipt-close'),
                    constraints: const BoxConstraints.tightFor(
                      width: HkControl.minTap,
                      height: HkControl.minTap,
                    ),
                    padding: const EdgeInsets.all(HkSpace.sm),
                    onPressed: _isUploading
                        ? null
                        : () => Navigator.of(context).pop(),
                    tooltip: '關閉',
                    icon: Container(
                      width: HkSpace.section,
                      height: HkSpace.section,
                      decoration: const BoxDecoration(
                        color: AppColors.textPrimary,
                        shape: BoxShape.circle,
                      ),
                      child: const Icon(
                        AppIcons.close,
                        size: HkSpace.xl,
                        color: AppIconColors.onBrand,
                      ),
                    ),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildActionButton({
    required Key key,
    required String label,
    required VoidCallback? onPressed,
  }) {
    return SizedBox(
      height: HkControl.minTap,
      child: FilledButton(
        key: key,
        onPressed: onPressed,
        style: FilledButton.styleFrom(
          minimumSize: const Size(0, HkControl.minTap),
          padding: const EdgeInsets.symmetric(horizontal: HkSpace.sm),
          backgroundColor: AppColors.primary,
          foregroundColor: AppColors.bgCard,
          disabledBackgroundColor: AppColors.primary.withValues(alpha: 0.5),
          disabledForegroundColor: AppColors.bgCard.withValues(alpha: 0.8),
          shape: const StadiumBorder(),
        ),
        child: HkButtonLabel(
          label,
          style: AppTypography.buttonLarge.copyWith(color: AppColors.bgCard),
        ),
      ),
    );
  }
}
