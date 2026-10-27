import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';

import '../../../customer/data/customer_model.dart';
import 'section_card.dart';

/// 依客戶標籤建立發放名單。
class CouponTagSelectionCard extends StatefulWidget {
  const CouponTagSelectionCard({
    super.key,
    required this.allTags,
    required this.selectedTagIds,
    required this.matchedCustomers,
    required this.isLoading,
    required this.onLoadTags,
    required this.onTagToggled,
    required this.onRemoveCustomer,
  });

  final List<CustomerTag> allTags;
  final Set<int> selectedTagIds;
  final List<Customer> matchedCustomers;
  final bool isLoading;
  final VoidCallback onLoadTags;
  final ValueChanged<int> onTagToggled;
  final ValueChanged<int> onRemoveCustomer;

  @override
  State<CouponTagSelectionCard> createState() => _CouponTagSelectionCardState();
}

class _CouponTagSelectionCardState extends State<CouponTagSelectionCard> {
  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => widget.onLoadTags());
  }

  @override
  Widget build(BuildContext context) {
    if (widget.isLoading && widget.allTags.isEmpty) {
      return const CouponSectionCard(
        child: Padding(
          padding: EdgeInsets.symmetric(vertical: HkSpace.xxl),
          child: Center(
            child: SizedBox.square(
              dimension: 22,
              child: CircularProgressIndicator(strokeWidth: 2),
            ),
          ),
        ),
      );
    }

    if (widget.allTags.isEmpty) {
      return const CouponSectionCard(child: _EmptyTagState());
    }

    return CouponSectionCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 40,
                height: 40,
                decoration: BoxDecoration(
                  color: AppColors.primarySoft,
                  borderRadius: BorderRadius.circular(HkRadius.md),
                ),
                child: const Center(
                  child: HkIcon(
                    HkIconName.tag,
                    size: 20,
                    color: AppIconColors.active,
                  ),
                ),
              ),
              const SizedBox(width: HkSpace.md),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    const Text(
                      '依標籤建立名單',
                      style: TextStyle(
                        color: AppColors.textEmphasis,
                        fontSize: 16,
                        fontWeight: FontWeight.w900,
                      ),
                    ),
                    const SizedBox(height: HkSpace.xs),
                    Text(
                      widget.selectedTagIds.isEmpty
                          ? '選擇一個或多個客戶標籤'
                          : '已選擇 ${widget.selectedTagIds.length} 個標籤',
                      style: const TextStyle(
                        color: AppColors.textMuted,
                        fontSize: 14,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: HkSpace.xl),
          Wrap(
            spacing: HkSpace.sm,
            runSpacing: HkSpace.sm,
            children: widget.allTags.map((tag) {
              final isSelected = widget.selectedTagIds.contains(tag.id);
              return _TagButton(
                label: tag.name,
                isSelected: isSelected,
                onTap: () => widget.onTagToggled(tag.id),
              );
            }).toList(),
          ),
          const SizedBox(height: HkSpace.xl),
          Container(
            width: double.infinity,
            padding: const EdgeInsets.all(HkSpace.md),
            decoration: BoxDecoration(
              color: widget.selectedTagIds.isEmpty
                  ? AppColors.surfaceMuted
                  : AppColors.successSoft,
              borderRadius: BorderRadius.circular(HkRadius.md),
              border: Border.all(
                color: widget.selectedTagIds.isEmpty
                    ? AppColors.cardBorder
                    : AppColors.success.withValues(alpha: 0.22),
              ),
            ),
            child: Row(
              children: [
                HkIcon(
                  widget.selectedTagIds.isEmpty
                      ? HkIconName.info
                      : HkIconName.users,
                  size: 20,
                  color: widget.selectedTagIds.isEmpty
                      ? AppIconColors.muted
                      : AppIconColors.success,
                ),
                const SizedBox(width: HkSpace.sm),
                Expanded(
                  child: Text(
                    widget.selectedTagIds.isEmpty
                        ? '點選標籤後會顯示符合的會員'
                        : '符合 ${widget.matchedCustomers.length} 位會員',
                    style: TextStyle(
                      color: widget.selectedTagIds.isEmpty
                          ? AppColors.textMuted
                          : AppColors.success,
                      fontSize: 14,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ),
                if (widget.isLoading)
                  const SizedBox.square(
                    dimension: 18,
                    child: CircularProgressIndicator(strokeWidth: 2),
                  ),
              ],
            ),
          ),
          if (!widget.isLoading && widget.matchedCustomers.isNotEmpty) ...[
            const SizedBox(height: HkSpace.md),
            ...widget.matchedCustomers.asMap().entries.map((entry) {
              final customer = entry.value;
              return _CustomerCard(
                name: customer.name,
                accountId: customer.id,
                phone: customer.phone,
                onRemove: () => widget.onRemoveCustomer(entry.key),
              );
            }),
          ],
        ],
      ),
    );
  }
}

class _EmptyTagState extends StatelessWidget {
  const _EmptyTagState();

  @override
  Widget build(BuildContext context) {
    return const Padding(
      padding: EdgeInsets.symmetric(vertical: HkSpace.md),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          HkIcon(HkIconName.tag, size: 24, color: AppIconColors.muted),
          SizedBox(width: HkSpace.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  '尚未建立客戶標籤',
                  style: TextStyle(
                    color: AppColors.textEmphasis,
                    fontSize: 16,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                SizedBox(height: HkSpace.xs),
                Text(
                  '可先到總覽的顧客管理，替個別顧客加入標籤。',
                  style: TextStyle(
                    color: AppColors.textMuted,
                    fontSize: 14,
                    height: 1.45,
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

class _TagButton extends StatelessWidget {
  const _TagButton({
    required this.label,
    required this.isSelected,
    required this.onTap,
  });

  final String label;
  final bool isSelected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: isSelected,
      label: '標籤 $label',
      child: Material(
        color: Colors.transparent,
        child: InkWell(
          onTap: onTap,
          borderRadius: BorderRadius.circular(HkRadius.pill),
          child: AnimatedContainer(
            duration: HkMotion.duration,
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
            decoration: BoxDecoration(
              color: isSelected
                  ? AppColors.brandPrimaryStrong
                  : AppColors.surfacePage,
              borderRadius: BorderRadius.circular(HkRadius.pill),
              border: Border.all(
                color: isSelected
                    ? AppColors.brandPrimaryStrong
                    : AppColors.borderInput,
              ),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                HkIcon(
                  isSelected ? HkIconName.check : HkIconName.add,
                  size: 18,
                  color: isSelected
                      ? AppIconColors.onBrand
                      : AppIconColors.active,
                ),
                const SizedBox(width: HkSpace.sm),
                Text(
                  label,
                  style: TextStyle(
                    color: isSelected
                        ? AppColors.textInverse
                        : AppColors.textPrimary,
                    fontSize: 14,
                    fontWeight: FontWeight.w700,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}

class _CustomerCard extends StatelessWidget {
  const _CustomerCard({
    required this.name,
    required this.accountId,
    required this.phone,
    required this.onRemove,
  });

  final String name;
  final String accountId;
  final String phone;
  final VoidCallback onRemove;

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.only(bottom: HkSpace.sm),
      padding: const EdgeInsets.fromLTRB(
        HkSpace.md,
        HkSpace.sm,
        HkSpace.xs,
        HkSpace.sm,
      ),
      decoration: BoxDecoration(
        color: AppColors.surfacePage,
        borderRadius: BorderRadius.circular(HkRadius.md),
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: Row(
        children: [
          Container(
            width: 40,
            height: 40,
            decoration: const BoxDecoration(
              color: AppColors.primarySoft,
              shape: BoxShape.circle,
            ),
            child: Center(
              child: Text(
                name.isNotEmpty ? name.characters.first : '?',
                style: const TextStyle(
                  color: AppColors.textAccentOnTint,
                  fontSize: 16,
                  fontWeight: FontWeight.w900,
                ),
              ),
            ),
          ),
          const SizedBox(width: HkSpace.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  name,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: AppColors.textEmphasis,
                    fontSize: 14,
                    fontWeight: FontWeight.w700,
                  ),
                ),
                const SizedBox(height: 2),
                Text(
                  [
                    if (accountId.isNotEmpty) 'ID $accountId',
                    if (phone.isNotEmpty) phone,
                  ].join(' · '),
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(
                    color: AppColors.textMuted,
                    fontSize: 12,
                  ),
                ),
              ],
            ),
          ),
          Semantics(
            button: true,
            label: '從名單移除 $name',
            child: IconButton(
              tooltip: '移除會員',
              onPressed: onRemove,
              icon: const HkIcon(
                HkIconName.close,
                size: 18,
                color: AppIconColors.muted,
              ),
            ),
          ),
        ],
      ),
    );
  }
}
