import '../../../features/seller/data/current_store.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:url_launcher/url_launcher.dart';
import '../../../config/image_config.dart';
import '../data/product_description_parser.dart';
import '../data/product_repository.dart';
import '../../../shared/utils/price_display.dart';
import '../../../shared/utils/product_image_resolver.dart';
import '../../media/data/media_repository.dart';
import '../../../shared/widgets/location_icon.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/optimized_image.dart';
import '../../seller/presentation/seller_store_avatar.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';

/// fontSize 可能為數字或 px 字串（web 存 "26px"）→ 統一解析為 double。
double parseDescFontSize(dynamic v) {
  if (v is num) return v.toDouble();
  if (v is String) {
    final m = RegExp(r'(\d+(?:\.\d+)?)').firstMatch(v);
    if (m != null) return double.tryParse(m.group(1)!) ?? 14.0;
  }
  return 14.0;
}

Color? parseDescHexColor(dynamic raw) {
  if (raw is! String) return null;
  final hex = raw.replaceFirst('#', '');
  if (hex.length == 6 || hex.length == 8) {
    final argb = hex.length == 6 ? 'FF$hex' : hex;
    final v = int.tryParse(argb, radix: 16);
    if (v != null) return Color(v);
  }
  return null;
}

/// inline runs（02 §3.6）→ TextSpan 清單：逐 run 套粗體/斜體/底線/顏色/連結。
List<InlineSpan> buildDescRunSpans(List<dynamic> runs, TextStyle base) {
  final spans = <InlineSpan>[];
  for (final r in runs) {
    if (r is! Map) continue;
    final text = r['text']?.toString() ?? '';
    if (text.isEmpty) continue;
    var style = base;
    if (r['bold'] == true) style = style.copyWith(fontWeight: FontWeight.bold);
    if (r['italic'] == true) {
      style = style.copyWith(fontStyle: FontStyle.italic);
    }
    final color = parseDescHexColor(r['color']);
    if (color != null) style = style.copyWith(color: color);
    final href = r['href'] is String ? r['href'] as String : null;
    final isLink =
        href != null &&
        (href.startsWith('http://') || href.startsWith('https://'));
    final decorations = <TextDecoration>[
      if (r['underline'] == true) TextDecoration.underline,
      if (isLink) TextDecoration.underline,
    ];
    if (isLink) {
      style = style.copyWith(
        color: AppColors.success,
        decoration: TextDecoration.combine(decorations),
      );
      spans.add(
        TextSpan(
          text: text,
          style: style,
          recognizer: TapGestureRecognizer()
            ..onTap = () {
              launchUrl(Uri.parse(href), mode: LaunchMode.externalApplication);
            },
        ),
      );
    } else {
      if (decorations.isNotEmpty) {
        style = style.copyWith(decoration: TextDecoration.combine(decorations));
      }
      spans.add(TextSpan(text: text, style: style));
    }
  }
  return spans;
}

/// 表格 block（tableJson）→ 橫向捲動 Table（使用者核准橫向捲動）。
Widget? buildDescPreviewTable(dynamic tableJson) {
  if (tableJson is! String || tableJson.isEmpty) return null;
  Map<String, dynamic> model;
  try {
    model = jsonDecode(tableJson) as Map<String, dynamic>;
  } catch (_) {
    return null;
  }
  final rows = model['rows'];
  if (rows is! List || rows.isEmpty) return null;
  final withHeadings = model['withHeadings'] == true;

  final baseStyle = TextStyle(
    fontSize: 14.sp,
    color: AppColors.textPrimary,
    height: 1.4,
  );
  var maxCols = 0;
  for (final row in rows) {
    if (row is List && row.length > maxCols) maxCols = row.length;
  }
  if (maxCols == 0) return null;

  final tableRows = <TableRow>[];
  for (var i = 0; i < rows.length; i++) {
    final row = rows[i];
    if (row is! List) continue;
    final isHead = withHeadings && i == 0;
    final cellStyle = isHead
        ? baseStyle.copyWith(
            fontWeight: FontWeight.bold,
            color: AppColors.textEmphasis,
          )
        : baseStyle;
    final cells = <Widget>[];
    for (var c = 0; c < maxCols; c++) {
      final cell = c < row.length ? row[c] : null;
      final cellRuns = (cell is Map && cell['runs'] is List)
          ? List<dynamic>.from(cell['runs'] as List)
          : const <dynamic>[];
      cells.add(
        Padding(
          padding: EdgeInsets.symmetric(horizontal: 8.w, vertical: 8.h),
          child: RichText(
            text: TextSpan(children: buildDescRunSpans(cellRuns, cellStyle)),
          ),
        ),
      );
    }
    tableRows.add(
      TableRow(
        decoration: isHead
            ? const BoxDecoration(color: AppColors.gray100)
            : null,
        children: cells,
      ),
    );
  }
  if (tableRows.isEmpty) return null;

  return SingleChildScrollView(
    scrollDirection: Axis.horizontal,
    child: Table(
      border: TableBorder.all(color: AppColors.dividerLight, width: 1),
      defaultColumnWidth: const IntrinsicColumnWidth(),
      children: tableRows,
    ),
  );
}

/// 清單 block（listJson `{ordered, items:[{runs}]}`）→ 有序帶序號 / 無序帶項目符號。
/// 每個 item 的 runs 複用 buildDescRunSpans 渲染（同 table cell）。
Widget? buildDescPreviewList(dynamic listJson) {
  if (listJson is! String || listJson.isEmpty) return null;
  Map<String, dynamic> model;
  try {
    model = jsonDecode(listJson) as Map<String, dynamic>;
  } catch (_) {
    return null;
  }
  final items = model['items'];
  if (items is! List || items.isEmpty) return null;
  final ordered = model['ordered'] == true;

  final baseStyle = TextStyle(
    fontSize: 14.sp,
    color: AppColors.textPrimary,
    height: 1.5,
  );

  final rows = <Widget>[];
  for (var i = 0; i < items.length; i++) {
    final item = items[i];
    final itemRuns = (item is Map && item['runs'] is List)
        ? List<dynamic>.from(item['runs'] as List)
        : const <dynamic>[];
    final marker = ordered ? '${i + 1}.' : '•';
    rows.add(
      Padding(
        padding: EdgeInsets.symmetric(vertical: 4.h),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.baseline,
          textBaseline: TextBaseline.alphabetic,
          children: [
            SizedBox(
              width: ordered ? 22.w : 16.w,
              child: Text(
                marker,
                textAlign: ordered ? TextAlign.right : TextAlign.left,
                style: baseStyle,
              ),
            ),
            SizedBox(width: 6.w),
            Expanded(
              child: RichText(
                text: TextSpan(
                  children: buildDescRunSpans(itemRuns, baseStyle),
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }
  if (rows.isEmpty) return null;
  return Column(crossAxisAlignment: CrossAxisAlignment.start, children: rows);
}

/// 商品預覽頁：模擬買家視角，含圖片輪播、規格選擇、評論與認證標章展示
class ProductPreviewScreen extends HookConsumerWidget {
  const ProductPreviewScreen({super.key, required this.product});

  final Map<String, dynamic> product;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final imagePageController = usePageController();
    final currentImageIndex = useState(0);
    final imageUrls = useState<List<String>>([]);
    final grades = useState<List<Map<String, dynamic>>>([]);
    final reviews = useState<List<Map<String, dynamic>>>([]);
    final selectedGradeIndex = useState<int?>(null);
    final loading = useState(true);
    final avgRating = useState<double?>(null);
    final reviewCount = useState<int?>(null);
    final mine = ref.watch(myStoreSnapshotProvider);

    final categoryL1 = useState<String?>(null);
    final categoryL2 = useState<String?>(null);
    final categoryL3 = useState<String?>(null);
    final origin = useState<String?>(null);
    final description = useState<String?>(null);
    final descriptionBlocks = useState<List<Map<String, dynamic>>?>(null);

    Future<List<Map<String, dynamic>>> loadReviews(
      String sellerId,
      Object skuId,
    ) async {
      try {
        final reviewList = await ref
            .read(productRepositoryProvider)
            .listPublicProductReviews(sellerId, skuId);
        if (context.mounted) {
          reviews.value = reviewList.take(2).toList();
        }
        return reviewList;
      } catch (e) {
        return [];
      }
    }

    Future<void> loadProductDetails() async {
      final p = product;
      // `ProductItem` 的賣家鍵是 `storeId`（§6 身分分離）。
      final String? sellerId = (p['storeId'] ?? ref.read(sellerStoreIdProvider))
          ?.toString();

      final productId = p['id'] ?? p['productId'];

      if (productId == null) {
        debugPrint(
          '[ProductPreview] No productId found in widget.product keys: ${p.keys}',
        );
        loading.value = false;
        return;
      }

      debugPrint(
        '[ProductPreview] Loading product detail for productId=$productId, sellerId=$sellerId',
      );

      try {
        // 商品評分摘要（`/public/products/{id}/rating`）與評論都讀公開端點。
        final results = await Future.wait([
          ref.read(productRepositoryProvider).getProductDetail(productId),
          ref
              .read(productRepositoryProvider)
              .getPublicProductRatingSummary(productId),
          if (sellerId != null)
            loadReviews(sellerId, productId)
          else
            Future.value(<Map<String, dynamic>>[]),
        ]);

        if (!context.mounted) return;

        final productResult = results[0] as Map<String, dynamic>;
        final productData =
            productResult['data'] as Map<String, dynamic>? ?? {};
        debugPrint(
          '[ProductPreview] Product detail loaded, keys: ${productData.keys}',
        );

        List<String> resolvedImageUrls = extractProductImageUrls(productData);
        debugPrint(
          '[ProductPreview] Parsed ${resolvedImageUrls.length} product image URLs: $resolvedImageUrls',
        );

        if (resolvedImageUrls.isEmpty) {
          final desc = productData['description'];
          if (desc is Map) {
            final blocks = desc['blocks'];
            if (blocks is List) {
              final descObjectKeys = <String>[];
              for (final b in blocks) {
                if (b is Map && b['type'] == 'image') {
                  final key = (b['imageKey'] ?? '').toString();
                  if (key.isNotEmpty) {
                    descObjectKeys.add(key);
                  }
                }
              }
              if (descObjectKeys.isNotEmpty) {
                final resolvedMap = MediaRepository.resolveUrls(descObjectKeys);
                for (final key in descObjectKeys) {
                  final url = resolvedMap[key];
                  if (url != null && url.isNotEmpty) {
                    resolvedImageUrls.add(url);
                  }
                }
              }
            }
          }
        }
        List<Map<String, dynamic>> loadedGrades = [];
        final variants = productData['variants'] ?? productData['skus'];
        if (variants is List && variants.isNotEmpty) {
          loadedGrades = variants.whereType<Map<String, dynamic>>().toList();
        }

        final loadedOrigin = productData['origin']?.toString();

        final parsedDescription = parseProductDescription(
          productData['description'],
        );
        final loadedDesc = parsedDescription.text;
        final loadedBlocks = parsedDescription.blocks.isNotEmpty
            ? parsedDescription.blocks
            : null;

        final categoryId =
            productData['category_id'] ?? productData['categoryId'];
        debugPrint(
          '[ProductPreview] category_id=$categoryId (type: ${categoryId.runtimeType})',
        );
        if (categoryId != null && context.mounted) {
          final catId = categoryId is int
              ? categoryId
              : int.tryParse(categoryId.toString());
          if (catId != null && catId > 0) {
            try {
              final categoryPath = await ref
                  .read(productRepositoryProvider)
                  .getCategoryPath(catId);
              debugPrint(
                '[ProductPreview] Category path result: $categoryPath',
              );
              if (context.mounted) {
                final rawData = categoryPath['data'];
                if (rawData is List && rawData.isNotEmpty) {
                  categoryL1.value = rawData.isNotEmpty
                      ? rawData[0]['name']?.toString()
                      : null;
                  categoryL2.value = rawData.length > 1
                      ? rawData[1]['name']?.toString()
                      : null;
                  categoryL3.value = rawData.length > 2
                      ? rawData[2]['name']?.toString()
                      : null;
                }
                debugPrint(
                  '[ProductPreview] Set categories: L1=${categoryL1.value}, L2=${categoryL2.value}, L3=${categoryL3.value}',
                );
              }
            } catch (e) {
              debugPrint('[ProductPreview] Failed to load category path: $e');
            }
          }
        }

        if (!context.mounted) return;

        final ratingSummary = results[1] as Map<String, dynamic>? ?? {};
        final ratingData = ratingSummary['data'] is Map
            ? ratingSummary['data'] as Map<String, dynamic>
            : ratingSummary;

        imageUrls.value = resolvedImageUrls;
        grades.value = loadedGrades;
        // OpenAPI handler.ProductRatingResponse：data.{rating, reviewCount}（string）
        // 之前讀 'avg'/'count' 永遠拿不到 → avgRating.value 恆為 0；同時讓
        // buildSellerCard (line 443: sellerRating ?? avgRating.value) 也壞。
        // 一併保留舊欄位 fallback 不破壞向下相容。
        final avgVal =
            ratingData['rating'] ??
            ratingData['avg'] ??
            ratingData['averageRating'] ??
            0;
        avgRating.value =
            (avgVal is num ? avgVal : num.tryParse(avgVal.toString()) ?? 0)
                .toDouble();
        final countVal =
            ratingData['reviewCount'] ??
            ratingData['count'] ??
            ratingData['totalReviews'] ??
            0;
        reviewCount.value =
            (countVal is num
                    ? countVal
                    : num.tryParse(countVal.toString()) ?? 0)
                .toInt();

        origin.value = loadedOrigin;
        description.value = loadedDesc;
        descriptionBlocks.value = loadedBlocks;
        if (loadedGrades.isNotEmpty) {
          selectedGradeIndex.value = 0;
        }
        loading.value = false;
      } catch (e) {
        debugPrint('[ProductPreview] Error loading details: $e');
        if (context.mounted) {
          loading.value = false;
        }
      }
    }

    useEffect(() {
      loadProductDetails();
      return null;
    }, const []);

    final p = product;
    final productName = (p['productName'] ?? p['name'] ?? p['skuName'] ?? '商品')
        .toString();
    final category = categoryL2.value ?? p['categoryL2'] ?? p['category'] ?? '';
    final subCategory =
        categoryL3.value ?? p['categoryL3'] ?? p['subCategory'] ?? '';
    final originVal =
        origin.value ?? (p['origin'] ?? p['productOrigin'] ?? '').toString();
    final fallbackDescription = parseProductDescription(
      p['description'] ?? p['productDescription'],
    );
    final descriptionVal = (description.value ?? '').trim().isNotEmpty
        ? description.value!.trim()
        : fallbackDescription.text;
    final effectiveDescriptionBlocks =
        descriptionBlocks.value != null && descriptionBlocks.value!.isNotEmpty
        ? descriptionBlocks.value!
        : fallbackDescription.blocks;
    final address = (mine?.addressText ?? '');

    Widget buildImageCarousel() {
      return Column(
        children: [
          Padding(
            padding: EdgeInsets.only(
              top: 16.h,
              bottom: 8.h,
              left: 24.w,
              right: 24.w,
            ),
            child: AspectRatio(
              aspectRatio: 4 / 3,
              child: imageUrls.value.isEmpty
                  ? Container(
                      decoration: BoxDecoration(
                        color: AppColors.bgPrimary,
                        borderRadius: BorderRadius.circular(16.r),
                      ),
                      child: Center(
                        child: Icon(
                          AppIcons.imageOutlined,
                          size: 64.sp,
                          color: AppIconColors.disabled,
                        ),
                      ),
                    )
                  : ClipRRect(
                      borderRadius: BorderRadius.circular(16.r),
                      child: PageView.builder(
                        controller: imagePageController,
                        onPageChanged: (index) =>
                            currentImageIndex.value = index,
                        itemCount: imageUrls.value.length,
                        itemBuilder: (context, index) {
                          return Container(
                            color: AppColors.bgPrimary,
                            child: OptimizedNetworkImage(
                              imageUrl: imageUrls.value[index],
                              scene: ImageScene.productDetail,
                              fit: BoxFit.contain,
                              errorWidget: Container(
                                color: AppColors.bgPrimary,
                                child: Icon(
                                  AppIcons.brokenImageOutlined,
                                  size: 64.sp,
                                  color: AppIconColors.disabled,
                                ),
                              ),
                            ),
                          );
                        },
                      ),
                    ),
            ),
          ),
          if (imageUrls.value.length > 1)
            Row(
              mainAxisAlignment: MainAxisAlignment.center,
              children: List.generate(imageUrls.value.length, (index) {
                final isActive = index == currentImageIndex.value;
                return GestureDetector(
                  onTap: () => imagePageController.animateToPage(
                    index,
                    duration: const Duration(milliseconds: 300),
                    curve: Curves.easeInOut,
                  ),
                  child: Container(
                    width: 8.w,
                    height: 8.h,
                    margin: EdgeInsets.symmetric(horizontal: 4.w),
                    decoration: BoxDecoration(
                      shape: BoxShape.circle,
                      color: isActive
                          ? AppColors.brandPrimaryStrong
                          : AppColors.cardBorder,
                    ),
                  ),
                );
              }),
            ),
        ],
      );
    }

    Widget buildSellerCard() {
      final mName = mine?.storeName ?? '';
      final rating = mine?.ratingAverage;

      // This is the seller's own preview, not a navigation target. Keeping a
      // bounce gesture with an empty callback made the card look actionable.
      return Container(
        margin: EdgeInsets.symmetric(horizontal: 24.w),
        padding: EdgeInsets.symmetric(horizontal: 20.w, vertical: 16.h),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 12,
              offset: Offset(0, 4.h),
            ),
          ],
        ),
        child: Row(
          children: [
            SellerStoreAvatar(size: 40),
            SizedBox(width: 7.w),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    mName,
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.bold,
                      color: AppColors.textPrimary,
                    ),
                  ),
                  Row(
                    children: [
                      Icon(
                        AppIcons.star,
                        size: 12.sp,
                        color: AppIconColors.gold,
                      ),
                      SizedBox(width: 4.w),
                      Text(
                        rating == null || rating <= 0
                            ? '--'
                            : rating.toStringAsFixed(1),
                        style: TextStyle(
                          fontSize: 14.sp,
                          color: AppColors.textPrimary,
                        ),
                      ),
                    ],
                  ),
                ],
              ),
            ),
          ],
        ),
      );
    }

    List<Widget> buildDescriptionBlocks(List<Map<String, dynamic>> blocks) {
      final widgets = <Widget>[];
      for (final block in blocks) {
        final type = block['type']?.toString() ?? 'text';
        if (type == 'image') {
          final url = block['imageUrl']?.toString() ?? '';
          final imageKey = block['imageKey']?.toString() ?? '';
          widgets.add(
            Padding(
              padding: EdgeInsets.symmetric(vertical: 8.h),
              child: _DescriptionImage(url: url, imageKey: imageKey),
            ),
          );
        } else if (type == 'table') {
          final tableWidget = buildDescPreviewTable(block['tableJson']);
          if (tableWidget != null) {
            widgets.add(
              Padding(
                padding: EdgeInsets.symmetric(vertical: 8.h),
                child: tableWidget,
              ),
            );
          }
        } else if (type == 'list') {
          final listWidget = buildDescPreviewList(block['listJson']);
          if (listWidget != null) {
            widgets.add(
              Padding(
                padding: EdgeInsets.symmetric(vertical: 8.h),
                child: listWidget,
              ),
            );
          }
        } else {
          final styleMap = block['style'] as Map<String, dynamic>? ?? {};
          final blockFontSize = parseDescFontSize(styleMap['fontSize']);
          final alignStr = styleMap['alignment']?.toString() ?? 'left';
          final textAlign = alignStr == 'center'
              ? TextAlign.center
              : alignStr == 'right'
              ? TextAlign.right
              : TextAlign.left;
          FontWeight blockFontWeight = FontWeight.w400;
          if (styleMap['fontWeight'] is num) {
            final wVal = (styleMap['fontWeight'] as num).toInt();
            final wIndex = (wVal ~/ 100) - 1;
            if (wIndex >= 0 && wIndex < FontWeight.values.length) {
              blockFontWeight = FontWeight.values[wIndex];
            }
          }
          const defaultColor = AppColors.textPrimary;
          final baseStyle = TextStyle(
            fontSize: blockFontSize.sp,
            fontWeight: blockFontWeight,
            color: defaultColor,
            height: 1.5,
          );

          final runs = block['runs'];
          if (runs is List && runs.isNotEmpty) {
            widgets.add(
              Padding(
                padding: EdgeInsets.symmetric(vertical: 4.h),
                child: SizedBox(
                  width: double.infinity,
                  child: RichText(
                    textAlign: textAlign,
                    text: TextSpan(
                      children: buildDescRunSpans(runs, baseStyle),
                    ),
                  ),
                ),
              ),
            );
          } else {
            final content = (block['content'] ?? block['text'] ?? '')
                .toString();
            if (content.isNotEmpty) {
              final isBold = styleMap['bold'] == true;
              final isItalic = styleMap['italic'] == true;
              Color legacyColor = defaultColor;
              final c = parseDescHexColor(styleMap['color']);
              if (c != null) legacyColor = c;
              widgets.add(
                Padding(
                  padding: EdgeInsets.symmetric(vertical: 4.h),
                  child: SizedBox(
                    width: double.infinity,
                    child: Text(
                      content,
                      textAlign: textAlign,
                      style: baseStyle.copyWith(
                        fontWeight: isBold ? FontWeight.bold : blockFontWeight,
                        fontStyle: isItalic
                            ? FontStyle.italic
                            : FontStyle.normal,
                        color: legacyColor,
                      ),
                    ),
                  ),
                ),
              );
            }
          }
        }
      }
      if (widgets.isEmpty) {
        widgets.add(
          Text(
            '--',
            style: TextStyle(
              fontSize: 14.sp,
              color: AppColors.textPrimary,
              height: 1.5,
            ),
          ),
        );
      }
      return widgets;
    }

    Widget buildProductDetails() {
      return Container(
        margin: EdgeInsets.symmetric(horizontal: 24.w),
        padding: EdgeInsets.all(16.w),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 12,
              offset: Offset(0, 4.h),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              productName,
              style: TextStyle(
                fontSize: 18.sp,
                fontWeight: FontWeight.bold,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 8.h),
            // 商品分類：L2 - L3 合併單行顯示（移除原本的「詳細分類」獨立欄位）
            Text(
              '商品分類：${_joinCategoryDisplay(category.toString(), subCategory.toString())}',
              style: TextStyle(fontSize: 16.sp, color: AppColors.textPrimary),
            ),
            SizedBox(height: 8.h),
            Text(
              '產地：${originVal.trim().isEmpty ? '未設定' : originVal}',
              style: TextStyle(fontSize: 16.sp, color: AppColors.textPrimary),
            ),
            SizedBox(height: 8.h),
            Row(
              children: [
                HawkrLocationIcon(
                  size: 24.sp,
                  color: AppIconColors.defaultColor,
                ),
                SizedBox(width: 5.w),
                Expanded(
                  child: Text(
                    address.trim().isEmpty ? '店舖地址尚未設定' : address,
                    style: TextStyle(
                      fontSize: 16.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      );
    }

    Widget buildDescriptionCard() {
      return Container(
        margin: EdgeInsets.symmetric(horizontal: 24.w),
        padding: EdgeInsets.all(16.w),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 12,
              offset: Offset(0, 4.h),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '商品說明',
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.bold,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 8.h),
            const Divider(color: AppColors.gray100, height: 1),
            SizedBox(height: 8.h),
            if (effectiveDescriptionBlocks.isNotEmpty)
              ...buildDescriptionBlocks(effectiveDescriptionBlocks)
            else
              Text(
                descriptionVal.isNotEmpty ? descriptionVal : '--',
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textPrimary,
                  height: 1.5,
                ),
              ),
          ],
        ),
      );
    }

    Widget buildReviewItem(Map<String, dynamic> review, {bool isLast = false}) {
      // `ReviewListItem`（gateway/internal/handler/review.go）的欄位是 `username`。
      final rawName = (review['username'] ?? '').toString();
      final userName = rawName.isEmpty ? '匿名' : rawName;
      final maskedName = userName.length > 1
          ? '${userName[0]}${'*' * (userName.length - 1)}'
          : userName;
      // 後端 rating 有時是 int / 有時是 string（"5"）→ 用 tryParse 容錯
      final ratingRaw = review['rating'] ?? 5;
      final num rating = ratingRaw is num
          ? ratingRaw
          : (num.tryParse(ratingRaw.toString()) ?? 5);
      final content = review['content'] ?? '';
      final date = review['createdAt'] ?? '';
      final dateStr = date.toString().length >= 10
          ? date.toString().substring(0, 10)
          : date.toString();

      return Container(
        padding: EdgeInsets.only(bottom: isLast ? 0 : 16),
        decoration: BoxDecoration(
          border: isLast
              ? null
              : const Border(bottom: BorderSide(color: AppColors.bgPrimary)),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Row(
                    children: [
                      Container(
                        width: 24.w,
                        height: 24.h,
                        decoration: const BoxDecoration(
                          shape: BoxShape.circle,
                          color: AppColors.bgPrimary,
                        ),
                        child: Center(
                          child: Text(
                            userName.isNotEmpty
                                ? userName[0].toUpperCase()
                                : 'U',
                            style: TextStyle(
                              fontSize: 12.sp,
                              fontWeight: FontWeight.bold,
                              color: AppColors.primary,
                            ),
                          ),
                        ),
                      ),
                      SizedBox(width: 8.w),
                      Expanded(
                        child: Text(
                          maskedName,
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.textPrimary,
                          ),
                        ),
                      ),
                    ],
                  ),
                ),
                SizedBox(width: 8.w),
                Text(
                  dateStr,
                  style: TextStyle(
                    fontSize: 12.sp,
                    color: AppColors.textSecondary,
                  ),
                ),
              ],
            ),
            SizedBox(height: 8.h),
            Row(
              children: List.generate(5, (i) {
                return Icon(
                  i < rating ? AppIcons.star : AppIcons.starBorder,
                  size: 12.sp,
                  color: AppIconColors.gold,
                );
              }),
            ),
            SizedBox(height: 8.h),
            Text(
              content,
              style: TextStyle(fontSize: 14.sp, color: AppColors.textPrimary),
              maxLines: 2,
              overflow: TextOverflow.ellipsis,
            ),
          ],
        ),
      );
    }

    Widget buildReviewSection() {
      final rCount = reviewCount.value ?? reviews.value.length;

      return Container(
        margin: EdgeInsets.symmetric(horizontal: 24.w),
        padding: EdgeInsets.all(16.w),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 12,
              offset: Offset(0, 4.h),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              mainAxisAlignment: MainAxisAlignment.spaceBetween,
              children: [
                Expanded(
                  child: Text(
                    '商品評價 ($rCount)',
                    style: TextStyle(
                      fontSize: 16.sp,
                      fontWeight: FontWeight.bold,
                      color: AppColors.textEmphasis,
                    ),
                  ),
                ),
                _BounceOnTap(
                  onTap: () {
                    final sellerId =
                        p['merchantId'] ??
                        p['merchant_id'] ??
                        ref.read(sellerStoreIdProvider);
                    final skuId = p['skuId'] ?? p['sku_id'] ?? p['id'];
                    final pName =
                        (p['productName'] ?? p['name'] ?? p['skuName'] ?? '商品')
                            .toString();

                    if (sellerId != null && skuId != null) {
                      context.push(
                        '/product/reviews',
                        extra: {
                          'sellerId': sellerId.toString(),
                          'skuId': skuId.toString(),
                          'productName': pName,
                        },
                      );
                    }
                  },
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(minHeight: 48),
                    child: Padding(
                      padding: EdgeInsets.only(left: 8.w),
                      child: Center(
                        child: Text(
                          '查看全部 >',
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.primary,
                          ),
                        ),
                      ),
                    ),
                  ),
                ),
              ],
            ),
            SizedBox(height: 16.h),
            if (reviews.value.isEmpty)
              Center(
                child: Padding(
                  padding: EdgeInsets.symmetric(vertical: 16.h),
                  child: Text(
                    '暫無評價',
                    style: TextStyle(
                      fontSize: 14.sp,
                      color: AppColors.textSecondary,
                    ),
                  ),
                ),
              )
            else
              ...List.generate(reviews.value.length.clamp(0, 2), (index) {
                final review = reviews.value[index];
                return buildReviewItem(
                  review,
                  isLast: index == reviews.value.length - 1,
                );
              }),
          ],
        ),
      );
    }

    Widget buildSpecSection() {
      final gradesVal = grades.value;
      final selIdx = selectedGradeIndex.value;
      final selectedGrade = selIdx != null && selIdx < gradesVal.length
          ? gradesVal[selIdx]
          : null;
      // PayUni #758: effective = salePrice ?? price；salePrice 有效時顯示原價刪除線
      final rawSkuPrice =
          selectedGrade?['price'] ?? p['price'] ?? p['priceMin'];
      final rawSkuSalePrice =
          selectedGrade?['salePrice'] ?? selectedGrade?['sale_price'];
      final effectivePrice =
          PriceDisplay.effectiveSkuPrice(
            price: rawSkuPrice,
            salePrice: rawSkuSalePrice,
          ) ??
          0;
      final hasSale = PriceDisplay.hasSkuSale(
        price: rawSkuPrice,
        salePrice: rawSkuSalePrice,
      );
      final originalPrice = PriceDisplay.parseMoney(rawSkuPrice);
      final quantity =
          selectedGrade?['stock_qty'] ??
          selectedGrade?['stockQty'] ??
          selectedGrade?['quantity'] ??
          selectedGrade?['remainingQuantity'] ??
          p['quantity'] ??
          0;

      return Container(
        margin: EdgeInsets.symmetric(horizontal: 24.w),
        padding: EdgeInsets.all(16.w),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(16.r),
          boxShadow: [
            BoxShadow(
              color: AppColors.cardShadow,
              blurRadius: 12,
              offset: Offset(0, 4.h),
            ),
          ],
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              '選擇規格',
              style: TextStyle(
                fontSize: 16.sp,
                fontWeight: FontWeight.bold,
                color: AppColors.textEmphasis,
              ),
            ),
            SizedBox(height: 10.h),
            if (gradesVal.isEmpty)
              Text(
                '暫無規格',
                style: TextStyle(
                  fontSize: 14.sp,
                  color: AppColors.textSecondary,
                ),
              )
            else
              Wrap(
                spacing: 14,
                runSpacing: 10,
                children: List.generate(gradesVal.length, (index) {
                  final grade = gradesVal[index];
                  final weight = grade['weight'];
                  final unit = grade['unit'] ?? '';
                  final specLevel = grade['spec_level'] ?? '';
                  final label =
                      grade['label'] ??
                      grade['name'] ??
                      grade['skuName'] ??
                      (weight != null
                          ? '$weight$unit'
                          : specLevel.isNotEmpty
                          ? specLevel
                          : '規格 ${index + 1}');
                  final isSelected = selectedGradeIndex.value == index;
                  return _BounceOnTap(
                    onTap: () => selectedGradeIndex.value = index,
                    child: Container(
                      padding: EdgeInsets.symmetric(
                        horizontal: 20.w,
                        vertical: 12.h,
                      ),
                      decoration: BoxDecoration(
                        color: isSelected
                            ? AppColors.primary
                            : AppColors.bgCard,
                        borderRadius: BorderRadius.circular(8.r),
                        border: Border.all(
                          color: isSelected
                              ? AppColors.primary
                              : AppColors.cardBorder,
                        ),
                      ),
                      child: Text(
                        label,
                        style: TextStyle(
                          fontSize: 16.sp,
                          fontWeight: isSelected
                              ? FontWeight.bold
                              : FontWeight.normal,
                          color: isSelected
                              ? AppColors.bgCard
                              : AppColors.primary,
                        ),
                      ),
                    ),
                  );
                }),
              ),
            SizedBox(height: 10.h),
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(
                  AppIcons.monetizationOnOutlined,
                  size: 24.sp,
                  color: AppIconColors.defaultColor,
                ),
                SizedBox(width: 5.w),
                Expanded(
                  child: Wrap(
                    spacing: 8.w,
                    runSpacing: 4.h,
                    crossAxisAlignment: WrapCrossAlignment.center,
                    children: [
                      Text(
                        '\$${effectivePrice.toInt()}',
                        style: TextStyle(
                          fontSize: 20.sp,
                          fontWeight: FontWeight.bold,
                          color: AppColors.warning,
                        ),
                      ),
                      if (hasSale && originalPrice != null)
                        Text(
                          '\$${originalPrice.toInt()}',
                          style: TextStyle(
                            fontSize: 14.sp,
                            color: AppColors.textSecondary,
                            decoration: TextDecoration.lineThrough,
                            decorationColor: AppColors.textSecondary,
                          ),
                        ),
                    ],
                  ),
                ),
              ],
            ),
            SizedBox(height: 8.h),
            Row(
              children: [
                HkIcon(
                  HkIconName.inventory,
                  size: 24.sp,
                  color: AppIconColors.defaultColor,
                ),
                SizedBox(width: 5.w),
                Expanded(
                  child: Text(
                    '庫存：$quantity 件',
                    style: TextStyle(
                      fontSize: 16.sp,
                      color: AppColors.textPrimary,
                    ),
                  ),
                ),
              ],
            ),
          ],
        ),
      );
    }

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: HkGlassAppBar(title: Text(productName), subtitle: '買家視角預覽'),
      body: loading.value
          ? const TruckLoadingOverlay()
          : SingleChildScrollView(
              padding: EdgeInsets.only(bottom: 32.h),
              child: Column(
                children: [
                  SizedBox(height: 16.h),
                  buildImageCarousel(),
                  SizedBox(height: 16.h),
                  buildSellerCard(),
                  SizedBox(height: 16.h),
                  buildProductDetails(),
                  SizedBox(height: 16.h),
                  buildDescriptionCard(),
                  SizedBox(height: 16.h),
                  buildReviewSection(),
                  SizedBox(height: 16.h),
                  buildSpecSection(),
                ],
              ),
            ),
    );
  }
}

/// 「商品分類」顯示用：L2 - L3 合併（缺一層時 graceful fallback）
String _joinCategoryDisplay(String l2, String l3) {
  final hasL2 = l2.trim().isNotEmpty;
  final hasL3 = l3.trim().isNotEmpty;
  if (hasL2 && hasL3) return '$l2 - $l3';
  if (hasL2) return l2;
  if (hasL3) return l3;
  return '--';
}

/// 描述區塊中的圖片 Widget
/// 會自動從 imageUrl 或 imageKey 解析出可用的圖片 URL
/// （契約：`DescriptionBlockResponse` / `PublicDescriptionBlock`，
/// gateway/internal/handler/product.go；兩者都是 `omitempty`）。
class _DescriptionImage extends StatelessWidget {
  const _DescriptionImage({required this.url, required this.imageKey});
  final String url;
  final String imageKey;

  @override
  Widget build(BuildContext context) {
    // imageUrl || imageKey：兩個欄位都是 omitempty，個別 block 可能只有 key。
    // MediaRepository.resolveUrl 兩種輸入都吃，但優先用 URL 可避免多餘的 CDN 拼接。
    final key = url.isNotEmpty ? url : imageKey;
    final resolvedUrl = MediaRepository.resolveUrl(key);

    if (resolvedUrl == null || resolvedUrl.isEmpty) {
      return Container(
        height: 120.h,
        decoration: BoxDecoration(
          color: AppColors.gray100,
          borderRadius: BorderRadius.circular(8.r),
        ),
        child: Center(
          child: Icon(
            AppIcons.brokenImageOutlined,
            size: 32.sp,
            color: AppIconColors.disabled,
          ),
        ),
      );
    }

    return ClipRRect(
      borderRadius: BorderRadius.circular(8.r),
      child: OptimizedNetworkImage(
        imageUrl: resolvedUrl,
        scene: ImageScene.productDetail,
        width: double.infinity,
        fit: BoxFit.fitWidth,
        errorWidget: Container(
          height: 120.h,
          decoration: BoxDecoration(
            color: AppColors.gray100,
            borderRadius: BorderRadius.circular(8.r),
          ),
          child: Center(
            child: Icon(
              AppIcons.brokenImageOutlined,
              size: 32.sp,
              color: AppIconColors.disabled,
            ),
          ),
        ),
      ),
    );
  }
}

/// 點擊時產生 bounce 縮放動畫的 wrapper
class _BounceOnTap extends StatefulWidget {
  const _BounceOnTap({required this.onTap, required this.child});
  final VoidCallback onTap;
  final Widget child;

  @override
  State<_BounceOnTap> createState() => _BounceOnTapState();
}

class _BounceOnTapState extends State<_BounceOnTap>
    with SingleTickerProviderStateMixin {
  late final AnimationController _ctrl;
  late final Animation<double> _scale;

  @override
  void initState() {
    super.initState();
    _ctrl = AnimationController(
      vsync: this,
      duration: const Duration(milliseconds: 150),
      reverseDuration: const Duration(milliseconds: 100),
    );
    _scale = Tween<double>(
      begin: 1.0,
      end: 0.92,
    ).animate(CurvedAnimation(parent: _ctrl, curve: Curves.easeInOut));
  }

  @override
  void dispose() {
    _ctrl.dispose();
    super.dispose();
  }

  Future<void> _handleTap() async {
    await _ctrl.forward();
    await _ctrl.reverse();
    widget.onTap();
  }

  @override
  Widget build(BuildContext context) {
    return GestureDetector(
      onTap: _handleTap,
      child: ScaleTransition(scale: _scale, child: widget.child),
    );
  }
}
