import 'dart:ui' show SemanticsAction;

import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/icons/app_icon_colors.dart';
import 'package:hawkr_buyer/shared/widgets/category_top_bar.dart';
import 'package:hawkr_buyer/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const widths = <double>[320, 390, 768];
  const variants = <String, double>{
    'compact': HkControl.searchFieldHeight,
    'prominent': HkControl.prominentSearchHeight,
  };

  for (final width in widths) {
    for (final variant in variants.entries) {
      testWidgets('${variant.key} search keeps an accessible height at '
          '${width.toInt()}px and 1.4x text', (tester) async {
        await _pumpSearch(
          tester,
          width: width,
          height: variant.value,
          readOnly: false,
        );

        final search = find.byType(CategorySearchField);
        final expectedHeight = variant.value < HkControl.minTap
            ? HkControl.minTap
            : variant.value;
        expect(search, findsOneWidget);
        expect(tester.getSize(search).height, expectedHeight);
        expect(tester.getSize(search).width, lessThanOrEqualTo(720));
        expect(tester.takeException(), isNull);

        final mediaQuery = tester.widget<MediaQuery>(
          find.ancestor(of: search, matching: find.byType(MediaQuery)).first,
        );
        expect(mediaQuery.data.textScaler.scale(10), 14);
      });

      testWidgets('${variant.key} read-only search does not overflow at '
          '${width.toInt()}px and 1.4x text', (tester) async {
        await _pumpSearch(
          tester,
          width: width,
          height: variant.value,
          readOnly: true,
        );

        final expectedHeight = variant.value < HkControl.minTap
            ? HkControl.minTap
            : variant.value;
        expect(
          tester.getSize(find.byType(CategorySearchField)).height,
          expectedHeight,
        );
        expect(tester.takeException(), isNull);
      });
    }
  }

  testWidgets('search surface uses the website semantic color roles', (
    tester,
  ) async {
    await _pumpSearch(
      tester,
      width: 390,
      height: HkControl.searchFieldHeight,
      readOnly: false,
    );

    final search = find.byType(CategorySearchField);
    final surfaceFinder = find.descendant(
      of: search,
      matching: find.byWidgetPredicate(
        (widget) =>
            widget is DecoratedBox && widget.decoration is BoxDecoration,
      ),
    );
    final surface = tester.widget<DecoratedBox>(surfaceFinder.first);
    final decoration = surface.decoration as BoxDecoration;
    final border = decoration.border! as Border;
    expect(decoration.color, AppColors.bgCard);
    expect(border.top.color, AppColors.borderInput);

    final field = tester.widget<TextField>(
      find.descendant(of: search, matching: find.byType(TextField)),
    );
    final fieldDecoration = field.decoration!;
    expect(fieldDecoration.hintStyle?.color, AppColors.textHint);
    expect(fieldDecoration.hintMaxLines, 1);
    expect(fieldDecoration.filled, isFalse);
    expect(fieldDecoration.fillColor, Colors.transparent);
    expect(fieldDecoration.border, InputBorder.none);
    expect(fieldDecoration.enabledBorder, InputBorder.none);
    expect(fieldDecoration.focusedBorder, InputBorder.none);
    expect(fieldDecoration.disabledBorder, InputBorder.none);
    expect(fieldDecoration.errorBorder, InputBorder.none);
    expect(fieldDecoration.focusedErrorBorder, InputBorder.none);
    expect(field.style?.fontSize, 16);
  });

  testWidgets('search has a localized default hint', (tester) async {
    await _pumpSearch(
      tester,
      width: 390,
      height: HkControl.searchFieldHeight,
      readOnly: false,
      hintText: null,
    );

    final field = tester.widget<TextField>(find.byType(TextField));
    expect(field.decoration?.hint, isA<ExcludeSemantics>());
    final hint = tester.widget<Text>(find.text('搜尋商品'));
    expect(hint.maxLines, 1);
    expect(hint.softWrap, isFalse);
    expect(hint.overflow, TextOverflow.ellipsis);
  });

  testWidgets('editable search exposes a clear semantic label and hint', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await _pumpSearch(
      tester,
      width: 320,
      height: HkControl.searchFieldHeight,
      readOnly: false,
    );

    final data = tester
        .getSemantics(find.byType(CategorySearchField))
        .getSemanticsData();
    expect(data.label, '商品搜尋');
    expect(data.hint, '輸入商品名稱或關鍵字');
    expect(data.flagsCollection.isTextField, isTrue);
    expect(tester.getSize(find.byType(CategorySearchField)).height, 48);

    await tester.enterText(find.byType(TextField), '高麗菜');
    await tester.pump();
    final populatedData = tester
        .getSemantics(find.byType(CategorySearchField))
        .getSemanticsData();
    expect(populatedData.label, '商品搜尋');
    expect(populatedData.hint, '輸入商品名稱或關鍵字');
    expect(populatedData.value, '高麗菜');
    semantics.dispose();
  });

  testWidgets('read-only search exposes one actionable semantic node', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await _pumpSearch(
      tester,
      width: 320,
      height: HkControl.searchFieldHeight,
      readOnly: true,
      semanticHint: '開啟商品搜尋',
    );

    final search = find.byType(CategorySearchField);
    final data = tester.getSemantics(search).getSemanticsData();
    expect(find.bySemanticsLabel('商品搜尋'), findsOneWidget);
    expect(data.label, '商品搜尋');
    expect(data.hint, '開啟商品搜尋');
    expect(data.flagsCollection.isButton, isTrue);
    expect(data.hasAction(SemanticsAction.tap), isTrue);
    expect(tester.getSize(search).height, 48);
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });
}

Future<void> _pumpSearch(
  WidgetTester tester, {
  required double width,
  required double height,
  required bool readOnly,
  String? hintText = '搜尋番茄、玉荷包、雞蛋、虱目魚與當季蔬菜…',
  String semanticLabel = '商品搜尋',
  String semanticHint = '輸入商品名稱或關鍵字',
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, 844);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  final search = hintText == null
      ? CategorySearchField(
          height: height,
          readOnly: readOnly,
          semanticLabel: semanticLabel,
          semanticHint: semanticHint,
          onTap: () {},
          suffixIcon: const _SearchSuffix(),
        )
      : CategorySearchField(
          height: height,
          hintText: hintText,
          readOnly: readOnly,
          semanticLabel: semanticLabel,
          semanticHint: semanticHint,
          onTap: () {},
          suffixIcon: const _SearchSuffix(),
        );

  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(390, 844),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      builder: (_, __) => MaterialApp(
        theme: ThemeData(useMaterial3: true),
        home: MediaQuery(
          data: MediaQueryData(
            size: Size(width, 844),
            textScaler: const TextScaler.linear(1.4),
          ),
          child: Scaffold(
            body: SafeArea(
              child: Align(
                alignment: Alignment.topCenter,
                child: Padding(
                  padding: const EdgeInsets.symmetric(horizontal: HkSpace.lg),
                  child: ConstrainedBox(
                    constraints: const BoxConstraints(maxWidth: 720),
                    child: search,
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

class _SearchSuffix extends StatelessWidget {
  const _SearchSuffix();

  @override
  Widget build(BuildContext context) => const SizedBox.square(
    dimension: HkControl.minTap,
    child: Center(
      child: HkIcon(HkIconName.close, size: 20, color: AppIconColors.muted),
    ),
  );
}
