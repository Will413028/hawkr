import 'package:flutter/material.dart';

import '../../../../core/theme/app_typography.dart';
import '../../../../core/theme/hk_tokens.dart';
import '../../../../shared/widgets/hk_icon.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import '../../../../shared/widgets/hk_sheet_drag_handle.dart';
import '../../data/address_model.dart';
import '../../data/store_pickup_model.dart';

/// 手機版地址設定的可收納區塊。
///
/// 收合時只保留預設項目摘要，展開後才顯示完整清單與新增操作。
class AddressCollapsibleSection extends StatelessWidget {
  const AddressCollapsibleSection({
    super.key,
    required this.title,
    required this.description,
    required this.summary,
    required this.leading,
    required this.expanded,
    required this.onToggle,
    required this.child,
    this.count = 0,
  });

  final String title;
  final String description;
  final String summary;
  final Widget leading;
  final bool expanded;
  final VoidCallback onToggle;
  final Widget child;
  final int count;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final radius = BorderRadius.circular(HkRadius.lg);
    return Material(
      key: ValueKey('address-section-$title'),
      color: scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: radius,
        side: BorderSide(color: scheme.outlineVariant),
      ),
      clipBehavior: Clip.antiAlias,
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Semantics(
            button: true,
            expanded: expanded,
            label: '$title，$count 筆，$summary，${expanded ? '點擊收合' : '點擊展開'}',
            onTap: onToggle,
            child: ExcludeSemantics(
              child: InkWell(
                onTap: onToggle,
                child: Padding(
                  padding: const EdgeInsets.all(HkSpace.md),
                  child: Row(
                    crossAxisAlignment: CrossAxisAlignment.center,
                    children: [
                      DecoratedBox(
                        decoration: BoxDecoration(
                          color: scheme.surfaceContainerLowest,
                          borderRadius: BorderRadius.circular(HkRadius.md),
                          border: Border.all(color: scheme.outlineVariant),
                          boxShadow: HkShadows.sm,
                        ),
                        child: SizedBox.square(
                          dimension: HkControl.minTap,
                          child: Center(child: leading),
                        ),
                      ),
                      const SizedBox(width: HkSpace.md),
                      Expanded(
                        child: Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          children: [
                            Text(
                              title,
                              style: AppTypography.bodyBold.copyWith(
                                color: scheme.onSurface,
                              ),
                            ),
                            const SizedBox(height: 2),
                            Text(
                              expanded ? description : summary,
                              maxLines: expanded ? 2 : 1,
                              overflow: TextOverflow.ellipsis,
                              style: AppTypography.meta.copyWith(
                                color: scheme.onSurfaceVariant,
                              ),
                            ),
                          ],
                        ),
                      ),
                      const SizedBox(width: HkSpace.sm),
                      DecoratedBox(
                        decoration: BoxDecoration(
                          color: scheme.surfaceContainer,
                          borderRadius: BorderRadius.circular(HkRadius.pill),
                        ),
                        child: Padding(
                          padding: const EdgeInsets.symmetric(
                            horizontal: HkSpace.sm,
                            vertical: HkSpace.xs,
                          ),
                          child: Text(
                            '$count',
                            style: AppTypography.caption.copyWith(
                              color: scheme.onSurfaceVariant,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ),
                      ),
                      const SizedBox(width: HkSpace.xs),
                      AnimatedRotation(
                        turns: expanded ? 0.5 : 0,
                        duration: HkMotion.duration,
                        curve: HkMotion.ease,
                        child: HkIcon(
                          HkIconName.chevronDown,
                          size: 22,
                          color: scheme.onSurfaceVariant,
                        ),
                      ),
                    ],
                  ),
                ),
              ),
            ),
          ),
          ClipRect(
            child: AnimatedSize(
              duration: HkMotion.duration,
              curve: HkMotion.ease,
              alignment: Alignment.topCenter,
              child: expanded
                  ? DecoratedBox(
                      decoration: BoxDecoration(
                        border: Border(
                          top: BorderSide(color: scheme.outlineVariant),
                        ),
                      ),
                      child: Padding(
                        padding: const EdgeInsets.all(HkSpace.md),
                        child: child,
                      ),
                    )
                  : const SizedBox.shrink(),
            ),
          ),
        ],
      ),
    );
  }
}

/// 對齊網頁版地址設定頁的宅配地址區塊標題。
class AddressSectionHeading extends StatelessWidget {
  const AddressSectionHeading({
    super.key,
    this.title = '宅配地址',
    this.description = '結帳選擇宅配到家時，會自動帶入這個預設地址。',
  });

  final String title;
  final String description;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Row(
      crossAxisAlignment: CrossAxisAlignment.center,
      children: [
        DecoratedBox(
          decoration: BoxDecoration(
            color: scheme.surface,
            borderRadius: BorderRadius.circular(HkRadius.md),
            border: Border.all(color: scheme.outlineVariant),
            boxShadow: HkShadows.sm,
          ),
          child: const SizedBox.square(
            dimension: HkControl.minTap,
            child: Center(child: HkIcon(HkIconName.truck, size: 32)),
          ),
        ),
        const SizedBox(width: HkSpace.md),
        Expanded(
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(
                title,
                style: AppTypography.h1.copyWith(color: scheme.onSurface),
              ),
              const SizedBox(height: HkSpace.xs),
              Text(
                description,
                style: AppTypography.meta.copyWith(
                  color: scheme.onSurfaceVariant,
                ),
              ),
            ],
          ),
        ),
      ],
    );
  }
}

/// 地址空狀態的共用元件。
class AddressEmptyState extends StatelessWidget {
  const AddressEmptyState({
    super.key,
    required this.title,
    required this.description,
    this.actionLabel,
    this.onAction,
    this.compact = false,
    this.icon = HkIconName.emptyAddress,
  });

  final String title;
  final String description;
  final String? actionLabel;
  final VoidCallback? onAction;
  final bool compact;
  final HkIconName icon;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final iconSurfaceSize = compact ? 80.0 : 96.0;
    return Semantics(
      container: true,
      label: '$title，$description',
      child: Padding(
        key: const ValueKey('address-empty-state'),
        padding: EdgeInsets.symmetric(
          horizontal: HkSpace.xl,
          vertical: compact ? HkSpace.xl : HkSpace.section,
        ),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            DecoratedBox(
              decoration: BoxDecoration(
                shape: BoxShape.circle,
                color: Color.alphaBlend(
                  scheme.tertiary.withValues(alpha: 0.10),
                  scheme.primary.withValues(alpha: 0.04),
                ),
              ),
              child: SizedBox.square(
                dimension: iconSurfaceSize,
                child: Center(
                  child: HkIcon(icon, size: 64, semanticLabel: '收件地址'),
                ),
              ),
            ),
            const SizedBox(height: HkSpace.md),
            Text(
              title,
              textAlign: TextAlign.center,
              style: AppTypography.bodyBold.copyWith(color: scheme.onSurface),
            ),
            const SizedBox(height: HkSpace.xs),
            ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 300),
              child: Text(
                description,
                textAlign: TextAlign.center,
                style: AppTypography.meta.copyWith(
                  color: scheme.onSurfaceVariant,
                  height: 1.6,
                ),
              ),
            ),
            if (actionLabel != null && onAction != null) ...[
              const SizedBox(height: HkSpace.lg),
              FilledButton.icon(
                key: const ValueKey('address-empty-primary-action'),
                onPressed: onAction,
                icon: const HkIcon(HkIconName.add, size: 20),
                label: HkButtonLabel(actionLabel!),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// 與網頁版 `cart-sheet__row` 相同的地址列。
class AddressChoiceCard extends StatelessWidget {
  const AddressChoiceCard({
    super.key,
    required this.address,
    required this.selected,
    required this.onTap,
    this.onEdit,
    this.onDelete,
    this.enabled = true,
  });

  final Address address;
  final bool selected;
  final VoidCallback onTap;
  final VoidCallback? onEdit;
  final VoidCallback? onDelete;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final receiverName = address.receiverName.trim().isEmpty
        ? '收件人'
        : address.receiverName.trim();
    final phone = address.receiverPhone.trim();
    final fullAddress = address.fullAddress.trim();
    final activeSurface = Color.alphaBlend(
      scheme.primary.withValues(alpha: 0.06),
      scheme.surface,
    );
    final radius = BorderRadius.circular(16);

    return Material(
      color: selected ? activeSurface : scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: radius,
        side: BorderSide(
          color: selected ? scheme.primary : scheme.outlineVariant,
        ),
      ),
      clipBehavior: Clip.antiAlias,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Semantics(
              button: true,
              selected: selected,
              enabled: enabled,
              label:
                  '$receiverName，$phone，$fullAddress${address.isDefault ? '，預設地址' : ''}',
              onTap: enabled ? onTap : null,
              child: ExcludeSemantics(
                child: InkWell(
                  onTap: enabled ? onTap : null,
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(
                      HkSpace.md,
                      HkSpace.md,
                      HkSpace.sm,
                      HkSpace.md,
                    ),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Padding(
                          padding: const EdgeInsets.only(top: 1),
                          child: _AddressRadio(selected: selected),
                        ),
                        const SizedBox(width: HkSpace.md),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Wrap(
                                spacing: HkSpace.sm,
                                runSpacing: HkSpace.xs,
                                crossAxisAlignment: WrapCrossAlignment.center,
                                children: [
                                  Text(
                                    receiverName,
                                    style: AppTypography.meta.copyWith(
                                      color: scheme.onSurface,
                                      fontWeight: FontWeight.w700,
                                    ),
                                  ),
                                  if (address.isDefault)
                                    DecoratedBox(
                                      decoration: BoxDecoration(
                                        color: scheme.primary,
                                        borderRadius: BorderRadius.circular(
                                          HkRadius.pill,
                                        ),
                                      ),
                                      child: Padding(
                                        padding: const EdgeInsets.symmetric(
                                          horizontal: HkSpace.sm,
                                          vertical: HkSpace.xs,
                                        ),
                                        child: Text(
                                          '預設',
                                          style: AppTypography.caption.copyWith(
                                            color: scheme.onPrimary,
                                            fontWeight: FontWeight.w700,
                                          ),
                                        ),
                                      ),
                                    ),
                                ],
                              ),
                              if (phone.isNotEmpty) ...[
                                const SizedBox(height: 2),
                                Text(
                                  phone,
                                  style: AppTypography.caption.copyWith(
                                    color: scheme.onSurfaceVariant,
                                  ),
                                ),
                              ],
                              if (fullAddress.isNotEmpty) ...[
                                const SizedBox(height: 2),
                                Text(
                                  fullAddress,
                                  style: AppTypography.caption.copyWith(
                                    color: scheme.onSurfaceVariant,
                                    height: 1.5,
                                  ),
                                ),
                              ],
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
          if (onEdit != null || onDelete != null)
            Padding(
              padding: const EdgeInsets.only(
                top: HkSpace.xs,
                right: HkSpace.xs,
                bottom: HkSpace.xs,
              ),
              child: _AddressActions(onEdit: onEdit, onDelete: onDelete),
            ),
        ],
      ),
    );
  }
}

/// 與網頁版 7-11 預設門市卡片相同的資訊順序。
class StorePickupChoiceCard extends StatelessWidget {
  const StorePickupChoiceCard({
    super.key,
    required this.pickup,
    required this.selected,
    required this.onTap,
    this.onEdit,
    this.onDelete,
    this.enabled = true,
  });

  final StorePickup pickup;
  final bool selected;
  final VoidCallback onTap;
  final VoidCallback? onEdit;
  final VoidCallback? onDelete;
  final bool enabled;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final storeName = pickup.storeName.isEmpty ? '7-11 取貨門市' : pickup.storeName;
    final recipient = [
      pickup.receiverName,
      pickup.receiverPhone,
    ].where((value) => value.isNotEmpty).join(' · ');
    final activeSurface = Color.alphaBlend(
      scheme.primary.withValues(alpha: 0.06),
      scheme.surface,
    );
    final radius = BorderRadius.circular(16);

    return Material(
      color: selected ? activeSurface : scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: radius,
        side: BorderSide(
          color: selected ? scheme.primary : scheme.outlineVariant,
        ),
      ),
      clipBehavior: Clip.antiAlias,
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Expanded(
            child: Semantics(
              button: true,
              selected: selected,
              enabled: enabled,
              label:
                  '$storeName，$recipient，7-11 店號 ${pickup.storeCode}，${pickup.storeAddress}'
                  '${pickup.isDefault ? '，預設門市' : ''}',
              onTap: enabled ? onTap : null,
              child: ExcludeSemantics(
                child: InkWell(
                  onTap: enabled ? onTap : null,
                  child: Padding(
                    padding: const EdgeInsets.fromLTRB(
                      HkSpace.md,
                      HkSpace.md,
                      HkSpace.sm,
                      HkSpace.md,
                    ),
                    child: Row(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Padding(
                          padding: const EdgeInsets.only(top: 1),
                          child: _AddressRadio(selected: selected),
                        ),
                        const SizedBox(width: HkSpace.md),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Wrap(
                                spacing: HkSpace.sm,
                                runSpacing: HkSpace.xs,
                                crossAxisAlignment: WrapCrossAlignment.center,
                                children: [
                                  Text(
                                    storeName,
                                    style: AppTypography.meta.copyWith(
                                      color: scheme.onSurface,
                                      fontWeight: FontWeight.w700,
                                    ),
                                  ),
                                  if (pickup.isDefault)
                                    _DefaultBadge(colorScheme: scheme),
                                ],
                              ),
                              if (recipient.isNotEmpty) ...[
                                const SizedBox(height: 2),
                                Text(
                                  recipient,
                                  style: AppTypography.caption.copyWith(
                                    color: scheme.onSurfaceVariant,
                                  ),
                                ),
                              ],
                              const SizedBox(height: 2),
                              Text(
                                '7-11 店號 ${pickup.storeCode.isEmpty ? '—' : pickup.storeCode}',
                                style: AppTypography.caption.copyWith(
                                  color: scheme.onSurfaceVariant,
                                ),
                              ),
                              if (pickup.storeAddress.isNotEmpty) ...[
                                const SizedBox(height: 2),
                                Text(
                                  pickup.storeAddress,
                                  style: AppTypography.caption.copyWith(
                                    color: scheme.onSurfaceVariant,
                                    height: 1.5,
                                  ),
                                ),
                              ],
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
          if (onEdit != null || onDelete != null)
            Padding(
              padding: const EdgeInsets.only(
                top: HkSpace.xs,
                right: HkSpace.xs,
                bottom: HkSpace.xs,
              ),
              child: _AddressActions(
                onEdit: onEdit,
                onDelete: onDelete,
                editLabel: '編輯門市',
                deleteLabel: '刪除門市',
              ),
            ),
        ],
      ),
    );
  }
}

class _DefaultBadge extends StatelessWidget {
  const _DefaultBadge({required this.colorScheme});

  final ColorScheme colorScheme;

  @override
  Widget build(BuildContext context) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: colorScheme.primary,
        borderRadius: BorderRadius.circular(HkRadius.pill),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: HkSpace.sm,
          vertical: HkSpace.xs,
        ),
        child: Text(
          '預設',
          style: AppTypography.caption.copyWith(
            color: colorScheme.onPrimary,
            fontWeight: FontWeight.w700,
          ),
        ),
      ),
    );
  }
}

class _AddressRadio extends StatelessWidget {
  const _AddressRadio({required this.selected});

  final bool selected;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return AnimatedContainer(
      duration: const Duration(milliseconds: 160),
      width: 20,
      height: 20,
      decoration: BoxDecoration(
        shape: BoxShape.circle,
        border: Border.all(
          color: selected ? scheme.primary : scheme.outlineVariant,
          width: 2,
        ),
      ),
      child: Center(
        child: AnimatedContainer(
          duration: const Duration(milliseconds: 160),
          width: 10,
          height: 10,
          decoration: BoxDecoration(
            shape: BoxShape.circle,
            color: selected ? scheme.primary : Colors.transparent,
          ),
        ),
      ),
    );
  }
}

class _AddressActions extends StatelessWidget {
  const _AddressActions({
    this.onEdit,
    this.onDelete,
    this.editLabel = '編輯地址',
    this.deleteLabel = '刪除地址',
  });

  final VoidCallback? onEdit;
  final VoidCallback? onDelete;
  final String editLabel;
  final String deleteLabel;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        if (onEdit != null)
          _AddressIconAction(
            label: editLabel,
            icon: HkIconName.edit,
            color: scheme.primary,
            onPressed: onEdit!,
          ),
        if (onDelete != null)
          _AddressIconAction(
            label: deleteLabel,
            icon: HkIconName.delete,
            color: scheme.error,
            onPressed: onDelete!,
          ),
      ],
    );
  }
}

class _AddressIconAction extends StatelessWidget {
  const _AddressIconAction({
    required this.label,
    required this.icon,
    required this.color,
    required this.onPressed,
  });

  final String label;
  final HkIconName icon;
  final Color color;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: label,
      child: IconButton(
        tooltip: label,
        onPressed: onPressed,
        color: color,
        iconSize: 20,
        constraints: const BoxConstraints.tightFor(
          width: HkControl.minTap,
          height: HkControl.minTap,
        ),
        icon: HkIcon(icon, size: 20),
      ),
    );
  }
}

/// 網頁版 sheet footer 的全寬虛線新增按鈕。
class AddressAddButton extends StatelessWidget {
  const AddressAddButton({
    super.key,
    required this.label,
    required this.onPressed,
  });

  final String label;
  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Semantics(
      button: true,
      label: label,
      child: CustomPaint(
        foregroundPainter: _DashedRoundedBorderPainter(
          color: scheme.primary,
          radius: HkRadius.md,
        ),
        child: Material(
          color: Colors.transparent,
          borderRadius: BorderRadius.circular(HkRadius.md),
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            onTap: onPressed,
            child: ConstrainedBox(
              constraints: const BoxConstraints(
                minHeight: HkControl.minTap,
                minWidth: double.infinity,
              ),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.center,
                children: [
                  HkIcon(HkIconName.add, size: 20, color: scheme.primary),
                  const SizedBox(width: HkSpace.sm),
                  Text(
                    label,
                    style: AppTypography.buttonMedium.copyWith(
                      color: scheme.primary,
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }
}

enum _LogisticsPickerMode { delivery, storePickup }

/// 首頁物流設定 sheet，同時管理宅配地址與 7-11 取貨門市。
class AddressPickerPanel extends StatefulWidget {
  const AddressPickerPanel({
    super.key,
    required this.addresses,
    required this.selectedId,
    required this.onSelect,
    required this.onAdd,
    required this.storePickups,
    required this.selectedStorePickupId,
    required this.onStorePickupSelect,
    required this.onAddStorePickup,
    required this.onClose,
  });

  final List<Address> addresses;
  final int? selectedId;
  final Future<bool> Function(Address address) onSelect;
  final VoidCallback onAdd;
  final List<StorePickup> storePickups;
  final String? selectedStorePickupId;
  final Future<bool> Function(StorePickup pickup) onStorePickupSelect;
  final VoidCallback onAddStorePickup;
  final VoidCallback onClose;

  @override
  State<AddressPickerPanel> createState() => _AddressPickerPanelState();
}

class _AddressPickerPanelState extends State<AddressPickerPanel> {
  _LogisticsPickerMode _mode = _LogisticsPickerMode.delivery;
  int? _selectedId;
  String? _selectedStorePickupId;
  int? _selectingId;
  String? _selectingStorePickupId;

  @override
  void initState() {
    super.initState();
    _selectedId = widget.selectedId;
    _selectedStorePickupId = widget.selectedStorePickupId;
  }

  Future<void> _select(Address address) async {
    if (_selectingId != null) return;
    final previous = _selectedId;
    setState(() {
      _selectedId = address.id;
      _selectingId = address.id;
    });
    final succeeded = await widget.onSelect(address);
    if (!mounted) return;
    if (succeeded) {
      widget.onClose();
      return;
    }
    setState(() {
      _selectedId = previous;
      _selectingId = null;
    });
  }

  Future<void> _selectStorePickup(StorePickup pickup) async {
    if (_selectingStorePickupId != null) return;
    final previous = _selectedStorePickupId;
    setState(() {
      _selectedStorePickupId = pickup.id;
      _selectingStorePickupId = pickup.id;
    });
    final succeeded = await widget.onStorePickupSelect(pickup);
    if (!mounted) return;
    if (succeeded) {
      widget.onClose();
      return;
    }
    setState(() {
      _selectedStorePickupId = previous;
      _selectingStorePickupId = null;
    });
  }

  void _add(VoidCallback action) {
    widget.onClose();
    WidgetsBinding.instance.addPostFrameCallback((_) => action());
  }

  void _setMode(_LogisticsPickerMode mode) {
    if (_mode == mode) return;
    setState(() => _mode = mode);
  }

  Widget _buildDeliveryContent() {
    if (widget.addresses.isEmpty) {
      return const SingleChildScrollView(
        child: AddressEmptyState(
          title: '還沒有宅配地址',
          description: '先新增一筆地址，結帳時就能一鍵帶入。',
          compact: true,
        ),
      );
    }
    return ListView.separated(
      shrinkWrap: true,
      padding: const EdgeInsets.symmetric(
        horizontal: HkSpace.xl,
        vertical: HkSpace.md,
      ),
      itemCount: widget.addresses.length,
      separatorBuilder: (_, __) => const SizedBox(height: HkSpace.sm),
      itemBuilder: (context, index) {
        final address = widget.addresses[index];
        return AddressChoiceCard(
          address: address,
          selected: address.id == _selectedId,
          enabled: _selectingId == null,
          onTap: () => _select(address),
        );
      },
    );
  }

  Widget _buildStorePickupContent() {
    if (widget.storePickups.isEmpty) {
      return const SingleChildScrollView(
        child: AddressEmptyState(
          title: '還沒有 7-11 取貨門市',
          description: '先設定常用門市，結帳時就能一鍵帶入。',
          compact: true,
          icon: HkIconName.merchantShop,
        ),
      );
    }
    return ListView.separated(
      shrinkWrap: true,
      padding: const EdgeInsets.symmetric(
        horizontal: HkSpace.xl,
        vertical: HkSpace.md,
      ),
      itemCount: widget.storePickups.length,
      separatorBuilder: (_, __) => const SizedBox(height: HkSpace.sm),
      itemBuilder: (context, index) {
        final pickup = widget.storePickups[index];
        return StorePickupChoiceCard(
          pickup: pickup,
          selected: pickup.id == _selectedStorePickupId,
          enabled: _selectingStorePickupId == null,
          onTap: () => _selectStorePickup(pickup),
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final bodyMaxHeight = (MediaQuery.sizeOf(context).height * 0.56).clamp(
      180.0,
      480.0,
    );
    return Material(
      key: const ValueKey('address-picker-panel'),
      color: scheme.surface,
      borderRadius: const BorderRadius.vertical(
        top: Radius.circular(HkRadius.xl),
      ),
      clipBehavior: Clip.antiAlias,
      child: SafeArea(
        top: false,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            const HkSheetDragHandle(
              key: ValueKey('address-picker-drag-handle'),
            ),
            const _AddressSheetHeader(),
            Padding(
              padding: const EdgeInsets.fromLTRB(
                HkSpace.xl,
                HkSpace.md,
                HkSpace.xl,
                HkSpace.xs,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: _LogisticsModeButton(
                      key: const ValueKey('logistics-delivery-tab'),
                      label: '宅配地址',
                      count: widget.addresses.length,
                      selected: _mode == _LogisticsPickerMode.delivery,
                      onTap: () => _setMode(_LogisticsPickerMode.delivery),
                    ),
                  ),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(
                    child: _LogisticsModeButton(
                      key: const ValueKey('logistics-store-pickup-tab'),
                      label: '7-11 取貨',
                      count: widget.storePickups.length,
                      selected: _mode == _LogisticsPickerMode.storePickup,
                      onTap: () => _setMode(_LogisticsPickerMode.storePickup),
                    ),
                  ),
                ],
              ),
            ),
            ConstrainedBox(
              constraints: BoxConstraints(maxHeight: bodyMaxHeight),
              child: _mode == _LogisticsPickerMode.delivery
                  ? _buildDeliveryContent()
                  : _buildStorePickupContent(),
            ),
            DecoratedBox(
              decoration: BoxDecoration(
                border: Border(top: BorderSide(color: scheme.outlineVariant)),
              ),
              child: Padding(
                padding: const EdgeInsets.fromLTRB(
                  HkSpace.xl,
                  HkSpace.md,
                  HkSpace.xl,
                  HkSpace.md,
                ),
                child: AddressAddButton(
                  label: _mode == _LogisticsPickerMode.delivery
                      ? '新增宅配地址'
                      : '新增 7-11 取貨門市',
                  onPressed: () => _add(
                    _mode == _LogisticsPickerMode.delivery
                        ? widget.onAdd
                        : widget.onAddStorePickup,
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

class _AddressSheetHeader extends StatelessWidget {
  const _AddressSheetHeader();

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return DecoratedBox(
      decoration: BoxDecoration(
        border: Border(bottom: BorderSide(color: scheme.outlineVariant)),
      ),
      child: Padding(
        padding: const EdgeInsets.symmetric(
          horizontal: HkSpace.xl,
          vertical: HkSpace.md,
        ),
        child: Align(
          alignment: Alignment.centerLeft,
          child: Text(
            '物流設定',
            style: AppTypography.sectionTitle.copyWith(color: scheme.onSurface),
          ),
        ),
      ),
    );
  }
}

class _LogisticsModeButton extends StatelessWidget {
  const _LogisticsModeButton({
    super.key,
    required this.label,
    required this.count,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final int count;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return Semantics(
      button: true,
      selected: selected,
      label: '$label，$count 筆',
      child: Material(
        color: selected ? scheme.primaryContainer : scheme.surfaceContainer,
        borderRadius: BorderRadius.circular(HkRadius.pill),
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: onTap,
          child: ConstrainedBox(
            constraints: const BoxConstraints(minHeight: HkControl.minTap),
            child: Center(
              child: Text(
                '$label  $count',
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: AppTypography.meta.copyWith(
                  color: selected
                      ? scheme.onPrimaryContainer
                      : scheme.onSurfaceVariant,
                  fontWeight: FontWeight.w700,
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

class _DashedRoundedBorderPainter extends CustomPainter {
  const _DashedRoundedBorderPainter({
    required this.color,
    required this.radius,
  });

  final Color color;
  final double radius;

  @override
  void paint(Canvas canvas, Size size) {
    final paint = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1;
    final path = Path()
      ..addRRect(
        RRect.fromRectAndRadius(Offset.zero & size, Radius.circular(radius)),
      );
    for (final metric in path.computeMetrics()) {
      var distance = 0.0;
      while (distance < metric.length) {
        canvas.drawPath(metric.extractPath(distance, distance + 6), paint);
        distance += 10;
      }
    }
  }

  @override
  bool shouldRepaint(covariant _DashedRoundedBorderPainter oldDelegate) =>
      oldDelegate.color != color || oldDelegate.radius != radius;
}
