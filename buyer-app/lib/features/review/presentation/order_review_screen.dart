import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';
import 'package:hawkr_buyer/shared/utils/hk_log.dart';

import '../../../config/image_config.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../media/data/media_repository.dart';
import '../../order/data/order_repository.dart';
import '../../product/data/product_repository.dart';
import '../../../shared/widgets/oss_image.dart';
import '../../order/presentation/order_list_controller.dart';
import '../data/review_repository.dart';

/// 給我好評頁面 — 對齊 Figma node 3421:50042
///
/// 支援多商品逐一評分 + 評價 + 上傳圖片（最多 4 張）
class OrderReviewScreen extends ConsumerStatefulWidget {
  const OrderReviewScreen({
    super.key,
    required this.orderSn,
    this.items = const [],
    // Legacy: 舊路由用 skuId 進入
    this.skuId = 0,
  });

  final String orderSn;
  final List<Map<String, dynamic>> items;
  final int skuId;

  @override
  ConsumerState<OrderReviewScreen> createState() => _OrderReviewScreenState();
}

class _OrderReviewScreenState extends ConsumerState<OrderReviewScreen> {
  // 每個 item 的評分 (1-5)
  final Map<int, int> _ratings = {};
  // 每個 item 的評價文字
  final Map<int, TextEditingController> _contentCtrls = {};
  // 每個 item 的上傳圖片
  final Map<int, List<File>> _images = {};
  // 每個 item 的提交狀態
  final Map<int, bool> _submitted = {};

  List<Map<String, dynamic>> _resolvedItems = [];
  bool _isLoading = true;

  @override
  void initState() {
    super.initState();
    _loadOrderItems();
  }

  Future<void> _loadOrderItems() async {
    // 如果已有 items 且包含 id，直接使用
    if (widget.items.isNotEmpty &&
        widget.items.first.containsKey('id') &&
        (widget.items.first['id']?.toString() ?? '').isNotEmpty) {
      _resolvedItems = widget.items;
    } else if (widget.orderSn.isNotEmpty) {
      // 從 order detail API 取得完整 items（含 id/orderItemId）
      try {
        final order = await ref
            .read(orderRepositoryProvider)
            .getOrderDetail(widget.orderSn);
        _resolvedItems = order.items
            .map(
              (item) => {
                'id': item.id,
                'skuId': item.skuId,
                'skuName': item.skuName,
                'productName': item.productName,
                'actualPayAmount': item.actualPayAmount,
                'imageUrl': item.imageUrl,
              },
            )
            .toList();
      } catch (_) {
        _resolvedItems = widget.items;
      }
    } else if (widget.skuId > 0) {
      _resolvedItems = [
        {
          'skuId': widget.skuId.toString(),
          'skuName': '商品',
          'productName': '',
          'actualPayAmount': '',
        },
      ];
    }

    // 用 product API 補齊商品名稱（skuName 可能只是規格名如 "D"）
    final productRepo = ref.read(productRepositoryProvider);
    for (var i = 0; i < _resolvedItems.length; i++) {
      final skuId = (_resolvedItems[i]['skuId'] ?? '').toString();
      if (skuId.isEmpty) continue;
      try {
        final info = await productRepo.getProductFullInfo(skuId);
        if (info.isNotEmpty) {
          final productName = (info['name'] ?? info['title'] ?? '').toString();
          if (productName.isNotEmpty) {
            _resolvedItems[i] = {
              ..._resolvedItems[i],
              'productName': productName,
            };
          }
        }
      } catch (_) {}
    }

    if (mounted) {
      setState(() {
        _isLoading = false;
        for (var i = 0; i < _resolvedItems.length; i++) {
          _ratings[i] = 0;
          _contentCtrls[i] = TextEditingController();
          _images[i] = [];
          _submitted[i] = false;
        }
      });
    }
  }

  List<Map<String, dynamic>> get _items => _resolvedItems;

  @override
  void dispose() {
    for (final c in _contentCtrls.values) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _pickImages(int index) async {
    final current = _images[index] ?? [];
    if (current.length >= 4) {
      showCommonSnackBar(context, '最多只能上傳 4 張圖片');
      return;
    }
    final picker = ImagePicker();
    final picked = await picker.pickMultiImage(
      imageQuality: 85,
      maxWidth: 1024,
    );
    if (picked.isNotEmpty && mounted) {
      final remaining = 4 - current.length;
      setState(() {
        _images[index] = [
          ...current,
          ...picked.take(remaining).map((x) => File(x.path)),
        ];
      });
    }
  }

  void _removeImage(int itemIndex, int imgIndex) {
    setState(() => _images[itemIndex]?.removeAt(imgIndex));
  }

  Future<void> _submitItem(int index) async {
    if (_submitted[index] == true) return;
    final rating = _ratings[index] ?? 0;
    if (rating == 0) {
      showCommonSnackBar(context, '請先評分');
      return;
    }

    setState(() => _submitted[index] = true);

    try {
      // 上傳圖片，收集 stagedToken（由後端呼叫 FinalizeImages 轉正）
      final imageFiles = _images[index] ?? [];
      final List<String> stagedTokens = [];
      if (imageFiles.isNotEmpty) {
        final mediaRepo = ref.read(mediaRepositoryProvider);
        for (final file in imageFiles) {
          final result = await mediaRepo.uploadImage(file: file);
          stagedTokens.add(result.stagedToken);
        }
      }

      // 提交評價
      final item = _items[index];
      final orderItemId = _toInt(item['id'] ?? item['orderItemId']);
      final skuId = _toInt(item['skuId']);
      final content = _contentCtrls[index]?.text.trim() ?? '';

      final reviewRepo = ref.read(reviewRepositoryProvider);
      await reviewRepo.addReview(
        orderNo: widget.orderSn,
        orderItemId: orderItemId,
        skuId: skuId,
        rating: rating,
        content: content.isNotEmpty ? content : '好評',
        stagedTokens: stagedTokens.isNotEmpty ? stagedTokens : null,
      );

      if (mounted) {
        showCommonSnackBar(context, '評價已提交');
        setState(() {});

        // 全部商品都已提交 → 刷新訂單列表狀態，停留在當前頁面
        final allDone = _items.asMap().keys.every((i) => _submitted[i] == true);
        if (allDone) {
          ref.read(orderListControllerProvider.notifier).refresh();
        }
      }
    } catch (e) {
      HKLog.logD('[OrderReviewScreen]', e);
      if (!mounted) return;
      final msg = e.toString().toLowerCase();
      // 已評論過 → 標記為已提交，停留在當前頁面
      if (msg.contains('already reviewed') ||
          msg.contains('already_reviewed') ||
          msg.contains('duplicate')) {
        showCommonSnackBar(context, '此商品已評論過');
        setState(() => _submitted[index] = true);
        return;
      }
      setState(() => _submitted[index] = false);
      if (msg.contains('已評論') || msg.contains('409')) {
        showCommonSnackBar(context, '此商品已評論過');
      } else if (msg.contains('410') || msg.contains('expired')) {
        showCommonSnackBar(context, '評價期限已過，無法提交評價');
      } else {
        showCommonSnackBar(context, '評價已提交');
      }
    }
  }

  int _toInt(dynamic v) {
    if (v is int) return v;
    if (v is num) return v.toInt();
    return int.tryParse(v?.toString() ?? '') ?? 0;
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      body: Column(
        children: [
          _buildAppBar(context),
          Expanded(
            child: _isLoading
                ? const TruckLoadingOverlay(message: '載入中...')
                : ListView(
                    padding: EdgeInsets.fromLTRB(20.w, 16.h, 20.w, 32.h),
                    children: [
                      _buildOrderNumberCard(),
                      SizedBox(height: 26.h),
                      for (var i = 0; i < _items.length; i++) ...[
                        _buildItemReviewCard(i),
                        if (i < _items.length - 1) SizedBox(height: 26.h),
                      ],
                    ],
                  ),
          ),
        ],
      ),
    );
  }

  // ── AppBar ──────────────────────────────────────────────────────────────
  Widget _buildAppBar(BuildContext context) {
    return commonTitleTopBar(
      context,
      title: '給我好評',
      onBack: () {
        if (Navigator.of(context).canPop()) {
          context.pop();
        } else {
          context.go('/');
        }
      },
    );
  }

  // ── 訂單編號卡片 ────────────────────────────────────────────────────────
  Widget _buildOrderNumberCard() {
    return Container(
      padding: EdgeInsets.all(16.w),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.textPrimary.withValues(alpha: 0.2)),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(AppIcons.assignmentOutlined, size: 24.sp),
              SizedBox(width: 6.w),
              Text(
                '訂單編號',
                style: TextStyle(
                  fontSize: 16,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textPrimary,
                  height: 24 / 16,
                ),
              ),
            ],
          ),
          SizedBox(height: 6.h),
          Text(
            widget.orderSn.isNotEmpty ? widget.orderSn : 'SKU-${widget.skuId}',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w400,
              color: AppColors.textEmphasis,
              height: 24 / 16,
            ),
          ),
        ],
      ),
    );
  }

  // ── 商品評價卡片 ────────────────────────────────────────────────────────
  Widget _buildItemReviewCard(int index) {
    final item = _items[index];
    final name = (item['productName'] ?? '商品').toString();
    final rawPrice = (item['actualPayAmount'] ?? item['price'] ?? '')
        .toString();
    final price = rawPrice.isNotEmpty ? 'NT\$ $rawPrice' : '';
    final imageUrl = (item['imageUrl'] ?? item['image'] ?? '').toString();
    final rating = _ratings[index] ?? 0;
    final images = _images[index] ?? [];
    final isSubmitted = _submitted[index] == true;

    return Container(
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(16.r),
        border: Border.all(color: AppColors.textPrimary.withValues(alpha: 0.2)),
        boxShadow: [
          BoxShadow(
            color: AppColors.cardShadow,
            blurRadius: 2,
            offset: const Offset(0, 1),
          ),
        ],
      ),
      padding: EdgeInsets.fromLTRB(16.w, 16.h, 16.w, 20.h),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Section title
          Row(
            children: [
              Icon(AppIcons.localFloristOutlined, size: 24.sp),
              SizedBox(width: 6.w),
              Text(
                '商品明細',
                style: TextStyle(
                  fontSize: 18,
                  fontWeight: FontWeight.w700,
                  color: AppColors.textPrimary,
                  height: 1.23,
                ),
              ),
            ],
          ),
          SizedBox(height: 12.h),

          // Product info row
          Row(
            children: [
              Container(
                width: 80.w,
                height: 80.w,
                decoration: BoxDecoration(
                  color: AppColors.gray100,
                  borderRadius: BorderRadius.circular(12.r),
                ),
                clipBehavior: Clip.antiAlias,
                child: imageUrl.isNotEmpty
                    ? OssImage(
                        imageSource: imageUrl,
                        width: 80.w,
                        height: 80.w,
                        fit: BoxFit.cover,
                        scene: ImageScene.orderItem,
                      )
                    : Icon(
                        AppIcons.image,
                        size: 32.sp,
                        color: AppIconColors.muted,
                      ),
              ),
              SizedBox(width: 12.w),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      name,
                      style: TextStyle(
                        fontSize: 18,
                        fontWeight: FontWeight.w400,
                        color: AppColors.textPrimary,
                        letterSpacing: 0.77,
                        height: 1.38,
                      ),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (price.isNotEmpty) ...[
                      SizedBox(height: 6.h),
                      Text(
                        price,
                        style: TextStyle(
                          fontSize: 18,
                          fontWeight: FontWeight.w400,
                          color: AppColors.textPrimary,
                          letterSpacing: 0.77,
                          height: 1.38,
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ],
          ),
          SizedBox(height: 16.h),

          // 商品評分
          Text(
            '商品評分',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
              height: 24 / 16,
            ),
          ),
          SizedBox(height: 6.h),
          Row(
            children: [
              Text(
                '失望',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w400,
                  color: AppColors.warning,
                  height: 20 / 14,
                ),
              ),
              SizedBox(width: 20.w),
              ...List.generate(5, (starIdx) {
                final filled = starIdx < rating;
                return GestureDetector(
                  onTap: isSubmitted
                      ? null
                      : () {
                          setState(() => _ratings[index] = starIdx + 1);
                        },
                  child: Padding(
                    padding: EdgeInsets.symmetric(horizontal: 4.w),
                    child: Icon(
                      filled ? AppIcons.star : AppIcons.starOutline,
                      size: 32.sp,
                      color: AppIconColors.rating,
                    ),
                  ),
                );
              }),
              SizedBox(width: 20.w),
              Text(
                '滿意',
                style: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w400,
                  color: AppColors.warning,
                  height: 20 / 14,
                ),
              ),
            ],
          ),
          SizedBox(height: 16.h),

          // 商品評價
          Text(
            '商品評價',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
              height: 24 / 16,
            ),
          ),
          SizedBox(height: 6.h),
          Container(
            height: 100.h,
            decoration: BoxDecoration(
              color: AppColors.textPrimary.withValues(alpha: 0.2),
              borderRadius: BorderRadius.circular(8.r),
              border: Border.all(color: AppColors.gray100, width: 0.58),
            ),
            child: TextField(
              controller: _contentCtrls[index],
              enabled: !isSubmitted,
              maxLines: 4,
              decoration: InputDecoration(
                hintText: '分享你的使用心得，幫助其他買家做出更好的選擇...',
                hintStyle: TextStyle(
                  fontSize: 14,
                  fontWeight: FontWeight.w400,
                  color: AppColors.textPrimary.withValues(alpha: 0.5),
                  height: 20 / 14,
                ),
                border: InputBorder.none,
                contentPadding: EdgeInsets.symmetric(
                  horizontal: 12.w,
                  vertical: 8.h,
                ),
              ),
              style: TextStyle(
                fontSize: 14,
                color: AppColors.textPrimary,
                height: 20 / 14,
              ),
            ),
          ),
          SizedBox(height: 16.h),

          // 上傳圖片
          Text(
            '上傳圖片（最多4張）',
            style: TextStyle(
              fontSize: 16,
              fontWeight: FontWeight.w700,
              color: AppColors.textPrimary,
              height: 24 / 16,
            ),
          ),
          SizedBox(height: 8.h),
          Wrap(
            spacing: 6.w,
            runSpacing: 6.h,
            children: [
              ...images.asMap().entries.map(
                (e) => _buildImageTile(
                  child: Image.file(e.value, fit: BoxFit.cover),
                  onRemove: isSubmitted
                      ? null
                      : () => _removeImage(index, e.key),
                ),
              ),
              if (images.length < 4 && !isSubmitted)
                _buildAddImageTile(() => _pickImages(index)),
            ],
          ),
          SizedBox(height: 16.h),

          // 提交按鈕
          SizedBox(
            width: double.infinity,
            height: 40.h,
            child: isSubmitted
                ? Container(
                    decoration: BoxDecoration(
                      borderRadius: BorderRadius.circular(8.r),
                      border: Border.all(color: AppColors.primary, width: 2),
                    ),
                    child: Center(
                      child: Opacity(
                        opacity: 0.5,
                        child: Text(
                          '已提交',
                          style: TextStyle(
                            fontSize: 18,
                            fontWeight: FontWeight.w700,
                            color: AppColors.primary,
                            height: 1.23,
                          ),
                        ),
                      ),
                    ),
                  )
                : GestureDetector(
                    onTap: rating > 0 ? () => _submitItem(index) : null,
                    child: Container(
                      decoration: BoxDecoration(
                        color: AppColors.primary,
                        borderRadius: BorderRadius.circular(12.r),
                      ),
                      child: Center(
                        child: Opacity(
                          opacity: rating > 0 ? 1.0 : 0.5,
                          child: Text(
                            '提交評價',
                            style: TextStyle(
                              fontSize: 16,
                              fontWeight: FontWeight.w700,
                              color: AppColors.bgCard,
                              height: 24 / 16,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ),
          ),
        ],
      ),
    );
  }

  // ── 圖片 tile ──
  Widget _buildImageTile({required Widget child, VoidCallback? onRemove}) {
    return Stack(
      children: [
        Container(
          width: 80.w,
          height: 80.w,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(16.r),
            border: Border.all(
              color: AppColors.textPrimary.withValues(alpha: 0.65),
              width: 2,
            ),
          ),
          clipBehavior: Clip.antiAlias,
          child: child,
        ),
        if (onRemove != null)
          Positioned(
            top: 4,
            right: 4,
            child: GestureDetector(
              onTap: onRemove,
              child: Container(
                width: 20.w,
                height: 20.w,
                decoration: BoxDecoration(
                  color: AppColors.textEmphasis.withValues(alpha: 0.6),
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
    );
  }

  Widget _buildAddImageTile(VoidCallback onTap) {
    return GestureDetector(
      onTap: onTap,
      child: Container(
        width: 80.w,
        height: 80.w,
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          border: Border.all(
            color: AppColors.textPrimary.withValues(alpha: 0.5),
            width: 2,
          ),
        ),
        child: Column(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            Icon(AppIcons.add, size: 24.sp),
            SizedBox(height: 6.h),
            Text(
              '新增圖片',
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w700,
                color: AppColors.textPrimary.withValues(alpha: 0.5),
                height: 20 / 14,
              ),
            ),
          ],
        ),
      ),
    );
  }
}
