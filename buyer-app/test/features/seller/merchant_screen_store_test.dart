import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart' as seller;
import 'package:hawkr_buyer/core/exceptions/app_exception.dart';
import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/core/session/session_model.dart';
import 'package:hawkr_buyer/core/session/session_provider.dart';
import 'package:hawkr_buyer/features/seller/data/seller_repository.dart';
import 'package:hawkr_buyer/features/seller/presentation/merchant_screen.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:mocktail/mocktail.dart';

class _MockSellerRepository extends Mock implements SellerRepository {}

class _GuestSession extends Session {
  @override
  Future<SessionModel?> build() async => null;
}

Dio _offlineDio() {
  final dio = Dio(BaseOptions(baseUrl: 'https://store.test'));
  dio.interceptors.add(
    InterceptorsWrapper(
      onRequest: (options, handler) => handler.reject(
        DioException(
          requestOptions: options,
          type: DioExceptionType.badResponse,
          response: Response<void>(requestOptions: options, statusCode: 404),
        ),
      ),
    ),
  );
  return dio;
}

Future<void> _pump(WidgetTester tester, SellerRepository repository) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = const Size(390, 844);
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);
  final dio = _offlineDio();
  addTearDown(dio.close);
  await tester.pumpWidget(
    ProviderScope(
      overrides: [
        dioProvider.overrideWithValue(dio),
        sellerRepositoryProvider.overrideWithValue(repository),
        sessionProvider.overrideWith(_GuestSession.new),
      ],
      child: ScreenUtilInit(
        designSize: const Size(390, 844),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) =>
            const MaterialApp(home: MerchantScreen(storeCode: 'jinhu')),
      ),
    ),
  );
  for (var i = 0; i < 5; i++) {
    await tester.pump(const Duration(milliseconds: 100));
  }
}

void main() {
  testWidgets('shows the GetStore name, location and stats', (tester) async {
    final repository = _MockSellerRepository();
    when(() => repository.getStore('jinhu')).thenAnswer(
      (_) async => seller.GetStoreResponse(
        store: seller.PublicStore(
          id: '9001',
          storeCode: 'jinhu',
          storeName: '金湖水產',
          townCode: '64000090',
        ),
        stats: seller.StoreStats(
          followerCount: 12,
          reviewCount: 36,
          ratingAverage: 4.5,
        ),
      ),
    );

    await _pump(tester, repository);

    expect(find.text('金湖水產'), findsWidgets);
    expect(find.textContaining('高雄市前鎮區'), findsWidgets);
    expect(find.byKey(const Key('merchant_certification_badge')), findsNothing);
    verify(() => repository.getStore('jinhu')).called(1);
  });

  testWidgets('a store that cannot be read shows the not found page', (
    tester,
  ) async {
    final repository = _MockSellerRepository();
    when(
      () => repository.getStore('jinhu'),
    ).thenThrow(const NotFoundException('missing'));

    await _pump(tester, repository);

    expect(find.text('找不到這家店'), findsOneWidget);
    expect(find.text('這家店可能暫停營業、已經關閉，或網址有誤。'), findsOneWidget);
    expect(find.text('返回'), findsOneWidget);
  });

  testWidgets('a store without rating stats shows no rating, not 0.0', (
    tester,
  ) async {
    final repository = _MockSellerRepository();
    when(() => repository.getStore('jinhu')).thenAnswer(
      (_) async => seller.GetStoreResponse(
        store: seller.PublicStore(
          id: '9001',
          storeCode: 'jinhu',
          storeName: '金湖水產',
        ),
        stats: seller.StoreStats(followerCount: 12),
      ),
    );

    await _pump(tester, repository);

    expect(find.text('尚無評分'), findsOneWidget);
    expect(find.textContaining('0.0'), findsNothing);
  });

  testWidgets(
    'a failure other than not_found offers retry, not the not found page',
    (tester) async {
      final repository = _MockSellerRepository();
      var calls = 0;
      when(() => repository.getStore('jinhu')).thenAnswer((_) async {
        calls++;
        if (calls == 1) throw const ServerException('down');
        return seller.GetStoreResponse(
          store: seller.PublicStore(
            id: '9001',
            storeCode: 'jinhu',
            storeName: '金湖水產',
          ),
        );
      });

      await _pump(tester, repository);

      expect(find.text('找不到這家店'), findsNothing);
      expect(find.text('重試'), findsOneWidget);

      await tester.tap(find.text('重試'));
      for (var i = 0; i < 5; i++) {
        await tester.pump(const Duration(milliseconds: 100));
      }

      expect(find.text('金湖水產'), findsWidgets);
      expect(find.text('重試'), findsNothing);
      expect(calls, 2);
    },
  );
}
