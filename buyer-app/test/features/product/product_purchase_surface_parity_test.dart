import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

void main() {
  const detailPath =
      'lib/features/product/presentation/merchant_item_detail_screen.dart';
  const sheetPath =
      'lib/features/home/presentation/home_widget/product_add_to_cart_sheet.dart';

  late String detailSource;
  late String sheetSource;

  setUpAll(() {
    detailSource = File(detailPath).readAsStringSync();
    sheetSource = File(sheetPath).readAsStringSync();
  });

  group('product purchase surface architecture', () {
    test('detail and sheet consume the shared purchase components', () {
      for (final surface in <({String name, String source})>[
        (name: 'product detail', source: detailSource),
        (name: 'add-to-cart sheet', source: sheetSource),
      ]) {
        expect(
          _sharedPanelImport.hasMatch(surface.source),
          isTrue,
          reason: '${surface.name} must import product_purchase_panel.dart',
        );
        expect(
          RegExp(r'\bProductPurchaseSelector\s*\(').hasMatch(surface.source),
          isTrue,
          reason: '${surface.name} must render ProductPurchaseSelector',
        );
        expect(
          RegExp(r'\bProductPurchaseFooter\s*\(').hasMatch(surface.source),
          isTrue,
          reason: '${surface.name} must render ProductPurchaseFooter',
        );
      }

      expect(
        RegExp(r'\bProductDeliveryMetadata\s*\(').hasMatch(detailSource),
        isTrue,
        reason: 'product detail must share the sheet delivery metadata UI',
      );
    });

    test('product detail no longer carries the legacy purchase UI', () {
      for (final legacyToken in const <String>[
        '_SpecSelectionSectionNew',
        'temperature-half',
        'AppIcons.monetizationOn',
        'assets/checkout/',
      ]) {
        expect(
          detailSource.contains(legacyToken),
          isFalse,
          reason: 'product detail must not retain legacy token $legacyToken',
        );
      }
    });

    test('surfaces do not redeclare shared private purchase atoms', () {
      for (final surface in <({String name, String source})>[
        (name: 'product detail', source: detailSource),
        (name: 'add-to-cart sheet', source: sheetSource),
      ]) {
        expect(
          RegExp(r'\bclass\s+_ActionButton\b').hasMatch(surface.source),
          isFalse,
          reason: '${surface.name} must use the shared purchase CTA',
        );
        expect(
          RegExp(r'\bclass\s+_ShippingTag\b').hasMatch(surface.source),
          isFalse,
          reason: '${surface.name} must use the shared shipping mark',
        );
      }
    });
  });
}

final RegExp _sharedPanelImport = RegExp(
  r'''import\s+['"][^'"]*product_purchase_panel\.dart['"]\s*;''',
);
