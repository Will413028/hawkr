import 'dart:convert';

import 'package:riverpod_annotation/riverpod_annotation.dart';
import 'package:shared_preferences/shared_preferences.dart';

part 'shop_tab_local_service.g.dart';

/// Stateless adapter for the legacy seller-products cache.
class ShopTabLocalService {
  const ShopTabLocalService();

  String _key(String sellerId) => 'seller_products_$sellerId';

  Future<List<Map<String, dynamic>>> readProducts(String sellerId) async {
    final preferences = await SharedPreferences.getInstance();
    final encoded = preferences.getString(_key(sellerId));
    if (encoded == null || encoded.isEmpty) return const [];
    final decoded = jsonDecode(encoded);
    if (decoded is! List) return const [];
    return List<Map<String, dynamic>>.unmodifiable(
      decoded.whereType<Map>().map(
        (entry) =>
            Map<String, dynamic>.unmodifiable(Map<String, dynamic>.from(entry)),
      ),
    );
  }

  Future<void> writeProducts(
    String sellerId,
    List<Map<String, dynamic>> products,
  ) async {
    final preferences = await SharedPreferences.getInstance();
    await preferences.setString(
      _key(sellerId),
      jsonEncode(products.take(200).toList(growable: false)),
    );
  }
}

@riverpod
ShopTabLocalService shopTabLocalService(Ref ref) {
  return const ShopTabLocalService();
}
