import 'package:hawkr_seller/features/customer/presentation/customer_detail_screen.dart';
import 'package:hawkr_seller/features/order/presentation/call_tcat_dialog.dart';
import 'package:hawkr_seller/features/photo_editor/presentation/image_editor_color_picker_dialog.dart';
import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:hawkr_seller/shared/widgets/dialog_close_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('dialog close keeps its compact icon inside one 48dp target', (
    tester,
  ) async {
    var tapped = false;
    final semantics = tester.ensureSemantics();
    tester.view.physicalSize = const Size(390, 844);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => MaterialApp(
          home: Scaffold(
            body: Center(
              child: DialogCloseButton(
                semanticLabel: '關閉標籤管理',
                onTap: () => tapped = true,
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.byType(HkTapTarget), findsOneWidget);
    expect(find.bySemanticsLabel('關閉標籤管理'), findsOneWidget);
    expect(find.byTooltip('關閉標籤管理'), findsOneWidget);
    final targetSize = tester.getSize(find.byType(DialogCloseButton));
    final visualSize = tester.getSize(
      find.descendant(
        of: find.byType(DialogCloseButton),
        matching: find.byType(Container),
      ),
    );
    expect(targetSize.width, greaterThanOrEqualTo(48));
    expect(targetSize.height, greaterThanOrEqualTo(48));
    expect(visualSize.width, 32);
    expect(visualSize.height, 32);
    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));

    await tester.tap(find.bySemanticsLabel('關閉標籤管理'));
    expect(tapped, isTrue);
    semantics.dispose();
  });

  testWidgets(
    'negative-offset production dialog keeps the original top-right visual rect',
    (tester) async {
      await _pumpCompactLargeText(tester, const CallTcatDialog());

      final close = find.byType(DialogCloseButton);
      final stackRect = _closestAncestorRect<Stack>(tester, close);
      final targetRect = tester.getRect(close);
      final visualRect = tester.getRect(_closeVisual(close));

      _expectAccessibleTarget(targetRect);
      expect(
        MediaQuery.textScalerOf(tester.element(close)).scale(10),
        closeTo(14, 0.001),
      );
      expect(targetRect.top, closeTo(stackRect.top - 8.h, 0.01));
      expect(targetRect.right, closeTo(stackRect.right + 8.w, 0.01));
      expect(visualRect.top, closeTo(targetRect.top, 0.01));
      expect(visualRect.right, closeTo(targetRect.right, 0.01));
      expect(visualRect.width, closeTo(32.w, 0.01));
      expect(visualRect.height, closeTo(32.w, 0.01));
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'customer tag title stack keeps its close visual anchored to the right',
    (tester) async {
      await _pumpCompactLargeText(
        tester,
        Align(
          alignment: Alignment.topLeft,
          child: Padding(
            padding: const EdgeInsets.symmetric(horizontal: 20),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [CustomerTagManagementHeader(onClose: _noop)],
            ),
          ),
        ),
      );

      final close = find.byType(DialogCloseButton);
      final stackRect = _closestAncestorRect<Stack>(tester, close);
      final targetRect = tester.getRect(close);
      final visualRect = tester.getRect(_closeVisual(close));

      _expectAccessibleTarget(targetRect);
      expect(
        MediaQuery.textScalerOf(tester.element(close)).scale(10),
        closeTo(14, 0.001),
      );
      expect(targetRect.right, closeTo(stackRect.right, 0.01));
      expect(visualRect.right, closeTo(stackRect.right, 0.01));
      expect(visualRect.center.dy, closeTo(stackRect.center.dy, 0.01));
      expect(visualRect.width, closeTo(32.w, 0.01));
      expect(visualRect.height, closeTo(32.w, 0.01));
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'production color picker keeps the compact close visual right-aligned',
    (tester) async {
      await _pumpCompactLargeText(
        tester,
        const ColorPickerDialog(initialColor: Colors.teal),
      );

      final close = find.byType(DialogCloseButton);
      final rowRect = _closestAncestorRect<Row>(tester, close);
      final targetRect = tester.getRect(close);
      final visualRect = tester.getRect(_closeVisual(close));

      _expectAccessibleTarget(targetRect);
      expect(
        MediaQuery.textScalerOf(tester.element(close)).scale(10),
        closeTo(14, 0.001),
      );
      expect(targetRect.right, closeTo(rowRect.right, 0.01));
      expect(visualRect.right, closeTo(rowRect.right, 0.01));
      expect(visualRect.center.dy, closeTo(rowRect.center.dy, 0.01));
      expect(visualRect.width, closeTo(28.w, 0.01));
      expect(visualRect.height, closeTo(28.w, 0.01));
      expect(tester.takeException(), isNull);
    },
  );
}

Finder _closeVisual(Finder close) =>
    find.descendant(of: close, matching: find.byType(Container));

void _expectAccessibleTarget(Rect targetRect) {
  expect(targetRect.width, greaterThanOrEqualTo(48));
  expect(targetRect.height, greaterThanOrEqualTo(48));
}

Rect _closestAncestorRect<T extends Widget>(
  WidgetTester tester,
  Finder descendant,
) {
  RenderBox? ancestorBox;
  tester.element(descendant).visitAncestorElements((element) {
    if (element.widget is T) {
      final renderObject = element.findRenderObject();
      if (renderObject is RenderBox) ancestorBox = renderObject;
      return false;
    }
    return true;
  });
  expect(ancestorBox, isNotNull, reason: 'missing ancestor of type $T');
  final box = ancestorBox!;
  return box.localToGlobal(Offset.zero) & box.size;
}

Future<void> _pumpCompactLargeText(WidgetTester tester, Widget child) async {
  tester.view.physicalSize = const Size(320, 844);
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(390, 844),
      builder: (_, __) => MaterialApp(
        builder: (context, appChild) => MediaQuery(
          data: MediaQuery.of(
            context,
          ).copyWith(textScaler: const TextScaler.linear(1.4)),
          child: appChild!,
        ),
        home: Scaffold(body: child),
      ),
    ),
  );
  await tester.pump();
}

void _noop() {}
