import 'package:firebase_analytics/firebase_analytics.dart';

import '../../features/cart/data/cart_model.dart';
import '../../features/order/data/order_model.dart';
import '../../features/product/data/product_model.dart';
import '../../shared/utils/price_display.dart';

/// Platform-neutral GA4 item. Mirrors web `lib/analytics/types.ts` + `items.ts`.
/// Data traps centralized here: item_id is a stringified int64 snowflake;
/// price is the numeric effective price (never a formatted "$" string);
/// seller name doubles as item_brand + affiliation.
class GA4Item {
  const GA4Item({
    required this.itemId,
    required this.itemName,
    this.price,
    this.quantity,
    this.itemVariant,
    this.itemBrand,
    this.itemCategory,
    this.index,
  });

  final String itemId;
  final String itemName;
  final num? price;
  final int? quantity;
  final String? itemVariant;
  final String? itemBrand;
  final String? itemCategory;
  final int? index;

  AnalyticsEventItem toAnalyticsEventItem() => AnalyticsEventItem(
    itemId: itemId,
    itemName: itemName,
    price: price?.toDouble(),
    quantity: quantity,
    itemVariant: itemVariant,
    itemBrand: itemBrand,
    affiliation: itemBrand,
    itemCategory: itemCategory,
    index: index,
  );
}

GA4Item productToGA4Item(Product p, {int? index}) => GA4Item(
  itemId: p.skuId.toString(),
  itemName: p.title,
  price: PriceDisplay.parseMoney(p.price),
  itemBrand: (p.merchantName?.isEmpty ?? true) ? null : p.merchantName,
  itemCategory: p.categoryId?.toString(),
  index: index,
);

GA4Item cartItemToGA4Item(CartItem c) => GA4Item(
  itemId: c.skuId.toString(),
  itemName: c.title ?? '',
  price: PriceDisplay.parseMoney(c.price),
  quantity: c.count,
  itemVariant: (c.specName?.isEmpty ?? true) ? null : c.specName,
  itemBrand: (c.merchantName?.isEmpty ?? true) ? null : c.merchantName,
);

GA4Item orderItemToGA4Item(OrderItem o) => GA4Item(
  itemId: o.skuId,
  itemName: o.productName,
  price: PriceDisplay.parseMoney(o.unitPrice),
  quantity: int.tryParse(o.quantity),
);

/// Event-level `value` = sum of price * quantity. Mirrors web `sumValue`.
num sumValue(List<GA4Item> items) => items.fold<num>(
  0,
  (total, it) => total + (it.price ?? 0) * (it.quantity ?? 1),
);
