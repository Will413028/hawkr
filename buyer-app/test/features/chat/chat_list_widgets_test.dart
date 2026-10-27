import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/app_typography.dart';
import 'package:hawkr_buyer/features/chat/presentation/widgets/chat_list_widgets.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in <double>[320, 390, 768]) {
    testWidgets(
      'empty state stays actionable at ${width.toInt()}px and 1.4x text',
      (tester) async {
        var tapped = false;
        final semantics = tester.ensureSemantics();
        try {
          await _pumpSurface(
            tester,
            width: width,
            child: ChatEmptyState(onStartAssistant: () => tapped = true),
          );

          expect(find.text('開始第一段對話'), findsOneWidget);
          expect(find.text('詢問智慧客服'), findsOneWidget);
          expect(find.textContaining('右下角'), findsNothing);
          final action = find.bySemanticsLabel('詢問智慧客服');
          expect(tester.getSize(action).height, greaterThanOrEqualTo(48));

          await tester.tap(action);
          await tester.pump();
          expect(tapped, isTrue);
          expect(tester.takeException(), isNull);
        } finally {
          semantics.dispose();
        }
      },
    );
  }

  testWidgets('search empty state explains the miss and clears the query', (
    tester,
  ) async {
    var cleared = false;
    await _pumpSurface(
      tester,
      width: 320,
      child: ChatSearchEmptyState(
        query: '台東超級無敵長的商家搜尋關鍵字',
        onClear: () => cleared = true,
      ),
    );

    expect(find.textContaining('台東超級無敵長'), findsOneWidget);
    expect(find.text('清除搜尋'), findsOneWidget);
    await tester.tap(find.byKey(const ValueKey('chat-search-clear-action')));
    await tester.pump();
    expect(cleared, isTrue);
    expect(tester.takeException(), isNull);
  });

  testWidgets('empty state is safe inside the production sliver container', (
    tester,
  ) async {
    await _pumpSurface(
      tester,
      width: 390,
      child: CustomScrollView(
        slivers: [
          SliverFillRemaining(
            hasScrollBody: false,
            child: ChatEmptyState(onStartAssistant: () {}),
          ),
        ],
      ),
    );

    expect(find.text('開始第一段對話'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('empty state remains usable at 200 percent text scale', (
    tester,
  ) async {
    await _pumpSurface(
      tester,
      width: 320,
      textScale: 2,
      child: ChatEmptyState(onStartAssistant: () {}),
    );

    expect(find.text('開始第一段對話'), findsOneWidget);
    expect(find.text('詢問智慧客服'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  for (final width in <double>[320, 390, 768]) {
    testWidgets(
      'conversation tile keeps hierarchy at ${width.toInt()}px and 1.4x text',
      (tester) async {
        var tapped = false;
        final semantics = tester.ensureSemantics();
        try {
          await _pumpSurface(
            tester,
            width: width,
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: ChatConversationTile(
                name: '台東山漾食品有限公司超長商家名稱',
                lastMessage: '您好，這是測試用的超長訊息內容，應該安全地省略顯示',
                date: '12分鐘前',
                unread: 140,
                onTap: () => tapped = true,
              ),
            ),
          );

          expect(find.text('99+'), findsOneWidget);
          expect(find.text('12分鐘前'), findsOneWidget);
          final tile = find.bySemanticsLabel(
            RegExp(r'與台東山漾食品有限公司超長商家名稱的對話.*140 則未讀訊息'),
          );
          expect(tester.getSize(tile).height, greaterThanOrEqualTo(84));

          await tester.tap(tile);
          await tester.pump();
          expect(tapped, isTrue);
          expect(tester.takeException(), isNull);
        } finally {
          semantics.dispose();
        }
      },
    );
  }

  testWidgets('assistant shortcut has an explicit accessible label', (
    tester,
  ) async {
    var tapped = false;
    final semantics = tester.ensureSemantics();
    try {
      await _pumpSurface(
        tester,
        width: 390,
        child: Align(
          alignment: Alignment.bottomRight,
          child: ChatAssistantFab(onPressed: () => tapped = true),
        ),
      );

      final action = find.bySemanticsLabel('開啟智慧客服');
      expect(tester.getSize(action).height, greaterThanOrEqualTo(48));
      await tester.tap(action);
      await tester.pump();
      expect(tapped, isTrue);
    } finally {
      semantics.dispose();
    }
  });
}

Future<void> _pumpSurface(
  WidgetTester tester, {
  required double width,
  double textScale = 1.4,
  required Widget child,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = Size(width, width >= 600 ? 900 : 640);
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);

  await tester.pumpWidget(
    MaterialApp(
      theme: ThemeData(
        useMaterial3: true,
        scaffoldBackgroundColor: AppColors.bgPrimary,
        textTheme: AppTypography.textTheme,
      ),
      home: MediaQuery(
        data: MediaQueryData(
          size: Size(width, width >= 600 ? 900 : 640),
          textScaler: TextScaler.linear(textScale),
        ),
        child: Scaffold(body: child),
      ),
    ),
  );
  await tester.pump();
}
