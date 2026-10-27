import 'package:hawkr_buyer/shared/widgets/product_card.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// `ProductCard` 是市集 / 首頁推薦 / 分類三個列表共用的商品卡，是 `"0"` 價格
/// 曝光度最高的渲染點（dev 實測就是在市集看到 `NT$ 0`）。
///
/// ⚠️ 錨必須下在 widget，不能只測 `PriceDisplay`：上一輪修的正是 helper，
/// 而這張卡自己 `num.tryParse(widget.price)` 再拼字串，helper 全綠、畫面照錯。
/// 這裡斷言的是**畫面上有沒有那串字**，不是某個函式的回傳值。
Widget _host(Widget child) => MaterialApp(
  home: Scaffold(
    body: Center(child: SizedBox(width: 180, height: 260, child: child)),
  ),
);

/// 卡片內所有 Text / Text.rich 的可見文字，攤平成一個字串。
String _visibleText(WidgetTester tester) {
  final buffer = StringBuffer();
  for (final w in tester.widgetList<Text>(find.byType(Text))) {
    buffer.write(w.data ?? w.textSpan?.toPlainText() ?? '');
    buffer.write('\n');
  }
  return buffer.toString();
}

void main() {
  testWidgets('priceMin/Max 都是 "0" 時整條價格列不渲染，畫面不得出現 NT\$', (tester) async {
    await tester.pumpWidget(
      _host(const ProductCard(name: '無價格商品', price: '0', priceMax: '0')),
    );

    expect(find.byKey(const ValueKey('product-card-price-row')), findsNothing);
    expect(_visibleText(tester), isNot(contains('NT\$')));
    expect(_visibleText(tester), isNot(contains('0')));
    // 商品名稱仍在 —— 隱藏的只有價格，不是整張卡。
    expect(find.text('無價格商品'), findsOneWidget);
  });

  testWidgets('有價格時照常渲染價格列', (tester) async {
    await tester.pumpWidget(
      _host(const ProductCard(name: '有價格商品', price: '120', priceMax: '120')),
    );

    expect(
      find.byKey(const ValueKey('product-card-price-row')),
      findsOneWidget,
    );
    expect(_visibleText(tester), contains('NT\$ 120'));
  });

  testWidgets('區間價格照常渲染成 min~max', (tester) async {
    await tester.pumpWidget(
      _host(const ProductCard(name: '區間商品', price: '120', priceMax: '250')),
    );

    expect(_visibleText(tester), contains('NT\$ 120~250'));
  });

  testWidgets('單邊為 "0" 時只印有價格的一邊，不印 0', (tester) async {
    await tester.pumpWidget(
      _host(const ProductCard(name: '單邊商品', price: '0', priceMax: '250')),
    );

    expect(_visibleText(tester), contains('NT\$ 250'));
    expect(_visibleText(tester), isNot(contains('0~')));
  });

  testWidgets('售價為 "0" 時不單獨掛一條被劃掉的原價', (tester) async {
    // 價格列整條不畫，所以刪除線原價也不該有機會出現。
    await tester.pumpWidget(
      _host(
        const ProductCard(
          name: '無售價有原價',
          price: '0',
          priceMax: '0',
          listPriceMin: '200',
          listPriceMax: '200',
        ),
      ),
    );

    expect(find.byKey(const ValueKey('product-card-price-row')), findsNothing);
    expect(_visibleText(tester), isNot(contains('200')));
  });

  testWidgets('listPrice 為 "0" 時不顯示刪除線，但售價照常顯示', (tester) async {
    tester.view.physicalSize = const Size(1400, 1600);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);

    await tester.pumpWidget(
      _host(
        const ProductCard(
          name: '無原價',
          price: '120',
          priceMax: '120',
          listPriceMin: '0',
          listPriceMax: '0',
        ),
      ),
    );

    expect(_visibleText(tester), contains('NT\$ 120'));
    // 刪除線那格若把 "0" 印出來，這裡會抓到一個獨立的 "0"。
    expect(_visibleText(tester).split('\n'), isNot(contains('0')));
  });
}
