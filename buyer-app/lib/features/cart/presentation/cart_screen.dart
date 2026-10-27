import 'cart_typography.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'dart:async';
import 'dart:convert';
import 'dart:math' as math;
import 'package:hawkr_buyer/core/codemeta/shipping_presentation.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'cart_controller.dart';
import 'cart_state.dart';
import 'store_pickup_form_screen.dart';
import '../../address/data/address_model.dart';
import '../../cart/data/cart_model.dart';

import '../../../core/analytics/analytics_providers.dart';
import '../../../core/analytics/ga4_item.dart';
import '../../../core/analytics/purchase_dedup.dart';
import '../../order/data/order_repository.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/chat_icon_with_badge.dart';
import '../../../shared/widgets/common_dialog.dart';
import '../../../shared/widgets/hk_empty_state.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/common_snackbar.dart';
import '../../../config/image_config.dart';
import 'widgets/cart_checkout_action.dart';
import 'widgets/cart_top_bar.dart';

// ── 拆分後的子組件 ──
import 'widgets/merchant_group.dart';
import 'widgets/coupon_helpers.dart';
import 'widgets/coupon_selection_sheet.dart';
import 'widgets/merchant_group_card.dart';
import 'widgets/delivery_notes_section.dart';
import 'widgets/payment_details_section.dart';
import 'widgets/shipping_section.dart';
import 'widgets/checkout_payment_sheet.dart';
import 'widgets/address_picker_sheet.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

class CartScreen extends ConsumerStatefulWidget {
  const CartScreen({super.key, this.preSelectedSkuId});
  final int? preSelectedSkuId;
  @override
  ConsumerState<CartScreen> createState() => _CartScreenState();
}

class _CartScreenState extends ConsumerState<CartScreen> {
  bool _isOpeningSheet = false;
  bool _isClearing = false;
  final Map<String, Map<String, dynamic>> _appliedCoupons = {};
  final TextEditingController _deliveryNotesController =
      TextEditingController();
  Future<List<Map<String, dynamic>>>? _couponFuture;
  String _couponCachedSellerId = '';

  // Analytics: view_cart fires once per screen session
  bool _viewCartSent = false;

  // Expanded shipping method groups (by shippingMethod key)

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) async {
      _skipRouteTransition();
      await ref.read(cartControllerProvider.future);
      if (!mounted) return;
      await ref
          .read(cartControllerProvider.notifier)
          .initialize(preSelectedSkuId: widget.preSelectedSkuId);
    });
  }

  @override
  void dispose() {
    _deliveryNotesController.dispose();
    super.dispose();
  }

  CartState get _cartState =>
      ref.read(cartControllerProvider).value ?? CartState.initial();
  List<MerchantGroup> get _groups =>
      _cartState.data?.groups ?? const <MerchantGroup>[];
  List<CartItem> get _cartItems => _cartState.data?.items ?? const <CartItem>[];
  Set<int> get _checkedSkuIds => _cartState.checkedSkuIds;
  Map<int, int> get _skuStock =>
      _cartState.data?.stockBySku ?? const <int, int>{};
  Map<int, List<String>> get _itemDeliveryTemps =>
      _cartState.data?.deliveryTempsBySku ?? const <int, List<String>>{};
  List<Address> get _userAddresses => _cartState.userAddresses;
  int? get _selectedAddressId => _cartState.selectedAddressId;
  List<Map<String, dynamic>> get _storePickups => _cartState.storePickups;
  String? get _selectedStorePickupId => _cartState.selectedStorePickupId;
  Map<String, dynamic>? get _validateResult => _cartState.validateResult;
  List<Map<String, dynamic>> get _shippingOptions => _cartState.shippingOptions;
  int get _selectedShippingIdx => _cartState.selectedShippingIndex;
  bool get _validateLoading => _cartState.validateLoading;

  /// 「現在選了哪一種配送」—— 一律問 state，不在本檔重新推導一次。
  CartShippingSelection get _shippingSelection => _cartState.shippingSelection;
  double? get _preciseShippingFee => _cartState.preciseShippingFee;
  DateTime? get _lastValidateTime => _cartState.lastValidatedAt;
  Map<int, String> get _skuProductIds =>
      _cartState.data?.productIdsBySku ?? const <int, String>{};

  void _skipRouteTransition() {
    final route = ModalRoute.of(context);
    if (route == null) return;

    void jumpToEnd(Animation<double>? anim) {
      if (anim == null) return;
      final parent = anim is ProxyAnimation ? anim.parent : anim;
      if (parent is AnimationController) {
        parent.value = 1;
        parent.stop();
      }
    }

    jumpToEnd(route.animation);
    jumpToEnd(route.secondaryAnimation);
  }

  Future<void> _loadUserAddresses() =>
      ref.read(cartControllerProvider.notifier).refreshAddresses();

  Future<void> _loadStorePickups() =>
      ref.read(cartControllerProvider.notifier).refreshStorePickups();

  Map<String, dynamic>? get _selectedStorePickup =>
      _cartState.selectedStorePickup;

  Address? get _selectedAddress => _cartState.selectedAddress;

  Future<void> _loadCartItems() =>
      ref.read(cartControllerProvider.notifier).refresh();

  // 舊版自動移除 / toast 邏輯已下線（issue #1127）。
  // 不可購買商品改由 CartProductTile 反灰 + badge + disabled checkbox 呈現，user 自主決定移除。

  Future<void> _deleteCartItem(CartItem item) async {
    final itemName = item.title ?? '商品';
    final success = await ref
        .read(cartControllerProvider.notifier)
        .removeItem(item.skuId);
    if (!mounted) return;
    if (success) {
      ref
          .read(analyticsEventsProvider)
          .trackRemoveFromCart(cartItemToGA4Item(item));
      showCommonSnackBar(context, '已從購物車移除「$itemName」');
    } else {
      showCommonSnackBar(context, '刪除失敗，已保留購物車內容');
    }
  }

  String _cartItemKeyFor(CartItem item) {
    final idPart = item.skuId.toString();
    final pricePart = item.price.toStringAsFixed(2);
    return '$idPart::$pricePart::${item.title ?? ''}';
  }

  void _syncCouponMap(List<CartItem> items) {
    final valid = items.map(_cartItemKeyFor).toSet();
    _appliedCoupons.removeWhere((key, _) => !valid.contains(key));
  }

  double _couponDeductionForItem(CartItem item) {
    final coupon = _appliedCoupons[_cartItemKeyFor(item)];
    if (coupon == null) return 0;
    final baseAmount = (item.price * item.count).toDouble();
    final effect = resolveCouponEffect(coupon, baseAmount);
    return effect?.deduction ?? 0;
  }

  // ---------------------------------------------------------------------------
  // Pricing computed properties
  // ---------------------------------------------------------------------------

  double get _productTotal {
    return _cartItems
        .where((i) => _checkedSkuIds.contains(i.skuId))
        .fold(0.0, (sum, i) => sum + (i.price * i.count).toDouble());
  }

  double get _shippingTotal {
    return switch (_shippingSelection) {
      ShippingUnselected() => 0.0,
      TieredShippingSelected(:final option) => () {
        // Use precise fee from shipping_fee API if available
        if (_preciseShippingFee != null) return _preciseShippingFee!;
        // Otherwise fall back to validate estimate
        if (option['freeShippingEligible'] == true) return 0.0;
        return double.tryParse(option['fee']?.toString() ?? '0') ?? 0.0;
      }(),
    };
  }

  double get _couponDiscount {
    return _cartItems
        .where((i) => _checkedSkuIds.contains(i.skuId))
        .fold(0.0, (sum, i) => sum + _couponDeductionForItem(i));
  }

  double get _grandTotal =>
      math.max(0, _productTotal + _shippingTotal - _couponDiscount);

  /// Coupon button is enabled only when exactly one seller is checked and shipping is available.
  bool get _canSelectCoupon {
    final checkedSellerIds = _groups
        .where((g) => g.items.any((i) => _checkedSkuIds.contains(i.skuId)))
        .map((g) => g.storeId)
        .toSet();
    if (checkedSellerIds.length != 1) return false;
    if (_shippingOptions.isEmpty) return false;
    return true;
  }

  /// Returns the storeId of the single checked seller, or null.
  String? get _checkedStoreId {
    final checkedSellerIds = _groups
        .where((g) => g.items.any((i) => _checkedSkuIds.contains(i.skuId)))
        .map((g) => g.storeId)
        .toSet();
    if (checkedSellerIds.length != 1) return null;
    return checkedSellerIds.first;
  }

  // ---------------------------------------------------------------------------
  // Group check/uncheck
  //
  // 單一賣家選擇模式：購物車一次只能勾選同一個賣家的商品
  // （後端不支援跨賣家合併出貨，避免使用者勾完才在「寄送方式」看到無法配送錯誤）。
  // 切換賣家時自動清掉舊賣家的勾選。
  // ---------------------------------------------------------------------------

  /// 取得目前已勾選商品所屬的 storeId（單選模式下所有勾選都是同一個）。
  /// 沒有勾選則回 null。
  void _onGroupCheckChanged(MerchantGroup group, bool checked) {
    ref.read(cartControllerProvider.notifier).selectGroup(group, checked);
  }

  void _onItemCheckChanged(int skuId, bool checked) {
    ref.read(cartControllerProvider.notifier).selectItem(skuId, checked);
  }

  void _triggerValidate() {
    ref.read(cartControllerProvider.notifier).scheduleValidation();
  }

  void _triggerShippingFee() {
    unawaited(ref.read(cartControllerProvider.notifier).calculateShippingFee());
  }

  void _updateCartItemQuantity(CartItem item, int quantity) {
    ref
        .read(cartControllerProvider.notifier)
        .updateQuantity(item.skuId, quantity);
  }
  // ---------------------------------------------------------------------------
  // Clear cart
  // ---------------------------------------------------------------------------

  Future<void> _clearCart() async {
    final confirmed =
        await showCommonDialog(
          context: context,
          title: '確認清空購物車',
          content: '確定要清空整個購物車嗎？此操作無法復原。',
          confirmText: '清空',
          cancelText: '取消',
          confirmColor: AppColors.danger,
        ) ??
        false;

    if (!confirmed) return;

    setState(() {
      _isClearing = true;
      _appliedCoupons.clear();
    });

    final cleared = await ref.read(cartControllerProvider.notifier).clearCart();
    if (!mounted) return;
    setState(() => _isClearing = false);
    if (cleared) {
      showCommonSnackBar(context, '購物車已清空');
    } else {
      showCommonSnackBar(context, '部分商品未能刪除，已保留目前購物車內容');
    }
  }

  // ---------------------------------------------------------------------------
  // Coupon sheet (order-level)
  // ---------------------------------------------------------------------------

  Future<List<Map<String, dynamic>>> _loadCoupons() {
    // Invalidate cache when seller changes
    final storeId = _checkedStoreId ?? '';
    if (_couponFuture != null && _couponCachedSellerId == storeId) {
      return _couponFuture!;
    }
    _couponCachedSellerId = storeId;
    final future = ref
        .read(cartControllerProvider.notifier)
        .loadAvailableCoupons(
          storeId: storeId,
          orderAmount: _productTotal.round(),
        )
        .catchError((_) => <Map<String, dynamic>>[]);
    _couponFuture = future;
    return future;
  }

  Future<Map<String, dynamic>?> _showCouponSheet() async {
    final future = _loadCoupons();
    final selected = await showModalBottomSheet<Map<String, dynamic>>(
      context: context,
      isScrollControlled: true,
      showDragHandle: true,
      shape: const RoundedRectangleBorder(
        borderRadius: BorderRadius.vertical(top: Radius.circular(16)),
      ),
      builder: (ctx) => CouponSelectionSheet(
        couponsFuture: future,
        initiallySelectedId: null,
      ),
    );
    if (!mounted || selected == null) return null;
    // Apply coupon to all checked items
    setState(() {
      for (final item in _cartItems) {
        if (_checkedSkuIds.contains(item.skuId)) {
          _appliedCoupons[_cartItemKeyFor(item)] = selected;
        }
      }
    });
    final title = couponDisplayTitle(selected);
    showCommonSnackBar(context, '已套用優惠券：$title');
    return selected;
  }

  // ---------------------------------------------------------------------------
  // Payment sheet (swipe-up checkout)
  // ---------------------------------------------------------------------------

  // Payment methods derived from POST /app/payments API spec.
  // Supported: 2002 (Apple Pay), 2004 (ATM), 2001 (Credit Card).
  // No list endpoint exists — methods are defined per spec capabilities.
  void _openPaymentSheet() {
    if (_isOpeningSheet) return;
    if (_cartState.checkoutPending) {
      showCommonSnackBar(context, '正在更新商品與配送資訊，請稍候');
      return;
    }
    final selectedItems = _cartItems
        .where((it) => _checkedSkuIds.contains(it.skuId))
        .toList();
    if (selectedItems.isEmpty) {
      showCommonSnackBar(context, '請先勾選要結帳的商品');
      return;
    }
    // 「現在選了什麼」只問一次（[CartState.shippingSelection]），不在這裡重新推導。
    final selection = _shippingSelection;
    final ShippingPresentation shipping;
    final Map<String, dynamic> shippingOption;
    switch (selection) {
      case ShippingUnselected():
        showCommonSnackBar(context, '請等待配送選項載入');
        return;
      case TieredShippingSelected(:final option):
        shippingOption = option;
        // 結帳前置檢查依配送方式**明確分派**。
        // 🔴 不得用 `!isStorePickup` 當宅配：那會讓未知 / 本版未支援的配送方式被要求
        // 填一個對它不適用的宅配地址，接著把宅配 payload 送進 /app/checkout/preview。
        shipping = shippingPresentationOf(option['shippingMethod']?.toString());
        switch (shipping) {
          case ShippingPresentation.cvsStorePickup:
            if (_selectedStorePickupId == null) {
              showCommonSnackBar(context, '請先選擇取貨門市');
              return;
            }
          case ShippingPresentation.homeDelivery:
            if (_selectedAddressId == null) {
              showCommonSnackBar(context, '請先選擇收件地址');
              return;
            }
          case ShippingPresentation.unsupported:
            showCommonSnackBar(context, '此配送方式暫不支援，請改選其他配送方式');
            return;
        }
    }
    final stale =
        _lastValidateTime == null ||
        DateTime.now().difference(_lastValidateTime!).inSeconds > 60;
    if (stale) {
      _triggerValidate();
      showCommonSnackBar(context, '正在重新驗證商品資訊，請稍候再試');
      return;
    }

    // GA4: begin_checkout — checked (selected) items only
    ref
        .read(analyticsEventsProvider)
        .trackBeginCheckout(selectedItems.map(cartItemToGA4Item).toList());

    _isOpeningSheet = true;
    // 🔴 merge origin/dev：上游把優惠券的取法改成「依勾選的品項反查」
    //    （_cartItemKeyFor + firstOrNull），取代舊的 `_appliedCoupons.values.first`。
    //    舊寫法在多賣家情境會取到別家的券。
    final selectedCoupon = selectedItems
        .map((item) => _appliedCoupons[_cartItemKeyFor(item)])
        .whereType<Map<String, dynamic>>()
        .firstOrNull;
    final appliedCouponId =
        selectedCoupon?['couponId']?.toString() ??
        selectedCoupon?['coupon_id']?.toString() ??
        selectedCoupon?['templateId']?.toString() ??
        '';

    showCheckoutPaymentSheet(
      context: context,
      params: CheckoutParams(
        selectedItems: selectedItems,
        shippingOption: shippingOption,
        // 宅配要收件地址；超商取貨才是門市。
        // 判準走 [ShippingPresentationX.usesRecipientAddress]，與 preview／confirm 同一份。
        selectedAddress: shipping.usesRecipientAddress
            ? _selectedAddress
            : null,
        selectedStorePickup: shipping == ShippingPresentation.cvsStorePickup
            ? _selectedStorePickup
            : null,
        skuProductIds: _skuProductIds,
        deliveryNotesText: _deliveryNotesController.text,
        appliedCouponId: appliedCouponId,
        couponDiscount: _couponDiscount,
        canSelectCoupon: _canSelectCoupon,
      ),
      callbacks: CheckoutCallbacks(
        onShowCouponSheet: _showCouponSheet,
        onReloadCart: _loadCartItems,
        onTriggerShippingFee: _triggerShippingFee,
        onClearCoupons: () => setState(() => _appliedCoupons.clear()),
        onNavigateToOrders: () {
          if (mounted) context.go('/?tab=3');
        },
        onNavigateHome: () {
          if (mounted) context.go('/?tab=0');
        },
        onNavigateToPaymentWebview:
            ({
              required String redirectUrl,
              required String tradeNo,
              required String orderNo,
              required String amount,
            }) {
              if (mounted) {
                context.go(
                  '/payment/webview',
                  extra: {
                    'redirectUrl': redirectUrl,
                    'tradeNo': tradeNo,
                    'orderNo': orderNo,
                    'amount': amount,
                  },
                );
              }
            },
        onInvalidateCartCount: () => ref.invalidate(cartCountProvider),
        onPayMethodChanged: (payType) {
          // GA4: add_payment_info
          final checkedItems = _cartItems
              .where((i) => _checkedSkuIds.contains(i.skuId))
              .toList();
          ref
              .read(analyticsEventsProvider)
              .trackAddPaymentInfo(
                checkedItems.map(cartItemToGA4Item).toList(),
                paymentType: payType,
              );
        },
        onPaymentSucceeded: (orderNo) async {
          // GA4 purchase for Apple Pay / Google Pay immediately-paid orders.
          // Deduped (markPurchaseTracked returns false if already tracked).
          // Fire-and-forget: errors are swallowed so analytics never breaks checkout.
          if (orderNo.isEmpty || !markPurchaseTracked(orderNo)) return;
          try {
            final order = await ref
                .read(orderRepositoryProvider)
                .getOrderDetail(orderNo);
            ref.read(analyticsEventsProvider).trackPurchase(order);
          } catch (_) {
            // Analytics must not break checkout — silently skip on fetch failure.
            HKLog.logW('[GA4] purchase fetch failed for orderNo=$orderNo');
          }
        },
      ),
    ).whenComplete(() {
      _isOpeningSheet = false;
    });
  }

  // ---------------------------------------------------------------------------
  // Image helper
  // ---------------------------------------------------------------------------

  ImageProvider? _resolveImageProvider(String? source) {
    if (source == null) return null;
    final raw = source.trim();
    if (raw.isEmpty || raw.toLowerCase() == 'null') return null;
    if (raw.startsWith('data:image')) {
      final comma = raw.indexOf(',');
      final dataPart = comma >= 0 ? raw.substring(comma + 1) : raw;
      try {
        final bytes = base64Decode(dataPart.replaceAll(RegExp(r'\s'), ''));
        if (bytes.isNotEmpty) return MemoryImage(bytes);
      } catch (_) {
        return null;
      }
    }
    if (raw.startsWith('//')) {
      return NetworkImage(
        ImageConfig.urlFor('https:$raw', ImageScene.avatarSmall),
      );
    }
    if (raw.contains('://')) {
      return NetworkImage(ImageConfig.urlFor(raw, ImageScene.avatarSmall));
    }
    final cleaned = raw.replaceAll(RegExp(r'\s'), '');
    final base64Pattern = RegExp(r'^[A-Za-z0-9+/=]+$');
    final bool looksBase64 =
        cleaned.length > 24 &&
        cleaned.length % 4 == 0 &&
        base64Pattern.hasMatch(cleaned);
    if (looksBase64) {
      try {
        final bytes = base64Decode(cleaned);
        if (bytes.isNotEmpty) return MemoryImage(bytes);
      } catch (_) {}
    }
    return null;
  }

  /// handler.CartGroupResponse.logoUrl — 直接使用 API 回傳的值
  String? _sellerAvatarUrl(String storeId, {String? groupLogoUrl}) {
    // handler.CartGroupResponse: logoUrl
    if (groupLogoUrl != null &&
        groupLogoUrl.isNotEmpty &&
        groupLogoUrl.toLowerCase() != 'null') {
      return _sanitizeImageUrl(groupLogoUrl);
    }
    return null;
  }

  /// Fix malformed double-prefixed URLs like
  /// `https://media.example.com/https://lh3.googleusercontent.com/...`
  static String _sanitizeImageUrl(String url) {
    final idx = url.indexOf('http', 1);
    if (idx > 0 && url.substring(idx).startsWith('http')) {
      return url.substring(idx);
    }
    return url;
  }

  // ---------------------------------------------------------------------------
  // BUILD
  // ---------------------------------------------------------------------------

  @override
  Widget build(BuildContext context) {
    final cartAsync = ref.watch(cartControllerProvider);
    final visibleState = cartAsync.value;
    ref.listen(cartControllerProvider, (previous, next) {
      final nextState = next.value;
      if (nextState == null) return;
      final items = nextState.data?.items ?? const <CartItem>[];
      _syncCouponMap(items);
      if (!_viewCartSent && items.isNotEmpty) {
        _viewCartSent = true;
        ref
            .read(analyticsEventsProvider)
            .trackViewCart(items.map(cartItemToGA4Item).toList());
      }

      final problem = nextState.problem;
      if (problem != null &&
          previous?.value?.problem?.revision != problem.revision) {
        WidgetsBinding.instance.addPostFrameCallback((_) {
          if (mounted) showCommonSnackBar(context, problem.message);
        });
      }
    });

    return Stack(
      children: [
        Scaffold(
          backgroundColor: AppColors.bgPrimary,
          body: SafeArea(
            top: false,
            child: Column(
              children: [
                // ── App Bar ──
                CartTopBar(
                  onBack: () => context.pop(),
                  onClear: _cartItems.isNotEmpty ? _clearCart : null,
                  chatAction: const ChatIconWithBadge(),
                ),

                // ── Content ──
                Expanded(
                  child: cartAsync.isLoading && visibleState == null
                      ? const Center(
                          key: ValueKey('cart-initial-loading'),
                          child: HkBrandLoader(size: HkBrandLoader.page),
                        )
                      : (visibleState?.status == CartContentStatus.failure ||
                            cartAsync.hasError)
                      ? Center(
                          key: const ValueKey('cart-load-error'),
                          child: Padding(
                            padding: const EdgeInsets.all(HkSpace.xl),
                            child: Column(
                              mainAxisSize: MainAxisSize.min,
                              children: [
                                Text(
                                  visibleState?.problem?.message ??
                                      '購物車載入失敗，請稍後再試',
                                  textAlign: TextAlign.center,
                                  style: CartTypography.body,
                                ),
                                const SizedBox(height: HkSpace.md),
                                FilledButton(
                                  onPressed: _loadCartItems,
                                  style: FilledButton.styleFrom(
                                    textStyle: CartTypography.button,
                                  ),
                                  child: const HkButtonLabel('重試'),
                                ),
                              ],
                            ),
                          ),
                        )
                      : _cartItems.isEmpty
                      ? const HkEmptyState(
                          key: ValueKey('cart-empty-state'),
                          iconKey: ValueKey('cart-empty-state-icon'),
                          icon: HkIconName.emptyProducts,
                          message: '購物車是空的',
                        )
                      : LayoutBuilder(
                          builder: (context, constraints) => Align(
                            alignment: Alignment.topCenter,
                            child: SizedBox(
                              width: math.min(
                                constraints.maxWidth,
                                HkControl.chromeMaxWidth,
                              ),
                              child: ListView(
                                key: const PageStorageKey<String>(
                                  'cart-content-list',
                                ),
                                padding: const EdgeInsets.symmetric(
                                  vertical: HkSpace.md,
                                ),
                                children: [
                                  if (visibleState?.problem?.kind ==
                                      CartProblemKind.staleData)
                                    Container(
                                      key: const ValueKey(
                                        'cart-stale-data-error',
                                      ),
                                      margin: const EdgeInsets.fromLTRB(
                                        HkSpace.lg,
                                        0,
                                        HkSpace.lg,
                                        HkSpace.sm,
                                      ),
                                      padding: const EdgeInsets.all(HkSpace.md),
                                      decoration: BoxDecoration(
                                        color: AppColors.warningSoft,
                                        borderRadius: BorderRadius.circular(
                                          HkRadius.md,
                                        ),
                                      ),
                                      child: Row(
                                        children: [
                                          const Icon(
                                            AppIcons.infoOutline,
                                            color: AppIconColors.warningText,
                                          ),
                                          const SizedBox(width: HkSpace.sm),
                                          Expanded(
                                            child: Text(
                                              visibleState!.problem!.message,
                                              style: CartTypography.body,
                                            ),
                                          ),
                                          TextButton(
                                            onPressed: _loadCartItems,
                                            style: FilledButton.styleFrom(
                                              textStyle: CartTypography.button,
                                            ),
                                            child: const HkButtonLabel('重試'),
                                          ),
                                        ],
                                      ),
                                    ),
                                  // Merchant group cards
                                  ..._groups.map(
                                    (group) => MerchantGroupCard(
                                      group: group,
                                      checkedSkuIds: _checkedSkuIds,
                                      skuStock: _skuStock,
                                      itemDeliveryTemps: _itemDeliveryTemps,
                                      onGroupCheckChanged: _onGroupCheckChanged,
                                      onItemCheckChanged: _onItemCheckChanged,
                                      onQuantityChanged:
                                          _updateCartItemQuantity,
                                      onDeleteItem: (item) {
                                        _appliedCoupons.remove(
                                          _cartItemKeyFor(item),
                                        );
                                        _deleteCartItem(item);
                                      },
                                      resolveImageProvider:
                                          _resolveImageProvider,
                                      sellerAvatarUrl: _sellerAvatarUrl,
                                    ),
                                  ),
                                  // Shipping section
                                  ShippingSection(
                                    shippingOptions: _shippingOptions,
                                    selectedShippingIdx: _selectedShippingIdx,
                                    validateResult: _validateResult,
                                    validateLoading: _validateLoading,
                                    selectedAddress: _selectedAddress,
                                    userAddresses: _userAddresses,
                                    storePickups: _storePickups,
                                    selectedStorePickupId:
                                        _selectedStorePickupId,
                                    checkedSkuIds: _checkedSkuIds,
                                    onShippingIdxChanged: (idx) {
                                      ref
                                          .read(cartControllerProvider.notifier)
                                          .selectShippingOption(idx);
                                      // GA4: add_shipping_info
                                      if (idx < _shippingOptions.length) {
                                        final checkedItems = _cartItems
                                            .where(
                                              (i) => _checkedSkuIds.contains(
                                                i.skuId,
                                              ),
                                            )
                                            .toList();
                                        final shippingTier =
                                            _shippingOptions[idx]['shippingMethod']
                                                ?.toString() ??
                                            '';
                                        ref
                                            .read(analyticsEventsProvider)
                                            .trackAddShippingInfo(
                                              checkedItems
                                                  .map(cartItemToGA4Item)
                                                  .toList(),
                                              shippingTier: shippingTier,
                                            );
                                      }
                                    },
                                    onStorePickupSelected: (id) => ref
                                        .read(cartControllerProvider.notifier)
                                        .selectStorePickup(id),
                                    onExpandedGroupsChanged: (groups) {},
                                    onShowAddressPicker: _showAddressPicker,
                                    onOpenStorePickupForm: _openStorePickupForm,
                                    onAddAddress: () async {
                                      await context.push('/settings/address');
                                      _loadUserAddresses();
                                    },
                                    onRetryValidate: _triggerValidate,
                                  ),
                                  // Delivery notes
                                  DeliveryNotesSection(
                                    controller: _deliveryNotesController,
                                  ),
                                  // Payment details
                                  PaymentDetailsSection(
                                    productTotal: _productTotal,
                                    shippingTotal: _shippingTotal,
                                    couponDiscount: _couponDiscount,
                                    grandTotal: _grandTotal,
                                    canSelectCoupon: _canSelectCoupon,
                                    onSelectCoupon: _showCouponSheet,
                                  ),
                                  CartCheckoutAction(
                                    onPressed:
                                        _checkedStoreId == null ||
                                            _cartState.checkoutPending ||
                                            _shippingOptions.isEmpty
                                        ? null
                                        : _openPaymentSheet,
                                    storeName: _groups
                                        .where(
                                          (group) =>
                                              group.storeId == _checkedStoreId,
                                        )
                                        .firstOrNull
                                        ?.sellerName,
                                    selectedItemCount: _checkedSkuIds.length,
                                    isValidating: _cartState.checkoutPending,
                                  ),
                                ],
                              ),
                            ),
                          ),
                        ),
                ),
              ],
            ),
          ),
        ),
        // ── 清空購物車遮罩 + 卡車動畫 ──
        if (_isClearing)
          Positioned.fill(
            child: Container(
              color: AppColors.textEmphasis.withValues(alpha: 0.35),
              child: const TruckLoadingOverlay(message: '清空購物車中...'),
            ),
          ),
      ],
    );
  }

  /// 打開超商取貨表單（新增或編輯）。
  Future<void> _openStorePickupForm(Map<String, dynamic>? existingStore) async {
    final changed = await showStorePickupFormSheet(
      context,
      existingStore: existingStore,
    );
    if (changed == true && mounted) {
      _loadStorePickups();
    }
  }

  Future<void> _showAddressPicker() async {
    final selected = await showAddressPickerSheet(
      context: context,
      userAddresses: _userAddresses,
      selectedAddressId: _selectedAddressId,
      onAddAddress: () async {
        await context.push('/settings/address');
        _loadUserAddresses();
      },
    );
    if (selected != null && mounted) {
      ref.read(cartControllerProvider.notifier).selectAddress(selected);
    }
  }

  // ---------------------------------------------------------------------------
  // Clear Cart Button
  // ---------------------------------------------------------------------------
}
