import 'package:hawkr_buyer/features/cart/presentation/cart_state.dart';
import 'package:flutter_test/flutter_test.dart';

/// `CartState.shippingSelection` —— 「買家現在選了哪一種配送」的**唯一**判別點。
void main() {
  const tierOption = <String, dynamic>{
    'shippingMethod': '3001',
    'label': '黑貓宅急便（常溫）',
    'fee': '120',
  };

  CartState stateWith({
    List<Map<String, dynamic>> shippingOptions = const [],
    int selectedShippingIndex = 0,
  }) => CartState(
    status: CartContentStatus.ready,
    data: null,
    shippingOptions: shippingOptions,
    selectedShippingIndex: selectedShippingIndex,
  );

  group('配送選項', () {
    test('index 落在範圍內 → TieredShippingSelected 並帶著那一筆選項', () {
      final selection = stateWith(
        shippingOptions: const [tierOption],
      ).shippingSelection;
      expect(selection, isA<TieredShippingSelected>());
      final tiered = selection as TieredShippingSelected;
      expect(tiered.index, 0);
      expect(tiered.shippingMethod, '3001');
    });

    test('沒有任何選項 → ShippingUnselected', () {
      expect(stateWith().shippingSelection, isA<ShippingUnselected>());
    });

    test('index 越界 → ShippingUnselected（不得回傳越界的那一筆）', () {
      for (final index in [5, -1]) {
        final selection = stateWith(
          shippingOptions: const [tierOption],
          selectedShippingIndex: index,
        ).shippingSelection;
        expect(selection, isA<ShippingUnselected>(), reason: 'index $index');
      }
    });
  });
}
