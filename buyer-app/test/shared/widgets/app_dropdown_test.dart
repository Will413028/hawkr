import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/app_dropdown.dart';
import 'package:flutter/material.dart';
import 'package:flutter/gestures.dart' show kLongPressTimeout;
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets(
    'search field owns one border and ignores the global input theme',
    (tester) async {
      final scheme = ColorScheme.fromSeed(seedColor: Colors.green);
      await tester.pumpWidget(
        MaterialApp(
          theme: ThemeData(
            useMaterial3: true,
            colorScheme: scheme,
            inputDecorationTheme: const InputDecorationThemeData(
              filled: true,
              fillColor: Colors.pink,
              focusedBorder: OutlineInputBorder(
                borderSide: BorderSide(color: Colors.purple, width: 5),
              ),
            ),
          ),
          home: Scaffold(
            body: Center(
              child: SizedBox(
                width: 320,
                child: AppDropdown<String>(
                  items: const ['臺北市', '新北市'],
                  itemLabel: (city) => city,
                  placeholder: '選擇縣市',
                  showSearch: true,
                  onSelected: (_) {},
                ),
              ),
            ),
          ),
        ),
      );

      await tester.tap(find.text('選擇縣市'));
      await tester.pump();

      final field = tester.widget<TextField>(
        find.byKey(const ValueKey('app-dropdown-search-field')),
      );
      expect(field.decoration?.filled, isFalse);
      expect(field.decoration?.border, InputBorder.none);
      expect(field.decoration?.enabledBorder, InputBorder.none);
      expect(field.decoration?.focusedBorder, InputBorder.none);
      expect(field.decoration?.disabledBorder, InputBorder.none);
      expect(field.decoration?.errorBorder, InputBorder.none);
      expect(field.decoration?.focusedErrorBorder, InputBorder.none);

      final surface = find.byKey(const ValueKey('app-dropdown-search-surface'));
      expect(tester.getSize(surface).height, HkControl.searchFieldHeight);

      BoxDecoration decoration() {
        return tester.widget<AnimatedContainer>(surface).decoration
            as BoxDecoration;
      }

      expect(decoration().borderRadius, BorderRadius.circular(HkRadius.md));
      expect(decoration().border?.top.color, scheme.outlineVariant);
      expect(decoration().border?.top.width, 1);
      expect(decoration().boxShadow, isNull);

      await tester.tap(find.byKey(const ValueKey('app-dropdown-search-field')));
      await tester.pump(const Duration(milliseconds: 200));

      expect(decoration().border?.top.color, scheme.primary);
      expect(decoration().border?.top.width, 1.5);
      expect(decoration().boxShadow, hasLength(1));
      expect(tester.takeException(), isNull);
    },
  );

  group('typed items', () {
    Future<void> pump<T>(
      WidgetTester tester, {
      required List<T> items,
      required String Function(T) itemLabel,
      required T? selectedItem,
      required ValueChanged<T> onSelected,
    }) {
      return tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: Align(
              alignment: Alignment.topCenter,
              child: SizedBox(
                width: 320,
                child: AppDropdown<T>(
                  items: items,
                  itemLabel: itemLabel,
                  selectedItem: selectedItem,
                  placeholder: '請選擇',
                  onSelected: onSelected,
                ),
              ),
            ),
          ),
        ),
      );
    }

    String intLabel(int? v) => v == null ? '無' : '第$v項';

    bool selected(WidgetTester tester, int index) => tester
        .getSemantics(find.byKey(ValueKey('app-dropdown-item-$index')))
        .getSemanticsData()
        .flagsCollection
        .isSelected
        .toBoolOrNull()!;

    testWidgets('tapping the entry whose value is null reports null', (
      tester,
    ) async {
      final picked = <int?>[];
      await pump<int?>(
        tester,
        items: const [null, 1, 2],
        itemLabel: intLabel,
        selectedItem: 2,
        onSelected: picked.add,
      );
      await tester.tap(find.text('第2項'));
      await tester.pump();
      await tester.tap(find.text('無'));
      await tester.pump();
      expect(picked, [null]);
    });

    testWidgets('long press dragged onto the null entry selects it once', (
      tester,
    ) async {
      final picked = <int?>[];
      await pump<int?>(
        tester,
        items: const [null, 1, 2],
        itemLabel: intLabel,
        selectedItem: 2,
        onSelected: picked.add,
      );
      await tester.tap(find.text('第2項'));
      await tester.pump();

      final gesture = await tester.startGesture(const Offset(700, 500));
      await tester.pump(kLongPressTimeout + const Duration(milliseconds: 50));
      await gesture.moveTo(tester.getCenter(find.text('無')));
      await tester.pump();
      await gesture.up();
      await tester.pump();
      expect(picked, [null]);
    });

    testWidgets('the entry equal to the current value is marked selected', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      try {
        await pump<int?>(
          tester,
          items: const [null, 1, 2],
          itemLabel: intLabel,
          selectedItem: null,
          onSelected: (_) {},
        );
        await tester.tap(find.text('請選擇'));
        await tester.pump();
        expect(
          [for (var i = 0; i < 3; i++) selected(tester, i)],
          [true, false, false],
        );
        await tester.tap(find.text('第1項'));
        await tester.pump();

        await pump<int?>(
          tester,
          items: const [null, 1, 2],
          itemLabel: intLabel,
          selectedItem: 2,
          onSelected: (_) {},
        );
        await tester.tap(find.text('第2項'));
        await tester.pump();
        expect(
          [for (var i = 0; i < 3; i++) selected(tester, i)],
          [false, false, true],
        );
      } finally {
        semantics.dispose();
      }
    });

    testWidgets('entries with the same label are told apart by value', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      try {
        await pump<int>(
          tester,
          items: const [1, 2],
          itemLabel: (_) => '同',
          selectedItem: 2,
          onSelected: (_) {},
        );
        await tester.tap(find.text('同'));
        await tester.pump();
        expect(selected(tester, 0), isFalse);
        expect(selected(tester, 1), isTrue);
      } finally {
        semantics.dispose();
      }
    });

    testWidgets('a value outside the items is shown but marks nothing', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      try {
        await pump<String>(
          tester,
          items: const ['甲', '乙'],
          itemLabel: (s) => s,
          selectedItem: null,
          onSelected: (_) {},
        );
        expect(find.text('請選擇'), findsOneWidget);
        await tester.tap(find.text('請選擇'));
        await tester.pump();
        expect(selected(tester, 0), isFalse);
        expect(selected(tester, 1), isFalse);
        await tester.tap(find.text('甲'));
        await tester.pump();

        await pump<String>(
          tester,
          items: const ['甲', '乙'],
          itemLabel: (s) => s,
          selectedItem: '不在清單',
          onSelected: (_) {},
        );
        expect(find.text('請選擇'), findsNothing);
        expect(find.text('不在清單'), findsOneWidget);
        await tester.tap(find.text('不在清單'));
        await tester.pump();
        expect(selected(tester, 0), isFalse);
        expect(selected(tester, 1), isFalse);
      } finally {
        semantics.dispose();
      }
    });
  });
}
