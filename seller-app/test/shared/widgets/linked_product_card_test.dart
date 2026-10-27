import 'package:hawkr_seller/config/flavor.dart';
import 'package:hawkr_seller/shared/widgets/linked_product_card.dart';
import 'package:hawkr_seller/shared/widgets/optimized_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

// fixture 取自 2026-08-05 實打 dev 的 `PostProductData`（貼文列表的單數
// `product`／詳情的 `products[0]`）：只有 id / name / price / imageKey / imageUrl。
const String _key =
    'public/product/2685914682/20260719/cd527201-0b42-417d-8476-98d780c92a47.jpg';
const String _mediaBase = 'http://localhost:8080';

Future<void> _pumpCard(
  WidgetTester tester,
  Map<String, dynamic> linkedProduct, {
  Size size = const Size(414, 896),
  double textScale = 1,
  bool inset = true,
  VoidCallback? onTap,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(414, 896),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      splitScreenMode: true,
      builder: (_, _) => MaterialApp(
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(
            context,
          ).copyWith(textScaler: TextScaler.linear(textScale)),
          child: child!,
        ),
        home: Scaffold(
          body: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              LinkedProductCard(
                linkedProduct: linkedProduct,
                onTap: onTap,
                inset: inset,
              ),
            ],
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

Finder _tapTarget() => find.descendant(
  of: find.byType(LinkedProductCard),
  matching: find.byType(InkWell),
);

bool _hasShadow(WidgetTester tester) {
  final decoratedBoxes = tester.widgetList<DecoratedBox>(
    find.descendant(
      of: find.byType(LinkedProductCard),
      matching: find.byType(DecoratedBox),
    ),
  );
  return decoratedBoxes.any((widget) {
    final decoration = widget.decoration;
    return decoration is BoxDecoration &&
        (decoration.boxShadow?.isNotEmpty ?? false);
  });
}

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  testWidgets('關聯商品圖走後端實際回傳的 imageKey（objectKey）', (tester) async {
    await _pumpCard(tester, <String, dynamic>{
      'id': '866542675787809994',
      'name': '台灣紅豆',
      'price': '195',
      'imageKey': _key,
      'imageUrl': '$_mediaBase/i/original/$_key',
    });

    final image = tester.widget<OptimizedNetworkImage>(
      find.byType(OptimizedNetworkImage),
    );
    expect(image.imageUrl, '$_mediaBase/i/original/$_key');
  });

  testWidgets('imageKey 為空字串（建立回應的形狀）→ 不渲染圖片元件', (tester) async {
    await _pumpCard(tester, <String, dynamic>{
      'id': '866542675787809994',
      'name': '台灣紅豆',
      'price': '195',
      'imageKey': '',
      'imageUrl': '',
    });

    expect(find.byType(OptimizedNetworkImage), findsNothing);
  });

  testWidgets('default variant keeps legacy inset, height, and card chrome', (
    tester,
  ) async {
    await _pumpCard(
      tester,
      <String, dynamic>{'name': '台灣紅豆', 'price': '195', 'imageKey': ''},
      size: const Size(320, 720),
      onTap: () {},
    );

    expect(_tapTarget(), findsOneWidget);
    expect(tester.getSize(_tapTarget()), const Size(288, 74));
    expect(_hasShadow(tester), isTrue);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'embedded inset=false fits 320pt at 1.4x with long text and 48dp target',
    (tester) async {
      var taps = 0;
      const name = '南歐王生技中都－台灣紅豆（500G／包）產地限定超長商品名稱';
      final semantics = tester.ensureSemantics();
      try {
        await _pumpCard(
          tester,
          <String, dynamic>{'name': name, 'price': '123456789', 'imageKey': ''},
          size: const Size(320, 720),
          textScale: 1.4,
          inset: false,
          onTap: () => taps += 1,
        );

        expect(_tapTarget(), findsOneWidget);
        final targetSize = tester.getSize(_tapTarget());
        expect(targetSize.width, 320);
        expect(targetSize.height, greaterThanOrEqualTo(48));
        expect(
          find.bySemanticsLabel('查看商品，$name，NT\$ 123456789'),
          findsOneWidget,
        );

        final nameText = tester.widget<Text>(find.text(name));
        expect(nameText.maxLines, 2);
        expect(nameText.overflow, TextOverflow.ellipsis);
        expect(_hasShadow(tester), isFalse);
        expect(tester.takeException(), isNull);

        await tester.tap(_tapTarget());
        await tester.pump();
        expect(taps, 1);
        expect(tester.takeException(), isNull);
      } finally {
        semantics.dispose();
      }
    },
  );
}
