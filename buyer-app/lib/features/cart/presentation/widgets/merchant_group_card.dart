import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import '../../../../config/image_config.dart';
import '../../../../shared/widgets/common_dialog.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/ios_swipe_action.dart';
import '../../data/cart_model.dart';
import 'cart_product_tile.dart';
import 'merchant_group.dart';

// ---------------------------------------------------------------------------
// 商家分組卡片。
//
// 白色圓角卡片，頂部為商家頭（勾選框 + 頭像 + 名稱/地點），
// 下方為該商家的所有商品行，每行支援左滑刪除。
// ---------------------------------------------------------------------------

/// 購物車中單個商家分組的卡片。
///
/// [group] 商家分組數據。
/// [checkedSkuIds] 當前勾選的 SKU ID 集合。
/// [skuStock] SKU 庫存 map。
/// [onGroupCheckChanged] 整組勾選/取消勾選。
/// [onItemCheckChanged] 單品勾選/取消勾選。
/// [onQuantityChanged] 數量變更（item, newQty）。
/// [onDeleteItem] 刪除單品。
/// [resolveImageProvider] 將圖片 URL 轉為 ImageProvider。
/// [sellerAvatarUrl] 取得商家頭像 URL。
class MerchantGroupCard extends StatelessWidget {
  const MerchantGroupCard({
    super.key,
    required this.group,
    required this.checkedSkuIds,
    required this.skuStock,
    required this.onGroupCheckChanged,
    required this.onItemCheckChanged,
    required this.onQuantityChanged,
    required this.onDeleteItem,
    required this.resolveImageProvider,
    required this.sellerAvatarUrl,
    this.onMerchantTap,
    this.storeCode = '',
    this.storeStatusLabel,
  });

  /// Optional navigation callback for deterministic previews.
  final VoidCallback? onMerchantTap;

  /// 店面網址的路徑段；空字串表示店家不是營業中（或還沒讀到），不能進店。
  final String storeCode;

  /// 店家暫停營業或已關閉時的標籤，取代進店按鈕的文字。
  final String? storeStatusLabel;
  final MerchantGroup group;
  final Set<int> checkedSkuIds;
  final Map<int, int> skuStock;
  final void Function(MerchantGroup group, bool checked) onGroupCheckChanged;
  final void Function(int skuId, bool checked) onItemCheckChanged;
  final void Function(CartItem item, int newQuantity) onQuantityChanged;
  final void Function(CartItem item) onDeleteItem;
  final ImageProvider? Function(String? source) resolveImageProvider;
  final String? Function(String storeId, {String? groupLogoUrl})
  sellerAvatarUrl;

  @override
  Widget build(BuildContext context) {
    final selectableItems = group.items.where((item) => item.purchasable);
    final selectedCount = selectableItems
        .where((item) => checkedSkuIds.contains(item.skuId))
        .length;
    final selectableCount = selectableItems.length;
    final allChecked = selectableCount > 0 && selectedCount == selectableCount;
    final someChecked = selectedCount > 0 && !allChecked;

    return Container(
      margin: const EdgeInsets.symmetric(
        horizontal: HkSpace.lg,
        vertical: HkSpace.sm,
      ),
      padding: const EdgeInsets.all(HkSpace.md),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.lg),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: HkShadows.card,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // ── 商家標題列 ──
          _buildMerchantHeader(
            context,
            allChecked: allChecked,
            someChecked: someChecked,
            enabled: selectableCount > 0,
            selectedCount: selectedCount,
            selectableCount: selectableCount,
          ),
          // ── 分隔線 ──
          const Padding(
            padding: EdgeInsets.symmetric(vertical: HkSpace.sm),
            child: Divider(height: 1, color: AppColors.dividerLight),
          ),
          // ── 商品行列表 ──
          ...group.items.asMap().entries.map((entry) {
            final index = entry.key;
            final item = entry.value;
            return Column(
              children: [
                if (index > 0)
                  const Padding(
                    padding: EdgeInsets.only(left: HkControl.minTap),
                    child: Divider(height: 1, color: AppColors.dividerLight),
                  ),
                IosSwipeAction(
                  key: Key('cart_product_${item.skuId}_$index'),
                  onAction: () => _confirmDelete(context, item),
                  child: CartProductTile(
                    item: item,
                    isChecked: checkedSkuIds.contains(item.skuId),
                    remainingStock: skuStock[item.skuId],
                    imageProvider: resolveImageProvider(item.image),
                    onCheckChanged: (value) =>
                        onItemCheckChanged(item.skuId, value),
                    onQuantityChanged: (quantity) =>
                        onQuantityChanged(item, quantity),
                  ),
                ),
              ],
            );
          }),
        ],
      ),
    );
  }

  /// Store identity and the scoped selection action are separate rows.
  Widget _buildMerchantHeader(
    BuildContext context, {
    required bool allChecked,
    required bool someChecked,
    required bool enabled,
    required int selectedCount,
    required int selectableCount,
  }) {
    final avatarUrl = sellerAvatarUrl(
      group.storeId,
      groupLogoUrl: group.logoUrl,
    );
    final checkboxValue = allChecked ? true : (someChecked ? null : false);
    final switchesStore = checkedSkuIds.isNotEmpty && selectedCount == 0;
    final actionLabel = !enabled
        ? '本店暫無可購買商品'
        : switchesStore
        ? '改選本店可購買商品'
        : allChecked
        ? '取消本店選取'
        : '選取本店可購買商品';
    final status = switchesStore && enabled
        ? '切換結帳店家，原店商品保留'
        : '已選 $selectedCount／$selectableCount 項可購買商品';
    final toggle = enabled
        ? () => onGroupCheckChanged(group, !allChecked)
        : null;
    final VoidCallback? openStore =
        onMerchantTap ??
        (storeCode.isEmpty ? null : () => context.push('/merchant/$storeCode'));

    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Row(
                    children: [
                      CircleAvatar(
                        radius: 20,
                        backgroundColor: AppColors.primarySoft,
                        backgroundImage: avatarUrl != null
                            ? NetworkImage(
                                ImageConfig.urlFor(
                                  avatarUrl,
                                  ImageScene.merchantAvatar,
                                ),
                              )
                            : const AssetImage('assets/hawkrlogo.png'),
                      ),
                      const SizedBox(width: HkSpace.sm),
                      Expanded(
                        child: Text(
                          group.sellerName,
                          key: ValueKey('cart-store-name-${group.storeId}'),
                          style: CartTypography.subtitle.copyWith(
                            color: AppColors.textEmphasis,
                          ),
                        ),
                      ),
                    ],
                  ),
                  if (group.locationText.isNotEmpty) ...[
                    const SizedBox(height: HkSpace.xs),
                    Row(
                      children: [
                        const Icon(
                          AppIcons.location,
                          size: 14,
                          color: AppIconColors.muted,
                        ),
                        const SizedBox(width: HkSpace.xs),
                        Expanded(
                          child: Text(
                            group.locationText,
                            style: CartTypography.supportMuted,
                          ),
                        ),
                      ],
                    ),
                  ],
                ],
              ),
            ),
            const SizedBox(width: HkSpace.sm),
            Semantics(
              label: '前往${group.sellerName}商店',
              child: TextButton(
                key: ValueKey('cart-store-link-${group.storeId}'),
                onPressed: openStore,
                style: TextButton.styleFrom(
                  foregroundColor: AppColors.actionPrimary,
                  minimumSize: const Size(HkControl.minTap, HkControl.minTap),
                  padding: const EdgeInsets.symmetric(horizontal: HkSpace.xs),
                  textStyle: CartTypography.button,
                ),
                child: HkButtonLabel(storeStatusLabel ?? '賣家商店'),
              ),
            ),
          ],
        ),
        const SizedBox(height: HkSpace.sm),
        Semantics(
          key: ValueKey('cart-store-selection-${group.storeId}'),
          button: true,
          enabled: enabled,
          checked: allChecked,
          mixed: someChecked,
          label: '${group.sellerName}，$actionLabel，$status',
          onTap: toggle,
          child: ExcludeSemantics(
            child: Material(
              color: Colors.transparent,
              child: InkWell(
                onTap: toggle,
                borderRadius: BorderRadius.circular(HkRadius.sm),
                child: ConstrainedBox(
                  constraints: const BoxConstraints(
                    minHeight: HkControl.minTap,
                  ),
                  child: Row(
                    children: [
                      SizedBox.square(
                        dimension: HkControl.minTap,
                        child: ExcludeFocus(
                          child: IgnorePointer(
                            child: Checkbox(
                              tristate: true,
                              value: checkboxValue,
                              onChanged: enabled ? (_) {} : null,
                              activeColor: AppColors.brandPrimaryStrong,
                              checkColor: AppColors.bgCard,
                              side: BorderSide(
                                color: someChecked || allChecked
                                    ? AppColors.brandPrimaryStrong
                                    : AppColors.gray300,
                                width: 1.5,
                              ),
                              shape: RoundedRectangleBorder(
                                borderRadius: BorderRadius.circular(
                                  HkRadius.xs,
                                ),
                              ),
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: HkSpace.xs),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              actionLabel,
                              style: CartTypography.button.copyWith(
                                color: enabled
                                    ? AppColors.actionPrimary
                                    : AppColors.textSecondary,
                              ),
                            ),
                            Text(status, style: CartTypography.supportMuted),
                          ],
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ],
    );
  }

  /// 左滑刪除前的確認對話框。
  Future<void> _confirmDelete(BuildContext context, CartItem item) async {
    final confirmed =
        await showCommonDialog(
          context: context,
          title: '確認刪除',
          content: '確定要從購物車中移除「${item.title ?? '商品'}」嗎？',
          confirmText: '刪除',
          cancelText: '取消',
          confirmColor: AppColors.danger,
        ) ??
        false;
    if (confirmed) {
      onDeleteItem(item);
    }
  }
}
