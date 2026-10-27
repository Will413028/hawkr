import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';
import 'package:hawkr_buyer/features/order/data/receipt_photo_local_service.dart';
import 'package:hawkr_buyer/features/order/data/receipt_photo_model.dart';
import 'package:hawkr_buyer/features/order/data/receipt_photo_repository.dart';
import 'package:hawkr_buyer/features/order/presentation/receipt_photo_controller.dart';
import 'package:hawkr_buyer/features/order/presentation/widgets/order_receipt_photos_section.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  test('backend photos win and stale local preview is removed', () async {
    final service = _FakeReceiptPhotoLocalService({
      'HK-1': 'https://local.test/preview.jpg',
    });
    final repository = ReceiptPhotoRepository(service);

    final state = await repository.load(
      orderSn: 'HK-1',
      backendUrls: const [' https://cdn.test/final.jpg '],
    );

    expect(state.source, ReceiptPhotoSource.backend);
    expect(state.photos, ['https://cdn.test/final.jpg']);
    expect(service.values, isNot(contains('HK-1')));
  });

  test('clearAll wipes every order in the local cache', () async {
    final service = _FakeReceiptPhotoLocalService({
      'HK-1': 'https://local.test/a.jpg',
      'HK-2': 'https://local.test/b.jpg',
    });
    final repository = ReceiptPhotoRepository(service);

    await repository.clearAll();

    expect(service.values, isEmpty);
  });

  test('controller cache command updates the same typed local state', () async {
    final service = _FakeReceiptPhotoLocalService();
    final container = ProviderContainer(
      overrides: [receiptPhotoLocalServiceProvider.overrideWithValue(service)],
    );
    addTearDown(container.dispose);
    final provider = receiptPhotoControllerProvider(
      orderSn: 'HK-2',
      backendUrls: const [],
    );
    await container.read(provider.future);

    final saved = await container
        .read(provider.notifier)
        .cachePreview(' https://local.test/receipt.jpg ');

    expect(saved, isTrue);
    expect(container.read(provider).requireValue.photos, [
      'https://local.test/receipt.jpg',
    ]);
    expect(service.values['HK-2'], 'https://local.test/receipt.jpg');
  });

  testWidgets(
    'extracted receipt section preserves seller/buyer labels and comparison',
    (tester) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = const Size(390, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);

      await tester.pumpWidget(
        ProviderScope(
          overrides: [
            receiptPhotoLocalServiceProvider.overrideWithValue(
              _FakeReceiptPhotoLocalService(),
            ),
          ],
          child: ScreenUtilInit(
            designSize: const Size(390, 844),
            enableScaleWH: () => false,
            enableScaleText: () => false,
            builder: (_, __) => const MaterialApp(
              home: Scaffold(
                body: OrderReceiptPhotosSection(
                  order: Order(
                    orderSn: 'HK-3',
                    shipmentPhotoUrls: ['https://example.test/seller.jpg'],
                    receiptPhotoUrls: ['https://example.test/buyer.jpg'],
                  ),
                ),
              ),
            ),
          ),
        ),
      );
      await tester.pump();

      expect(
        find.byKey(const ValueKey('order-receipt-photos-section')),
        findsOneWidget,
      );
      expect(find.text('買賣家上傳照片'), findsOneWidget);
      expect(find.text('賣家出貨照片'), findsOneWidget);
      expect(find.text('買家收貨照片'), findsOneWidget);
      expect(find.text('vs'), findsOneWidget);
      expect(tester.takeException(), isNull);
    },
  );
}

final class _FakeReceiptPhotoLocalService implements ReceiptPhotoLocalService {
  _FakeReceiptPhotoLocalService([Map<String, String>? initial])
    : values = {...?initial};

  final Map<String, String> values;

  @override
  Future<String?> read(String orderSn) async => values[orderSn];

  @override
  Future<void> remove(String orderSn) async {
    values.remove(orderSn);
  }

  @override
  Future<void> write(String orderSn, String url) async {
    values[orderSn] = url;
  }

  @override
  Future<void> clearAll() async {
    values.clear();
  }
}
