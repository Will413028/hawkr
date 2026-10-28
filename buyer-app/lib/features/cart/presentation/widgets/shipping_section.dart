import '../cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:flutter/material.dart';
import '../../../../shared/widgets/hk_button_label.dart';
import 'package:hawkr_buyer/core/codemeta/code_meta.dart';
import 'package:hawkr_buyer/core/codemeta/shipping_presentation.dart';

import '../../../../shared/widgets/location_icon.dart';
import '../../../address/data/address_model.dart';
import 'temperature_tags.dart';
import 'package:hawkr_buyer/shared/widgets/hk_temperature_tag.dart';

import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

// ---------------------------------------------------------------------------
// 配送方式區塊。
//
// 白色圓角卡片，包含：
// - 驗證錯誤提示面板（無地址 / 超重超材積）
// - 分組可折疊的配送選項（黑貓宅急便 / 7-11 超取 等）
// - 每組內的 Radio 選擇行
// - 收件地址顯示 / 超商取貨門市選擇
//
// 所有狀態由外部傳入，互動透過回調通知主畫面。
// ---------------------------------------------------------------------------

/// 購物車的配送方式區塊。
///
/// 顯示配送選項（分組、可折疊）、驗證錯誤提示、
/// 收件地址、超商取貨門市等。
class ShippingSection extends StatefulWidget {
  const ShippingSection({
    super.key,
    required this.shippingOptions,
    required this.selectedShippingIdx,
    required this.validateResult,
    required this.validateLoading,
    required this.selectedAddress,
    required this.userAddresses,
    required this.storePickups,
    required this.selectedStorePickupId,
    required this.checkedSkuIds,
    required this.onShippingIdxChanged,
    required this.onStorePickupSelected,
    required this.onExpandedGroupsChanged,
    required this.onShowAddressPicker,
    required this.onOpenStorePickupForm,
    required this.onAddAddress,
    required this.onRetryValidate,
  });

  final List<Map<String, dynamic>> shippingOptions;
  final int selectedShippingIdx;
  final Map<String, dynamic>? validateResult;
  final bool validateLoading;
  final Address? selectedAddress;
  final List<Address> userAddresses;
  final List<Map<String, dynamic>> storePickups;
  final String? selectedStorePickupId;
  final Set<int> checkedSkuIds;

  /// 選擇配送選項時回調（傳入新的 index）。
  final ValueChanged<int> onShippingIdxChanged;

  /// 選擇超商門市時回調。
  final ValueChanged<String> onStorePickupSelected;

  /// 展開/收合配送分組時回調。
  final ValueChanged<Set<String>> onExpandedGroupsChanged;

  /// 點擊地址選擇。
  final VoidCallback onShowAddressPicker;

  /// 打開超商取貨表單（null = 新增，非 null = 編輯）。
  final void Function(Map<String, dynamic>? existingStore)
  onOpenStorePickupForm;

  /// 新增地址。
  final VoidCallback onAddAddress;

  /// 重新驗證。
  final VoidCallback onRetryValidate;

  @override
  State<ShippingSection> createState() => _ShippingSectionState();
}

class _ShippingSectionState extends State<ShippingSection> {
  /// 當前展開的配送分組 key 集合。
  Set<String> _expandedGroups = {};

  @override
  void didUpdateWidget(covariant ShippingSection oldWidget) {
    super.didUpdateWidget(oldWidget);
    // 自動展開包含已選選項的分組
    if (widget.shippingOptions.isNotEmpty && _expandedGroups.isEmpty) {
      final selectedOpt = _safeSelectedOption();
      final method = selectedOpt['shippingMethod']?.toString() ?? '';
      if (method.isNotEmpty) {
        _expandedGroups = {method};
      }
    }
  }

  @override
  void initState() {
    super.initState();
    // 初始展開
    if (widget.shippingOptions.isNotEmpty) {
      final selectedOpt = _safeSelectedOption();
      final method = selectedOpt['shippingMethod']?.toString() ?? '';
      if (method.isNotEmpty) {
        _expandedGroups = {method};
      }
    }
  }

  Map<String, dynamic> _safeSelectedOption() {
    if (widget.shippingOptions.isEmpty) return const {};
    final index = widget.selectedShippingIdx;
    if (index >= 0 && index < widget.shippingOptions.length) {
      return widget.shippingOptions[index];
    }
    return widget.shippingOptions.first;
  }

  /// 根據配送方式估算到貨日期範圍（API 未提供時的 fallback）。
  static String _estimateDeliveryRange(String methodKey) {
    final now = DateTime.now();
    int minDays;
    int maxDays;
    if (methodKey == ShippingMethod.taqbin.value) {
      // 黑貓宅急便：1–3 個工作天
      minDays = 1;
      maxDays = 3;
    } else if (methodKey == ShippingMethod.sevenElevenStorePickup.value) {
      // 7-11 超商取貨：2–4 個工作天
      minDays = 2;
      maxDays = 4;
    } else {
      return '';
    }
    final from = now.add(Duration(days: minDays));
    final to = now.add(Duration(days: maxDays));
    return '${from.month}月${from.day}日–${to.month}月${to.day}日';
  }

  /// 將 `availableShippingOptions` 按 shippingMethod 分組。
  ///
  /// 🔴 只留本版 App 有對應 UI 的方式（[ShippingPresentationX.isSupported]）：
  /// 擋住 3003／3005 與任何後端新增的碼值。選得到卻組不出正確
  /// payload 才是真正的 P0（見 [CartScreen] 結帳前置檢查與 checkout preview 的分派）。
  Map<String, List<MapEntry<int, Map<String, dynamic>>>> _groupOptions() {
    final grouped = <String, List<MapEntry<int, Map<String, dynamic>>>>{};
    for (var i = 0; i < widget.shippingOptions.length; i++) {
      final opt = widget.shippingOptions[i];
      final method = opt['shippingMethod']?.toString() ?? '';
      if (!shippingPresentationOf(method).isSupported) continue;
      grouped.putIfAbsent(method, () => []).add(MapEntry(i, opt));
    }
    return grouped;
  }

  /// 從分組的第一個選項取得顯示名稱。
  String _groupName(List<MapEntry<int, Map<String, dynamic>>> options) {
    if (options.isEmpty) return '';
    final rawLabel = options.first.value['label']?.toString() ?? '';
    return rawLabel.replaceAll(RegExp(r'[（(].+?[）)]'), '').trim();
  }

  @override
  Widget build(BuildContext context) {
    final addr = widget.selectedAddress;
    final hasAddress = addr != null;
    final addrLabel = addr?.fullAddress ?? '';
    final receiverName = addr?.receiverName ?? '';
    final receiverPhone = addr?.receiverPhone ?? '';

    final validateErrors = (widget.validateResult?['errors'] as List? ?? [])
        .cast<Map<String, dynamic>>();
    final validateValid = widget.validateResult?['valid'] as bool? ?? true;
    final needsAddress =
        widget.checkedSkuIds.isNotEmpty && widget.userAddresses.isEmpty;
    final groupedOptions = _groupOptions();

    return Container(
      margin: const EdgeInsets.symmetric(
        horizontal: HkSpace.lg,
        vertical: HkSpace.sm,
      ),
      padding: const EdgeInsets.all(HkSpace.lg),
      decoration: BoxDecoration(
        color: AppColors.bgCard,
        borderRadius: BorderRadius.circular(HkRadius.lg),
        border: Border.all(color: AppColors.cardBorder),
        boxShadow: HkShadows.card,
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // 標題
          Row(
            children: [
              Icon(
                AppIcons.localShippingOutlined,
                size: HkControl.icon,
                color: AppIconColors.defaultColor,
              ),
              const SizedBox(width: HkSpace.sm),
              Text('寄送方式', style: CartTypography.section),
            ],
          ),
          const SizedBox(height: HkSpace.md),

          // 狀態互斥：避免「請先設定地址」警示與提示文字同時出現。
          if (widget.checkedSkuIds.isEmpty) ...[
            _buildSelectionPrompt(),
          ] else if (needsAddress) ...[
            _buildAddressRequiredCard(),
          ] else if (widget.validateLoading) ...[
            _buildLoadingRow(),
          ] else if (!validateValid && validateErrors.isNotEmpty) ...[
            _buildValidateErrorPanel(),
          ] else if (groupedOptions.isNotEmpty) ...[
            ...groupedOptions.entries.expand((groupEntry) {
              final methodKey = groupEntry.key;
              final options = groupEntry.value;
              final groupName = _groupName(options);
              final isExpanded = _expandedGroups.contains(methodKey);
              final groupHasSelected = options.any(
                (e) => e.key == widget.selectedShippingIdx,
              );

              return [
                // 分組分隔線
                if (groupEntry.key != groupedOptions.keys.first)
                  Divider(
                    height: 16,
                    thickness: 0.5,
                    color: AppColors.dividerLight,
                  ),

                // 分組標題行
                _buildGroupHeader(methodKey, groupName, isExpanded, options),

                // 收合時仍以標準選項列顯示已選項目，讓各配送方式 UI 一致。
                if (!isExpanded && groupHasSelected)
                  _buildSelectedOptionRow(options),

                // 展開時顯示 Radio 選項 + 地址/門市
                if (isExpanded) ...[
                  SizedBox(height: 8),
                  ...options.map((entry) => _buildOptionRow(entry)),
                  if (groupHasSelected) ...[
                    SizedBox(height: 4),
                    // 明確分派：超商顯示門市清單、宅配顯示地址卡；
                    // 未支援的配送方式已被 _groupOptions() 濾掉，這裡是第二道防線
                    // （不得因為「不是超商」就預設要地址）。
                    if (shippingPresentationOf(methodKey) ==
                        ShippingPresentation.cvsStorePickup)
                      ..._buildStorePickupList()
                    else if (shippingPresentationOf(methodKey) ==
                        ShippingPresentation.homeDelivery) ...[
                      if (hasAddress)
                        _buildAddressCard(
                          addrLabel,
                          receiverName,
                          receiverPhone,
                        ),
                      _buildAddAddressRow(),
                    ],
                  ],
                ],
              ];
            }),
          ] else ...[
            _buildRetryRow(),
          ],
        ],
      ),
    );
  }

  // ── 子組件 ──

  Widget _buildAddressRequiredCard() {
    return Semantics(
      button: true,
      label: '新增收貨地址',
      hint: '設定後即可查看配送方式與運費',
      onTap: widget.onAddAddress,
      child: ExcludeSemantics(
        child: Material(
          key: const ValueKey('shipping-address-required'),
          color: AppColors.warningSoft,
          shape: RoundedRectangleBorder(
            borderRadius: BorderRadius.circular(HkRadius.md),
            side: BorderSide(color: AppColors.warning.withValues(alpha: 0.42)),
          ),
          clipBehavior: Clip.antiAlias,
          child: InkWell(
            onTap: widget.onAddAddress,
            child: Padding(
              padding: const EdgeInsets.all(HkSpace.md),
              child: Row(
                children: [
                  const SizedBox.square(
                    dimension: HkControl.minTap,
                    child: Center(
                      child: HawkrLocationIcon(
                        size: HkControl.icon,
                        color: AppIconColors.warningText,
                      ),
                    ),
                  ),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text('新增收貨地址', style: CartTypography.subtitle),
                        const SizedBox(height: HkSpace.xs),
                        Text(
                          '設定地址後，即可查看配送方式與運費。',
                          style: CartTypography.support,
                        ),
                      ],
                    ),
                  ),
                  const SizedBox(width: HkSpace.sm),
                  const Icon(
                    AppIcons.chevronRight,
                    size: HkControl.icon,
                    color: AppIconColors.warningText,
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _buildSelectionPrompt() {
    return Container(
      key: const ValueKey('shipping-selection-prompt'),
      padding: const EdgeInsets.all(HkSpace.md),
      decoration: BoxDecoration(
        color: AppColors.primarySubtle,
        borderRadius: BorderRadius.circular(HkRadius.md),
        border: Border.all(color: AppColors.cardBorder),
      ),
      child: Row(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Padding(
            padding: EdgeInsets.only(top: 2),
            child: Icon(
              AppIcons.localShippingOutlined,
              size: HkControl.icon,
              color: AppIconColors.muted,
            ),
          ),
          const SizedBox(width: HkSpace.md),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text('先選擇要結帳的商品', style: CartTypography.subtitle),
                const SizedBox(height: HkSpace.xs),
                Text('配送方式會依商家與溫層自動整理。', style: CartTypography.supportMuted),
              ],
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildValidateErrorPanel() {
    return Container(
      padding: EdgeInsets.all(12),
      decoration: BoxDecoration(
        color: AppColors.warningSoft,
        borderRadius: BorderRadius.circular(12),
        border: Border.all(color: AppColors.accentFill.withValues(alpha: 0.3)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(
                AppIcons.warningAmberRounded,
                size: HkControl.icon,
                color: AppIconColors.danger,
              ),
              SizedBox(width: 8),
              Expanded(
                child: Text(
                  '無法配送',
                  style: CartTypography.subtitle.copyWith(
                    color: AppColors.danger,
                  ),
                ),
              ),
            ],
          ),
          SizedBox(height: 8),
          Text(
            '目前勾選的商品總重量或體積超出所有配送方式的上限，暫時無法寄送。',
            style: CartTypography.support.copyWith(
              color: AppColors.primaryDark,
            ),
          ),
          SizedBox(height: 6),
          Text(
            '建議您減少商品數量，或分次下單。',
            style: CartTypography.support.copyWith(
              color: AppColors.primaryDark,
              fontWeight: FontWeight.w600,
            ),
          ),
        ],
      ),
    );
  }

  Widget _buildLoadingRow() {
    return Row(
      children: [
        SizedBox(
          width: 16,
          height: 16,
          child: HkBrandLoader(
            tone: HkBrandLoaderTone.mono,
            color: AppColors.primary,
          ),
        ),
        SizedBox(width: 8),
        Text('載入配送選項...', style: CartTypography.supportMuted),
      ],
    );
  }

  Widget _buildGroupHeader(
    String methodKey,
    String groupName,
    bool isExpanded,
    List<MapEntry<int, Map<String, dynamic>>> options,
  ) {
    void toggleExpanded() {
      setState(() {
        if (isExpanded) {
          _expandedGroups.remove(methodKey);
        } else {
          _expandedGroups.add(methodKey);
        }
      });
      widget.onExpandedGroupsChanged(_expandedGroups);
    }

    return Semantics(
      button: true,
      label: '$groupName配送方式，${isExpanded ? '已展開' : '已收合'}',
      onTap: toggleExpanded,
      child: ExcludeSemantics(
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            onTap: toggleExpanded,
            child: ConstrainedBox(
              constraints: const BoxConstraints(minHeight: HkControl.minTap),
              child: Row(
                children: [
                  Icon(
                    AppIcons.localShippingOutlined,
                    size: HkControl.icon,
                    color: AppIconColors.defaultColor,
                  ),
                  const SizedBox(width: HkSpace.sm),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text(groupName, style: CartTypography.subtitle),
                        Builder(
                          builder: (_) {
                            // 優先使用 API 回傳的預估時間，否則用客戶端計算
                            final estDelivery =
                                options.first.value['estimatedDelivery']
                                    ?.toString() ??
                                '';
                            final fallback = _estimateDeliveryRange(methodKey);
                            final displayText = estDelivery.isNotEmpty
                                ? '預計配送時間：$estDelivery'
                                : fallback.isNotEmpty
                                ? '預計配送時間：$fallback'
                                : '';
                            if (displayText.isEmpty) {
                              return const SizedBox.shrink();
                            }
                            return Padding(
                              padding: const EdgeInsets.only(top: HkSpace.xs),
                              child: Text(
                                displayText,
                                style: CartTypography.supportMuted,
                              ),
                            );
                          },
                        ),
                      ],
                    ),
                  ),
                  AnimatedRotation(
                    turns: isExpanded ? 0.5 : 0.0,
                    duration: HkMotion.duration,
                    child: const Icon(
                      AppIcons.expandMore,
                      size: HkControl.icon,
                      color: AppIconColors.defaultColor,
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

  Widget _buildSelectedOptionRow(
    List<MapEntry<int, Map<String, dynamic>>> options,
  ) {
    final selectedEntry = options.firstWhere(
      (entry) => entry.key == widget.selectedShippingIdx,
      orElse: () => options.first,
    );
    return Padding(
      padding: EdgeInsets.only(top: 4),
      child: _buildOptionRow(selectedEntry),
    );
  }

  Widget _buildOptionRow(MapEntry<int, Map<String, dynamic>> entry) {
    final idx = entry.key;
    final opt = entry.value;
    final rawLabel = opt['label']?.toString() ?? '';
    final label = rawLabel.replaceAll(RegExp(r'[（(].+?[）)]'), '').trim();
    final fee = opt['fee']?.toString() ?? '0';
    final deliveryTemp = opt['deliveryTemp']?.toString() ?? '';
    final freeEligible = opt['freeShippingEligible'] as bool? ?? false;
    final isSelected = widget.selectedShippingIdx == idx;

    final tempLabel = tempLabelMap[deliveryTemp] ?? deliveryTemp;
    final feeLabel = freeEligible ? '免運' : 'NT\$ $fee';
    return Semantics(
      button: true,
      selected: isSelected,
      label: [
        label,
        if (deliveryTemp.isNotEmpty) tempLabel,
        feeLabel,
      ].join('，'),
      onTap: () => widget.onShippingIdxChanged(idx),
      child: ExcludeSemantics(
        child: Material(
          color: Colors.transparent,
          child: InkWell(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            onTap: () => widget.onShippingIdxChanged(idx),
            child: ConstrainedBox(
              constraints: const BoxConstraints(minHeight: HkControl.minTap),
              child: LayoutBuilder(
                builder: (context, constraints) {
                  final labelStyle = CartTypography.body.copyWith(
                    fontWeight: isSelected ? FontWeight.w600 : FontWeight.w400,
                    color: isSelected
                        ? AppColors.textEmphasis
                        : AppColors.textPrimary,
                  );
                  final amountStyle = labelStyle.copyWith(
                    color: isSelected
                        ? AppColors.brandPrimaryStrong
                        : AppColors.textPrimary,
                  );
                  double textWidth(String value, TextStyle style) {
                    final painter = TextPainter(
                      text: TextSpan(text: value, style: style),
                      textDirection: Directionality.of(context),
                      textScaler: MediaQuery.textScalerOf(context),
                    )..layout();
                    final width = painter.width;
                    painter.dispose();
                    return width;
                  }

                  final stackAmount =
                      constraints.maxWidth <
                          280 *
                              MediaQuery.textScalerOf(context).scale(16) /
                              16 ||
                      HkControl.icon +
                              HkSpace.sm * 3 +
                              textWidth(label, labelStyle) +
                              textWidth(feeLabel, amountStyle) >
                          constraints.maxWidth;
                  final amount = Text(feeLabel, style: amountStyle);
                  final details = Row(
                    children: [
                      Icon(
                        isSelected
                            ? AppIcons.radioButtonChecked
                            : AppIcons.radioButtonUnchecked,
                        size: HkControl.icon,
                        color: isSelected
                            ? AppIconColors.active
                            : AppIconColors.disabled,
                      ),
                      const SizedBox(width: HkSpace.sm),
                      Expanded(
                        child: Wrap(
                          crossAxisAlignment: WrapCrossAlignment.center,
                          spacing: HkSpace.sm,
                          runSpacing: HkSpace.xs,
                          children: [
                            Text(label, style: labelStyle),
                            if (deliveryTemp.isNotEmpty)
                              HkTemperatureTag(value: deliveryTemp),
                          ],
                        ),
                      ),
                      if (!stackAmount) ...[
                        const SizedBox(width: HkSpace.sm),
                        amount,
                      ],
                    ],
                  );
                  if (!stackAmount) return details;
                  return Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      details,
                      const SizedBox(height: HkSpace.xs),
                      Align(alignment: Alignment.centerRight, child: amount),
                    ],
                  );
                },
              ),
            ),
          ),
        ),
      ),
    );
  }

  /// Selection and editing have separate, labelled 48px touch targets.
  List<Widget> _buildStorePickupList() => [
    ...widget.storePickups.map((store) {
      final storeId = store['id']?.toString() ?? '';
      final isSelected = storeId == widget.selectedStorePickupId;
      final name = store['storeName']?.toString() ?? '';
      final address = store['storeAddress']?.toString() ?? '';
      final receiver = [store['receiverName'], store['receiverPhone']]
          .where((value) => value != null && value.toString().isNotEmpty)
          .join('  ');
      return Padding(
        padding: const EdgeInsets.symmetric(vertical: HkSpace.sm),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Semantics(
              button: true,
              selected: isSelected,
              label: '選取$name，$address，$receiver',
              onTap: () => widget.onStorePickupSelected(storeId),
              child: ExcludeSemantics(
                child: InkWell(
                  onTap: () => widget.onStorePickupSelected(storeId),
                  borderRadius: BorderRadius.circular(HkRadius.sm),
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(
                      minHeight: HkControl.minTap,
                    ),
                    child: Row(
                      children: [
                        Icon(
                          isSelected
                              ? AppIcons.radioButtonChecked
                              : AppIcons.radioButtonUnchecked,
                          size: HkControl.icon,
                          color: isSelected
                              ? AppIconColors.active
                              : AppIconColors.disabled,
                        ),
                        const SizedBox(width: HkSpace.sm),
                        Expanded(
                          child: Column(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              Text(
                                name,
                                style: isSelected
                                    ? CartTypography.subtitle
                                    : CartTypography.body,
                              ),
                              Text(address, style: CartTypography.supportMuted),
                              if (receiver.isNotEmpty)
                                Text(
                                  receiver,
                                  style: CartTypography.supportMuted,
                                ),
                            ],
                          ),
                        ),
                      ],
                    ),
                  ),
                ),
              ),
            ),
            const SizedBox(height: HkSpace.sm),
            Align(
              alignment: Alignment.centerRight,
              child: OutlinedButton(
                key: ValueKey('cart-edit-store-$storeId'),
                style: OutlinedButton.styleFrom(
                  textStyle: CartTypography.button,
                ),
                onPressed: () => widget.onOpenStorePickupForm(store),
                child: const HkButtonLabel('編輯門市'),
              ),
            ),
          ],
        ),
      );
    }),
    _addAction('新增 7-11 門市', () => widget.onOpenStorePickupForm(null)),
  ];

  Widget _buildAddressCard(String address, String name, String phone) =>
      Container(
        margin: const EdgeInsets.only(top: HkSpace.sm),
        padding: const EdgeInsets.all(HkSpace.md),
        decoration: BoxDecoration(
          color: AppColors.bgCard,
          borderRadius: BorderRadius.circular(HkRadius.sm),
          border: Border.all(color: AppColors.borderInput),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            InkWell(
              onTap: widget.onShowAddressPicker,
              borderRadius: BorderRadius.circular(HkRadius.sm),
              child: ConstrainedBox(
                constraints: const BoxConstraints(minHeight: HkControl.minTap),
                child: Row(
                  children: [
                    const HawkrLocationIcon(
                      size: HkControl.icon,
                      color: AppIconColors.active,
                    ),
                    const SizedBox(width: HkSpace.sm),
                    Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(address, style: CartTypography.body),
                          if (name.isNotEmpty || phone.isNotEmpty)
                            Text(
                              '$name  $phone',
                              style: CartTypography.supportMuted,
                            ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
            ),
            const SizedBox(height: HkSpace.sm),
            Align(
              alignment: Alignment.centerRight,
              child: OutlinedButton(
                key: const ValueKey('cart-edit-address'),
                style: OutlinedButton.styleFrom(
                  textStyle: CartTypography.button,
                ),
                onPressed: widget.onShowAddressPicker,
                child: const HkButtonLabel('編輯地址'),
              ),
            ),
          ],
        ),
      );

  Widget _addAction(String label, VoidCallback onPressed) => TextButton.icon(
    onPressed: onPressed,
    icon: const Icon(AppIcons.addCircleOutline, size: HkControl.icon),
    label: HkButtonLabel(label),
    style: TextButton.styleFrom(
      alignment: Alignment.centerLeft,
      textStyle: CartTypography.button,
    ),
  );

  Widget _buildAddAddressRow() => _addAction('新增住址', widget.onAddAddress);

  Widget _buildRetryRow() => Row(
    children: [
      Expanded(child: Text('配送選項暫無法取得', style: CartTypography.body)),
      const SizedBox(width: HkSpace.sm),
      TextButton(
        onPressed: widget.onRetryValidate,
        style: TextButton.styleFrom(textStyle: CartTypography.button),
        child: const HkButtonLabel('重試'),
      ),
    ],
  );
}
