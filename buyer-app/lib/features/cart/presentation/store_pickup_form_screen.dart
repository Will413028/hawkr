import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/recipient_name_validator.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_sheet_drag_handle.dart';
import '../../../shared/widgets/floating_toast.dart';
import '../../address/data/store_pickup_model.dart';
import '../../address/presentation/store_pickup_controller.dart';
import 'store_map_webview_screen.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

/// 7-11 門市流程共用的新增／編輯表單入口。
///
/// 首頁、地址管理與購物車都必須走同一個 bottom sheet，避免交互分叉。
Future<bool?> showStorePickupFormSheet(
  BuildContext context, {
  Map<String, dynamic>? existingStore,
}) {
  final scheme = Theme.of(context).colorScheme;
  return showModalBottomSheet<bool>(
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
    builder: (_) => StorePickupFormSheet(existingStore: existingStore),
  );
}

/// 與宅配地址表單採用相同版型與操作節奏的 7-11 門市表單。
class StorePickupFormSheet extends ConsumerStatefulWidget {
  const StorePickupFormSheet({super.key, this.existingStore});

  final Map<String, dynamic>? existingStore;

  @override
  ConsumerState<StorePickupFormSheet> createState() =>
      _StorePickupFormSheetState();
}

class _StorePickupFormSheetState extends ConsumerState<StorePickupFormSheet> {
  final _nameController = TextEditingController();
  final _phoneController = TextEditingController();

  bool _isDefault = false;
  bool _isSaving = false;
  bool _isOpeningMap = false;
  String? _storeCode;
  String? _storeName;
  String? _storeAddress;

  bool get _isEditing => widget.existingStore != null;

  bool get _isFormComplete =>
      _nameController.text.trim().isNotEmpty &&
      _phoneController.text.trim().isNotEmpty &&
      (_storeCode?.isNotEmpty ?? false) &&
      (_storeName?.isNotEmpty ?? false);

  bool get _canSave => _isFormComplete && !_isSaving;

  @override
  void initState() {
    super.initState();
    final store = widget.existingStore;
    if (store != null) {
      _nameController.text = store['receiverName']?.toString() ?? '';
      _phoneController.text = store['receiverPhone']?.toString() ?? '';
      _storeCode = store['storeCode']?.toString();
      _storeName = store['storeName']?.toString();
      _storeAddress = store['storeAddress']?.toString();
      final rawDefault = store['isDefault'];
      _isDefault = rawDefault == true || rawDefault?.toString() == 'true';
    }
    _nameController.addListener(_onFieldChanged);
    _phoneController.addListener(_onFieldChanged);
  }

  @override
  void dispose() {
    _nameController
      ..removeListener(_onFieldChanged)
      ..dispose();
    _phoneController
      ..removeListener(_onFieldChanged)
      ..dispose();
    super.dispose();
  }

  void _onFieldChanged() => setState(() {});

  Future<void> _openStoreMap() async {
    if (_isOpeningMap) return;
    setState(() => _isOpeningMap = true);
    try {
      final redirectUrl = await ref
          .read(storePickupControllerProvider.notifier)
          .createMapSession();
      if (!mounted) return;
      setState(() => _isOpeningMap = false);
      final result = await Navigator.of(context).push<Map<String, String>>(
        MaterialPageRoute(
          builder: (_) => StoreMapWebViewScreen(redirectUrl: redirectUrl),
        ),
      );
      if (result == null || !mounted) return;
      final error = result['error'] ?? result['message'] ?? '';
      if (error.isNotEmpty) {
        FloatingToast.showError(error);
        return;
      }
      final storeId = result['storeId'] ?? '';
      if (storeId.isEmpty) {
        FloatingToast.showError('未取得門市資料，請重新選擇');
        return;
      }
      setState(() {
        _storeCode = storeId;
        _storeName = result['storeName'] ?? '';
        _storeAddress = result['address'] ?? '';
      });
    } catch (error) {
      if (mounted) FloatingToast.showError(error.toString());
    } finally {
      if (mounted && _isOpeningMap) {
        setState(() => _isOpeningMap = false);
      }
    }
  }

  Future<void> _save() async {
    if (!_canSave) return;
    final nameError = RecipientNameValidator.validateStorePickup(
      _nameController.text,
    );
    if (nameError != null) {
      FloatingToast.showError(nameError);
      return;
    }

    setState(() => _isSaving = true);
    final pickup = StorePickup(
      id: widget.existingStore?['id']?.toString() ?? '',
      receiverName: _nameController.text.trim(),
      receiverPhone: _phoneController.text.trim(),
      storeCode: _storeCode ?? '',
      storeName: _storeName ?? '',
      storeAddress: _storeAddress ?? '',
      isDefault: _isDefault,
    );
    try {
      await ref.read(storePickupControllerProvider.notifier).save(pickup);
      if (!mounted) return;
      FloatingToast.showSuccess(_isEditing ? '門市已更新' : '門市已新增');
      Navigator.of(context).pop(true);
    } catch (error) {
      if (!mounted) return;
      FloatingToast.showError(error.toString());
      setState(() => _isSaving = false);
    }
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
                key: ValueKey('store-pickup-form-drag-handle'),
              ),
              Padding(
                padding: const EdgeInsets.symmetric(
                  horizontal: HkSpace.lg,
                  vertical: HkSpace.md,
                ),
                child: Align(
                  alignment: Alignment.centerLeft,
                  child: Text(
                    _isEditing ? '編輯 7-11 取貨門市' : '新增 7-11 取貨門市',
                    key: const ValueKey('store-pickup-form-title'),
                    style: AppTypography.sectionTitle.copyWith(
                      color: scheme.onSurface,
                    ),
                  ),
                ),
              ),
              Divider(height: 1, color: scheme.outlineVariant),
              Flexible(
                child: SingleChildScrollView(
                  key: const ValueKey('store-pickup-form-scroll'),
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
                      _buildIdentityNotice(scheme),
                      const SizedBox(height: HkSpace.md),
                      _buildTextField(
                        key: const ValueKey('store-pickup-name-field'),
                        label: '收件人姓名',
                        controller: _nameController,
                        keyboardType: TextInputType.name,
                        maxLength: 30,
                        errorText: _nameController.text.trim().isEmpty
                            ? null
                            : RecipientNameValidator.validateStorePickup(
                                _nameController.text,
                              ),
                      ),
                      const SizedBox(height: HkSpace.md),
                      _buildTextField(
                        key: const ValueKey('store-pickup-phone-field'),
                        label: '聯絡電話',
                        controller: _phoneController,
                        keyboardType: TextInputType.phone,
                        maxLength: 10,
                        inputFormatters: [
                          FilteringTextInputFormatter.digitsOnly,
                        ],
                      ),
                      const SizedBox(height: HkSpace.md),
                      _buildStoreSelectionField(scheme),
                      const SizedBox(height: HkSpace.sm),
                      SizedBox(
                        height: HkControl.minTap,
                        child: FilledButton.icon(
                          key: const ValueKey('store-pickup-map-button'),
                          onPressed: _isOpeningMap ? null : _openStoreMap,
                          style: FilledButton.styleFrom(
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(HkRadius.md),
                            ),
                          ),
                          icon: _isOpeningMap
                              ? SizedBox.square(
                                  dimension: HkSpace.xl,
                                  child: HkBrandLoader(
                                    tone: HkBrandLoaderTone.mono,
                                    color: scheme.onPrimary,
                                  ),
                                )
                              : const HkIcon(HkIconName.location, size: 22),
                          label: HkButtonLabel(
                            _isOpeningMap
                                ? '開啟門市地圖中…'
                                : _storeCode == null
                                ? '選擇 7-11 門市'
                                : '重新選擇門市',
                          ),
                        ),
                      ),
                      const SizedBox(height: HkSpace.sm),
                      CheckboxListTile(
                        key: const ValueKey('store-pickup-default-checkbox'),
                        value: _isDefault,
                        onChanged: (value) {
                          setState(() => _isDefault = value ?? false);
                        },
                        title: Text(
                          '設為預設 7-11 門市',
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
                                key: const ValueKey(
                                  'store-pickup-cancel-button',
                                ),
                                onPressed: () => Navigator.of(context).pop(),
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
                                key: const ValueKey('store-pickup-save-button'),
                                onPressed: _canSave ? _save : null,
                                style: FilledButton.styleFrom(
                                  shape: const StadiumBorder(),
                                ),
                                child: _isSaving
                                    ? SizedBox.square(
                                        dimension: HkSpace.xl,
                                        child: HkBrandLoader(
                                          tone: HkBrandLoaderTone.mono,
                                          color: scheme.onPrimary,
                                        ),
                                      )
                                    : HkButtonLabel(
                                        _isEditing ? '更新門市' : '儲存門市',
                                      ),
                              ),
                            ),
                          ),
                        ],
                      ),
                      if (!_isFormComplete) ...[
                        const SizedBox(height: HkSpace.sm),
                        Text(
                          '請填寫收件資料並選擇門市',
                          textAlign: TextAlign.right,
                          style: AppTypography.caption.copyWith(
                            color: scheme.onSurface.withValues(alpha: 0.60),
                          ),
                        ),
                      ],
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

  Widget _buildIdentityNotice(ColorScheme scheme) {
    return DecoratedBox(
      decoration: BoxDecoration(
        color: AppColors.warningSoft,
        borderRadius: BorderRadius.circular(HkRadius.md),
      ),
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.md),
        child: Row(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            HkIcon(
              HkIconName.warning,
              size: 22,
              color: scheme.onSurfaceVariant,
            ),
            const SizedBox(width: HkSpace.sm),
            Expanded(
              child: Text(
                '請填寫與身分證相符的真實姓名，以便門市人員核對取貨。',
                style: AppTypography.meta.copyWith(
                  color: scheme.onSurface,
                  height: 1.5,
                ),
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _buildStoreSelectionField(ColorScheme scheme) {
    final hasStore =
        (_storeCode?.isNotEmpty ?? false) && (_storeName?.isNotEmpty ?? false);
    return Material(
      key: const ValueKey('store-pickup-selection-field'),
      color: scheme.surface,
      shape: RoundedRectangleBorder(
        borderRadius: BorderRadius.circular(HkRadius.md),
        side: BorderSide(color: scheme.outlineVariant, width: 2),
      ),
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: _isOpeningMap ? null : _openStoreMap,
        child: ConstrainedBox(
          constraints: const BoxConstraints(minHeight: HkControl.minTap),
          child: Padding(
            padding: const EdgeInsets.symmetric(
              horizontal: HkSpace.md,
              vertical: HkSpace.sm,
            ),
            child: Row(
              children: [
                const SizedBox(
                  width: 32,
                  height: 32,
                  child: Center(
                    child: HkIcon(
                      HkIconName.store,
                      size: 24,
                      color: AppIconColors.defaultColor,
                    ),
                  ),
                ),
                const SizedBox(width: HkSpace.md),
                Expanded(
                  child: hasStore
                      ? Column(
                          crossAxisAlignment: CrossAxisAlignment.start,
                          mainAxisSize: MainAxisSize.min,
                          children: [
                            Text(
                              _storeName!,
                              style: AppTypography.meta.copyWith(
                                color: scheme.onSurface,
                                fontWeight: FontWeight.w700,
                              ),
                            ),
                            const SizedBox(height: 2),
                            Text(
                              [
                                '店號 ${_storeCode!}',
                                if (_storeAddress?.isNotEmpty ?? false)
                                  _storeAddress!,
                              ].join(' · '),
                              maxLines: 2,
                              overflow: TextOverflow.ellipsis,
                              style: AppTypography.caption.copyWith(
                                color: scheme.onSurfaceVariant,
                                height: 1.4,
                              ),
                            ),
                          ],
                        )
                      : Text(
                          '尚未選擇 7-11 門市',
                          style: AppTypography.body.copyWith(
                            color: scheme.onSurface.withValues(alpha: 0.55),
                          ),
                        ),
                ),
                const SizedBox(width: HkSpace.sm),
                HkIcon(
                  HkIconName.chevronRight,
                  size: HkControl.icon,
                  color: scheme.onSurfaceVariant,
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildTextField({
    Key? key,
    required String label,
    required TextEditingController controller,
    TextInputType? keyboardType,
    String? errorText,
    int? maxLength,
    List<TextInputFormatter>? inputFormatters,
  }) {
    final scheme = Theme.of(context).colorScheme;
    final radius = BorderRadius.circular(HkRadius.md);
    return TextField(
      key: key,
      controller: controller,
      keyboardType: keyboardType,
      maxLength: maxLength,
      maxLengthEnforcement: MaxLengthEnforcement.enforced,
      inputFormatters: inputFormatters,
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
