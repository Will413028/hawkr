import 'dart:ui' show SemanticsAction, SemanticsActionEvent;

import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/shared/icons/app_icons.dart';
import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

Widget _app(Widget child) => ScreenUtilInit(
  designSize: const Size(390, 844),
  enableScaleWH: () => false,
  enableScaleText: () => false,
  minTextAdapt: false,
  builder: (_, _) => MaterialApp(
    theme: AppTheme.light,
    home: Scaffold(body: Center(child: child)),
  ),
);

void main() {
  testWidgets('more action has one accessible target and keyboard activation', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    var calls = 0;
    try {
      await tester.pumpWidget(
        _app(
          HkFrostedIconButton(
            label: '更多選項',
            icon: AppIcons.moreHoriz,
            onPressed: () => calls++,
          ),
        ),
      );
      await tester.pump();
      expect(find.bySemanticsLabel('更多選項'), findsOneWidget);
      await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
      await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
      await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
      await tester.tap(find.byTooltip('更多選項'));
      expect(calls, 1);
      await tester.sendKeyEvent(LogicalKeyboardKey.tab);
      await tester.pump();
      await tester.sendKeyEvent(LogicalKeyboardKey.enter);
      expect(calls, 2);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets('loading and disabled actions suppress taps without moving', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    var calls = 0;
    var loading = false;
    var enabled = true;
    late StateSetter update;
    try {
      await tester.pumpWidget(
        _app(
          StatefulBuilder(
            builder: (context, setState) {
              update = setState;
              return HkFrostedIconButton(
                label: '送出留言',
                icon: AppIcons.send,
                semanticHint: '送出目前輸入的留言',
                loading: loading,
                loadingLabel: '留言傳送中',
                onPressed: enabled
                    ? () {
                        calls++;
                        setState(() => loading = true);
                      }
                    : null,
              );
            },
          ),
        ),
      );
      final action = find.byType(HkFrostedIconButton);
      final before = tester.getRect(action);
      expect(before.size, const Size.square(48));
      await tester.tap(action);
      await tester.pump();
      expect(calls, 1);
      expect(find.bySemanticsLabel('留言傳送中'), findsOneWidget);
      expect(tester.getRect(action), before);
      await tester.tap(action);
      expect(calls, 1);
      update(() {
        loading = false;
        enabled = false;
      });
      await tester.pump();
      expect(tester.getRect(action), before);
      expect(find.byType(HkBrandLoader), findsNothing);
      expect(
        tester.widget<IconButton>(find.byType(IconButton)).onPressed,
        isNull,
      );
      await tester.tap(action);
      expect(calls, 1);
      expect(tester.takeException(), isNull);
    } finally {
      semantics.dispose();
    }
  });

  testWidgets(
    'whole visible circle is clickable without triggering its parent',
    (tester) async {
      var calls = 0;
      var parentCalls = 0;
      await tester.pumpWidget(
        _app(
          GestureDetector(
            behavior: HitTestBehavior.opaque,
            onTap: () => parentCalls++,
            child: Padding(
              padding: const EdgeInsets.all(24),
              child: HkFrostedIconButton(
                label: '更多選項',
                icon: AppIcons.moreHoriz,
                onPressed: () => calls++,
              ),
            ),
          ),
        ),
      );
      final rect = tester.getRect(find.byType(HkFrostedIconButton));
      const offsets = [
        Offset.zero,
        Offset(22, 0),
        Offset(-22, 0),
        Offset(0, 22),
        Offset(0, -22),
        Offset(15, 15),
        Offset(-15, 15),
        Offset(15, -15),
        Offset(-15, -15),
      ];
      for (var i = 0; i < offsets.length; i++) {
        await tester.tapAt(rect.center + offsets[i]);
        await tester.pump();
        expect(calls, i + 1, reason: 'visible circle at ${offsets[i]}');
        expect(parentCalls, 0);
      }
      await tester.tapAt(rect.center + const Offset(30, 0));
      expect(calls, offsets.length);
      expect(parentCalls, 1);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'pointer cancel and becoming busy before release do not activate',
    (tester) async {
      var calls = 0;
      var loading = false;
      late StateSetter update;
      await tester.pumpWidget(
        _app(
          StatefulBuilder(
            builder: (context, setState) {
              update = setState;
              return HkFrostedIconButton(
                label: '送出留言',
                icon: AppIcons.send,
                loading: loading,
                onPressed: () => calls++,
              );
            },
          ),
        ),
      );
      final center = tester.getCenter(find.byType(HkFrostedIconButton));
      final cancelled = await tester.startGesture(center);
      expect(calls, 0);
      await cancelled.cancel();
      await tester.pump();
      expect(calls, 0);

      final interrupted = await tester.startGesture(center);
      update(() => loading = true);
      await tester.pump();
      await interrupted.up();
      await tester.pump();
      expect(calls, 0);
      update(() => loading = false);
      await tester.pump();
      await tester.tap(find.byType(HkFrostedIconButton));
      expect(calls, 1);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets(
    'assistive tap and space activate once and disabled state blocks both',
    (tester) async {
      final semantics = tester.ensureSemantics();
      var calls = 0;
      var enabled = true;
      late StateSetter update;
      try {
        await tester.pumpWidget(
          _app(
            StatefulBuilder(
              builder: (context, setState) {
                update = setState;
                return HkFrostedIconButton(
                  label: '更多選項',
                  icon: AppIcons.moreHoriz,
                  onPressed: enabled ? () => calls++ : null,
                );
              },
            ),
          ),
        );
        await tester.pump();
        final node = tester.getSemantics(find.bySemanticsLabel('更多選項'));
        tester.binding.performSemanticsAction(
          SemanticsActionEvent(
            type: SemanticsAction.tap,
            viewId: tester.view.viewId,
            nodeId: node.id,
          ),
        );
        expect(calls, 1);
        await tester.sendKeyEvent(LogicalKeyboardKey.tab);
        await tester.pump();
        await tester.sendKeyEvent(LogicalKeyboardKey.space);
        expect(calls, 2);
        update(() => enabled = false);
        await tester.pump();
        final disabled = tester
            .getSemantics(find.bySemanticsLabel('更多選項'))
            .getSemanticsData();
        expect(disabled.hasAction(SemanticsAction.tap), isFalse);
        await tester.sendKeyEvent(LogicalKeyboardKey.space);
        await tester.sendKeyEvent(LogicalKeyboardKey.enter);
        expect(calls, 2);
        expect(tester.takeException(), isNull);
      } finally {
        semantics.dispose();
      }
    },
  );
}
