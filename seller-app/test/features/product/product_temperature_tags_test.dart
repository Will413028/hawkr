import 'package:hawkr_seller/features/product/presentation/widgets/product_temperature_tags.dart';
import 'package:hawkr_seller/shared/widgets/hk_temperature_tag.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  Future<void> pump(
    WidgetTester tester,
    Object? raw, {
    double width = 320,
    double scale = 1.4,
  }) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Center(
            child: MediaQuery(
              data: MediaQueryData(textScaler: TextScaler.linear(scale)),
              child: SizedBox(
                width: width,
                child: ProductTemperatureTags(deliveryTemp: raw),
              ),
            ),
          ),
        ),
      ),
    );
  }

  testWidgets(
    'product data renders distinct real values without ambient fallback',
    (tester) async {
      await pump(tester, ['3202', 'CHILLED', 3203, 'future-code']);
      expect(find.text('冷藏'), findsOneWidget);
      expect(find.text('冷凍'), findsOneWidget);
      expect(find.text('future-code'), findsOneWidget);
      expect(find.text('常溫'), findsNothing);
      expect(find.byType(HkTemperatureTag), findsNWidgets(3));
      await pump(tester, '3203,3201');
      expect(find.text('冷凍'), findsOneWidget);
      expect(find.text('常溫'), findsOneWidget);
      expect(find.text('冷藏'), findsNothing);
    },
  );

  testWidgets('absent or invalid product temperature displays unspecified', (
    tester,
  ) async {
    for (final value in [
      null,
      '',
      [],
      <String, Object>{'label': '冷藏'},
    ]) {
      await pump(tester, value);
      expect(find.text('尚未指定'), findsOneWidget);
      expect(find.byType(HkTemperatureTag), findsNothing);
    }
  });

  testWidgets(
    'multiple temperature tags wrap under narrow layout and large text',
    (tester) async {
      await pump(tester, ['3201', '3202', '3203'], width: 110, scale: 2);
      expect(
        tester.getTopLeft(find.text('冷凍')).dy,
        greaterThan(tester.getTopLeft(find.text('常溫')).dy),
      );
      expect(tester.takeException(), isNull);
    },
  );
}
