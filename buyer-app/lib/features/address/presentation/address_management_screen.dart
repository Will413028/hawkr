import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_twaddr/hawkr_twaddr.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/recipient_name_validator.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/common_top_bar.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../shared/widgets/app_dropdown.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_sheet_drag_handle.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../data/address_model.dart';
import '../data/store_pickup_model.dart';
import '../../cart/presentation/store_pickup_form_screen.dart';
import 'address_controller.dart';
import 'store_pickup_controller.dart';
import 'widgets/address_selection_widgets.dart';

/// 收貨地址管理頁面（使用後端 API）
class AddressManagementScreen extends ConsumerStatefulWidget {
  const AddressManagementScreen({super.key});

  @override
  ConsumerState<AddressManagementScreen> createState() =>
      _AddressManagementScreenState();
}

/// 地址表單送出的欄位。
typedef AddressFormResult = ({
  String receiverName,
  String receiverPhone,
  String townCode,
  String addressDetail,
  bool isDefault,
});

/// 地址流程共用的新增／編輯表單入口。
///
/// 首頁物流列與地址管理頁都必須走這裡，避免 sheet 外觀與欄位行為分叉。
Future<AddressFormResult?> showAddressFormSheet(
  BuildContext context, {
  Address? address,
}) {
  final scheme = Theme.of(context).colorScheme;
  return showModalBottomSheet<AddressFormResult>(
    context: context,
    backgroundColor: scheme.surface,
    barrierColor: scheme.scrim.withValues(alpha: 0.40),
    elevation: 0,
    isScrollControlled: true,
    showDragHandle: false,
    clipBehavior: Clip.antiAlias,
    constraints: const BoxConstraints(maxWidth: 520),
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(HkRadius.xl)),
    ),
    builder: (_) => AddressFormSheet(address: address),
  );
}

class _AddressManagementScreenState
    extends ConsumerState<AddressManagementScreen> {
  bool _deliveryExpanded = true;
  bool _storePickupExpanded = false;

  Future<void> _showAddAddressBottomSheet() async {
    final result = await showAddressFormSheet(context);
    if (result != null) {
      if (mounted) TruckLoadingOverlay.show(context);
      try {
        await ref
            .read(addressControllerProvider.notifier)
            .add(
              receiverName: result.receiverName,
              receiverPhone: result.receiverPhone,
              townCode: result.townCode,
              addressDetail: result.addressDetail,
              isDefault: result.isDefault,
            );
        if (mounted) {
          TruckLoadingOverlay.hide(context);
          showFloatingToast(context, '地址新增成功');
        }
      } catch (e) {
        if (mounted) {
          TruckLoadingOverlay.hide(context);
          showFloatingToast(context, '新增地址失敗：$e', isSuccess: false);
        }
      }
    }
  }

  Future<void> _showEditAddressBottomSheet(Address addr) async {
    final result = await showAddressFormSheet(context, address: addr);
    if (result != null) {
      if (mounted) TruckLoadingOverlay.show(context);
      try {
        await ref
            .read(addressControllerProvider.notifier)
            .updateAddress(
              addr.id,
              etag: addr.etag,
              receiverName: result.receiverName,
              receiverPhone: result.receiverPhone,
              townCode: result.townCode,
              addressDetail: result.addressDetail,
              isDefault: result.isDefault,
            );
        if (mounted) {
          TruckLoadingOverlay.hide(context);
          showFloatingToast(context, '地址更新成功');
        }
      } catch (e) {
        if (mounted) {
          TruckLoadingOverlay.hide(context);
          showFloatingToast(context, '更新地址失敗：$e', isSuccess: false);
        }
      }
    }
  }

  Future<void> _setDefault(Address address) async {
    try {
      await ref.read(addressControllerProvider.notifier).setDefault(address.id);
    } catch (e) {
      if (mounted) {
        showFloatingToast(context, '設定預設地址失敗：$e', isSuccess: false);
      }
    }
  }

  Future<void> _confirmDelete(Address addr) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (ctx) => AlertDialog(
        title: const Text('刪除地址'),
        content: Text('確定要刪除「${addr.receiverName}」的地址嗎？'),
        actions: [
          TextButton(
            onPressed: () => ctx.pop(false),
            child: const HkButtonLabel('取消'),
          ),
          TextButton(
            onPressed: () => ctx.pop(true),
            child: const HkButtonLabel(
              '刪除',
              style: TextStyle(color: AppColors.danger),
            ),
          ),
        ],
      ),
    );
    if (confirmed == true) {
      if (mounted) TruckLoadingOverlay.show(context);
      try {
        await ref.read(addressControllerProvider.notifier).delete(addr.id);
        if (mounted) {
          TruckLoadingOverlay.hide(context);
          showFloatingToast(context, '地址已刪除');
        }
      } catch (e) {
        if (mounted) {
          TruckLoadingOverlay.hide(context);
          showFloatingToast(context, '刪除失敗：$e', isSuccess: false);
        }
      }
    }
  }

  Future<void> _openStorePickupForm([StorePickup? pickup]) async {
    final changed = await showStorePickupFormSheet(
      context,
      existingStore: pickup?.toJson(),
    );
    if (changed == true && mounted) {
      setState(() => _storePickupExpanded = true);
    }
  }

  Future<void> _setDefaultStorePickup(StorePickup pickup) async {
    try {
      await ref.read(storePickupControllerProvider.notifier).setDefault(pickup);
    } catch (error) {
      if (mounted) {
        showFloatingToast(context, '設定預設門市失敗：$error', isSuccess: false);
      }
    }
  }

  Future<void> _confirmDeleteStorePickup(StorePickup pickup) async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('刪除取貨門市'),
        content: Text('確定要刪除「${pickup.storeName}」嗎？'),
        actions: [
          TextButton(
            onPressed: () => dialogContext.pop(false),
            child: const HkButtonLabel('取消'),
          ),
          TextButton(
            onPressed: () => dialogContext.pop(true),
            child: const HkButtonLabel(
              '刪除',
              style: TextStyle(color: AppColors.danger),
            ),
          ),
        ],
      ),
    );
    if (confirmed != true) return;
    if (mounted) TruckLoadingOverlay.show(context);
    try {
      await ref.read(storePickupControllerProvider.notifier).delete(pickup.id);
      if (mounted) {
        TruckLoadingOverlay.hide(context);
        showFloatingToast(context, '取貨門市已刪除');
      }
    } catch (error) {
      if (mounted) {
        TruckLoadingOverlay.hide(context);
        showFloatingToast(context, '刪除取貨門市失敗：$error', isSuccess: false);
      }
    }
  }

  String _addressSummary(AsyncValue<List<Address>> state) => switch (state) {
    AsyncData(:final value) when value.isNotEmpty => () {
      final address =
          value.where((item) => item.isDefault).firstOrNull ?? value.first;
      final name = address.receiverName.trim();
      return [
        name,
        address.fullAddress,
      ].where((part) => part.isNotEmpty).join(' · ');
    }(),
    AsyncData() => '尚未設定',
    AsyncError() => '暫時無法載入',
    _ => '載入中…',
  };

  String _storePickupSummary(AsyncValue<List<StorePickup>> state) =>
      switch (state) {
        AsyncData(:final value) when value.isNotEmpty => () {
          final pickup =
              value.where((item) => item.isDefault).firstOrNull ?? value.first;
          final storeName = pickup.storeName.isEmpty
              ? '7-11 取貨門市'
              : pickup.storeName;
          return '$storeName · ${pickup.storeCode}';
        }(),
        AsyncData() => '尚未設定',
        AsyncError() => '暫時無法載入',
        _ => '載入中…',
      };

  Widget _buildAddressContent(AsyncValue<List<Address>> state) {
    return state.when(
      loading: () => const SizedBox(height: 160, child: TruckLoadingOverlay()),
      error: (error, _) => _AddressLoadError(message: '載入地址失敗：$error'),
      data: (addresses) {
        if (addresses.isEmpty) {
          return AddressEmptyState(
            title: '還沒有宅配地址',
            description: '先新增至少一筆地址，結帳時就能一鍵帶入。',
            actionLabel: '新增第一筆地址',
            onAction: _showAddAddressBottomSheet,
            compact: true,
          );
        }
        return Column(
          children: [
            for (var index = 0; index < addresses.length; index++) ...[
              AddressChoiceCard(
                address: addresses[index],
                selected: addresses[index].isDefault,
                onTap: () => _setDefault(addresses[index]),
                onEdit: () => _showEditAddressBottomSheet(addresses[index]),
                onDelete: () => _confirmDelete(addresses[index]),
              ),
              if (index != addresses.length - 1)
                const SizedBox(height: HkSpace.sm),
            ],
            const SizedBox(height: HkSpace.lg),
            AddressAddButton(
              label: '新增宅配地址',
              onPressed: _showAddAddressBottomSheet,
            ),
          ],
        );
      },
    );
  }

  Widget _buildStorePickupContent(AsyncValue<List<StorePickup>> state) {
    return state.when(
      loading: () => const SizedBox(height: 160, child: TruckLoadingOverlay()),
      error: (error, _) => _AddressLoadError(message: '載入取貨門市失敗：$error'),
      data: (pickups) {
        if (pickups.isEmpty) {
          return AddressEmptyState(
            title: '還沒有取貨門市',
            description: '先設定常用的 7-11 門市，結帳時就能一鍵帶入。',
            actionLabel: '新增取貨門市',
            onAction: _openStorePickupForm,
            compact: true,
            icon: HkIconName.merchantShop,
          );
        }
        return Column(
          children: [
            for (var index = 0; index < pickups.length; index++) ...[
              StorePickupChoiceCard(
                pickup: pickups[index],
                selected: pickups[index].isDefault,
                onTap: () => _setDefaultStorePickup(pickups[index]),
                onEdit: () => _openStorePickupForm(pickups[index]),
                onDelete: () => _confirmDeleteStorePickup(pickups[index]),
              ),
              if (index != pickups.length - 1)
                const SizedBox(height: HkSpace.sm),
            ],
            const SizedBox(height: HkSpace.lg),
            AddressAddButton(
              label: '新增 7-11 取貨門市',
              onPressed: _openStorePickupForm,
            ),
          ],
        );
      },
    );
  }

  @override
  Widget build(BuildContext context) {
    final addressState = ref.watch(addressControllerProvider);
    final storePickupState = ref.watch(storePickupControllerProvider);
    final addressCount = addressState.value?.length ?? 0;
    final storePickupCount = storePickupState.value?.length ?? 0;

    return Scaffold(
      backgroundColor: AppColors.bgPrimary,
      appBar: buildCommonTitleAppBar(
        context,
        title: '收件地址',
        onBack: () => context.pop(),
      ),
      body: SafeArea(
        top: false,
        child: LayoutBuilder(
          builder: (context, constraints) {
            final horizontalPadding = HkBreakpoints.pageGutter(
              constraints.maxWidth,
            );
            return SingleChildScrollView(
              padding: EdgeInsets.fromLTRB(
                horizontalPadding,
                HkSpace.xl,
                horizontalPadding,
                HkSpace.large,
              ),
              child: Center(
                child: ConstrainedBox(
                  constraints: const BoxConstraints(maxWidth: 720),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(
                        '管理宅配地址與 7-11 取貨門市，結帳時會自動帶入預設選項。',
                        style: AppTypography.metaMuted,
                      ),
                      const SizedBox(height: HkSpace.xxl),
                      AddressCollapsibleSection(
                        title: '宅配地址',
                        description: '結帳選擇宅配到家時，會自動帶入預設地址。',
                        summary: _addressSummary(addressState),
                        count: addressCount,
                        expanded: _deliveryExpanded,
                        onToggle: () => setState(
                          () => _deliveryExpanded = !_deliveryExpanded,
                        ),
                        leading: const HkIcon(HkIconName.truck, size: 32),
                        child: _buildAddressContent(addressState),
                      ),
                      const SizedBox(height: HkSpace.md),
                      AddressCollapsibleSection(
                        title: '7-11 取貨門市',
                        description: '結帳選擇 7-11 取貨時，會自動帶入預設門市。',
                        summary: _storePickupSummary(storePickupState),
                        count: storePickupCount,
                        expanded: _storePickupExpanded,
                        onToggle: () => setState(
                          () => _storePickupExpanded = !_storePickupExpanded,
                        ),
                        leading: const SizedBox(
                          width: 34,
                          height: 34,
                          child: Center(
                            child: HkIcon(
                              HkIconName.store,
                              size: 24,
                              color: AppIconColors.defaultColor,
                            ),
                          ),
                        ),
                        child: _buildStorePickupContent(storePickupState),
                      ),
                    ],
                  ),
                ),
              ),
            );
          },
        ),
      ),
    );
  }
}

class _AddressLoadError extends StatelessWidget {
  const _AddressLoadError({required this.message});

  final String message;

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    return DecoratedBox(
      decoration: BoxDecoration(
        color: scheme.errorContainer,
        borderRadius: BorderRadius.circular(HkRadius.md),
        border: Border.all(color: scheme.error.withValues(alpha: 0.30)),
      ),
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.xl),
        child: Text(
          message,
          textAlign: TextAlign.center,
          style: AppTypography.meta.copyWith(color: scheme.onErrorContainer),
        ),
      ),
    );
  }
}

/// 新增／編輯宅配地址表單，視覺與 Web `AddressPickerSheet` 共用同一規範。
class AddressFormSheet extends ConsumerStatefulWidget {
  const AddressFormSheet({super.key, this.address});

  final Address? address;

  @override
  ConsumerState<AddressFormSheet> createState() => _AddressFormSheetState();
}

class _AddressFormSheetState extends ConsumerState<AddressFormSheet> {
  final _nameController = TextEditingController();
  final _phoneController = TextEditingController();
  final _addressController = TextEditingController();

  // 縣市 + 鄉鎮市區下拉：值是 hawkr_twaddr 的 County／Town，送出的是 Town.code。
  County? _county;
  Town? _town;
  bool _isDefault = false;

  bool get _isEditing => widget.address != null;

  @override
  void initState() {
    super.initState();
    if (widget.address != null) {
      final a = widget.address!;
      _nameController.text = a.receiverName;
      _phoneController.text = a.receiverPhone;
      _town = townByCode(a.townCode);
      final town = _town;
      _county = town == null ? null : countyByCode(town.countyCode);
      _addressController.text = a.addressDetail;
      _isDefault = a.isDefault;
    }
  }

  @override
  void dispose() {
    _nameController.dispose();
    _phoneController.dispose();
    _addressController.dispose();
    super.dispose();
  }

  bool get _isFormValid {
    final hasName = _nameController.text.trim().isNotEmpty;
    final hasPhone = _phoneController.text.trim().isNotEmpty;
    final hasCounty = _county != null;
    final hasTown = _town != null;
    final hasAddress = _addressController.text.trim().isNotEmpty;
    return hasName && hasPhone && hasCounty && hasTown && hasAddress;
  }

  Future<void> _onSave() async {
    if (!_isFormValid) return;

    // 收貨人姓名顯示長度檢查（對齊後端 PayUni 約束 [4,30]）
    final nameError = RecipientNameValidator.validateAddress(
      _nameController.text,
    );
    if (nameError != null) {
      showFloatingToast(context, nameError, isSuccess: false);
      return;
    }

    if (!mounted) return;
    final AddressFormResult result = (
      receiverName: _nameController.text.trim(),
      receiverPhone: _phoneController.text.trim(),
      townCode: _town!.code,
      addressDetail: _addressController.text.trim(),
      isDefault: _isDefault,
    );
    Navigator.pop(context, result);
  }

  @override
  Widget build(BuildContext context) {
    final scheme = Theme.of(context).colorScheme;
    final media = MediaQuery.of(context);
    final maxSheetHeight =
        (media.size.height - media.padding.top - media.viewInsets.bottom) *
        0.86;

    return SafeArea(
      top: false,
      child: AnimatedPadding(
        duration: HkMotion.duration,
        curve: HkMotion.ease,
        padding: EdgeInsets.only(bottom: media.viewInsets.bottom),
        child: ConstrainedBox(
          constraints: BoxConstraints(maxHeight: maxSheetHeight),
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const HkSheetDragHandle(
                key: ValueKey('address-form-drag-handle'),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: HkSpace.lg,
                  vertical: HkSpace.md,
                ),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    _isEditing ? '編輯宅配地址' : '新增宅配地址',
                    key: const ValueKey('address-form-title'),
                    style: AppTypography.sectionTitle.copyWith(
                      color: scheme.onSurface,
                    ),
                  ),
                ),
              ),
              Divider(height: 1, color: scheme.outlineVariant),
              Flexible(
                child: SingleChildScrollView(
                  key: const ValueKey('address-form-scroll'),
                  keyboardDismissBehavior:
                      ScrollViewKeyboardDismissBehavior.onDrag,
                  padding: const EdgeInsets.fromLTRB(
                    HkSpace.lg,
                    HkSpace.md,
                    HkSpace.lg,
                    HkSpace.lg,
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      _buildWebTextField(
                        key: const ValueKey('address-name-field'),
                        label: '收件人姓名',
                        controller: _nameController,
                        maxLength: 20,
                        errorText: _nameController.text.trim().isEmpty
                            ? null
                            : RecipientNameValidator.validateAddress(
                                _nameController.text,
                              ),
                      ),
                      const SizedBox(height: HkSpace.md),
                      _buildWebTextField(
                        key: const ValueKey('address-phone-field'),
                        label: '聯絡電話',
                        controller: _phoneController,
                        keyboardType: TextInputType.phone,
                        maxLength: 20,
                      ),
                      const SizedBox(height: HkSpace.md),
                      AppDropdown<County>(
                        key: const ValueKey('address-city-dropdown'),
                        items: counties,
                        itemLabel: (c) => c.name,
                        selectedItem: _county,
                        placeholder: '縣市',
                        height: HkControl.minTap,
                        showSearch: true,
                        semanticLabel: '縣市下拉選單',
                        onSelected: (county) => setState(() {
                          _county = county;
                          _town = null;
                        }),
                      ),
                      const SizedBox(height: HkSpace.md),
                      AppDropdown<Town>(
                        key: const ValueKey('address-district-dropdown'),
                        items: _county == null
                            ? const []
                            : townsByCounty(_county!.code),
                        itemLabel: (t) => t.name,
                        selectedItem: _town,
                        placeholder: _county == null ? '請先選擇縣市' : '鄉鎮市區',
                        height: HkControl.minTap,
                        showSearch: true,
                        enabled: _county != null,
                        semanticLabel: '鄉鎮市區下拉選單',
                        onSelected: (town) => setState(() => _town = town),
                      ),
                      const SizedBox(height: HkSpace.md),
                      _buildWebTextField(
                        label: '詳細地址',
                        controller: _addressController,
                        maxLength: 255,
                      ),
                      const SizedBox(height: HkSpace.sm),
                      CheckboxListTile(
                        key: const ValueKey('address-default-checkbox'),
                        value: _isDefault,
                        onChanged: (value) {
                          setState(() => _isDefault = value ?? false);
                        },
                        title: Text(
                          '設為預設地址',
                          style: AppTypography.metaMuted.copyWith(
                            color: scheme.onSurface.withValues(alpha: 0.68),
                          ),
                        ),
                        activeColor: scheme.primary,
                        checkColor: scheme.onPrimary,
                        side: BorderSide(color: scheme.outline),
                        controlAffinity: ListTileControlAffinity.leading,
                        contentPadding: EdgeInsets.zero,
                        visualDensity: VisualDensity.compact,
                      ),
                      const SizedBox(height: HkSpace.xxl),
                      Row(
                        children: [
                          Expanded(
                            child: SizedBox(
                              height: HkControl.minTap,
                              child: OutlinedButton(
                                key: const ValueKey('address-cancel-button'),
                                onPressed: () => Navigator.pop(context),
                                style: OutlinedButton.styleFrom(
                                  foregroundColor: scheme.onSurface,
                                  backgroundColor: scheme.surface,
                                  side: BorderSide(
                                    color: scheme.outlineVariant,
                                  ),
                                  shape: const StadiumBorder(),
                                ),
                                child: const HkButtonLabel('取消'),
                              ),
                            ),
                          ),
                          const SizedBox(width: HkSpace.sm),
                          Expanded(
                            child: SizedBox(
                              height: HkControl.minTap,
                              child: FilledButton(
                                key: const ValueKey('address-save-button'),
                                onPressed: _isFormValid ? _onSave : null,
                                style: FilledButton.styleFrom(
                                  shape: const StadiumBorder(),
                                ),
                                child: HkButtonLabel(
                                  _isEditing ? '更新地址' : '儲存地址',
                                ),
                              ),
                            ),
                          ),
                        ],
                      ),
                    ],
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _buildWebTextField({
    Key? key,
    required String label,
    required TextEditingController controller,
    TextInputType? keyboardType,
    String? errorText,
    int? maxLength,
    Widget? suffixIcon,
  }) {
    final scheme = Theme.of(context).colorScheme;
    final radius = BorderRadius.circular(HkRadius.md);
    return TextField(
      key: key,
      controller: controller,
      keyboardType: keyboardType,
      maxLength: maxLength,
      onChanged: (_) => setState(() {}),
      style: AppTypography.body.copyWith(color: scheme.onSurface),
      decoration: InputDecoration(
        hintText: label,
        hintStyle: AppTypography.body.copyWith(
          color: scheme.onSurface.withValues(alpha: 0.55),
        ),
        errorText: errorText,
        errorStyle: AppTypography.caption.copyWith(color: scheme.error),
        counterText: '',
        filled: true,
        fillColor: scheme.surface,
        isDense: true,
        constraints: const BoxConstraints(minHeight: HkControl.minTap),
        contentPadding: const EdgeInsets.symmetric(
          horizontal: HkSpace.lg,
          vertical: HkSpace.md,
        ),
        suffixIcon: suffixIcon,
        suffixIconConstraints: const BoxConstraints(
          minWidth: HkControl.minTap,
          minHeight: HkControl.minTap,
        ),
        border: OutlineInputBorder(
          borderRadius: radius,
          borderSide: BorderSide(color: scheme.outlineVariant, width: 2),
        ),
        enabledBorder: OutlineInputBorder(
          borderRadius: radius,
          borderSide: BorderSide(color: scheme.outlineVariant, width: 2),
        ),
        focusedBorder: OutlineInputBorder(
          borderRadius: radius,
          borderSide: BorderSide(color: scheme.primary, width: 2),
        ),
        errorBorder: OutlineInputBorder(
          borderRadius: radius,
          borderSide: BorderSide(color: scheme.error, width: 2),
        ),
        focusedErrorBorder: OutlineInputBorder(
          borderRadius: radius,
          borderSide: BorderSide(color: scheme.error, width: 2),
        ),
      ),
    );
  }
}
