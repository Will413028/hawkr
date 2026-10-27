import 'dart:async';
import 'dart:io';

import 'package:hawkr_seller/features/post/data/shop_tab_local_service.dart';
import 'package:hawkr_seller/features/post/data/shop_tab_repository.dart';
import 'package:hawkr_seller/features/post/presentation/shop_tab_controller.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:shared_preferences/shared_preferences.dart';

void main() {
  test('local service round-trips the bounded seller product cache', () async {
    SharedPreferences.setMockInitialValues(const {});
    const service = ShopTabLocalService();
    final products = [
      for (var index = 0; index < 205; index++)
        <String, dynamic>{'id': index, 'name': '商品 $index'},
    ];

    await service.writeProducts('7', products);
    final restored = await service.readProducts('7');

    expect(restored, hasLength(200));
    expect(restored.first['id'], 0);
    expect(restored.last['id'], 199);
    expect(() => restored.first['name'] = 'changed', throwsUnsupportedError);
  });

  test(
    'controller renders cached products before remote refresh completes',
    () async {
      final repository = _FakeShopTabRepository();
      final container = ProviderContainer(
        overrides: [shopTabRepositoryProvider.overrideWithValue(repository)],
      );
      addTearDown(container.dispose);
      final provider = shopTabControllerProvider('7');
      final subscription = container.listen(provider, (_, __) {});
      addTearDown(subscription.close);
      final controller = container.read(provider.notifier);

      final operation = controller.refreshProducts();
      await Future<void>.delayed(Duration.zero);

      expect(container.read(provider).products.single['id'], 'cached');
      expect(container.read(provider).isLoadingProducts, isTrue);

      repository.remote.complete([
        {'id': 'remote'},
      ]);
      await operation;

      final state = container.read(provider);
      expect(state.products.single['id'], 'remote');
      expect(state.isLoadingProducts, isFalse);
      expect(
        () => state.products.add({'id': 'mutated'}),
        throwsUnsupportedError,
      );
    },
  );

  test('shop view has no low-level API or SharedPreferences access', () {
    final source = File(
      'lib/features/post/presentation/widgets/page_shop_tab.dart',
    ).readAsStringSync();

    expect(source, isNot(contains('SharedPreferences')));
    expect(source, isNot(contains('.apiGet(')));
    expect(source, contains('shopTabControllerProvider'));
    expect(source, isNot(contains('SellerRoutes.certification')));
    expect(source, isNot(contains('getSellerInfo')));
  });
}

class _FakeShopTabRepository implements ShopTabRepository {
  final remote = Completer<List<Map<String, dynamic>>>();

  @override
  Future<List<Map<String, dynamic>>> readCachedProducts(String sellerId) async {
    return const [
      {'id': 'cached'},
    ];
  }

  @override
  Future<List<Map<String, dynamic>>> refreshProducts(String sellerId) {
    return remote.future;
  }
}
