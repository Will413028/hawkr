import 'dart:ui' show Tristate;

import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/features/order/data/order_model.dart';
import 'package:hawkr_buyer/features/order/data/order_repository.dart';
import 'package:hawkr_buyer/features/order/presentation/order_card.dart';
import 'package:hawkr_buyer/features/order/presentation/order_detail_transition.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:hawkr_buyer/shared/utils/date_format_utils.dart';
import 'package:hawkr_buyer/shared/widgets/oss_image.dart';
import 'package:hawkr_buyer/shared/widgets/product_card_hero.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('列表卡片顯示相同狀態、商品、配送與操作層級', (tester) async {
    final semantics = tester.ensureSemantics();
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 760);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) => const Scaffold(
            body: SingleChildScrollView(
              padding: EdgeInsets.all(12),
              child: OrderCard(order: _order),
            ),
          ),
        ),
        GoRoute(
          path: '/merchant/:sellerId',
          builder: (_, state) => Scaffold(
            body: Text(
              '商家 ${state.pathParameters['sellerId']}',
              key: const ValueKey('merchant-route'),
            ),
          ),
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, __) => MaterialApp.router(
            routerConfig: router,
            builder: (context, child) => MediaQuery(
              data: AppMediaQueryPolicy.apply(
                MediaQuery.of(
                  context,
                ).copyWith(textScaler: const TextScaler.linear(2)),
              ),
              child: child!,
            ),
          ),
        ),
      ),
    );
    await tester.pump();

    expect(find.text('配送中'), findsOneWidget);
    expect(find.text('蟹味棒'), findsOneWidget);
    expect(find.textContaining('共 2 件'), findsOneWidget);
    expect(find.text('黑貓宅急便 · 已叫件'), findsOneWidget);
    expect(find.text('桃園市桃園區鎮撫街1號'), findsOneWidget);
    expect(find.text(_expectedOrderTime), findsNothing);
    expect(find.text('訂單編號'), findsNothing);
    expect(find.text('O125369895'), findsNothing);
    expect(find.text('細節'), findsNothing);
    final orderCardContext = tester.element(find.byType(OrderCard));
    expect(MediaQuery.textScalerOf(orderCardContext).scale(10), 14);

    final productImage = tester.widget<OssImage>(find.byType(OssImage));
    expect(productImage.fit, BoxFit.cover);
    expect(productImage.errorWidget, isA<ColoredBox>());
    final carrierIcons = tester
        .widgetList<HkIcon>(find.byType(HkIcon))
        .where((icon) => icon.name == HkIconName.truck);
    expect(carrierIcons, hasLength(1));

    final hero = tester.widget<Hero>(find.byType(Hero));
    expect(hero.tag, orderDetailHeroTag(_order.orderSn));
    expect(
      hero.flightShuttleBuilder,
      same(productCardHeroFlightShuttleBuilder),
    );
    final statusIcons = tester
        .widgetList<HkIcon>(find.byType(HkIcon))
        .where((icon) => icon.name == HkIconName.orderShipping);
    expect(statusIcons, hasLength(1));
    final sellerAction = find.bySemanticsLabel('查看商家 金湖水產');
    expect(sellerAction, findsOneWidget);
    expect(tester.getSize(sellerAction), const Size.square(48));
    var cardAction = find.bySemanticsLabel('展開訂單 O125369895');
    var cardSemantics = tester.getSemantics(cardAction).getSemanticsData();
    expect(cardSemantics.flagsCollection.isButton, isTrue);
    expect(cardSemantics.flagsCollection.isExpanded, Tristate.isFalse);
    expect(tester.takeException(), isNull);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));

    await tester.tap(sellerAction);
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 400));
    expect(find.byKey(const ValueKey('merchant-route')), findsOneWidget);
    router.pop();
    await tester.pump();
    await tester.pump(const Duration(seconds: 1));
    expect(
      find.byKey(const ValueKey('order-card-expanded-content')),
      findsNothing,
    );
    expect(find.bySemanticsLabel('展開訂單 O125369895'), findsOneWidget);

    await tester.tap(find.byKey(const ValueKey('order-card-collapsed')));
    await tester.pump(const Duration(milliseconds: 300));

    expect(find.text('訂單編號'), findsOneWidget);
    expect(find.text('O125369895'), findsOneWidget);
    expect(find.text('下單 $_expectedOrderTime'), findsOneWidget);
    expect(find.text('細節'), findsOneWidget);
    expect(
      find.byWidgetPredicate(
        (widget) => widget is HkIcon && widget.name == HkIconName.copy,
      ),
      findsOneWidget,
    );
    cardAction = find.bySemanticsLabel('收合訂單 O125369895');
    cardSemantics = tester.getSemantics(cardAction).getSemanticsData();
    expect(cardSemantics.flagsCollection.isExpanded, Tristate.isTrue);
    expect(tester.takeException(), isNull);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
  });

  testWidgets('訂單卡進入詳情的 Hero 動畫不會破版', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 760);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final heroTag = orderDetailHeroTag(_order.orderSn)!;
    final router = GoRouter(
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) => const Scaffold(
            body: SingleChildScrollView(
              padding: EdgeInsets.all(12),
              child: OrderCard(order: _order),
            ),
          ),
        ),
        GoRoute(
          path: '/orders/detail',
          builder: (_, _) => Scaffold(
            body: Hero(
              tag: heroTag,
              flightShuttleBuilder: productCardHeroFlightShuttleBuilder,
              child: const Material(child: SizedBox.expand()),
            ),
          ),
        ),
      ],
    );
    addTearDown(router.dispose);

    await tester.pumpWidget(
      ProviderScope(
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, __) => MaterialApp.router(routerConfig: router),
        ),
      ),
    );
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 50));

    router.push('/orders/detail');
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 100));

    expect(tester.takeException(), isNull);
    await tester.pump(const Duration(milliseconds: 500));
    expect(tester.takeException(), isNull);
  });

  testWidgets('列表契約缺少商品圖時會以訂單詳情照片補齊', (tester) async {
    tester.view.devicePixelRatio = 1;
    tester.view.physicalSize = const Size(320, 760);
    addTearDown(tester.view.resetDevicePixelRatio);
    addTearDown(tester.view.resetPhysicalSize);

    final dio = Dio();
    addTearDown(dio.close);
    final listOrder = _order.copyWith(
      items: [_order.items.first.copyWith(imageUrl: '')],
    );

    await tester.pumpWidget(
      ProviderScope(
        overrides: [
          orderRepositoryProvider.overrideWithValue(_ImageOrderRepository(dio)),
        ],
        child: ScreenUtilInit(
          designSize: const Size(390, 844),
          builder: (_, __) => MaterialApp(
            home: Scaffold(body: OrderCard(order: listOrder)),
          ),
        ),
      ),
    );
    await tester.pump();
    await tester.pump();

    final productImage = tester.widget<OssImage>(find.byType(OssImage));
    expect(productImage.imageSource, 'products/detail-red-bean.jpg');
    expect(productImage.fit, BoxFit.cover);
    expect(tester.takeException(), isNull);
  });
}

class _ImageOrderRepository extends OrderRepository {
  _ImageOrderRepository(super.dio);

  @override
  Future<Order> getOrderDetail(String orderSn) async => _order.copyWith(
    items: [
      _order.items.first.copyWith(
        productName: '台灣紅豆',
        imageUrl: 'products/detail-red-bean.jpg',
      ),
    ],
  );
}

/// 下單時間的期望顯示字串。
///
/// 不能硬編碼：[DateFormatUtils.formatDateTime] 走 `toLocal()`，同一筆
/// `+08:00` 的 createTime 在 UTC+8 顯示 `2026/07/16 06:38`、在 UTC 則是
/// `2026/07/15 22:38`。硬編碼會讓測試在開發者本機過、在 CI（UTC）紅。
/// 用與 UI 相同的函式推導期望值，驗的仍是「卡片顯示了正確格式化的下單時間」。
final _expectedOrderTime = DateFormatUtils.formatDateTime(_order.createTime);

const _order = Order(
  orderSn: 'O125369895',
  status: '30',
  totalActualAmount: '250',
  createTime: '2026-07-16T06:38:00+08:00',
  sellerName: '金湖水產',
  storeId: 'seller-1',
  recipientPhone: '0912345678',
  shippingAddress: '桃園市桃園區鎮撫街1號',
  shippingMethod: '3001',
  shippingStatus: '210',
  deliveryTemp: '3203',
  itemCount: '2',
  items: [
    OrderItem(
      productName: '蟹味棒',
      skuName: '家庭號',
      quantity: '2',
      unitPrice: '125',
      actualPayAmount: '250',
      imageUrl: 'products/crab-stick.jpg',
    ),
  ],
);
