import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../data/product_certification_meta.dart';
import '../data/product_repository.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/setup_bottom_bar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import 'ai_assistant_button.dart';
import 'product_listing_draft_controller.dart';
import 'product_setup_progress_bar.dart';
import 'product_setup_top_bar.dart';

/// 商品上架第 4 步：選擇農業認證標章（有機/TAP/CAS/HACCP/ISO/GMP），含序號驗證
class ProductSetupTagScreen extends ConsumerStatefulWidget {
  const ProductSetupTagScreen({
    super.key,
    required this.images,
    this.skuId,
    required this.productName,
    required this.category,
    required this.subCategory,
    this.catalogId,
    this.isCandidateCategory = false,
    // 完整分類資訊
    this.l1Id,
    this.l1Name,
    this.l2Id,
    this.l2Name,
    this.l3Name,
    this.origin,
    this.description,
    this.descriptionBlocks,
    this.descriptionStagedTokens = const [],
    this.isEditMode = false,
    this.existingProduct,
  });

  final List<Uint8List> images;
  final Object? skuId;
  final String productName;
  final String category;
  final String subCategory;
  final int? catalogId; // 分類 ID（可能是正式或暫時）
  final bool
  isCandidateCategory; // true=來自 category_candidate, false=來自 category
  // 完整分類資訊
  final int? l1Id;
  final String? l1Name;
  final int? l2Id;
  final String? l2Name;
  final String? l3Name;
  final String? origin;
  final String? description;
  final List<Map<String, dynamic>>? descriptionBlocks;
  final List<String> descriptionStagedTokens;
  final bool isEditMode;
  final Map<String, dynamic>? existingProduct;

  @override
  ConsumerState<ProductSetupTagScreen> createState() =>
      _ProductSetupTagScreenState();
}

class _ProductSetupTagScreenState extends ConsumerState<ProductSetupTagScreen> {
  // 選中的標籤及其認證號碼
  final Map<int, String> _selectedTags = {}; // index -> 認證號碼

  // 標籤資料
  final List<Map<String, String>> _tags = [
    {'badge': '有機', 'title': '有機農產品標章', 'subtitle': '通過有機驗證機構\n認證的有機農產品'},
    {'badge': 'TAP', 'title': '產銷履歷標章TAP', 'subtitle': '完整生產紀錄農產品'},
    {'badge': 'CAS', 'title': 'CAS 優良農產品', 'subtitle': '台灣優良農產品標'},
    {'badge': 'HACCP', 'title': 'HACCP 認證', 'subtitle': '食品安全管制系統認證'},
    {'badge': 'ISO 22000', 'title': 'ISO 認證', 'subtitle': '國際標準化組織認證'},
    {'badge': 'GMP', 'title': 'GMP 認證', 'subtitle': '優良製造規範認證'},
  ];

  Future<void> _onNext() async {
    // 導航到商品規格頁面
    final result = await context.push(
      '/product/specs',
      extra: {
        'images': widget.images,
        'skuId': widget.skuId,
        'productName': widget.productName,
        'category': widget.category,
        'subCategory': widget.subCategory,
        'catalogId': widget.catalogId,
        'isCandidateCategory': widget.isCandidateCategory,
        'l1Id': widget.l1Id,
        'l1Name': widget.l1Name,
        'l2Id': widget.l2Id,
        'l2Name': widget.l2Name,
        'l3Name': widget.l3Name,
        'origin': widget.origin,
        'description': widget.description,
        'descriptionBlocks': widget.descriptionBlocks,
        'descriptionStagedTokens': widget.descriptionStagedTokens,
        'selectedTags': _selectedTags,
        'isEditMode': widget.isEditMode,
        'existingProduct': widget.existingProduct,
      },
    );
    // 如果收到 true，表示商品上傳成功，傳遞回 info_setup
    if (result == true && mounted) {
      context.pop(true);
    }
  }

  String _certTypeForIndex(int index) {
    return certificationValueForIndex(index);
  }

  Future<bool> _verifyCertification(int index, String serial) {
    return ref
        .read(productRepositoryProvider)
        .verifyCertification(
          certNo: serial.trim(),
          type: _certTypeForIndex(index),
        );
  }

  Future<void> _showCertificationDialog(int index) async {
    final tag = _tags[index];
    final serialController = TextEditingController();

    // 如果已經有認證號碼，預填
    if (_selectedTags.containsKey(index)) {
      serialController.text = _selectedTags[index]!;
    }

    final result = await showDialog<String>(
      context: context,
      barrierDismissible: true,
      builder: (ctx) {
        bool hasInput = serialController.text.trim().isNotEmpty;
        return StatefulBuilder(
          builder: (ctx, setDialogState) => Dialog(
            backgroundColor: Colors.transparent,
            insetPadding: EdgeInsets.symmetric(horizontal: 32.w),
            child: Container(
              decoration: BoxDecoration(
                color: AppColors.bgCard,
                borderRadius: BorderRadius.circular(16.r),
              ),
              child: SingleChildScrollView(
                child: Column(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    // 標籤圖標和標題
                    Padding(
                      padding: EdgeInsets.fromLTRB(20.w, 24.h, 20.w, 16.h),
                      child: Row(
                        children: [
                          _CertBadge(tag['badge']!, width: 64.w, height: 48.h),
                          SizedBox(width: 12.w),
                          Expanded(
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  tag['title']!,
                                  style: TextStyle(
                                    fontSize: 16.sp,
                                    fontWeight: FontWeight.w700,
                                    color: AppColors.textEmphasis,
                                  ),
                                ),
                                SizedBox(height: 4.h),
                                Text(
                                  tag['subtitle']!.replaceAll('\\n', ''),
                                  style: TextStyle(
                                    fontSize: 12.sp,
                                    color: AppColors.textSecondary,
                                  ),
                                ),
                              ],
                            ),
                          ),
                        ],
                      ),
                    ),
                    // 認證號碼輸入
                    Padding(
                      padding: EdgeInsets.symmetric(horizontal: 20.w),
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '認證號碼',
                            style: TextStyle(
                              fontSize: 14.sp,
                              fontWeight: FontWeight.w700,
                              color: AppColors.textEmphasis,
                            ),
                          ),
                          SizedBox(height: 8.h),
                          Container(
                            decoration: BoxDecoration(
                              color: AppColors.gray100,
                              borderRadius: BorderRadius.circular(8.r),
                            ),
                            child: TextField(
                              controller: serialController,
                              onChanged: (value) {
                                setDialogState(() {
                                  hasInput = value.trim().isNotEmpty;
                                });
                              },
                              decoration: InputDecoration(
                                hintText: '1-001-XXXXXX',
                                hintStyle: TextStyle(
                                  color: AppColors.gray300,
                                  fontSize: 14.sp,
                                ),
                                border: InputBorder.none,
                                contentPadding: EdgeInsets.symmetric(
                                  horizontal: 12.w,
                                  vertical: 12.h,
                                ),
                              ),
                              style: TextStyle(
                                fontSize: 14.sp,
                                color: AppColors.textEmphasis,
                              ),
                            ),
                          ),
                          SizedBox(height: 8.h),
                          Text(
                            '請輸入完整的認證號碼，例如：1-001-XXXXXX',
                            style: TextStyle(
                              fontSize: 12.sp,
                              color: AppColors.textSecondary,
                            ),
                          ),
                        ],
                      ),
                    ),
                    SizedBox(height: 20.h),
                    // 按鈕
                    Padding(
                      padding: EdgeInsets.fromLTRB(20.w, 0, 20.w, 20.h),
                      child: Row(
                        children: [
                          // 取消按鈕
                          Expanded(
                            child: SizedBox(
                              height: 48,
                              child: OutlinedButton(
                                onPressed: () => Navigator.of(ctx).pop(null),
                                child: const HkButtonLabel(
                                  '取消',
                                  semanticsLabel: '取消認證',
                                ),
                              ),
                            ),
                          ),
                          SizedBox(width: 12.w),
                          // 確認按鈕 - 有輸入時才能點擊
                          Expanded(
                            child: SizedBox(
                              height: 48,
                              child: FilledButton(
                                onPressed: hasInput
                                    ? () async {
                                        final entered = serialController.text
                                            .trim();

                                        // 顯示載入中
                                        TruckLoadingOverlay.show(ctx);

                                        bool ok = false;
                                        try {
                                          ok = await _verifyCertification(
                                            index,
                                            entered,
                                          );
                                        } finally {
                                          if (ctx.mounted) {
                                            Navigator.of(ctx).pop();
                                          }
                                        }

                                        if (!ok) {
                                          if (ctx.mounted) {
                                            // ScaffoldMessenger.of(ctx).showSnackBar(
                                            //   const SnackBar(content: Text('此序號尚未登記，請再次檢查或重新輸入')),
                                            // );
                                            FloatingToast.error(
                                              ctx,
                                              '此序號尚未登記，請再次檢查或重新輸入',
                                            );
                                          }
                                          return;
                                        }

                                        if (ctx.mounted) {
                                          Navigator.of(ctx).pop(entered);
                                        }
                                      }
                                    : null,
                                child: const HkButtonLabel(
                                  '確認',
                                  semanticsLabel: '確認認證',
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
          ),
        );
      },
    );

    if (result != null && mounted) {
      setState(() {
        _selectedTags[index] = result;
      });
    }
  }

  void _removeTag(int index) {
    setState(() {
      _selectedTags.remove(index);
    });
  }

  @override
  Widget build(BuildContext context) {
    // AI 助手累積的認證編號 → 進入時套入 _selectedTags（只在尚未選時套用）。
    // tag index 依 _tags[i].title 對應 AI patch 裡的 name。
    ref.listen(productListingDraftControllerProvider, (_, next) {
      if (next.certifications.isEmpty) return;
      final updates = <int, String>{};
      for (final cert in next.certifications) {
        final idx = _tags.indexWhere((t) => t['title'] == cert.name);
        if (idx >= 0 && !_selectedTags.containsKey(idx)) {
          updates[idx] = cert.certNo;
        }
      }
      if (updates.isNotEmpty && mounted) {
        setState(() => _selectedTags.addAll(updates));
      }
    });

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          ProductSetupTopBar(
            title: widget.isEditMode ? '編輯商品' : '新增商品',
            subtitle: '標籤認證設定',
            trailing: kAiProductListingAssistantEnabled
                ? const AiAssistantButton()
                : null,
          ),

          // 導航步驟列 - 使用共用組件
          const ProductSetupProgressBar(currentStep: 4),

          // 主要內容
          Expanded(
            child: SingleChildScrollView(
              child: Column(
                children: [
                  SizedBox(height: 15.h),
                  // 標章名稱和說明保留完整文字；窄螢幕或放大文字時改單欄。
                  Padding(
                    padding: EdgeInsets.symmetric(horizontal: 16.w),
                    child: Center(
                      child: ConstrainedBox(
                        constraints: const BoxConstraints(maxWidth: 720),
                        child: LayoutBuilder(
                          builder: (context, constraints) {
                            final minTileWidth =
                                172 *
                                MediaQuery.textScalerOf(context).scale(14) /
                                14;
                            final columns =
                                constraints.maxWidth >= minTileWidth * 2 + 12
                                ? 2
                                : 1;
                            final width =
                                (constraints.maxWidth - (columns - 1) * 12) /
                                columns;
                            return Wrap(
                              spacing: 12,
                              runSpacing: 12,
                              children: List.generate(
                                _tags.length,
                                (index) => SizedBox(
                                  width: width,
                                  child: _buildTagItem(index),
                                ),
                              ),
                            );
                          },
                        ),
                      ),
                    ),
                  ),
                  SizedBox(height: 16.h),
                  // 底部提示
                  Center(
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        const ExcludeSemantics(
                          child: Icon(
                            AppIcons.infoOutline,
                            size: 16,
                            color: AppIconColors.info,
                          ),
                        ),
                        SizedBox(width: 6.w),
                        Flexible(
                          child: Text(
                            '選擇適合的認證標章，提升商品信任度',
                            style: TextStyle(
                              fontSize: 12.sp,
                              color: AppColors.textSecondary,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
            ),
          ),

          SetupBottomBar(onBack: () => context.pop(), onNext: _onNext),
        ],
      ),
    );
  }

  Widget _buildTagItem(int index) {
    final tag = _tags[index];
    final isSelected = _selectedTags.containsKey(index);

    return GestureDetector(
      onTap: () => _showCertificationDialog(index),
      child: Container(
        constraints: const BoxConstraints(minHeight: 80),
        padding: EdgeInsets.symmetric(horizontal: 12.w, vertical: 12.h),
        decoration: BoxDecoration(
          color: isSelected ? AppColors.gray100 : Colors.transparent,
          border: Border.all(
            color: isSelected ? AppColors.primary : AppColors.cardBorder,
            width: isSelected ? 2 : 1,
          ),
          borderRadius: BorderRadius.circular(8.r),
        ),
        child: Stack(
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.center,
              children: [
                // 圖標
                _CertBadge(tag['badge']!, width: 56.w, height: 36.h),
                SizedBox(width: 8.w),
                // 文字
                Expanded(
                  child: Column(
                    mainAxisSize: MainAxisSize.min,
                    crossAxisAlignment: CrossAxisAlignment.start,
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      Text(
                        tag['title']!,
                        style: TextStyle(
                          fontSize: 14.sp,
                          fontWeight: FontWeight.w700,
                          color: AppColors.textEmphasis,
                        ),
                      ),
                      SizedBox(height: 2.h),
                      Text(
                        tag['subtitle']!.replaceAll('\\n', ''),
                        style: TextStyle(
                          fontSize: 12.sp,
                          color: AppColors.textSecondary,
                        ),
                      ),
                    ],
                  ),
                ),
              ],
            ),
            // 選中狀態顯示勾選圖標
            if (isSelected)
              Positioned(
                right: 0,
                top: 0,
                child: GestureDetector(
                  onTap: () => _removeTag(index),
                  child: Container(
                    width: 20.w,
                    height: 20.h,
                    decoration: const BoxDecoration(
                      color: AppColors.primary,
                      shape: BoxShape.circle,
                    ),
                    child: Icon(
                      AppIcons.check,
                      size: 14.sp,
                      color: AppIconColors.onBrand,
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

class _CertBadge extends StatelessWidget {
  const _CertBadge(this.label, {required this.width, required this.height});

  final String label;
  final double width;
  final double height;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: width,
      height: height,
      padding: EdgeInsets.symmetric(horizontal: 4.w),
      alignment: Alignment.center,
      decoration: BoxDecoration(
        color: AppColors.brandPrimarySoft,
        borderRadius: BorderRadius.circular(8.r),
      ),
      child: Text(
        label,
        maxLines: 2,
        overflow: TextOverflow.ellipsis,
        textAlign: TextAlign.center,
        style: TextStyle(
          fontSize: 12.sp,
          fontWeight: FontWeight.w700,
          color: AppColors.brandInk,
          height: 1.2,
        ),
      ),
    );
  }
}
