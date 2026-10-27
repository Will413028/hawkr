import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/core/storage/auth_storage.dart';
import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/features/cart/data/cart_repository.dart';
import 'package:hawkr_buyer/features/cart/data/cart_error_message.dart';
import 'package:hawkr_buyer/features/cart/presentation/cart_controller.dart';
import 'package:hawkr_buyer/features/product/data/product_repository.dart';
import 'package:hawkr_buyer/features/product/presentation/widgets/product_purchase_panel.dart';
import 'package:hawkr_buyer/shared/utils/price_display.dart';
import 'package:hawkr_buyer/shared/utils/require_login.dart';
import 'package:hawkr_buyer/shared/widgets/common_snackbar.dart';
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

/// Opens the compact purchase flow from a product card.
///
/// The purchase content and footer are shared with the full product detail
/// page. This wrapper only owns data loading, cart mutation and navigation.
void showProductAddToCartSheet(
  BuildContext context,
  WidgetRef ref, {
  required String skuId,
  required String productName,
  required String imageUrl,
  double? initialPrice,
}) {
  final viewport = MediaQuery.sizeOf(context);
  showModalBottomSheet(
    context: context,
    isScrollControlled: true,
    useSafeArea: true,
    showDragHandle: true,
    backgroundColor: AppColors.bgCard,
    constraints: BoxConstraints(
      maxWidth: 520,
      maxHeight: viewport.height * 0.86,
    ),
    shape: const RoundedRectangleBorder(
      borderRadius: BorderRadius.vertical(top: Radius.circular(HkRadius.xl)),
    ),
    builder: (_) => UncontrolledProviderScope(
      container: ProviderScope.containerOf(context),
      child: _ProductAddToCartSheet(
        skuId: skuId,
        productName: productName,
        imageUrl: imageUrl,
        initialPrice: initialPrice,
      ),
    ),
  );
}

class _ProductAddToCartSheet extends ConsumerStatefulWidget {
  const _ProductAddToCartSheet({
    required this.skuId,
    required this.productName,
    required this.imageUrl,
    this.initialPrice,
  });

  final String skuId;
  final String productName;
  final String imageUrl;
  final double? initialPrice;

  @override
  ConsumerState<_ProductAddToCartSheet> createState() =>
      _ProductAddToCartSheetState();
}

class _ProductAddToCartSheetState
    extends ConsumerState<_ProductAddToCartSheet> {
  bool _loading = true;
  List<Map<String, dynamic>> _specs = const [];
  String? _selectedSpecId;
  int _quantity = 1;
  bool _addLoading = false;
  bool _buyLoading = false;
  bool _selfOwned = false;
  List<String> _temperatureCodes = const [];
  List<String> _shippingMethodCodes = const [];
  double _rating = 0;
  int _reviewCount = 0;

  Map<String, dynamic>? get _selectedSpec {
    for (final spec in _specs) {
      if ((spec['id'] ?? '').toString() == _selectedSpecId) return spec;
    }
    return null;
  }

  double get _currentPrice {
    final spec = _selectedSpec;
    if (spec != null) {
      final effective = PriceDisplay.effectiveSkuPrice(
        price: spec['price'],
        salePrice: spec['salePrice'],
      );
      if (effective != null) return effective.toDouble();
    }
    return widget.initialPrice ?? 0;
  }

  double? get _currentListPrice {
    final spec = _selectedSpec;
    if (spec == null ||
        !PriceDisplay.hasSkuSale(
          price: spec['price'],
          salePrice: spec['salePrice'],
        )) {
      return null;
    }
    return PriceDisplay.parseMoney(spec['price'])?.toDouble();
  }

  int get _currentStock => purchaseSpecStock(_selectedSpec);

  @override
  void initState() {
    super.initState();
    _loadProductDetail();
  }

  Future<void> _loadProductDetail() async {
    final productId = widget.skuId.trim();
    if (productId.isEmpty) {
      if (mounted) setState(() => _loading = false);
      return;
    }

    try {
      final info = await ref
          .read(productRepositoryProvider)
          .getProductFullInfo(productId);
      if (!mounted || info.isEmpty) {
        if (mounted) setState(() => _loading = false);
        return;
      }

      final shippingMethods = info['shippingMethods'];
      final shippingCodes = shippingMethods is List
          ? shippingMethods
                .map(
                  (method) => method is Map
                      ? (method['type'] ?? '').toString()
                      : method.toString(),
                )
                .where((code) => code.isNotEmpty)
                .toSet()
                .toList(growable: false)
          : const <String>[];
      final deliveryTemperatures = info['deliveryTemp'];
      final temperatureCodes = deliveryTemperatures is List
          ? deliveryTemperatures
                .map((temperature) => temperature.toString())
                .where((code) => code.isNotEmpty)
                .toSet()
                .toList(growable: false)
          : const <String>[];
      final specs = normalizeProductPurchaseSpecs(info['skus']);
      final initialSpec =
          firstAvailablePurchaseSpec(specs) ??
          (specs.isNotEmpty ? specs.first : null);
      final ratingRaw = info['rating'];
      final reviews = info['reviews'];

      setState(() {
        _shippingMethodCodes = shippingCodes;
        _temperatureCodes = temperatureCodes;
        _specs = specs;
        _selectedSpecId = initialSpec?['id']?.toString();
        _quantity = 1;
        _rating = ratingRaw is num
            ? ratingRaw.toDouble()
            : double.tryParse(ratingRaw?.toString() ?? '') ?? 0;
        _reviewCount = reviews is List ? reviews.length : 0;
        _selfOwned = info['selfOwned'] == true;
        _loading = false;
      });
    } catch (_) {
      if (mounted) setState(() => _loading = false);
    }
  }

  Future<String?> _resolveGuestId() async {
    final session = ref.read(sessionProvider).value;
    if (hasUsableSession(session)) return null;
    return AuthStorage.guestCartId();
  }

  Future<void> _addToCart({required bool checkout}) async {
    if (_selfOwned) {
      showFloatingToast(context, '自家商品不可購買', isSuccess: false);
      return;
    }
    if (!await requireLogin(context, ref) || !mounted) return;
    if (_selectedSpecId == null) {
      showFloatingToast(context, '請先選擇規格', isSuccess: false);
      return;
    }
    if (_currentStock <= 0) {
      showFloatingToast(context, '此規格已無庫存', isSuccess: false);
      return;
    }

    setState(() {
      _addLoading = !checkout;
      _buyLoading = checkout;
    });
    try {
      final selectedSkuId = int.tryParse(_selectedSpecId!) ?? 0;
      final guestId = await _resolveGuestId();
      await ref
          .read(cartRepositoryProvider)
          .addToCart(selectedSkuId, _quantity, guestId: guestId);
      ref.invalidate(cartCountProvider);
      if (!mounted) return;
      Navigator.of(context).pop();
      if (checkout) {
        context.push('/cart');
      } else {
        showFloatingToast(context, '已加入購物車');
      }
    } catch (error) {
      if (!mounted) return;
      setState(() {
        _addLoading = false;
        _buyLoading = false;
      });
      showFloatingToast(
        context,
        addToCartErrorMessage(error, fallback: '加入失敗，請稍後再試'),
        isSuccess: false,
      );
    }
  }

  void _selectSpec(Map<String, dynamic> spec) {
    setState(() {
      _selectedSpecId = (spec['id'] ?? '').toString();
      _quantity = 1;
    });
  }

  void _changeQuantity(int quantity) {
    if (_currentStock <= 0) return;
    setState(() => _quantity = quantity.clamp(1, _currentStock));
  }

  @override
  Widget build(BuildContext context) {
    final submitting = _addLoading || _buyLoading;
    return Padding(
      padding: EdgeInsets.only(bottom: MediaQuery.viewInsetsOf(context).bottom),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Flexible(
            child: SingleChildScrollView(
              padding: const EdgeInsets.fromLTRB(
                HkSpace.xl,
                HkSpace.sm,
                HkSpace.xl,
                HkSpace.lg,
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  ProductPurchaseSummaryCard(
                    productName: widget.productName,
                    imageUrl: widget.imageUrl,
                    rating: _rating,
                    reviewCount: _reviewCount,
                    temperatureCodes: _temperatureCodes,
                    shippingMethodCodes: _shippingMethodCodes,
                    loading: _loading,
                  ),
                  const SizedBox(height: HkSpace.md),
                  SizedBox(
                    height: 1,
                    child: ColoredBox(
                      color: AppColors.textPrimary.withValues(alpha: 0.08),
                    ),
                  ),
                  const SizedBox(height: HkSpace.md),
                  ProductPurchaseSelector(
                    specs: _specs,
                    selectedSpecId: _selectedSpecId,
                    quantity: _quantity,
                    currentPrice: _currentPrice,
                    currentListPrice: _currentListPrice,
                    loading: _loading,
                    onSpecSelected: _selectSpec,
                    onQuantityChanged: _changeQuantity,
                  ),
                  if (_selfOwned) ...[
                    const SizedBox(height: HkSpace.md),
                    Semantics(
                      container: true,
                      label: '自家商品不可購買',
                      child: ExcludeSemantics(
                        child: Container(
                          key: const ValueKey(
                            'product-purchase-self-owned-notice',
                          ),
                          width: double.infinity,
                          padding: const EdgeInsets.symmetric(
                            horizontal: HkSpace.lg,
                            vertical: HkSpace.md,
                          ),
                          decoration: BoxDecoration(
                            color: AppColors.primarySoft,
                            borderRadius: BorderRadius.circular(HkRadius.md),
                          ),
                          child: Text(
                            '這是你的商品，買家購買功能已停用。',
                            style: AppTypography.meta.copyWith(
                              color: AppColors.textPrimary,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                        ),
                      ),
                    ),
                  ],
                ],
              ),
            ),
          ),
          ProductPurchaseFooter(
            hasSelection: _selectedSpecId != null,
            stock: _currentStock,
            enabled: !_loading && !submitting && !_selfOwned,
            addLoading: _addLoading,
            buyLoading: _buyLoading,
            onAdd: () => _addToCart(checkout: false),
            onBuy: () => _addToCart(checkout: true),
          ),
        ],
      ),
    );
  }
}
