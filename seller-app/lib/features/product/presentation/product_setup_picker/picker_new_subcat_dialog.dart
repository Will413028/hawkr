import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../data/product_repository.dart';

/// 顯示新增子分類 Dialog
///
/// 用戶輸入名稱後：
/// 1. 立即將新 L3 加入本地 [l3ByL2]（即時顯示）
/// 2. 呼叫 API 建立 active 分類，取得 catalogId
/// 3. 導航至 /product/info 繼續填寫商品資訊
/// 4. 收到結果後 pop picker 頁面並帶回結果
void showPickerNewSubcatDialog({
  required BuildContext context,
  required WidgetRef ref,
  required String parentName,
  required int parentId,
  required String l1Key,
  required String l1Label,
  required int l1Id,
  required bool isEditMode,
  required Map<String, dynamic>? existingProduct,
  required ValueNotifier<Map<String, List<String>>> l3ByL2,
}) {
  final controller = TextEditingController();

  showDialog(
    context: context,
    barrierDismissible: true,
    barrierColor: AppColors.textPrimary,
    builder: (ctx) => StatefulBuilder(
      builder: (dialogContext, setDialogState) {
        final hasText = controller.text.trim().isNotEmpty;

        return Dialog(
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(12.r),
          ),
          insetPadding: EdgeInsets.symmetric(horizontal: 40.w),
          child: Container(
            width: 280.w,
            constraints: const BoxConstraints(maxWidth: 280),
            decoration: BoxDecoration(
              color: AppColors.bgCard,
              borderRadius: BorderRadius.circular(12.r),
            ),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Padding(
                  padding: EdgeInsets.fromLTRB(16.w, 20.h, 16.w, 12.h),
                  child: Text(
                    '新增子分類',
                    textAlign: TextAlign.center,
                    style: TextStyle(
                      color: AppColors.textPrimary,
                      fontSize: 16.sp,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                Padding(
                  padding: EdgeInsets.symmetric(horizontal: 16.w),
                  child: TextField(
                    controller: controller,
                    onChanged: (_) => setDialogState(() {}),
                    decoration: InputDecoration(
                      hintText: '輸入子分類名稱',
                      hintStyle: TextStyle(
                        color: AppColors.textSecondary,
                        fontSize: 14.sp,
                      ),
                      contentPadding: EdgeInsets.symmetric(
                        horizontal: 12.w,
                        vertical: 12.h,
                      ),
                      border: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8.r),
                        borderSide: const BorderSide(
                          color: AppColors.cardBorder,
                        ),
                      ),
                      enabledBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8.r),
                        borderSide: const BorderSide(
                          color: AppColors.cardBorder,
                        ),
                      ),
                      focusedBorder: OutlineInputBorder(
                        borderRadius: BorderRadius.circular(8.r),
                        borderSide: BorderSide(
                          color: AppColors.primary,
                          width: 1.5.w,
                        ),
                      ),
                    ),
                  ),
                ),
                SizedBox(height: 16.h),
                Container(
                  width: double.infinity,
                  height: 1,
                  color: AppColors.textSecondary,
                ),
                SizedBox(
                  height: 44.h,
                  child: Row(
                    children: [
                      Expanded(
                        child: GestureDetector(
                          onTap: () => Navigator.pop(ctx),
                          behavior: HitTestBehavior.opaque,
                          child: Center(
                            child: Text(
                              '取消',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: AppColors.textPrimary,
                                fontSize: 16.sp,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                          ),
                        ),
                      ),
                      Container(
                        width: 1,
                        height: 44.h,
                        color: AppColors.textSecondary,
                      ),
                      Expanded(
                        child: GestureDetector(
                          onTap: hasText
                              ? () async {
                                  final name = controller.text.trim();
                                  Navigator.pop(ctx);

                                  // 立即將新分類加入本地列表
                                  final newL3ByL2 =
                                      Map<String, List<String>>.from(
                                        l3ByL2.value,
                                      );
                                  final l3List = List<String>.from(
                                    newL3ByL2[parentName] ?? [],
                                  );
                                  if (!l3List.contains(name)) {
                                    l3List.add(name);
                                    newL3ByL2[parentName] = l3List;
                                    l3ByL2.value = newL3ByL2;
                                  }

                                  // 建立 active 分類，取得 catalogId
                                  final candidateResult = await ref
                                      .read(productRepositoryProvider)
                                      .submitCategoryCandidate(
                                        name: name,
                                        parentId: parentId,
                                      );

                                  int? catalogId;
                                  if (candidateResult['code'] == 0) {
                                    final data = candidateResult['data'];
                                    final rawId = data is Map
                                        ? (data['id'] ?? data['candidateId'])
                                        : candidateResult['id'];
                                    catalogId = rawId is int
                                        ? rawId
                                        : int.tryParse(rawId?.toString() ?? '');
                                  }

                                  if (!context.mounted) return;

                                  String? initialProductName;
                                  if (isEditMode && existingProduct != null) {
                                    initialProductName =
                                        (existingProduct['productName'] ??
                                                existingProduct['name'] ??
                                                existingProduct['skuName'])
                                            ?.toString();
                                  }

                                  final skuIdForEdit =
                                      isEditMode && existingProduct != null
                                      ? (existingProduct['skuId'] ??
                                            existingProduct['id'])
                                      : null;

                                  final result = await context.push(
                                    '/product/info',
                                    extra: {
                                      'skuId': skuIdForEdit,
                                      'initialProductName': initialProductName,
                                      'initialCategory':
                                          '${l1Label}_$parentName',
                                      'initialSubCategory': name,
                                      'initialCatalogId': catalogId,
                                      'isCandidateCategory': false,
                                      'l1Id': l1Id,
                                      'l1Name': l1Label,
                                      'l2Id': parentId,
                                      'l2Name': parentName,
                                      'l3Name': name,
                                      'isEditMode': isEditMode,
                                      'existingProduct': existingProduct,
                                    },
                                  );

                                  if (context.mounted && result != null) {
                                    context.pop(result);
                                  }
                                }
                              : null,
                          behavior: HitTestBehavior.opaque,
                          child: Center(
                            child: Text(
                              '確定',
                              textAlign: TextAlign.center,
                              style: TextStyle(
                                color: hasText
                                    ? AppColors.primary
                                    : AppColors.textSecondary,
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
              ],
            ),
          ),
        );
      },
    ),
  );
}
