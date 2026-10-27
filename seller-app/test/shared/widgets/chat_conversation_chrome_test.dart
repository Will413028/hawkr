import 'package:hawkr_seller/shared/widgets/chat_conversation_chrome.dart';
import 'package:hawkr_seller/shared/widgets/hk_glass_chrome_surface.dart';
import 'package:hawkr_seller/shared/widgets/hk_icon.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('chat top bar matches buyer chrome and remains accessible', (
    tester,
  ) async {
    var didBack = false;
    final semantics = tester.ensureSemantics();

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(393, 852),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          home: Scaffold(
            body: ChatConversationTopBar(
              avatar: const ColoredBox(color: Colors.green),
              title: '一起吃水果',
              statusLabel: '線上',
              statusColor: Colors.green,
              onBack: () => didBack = true,
            ),
          ),
        ),
      ),
    );

    expect(find.byType(HkGlassChromeSurface), findsOneWidget);
    expect(find.byType(HkIcon), findsOneWidget);
    expect(find.text('一起吃水果'), findsOneWidget);
    expect(find.text('線上'), findsOneWidget);
    expect(find.bySemanticsLabel('返回'), findsOneWidget);

    await tester.tap(find.byTooltip('返回'));
    expect(didBack, isTrue);
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });

  testWidgets('chat chrome inherits nonlinear text and stays compact', (
    tester,
  ) async {
    const scaler = _RoleAwareTextScaler();
    const title = '非常長的商家名稱也不能讓聊天室標題溢位';
    const status = 'AI 服務中・訂單、出貨與金流';
    tester.view.physicalSize = const Size(320, 720);
    tester.view.devicePixelRatio = 1;
    addTearDown(tester.view.resetPhysicalSize);
    addTearDown(tester.view.resetDevicePixelRatio);

    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(393, 852),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          home: MediaQuery(
            data: const MediaQueryData(textScaler: scaler),
            child: Scaffold(
              body: Column(
                children: [
                  ChatConversationTopBar(
                    avatar: const ColoredBox(color: Colors.green),
                    title: title,
                    statusLabel: status,
                    statusColor: Colors.green,
                    onBack: () {},
                  ),
                  const Expanded(child: ChatConversationDotPattern()),
                ],
              ),
            ),
          ),
        ),
      ),
    );

    expect(find.byType(ChatConversationDotPattern), findsOneWidget);
    final titleParagraph = tester.renderObject<RenderParagraph>(
      find.text(title),
    );
    final statusParagraph = tester.renderObject<RenderParagraph>(
      find.text(status),
    );
    expect(titleParagraph.textScaler, same(scaler));
    expect(statusParagraph.textScaler, same(scaler));

    final titleWidget = tester.widget<Text>(find.text(title));
    final statusWidget = tester.widget<Text>(find.text(status));
    expect(titleWidget.maxLines, 1);
    expect(titleWidget.softWrap, isFalse);
    expect(titleWidget.overflow, TextOverflow.ellipsis);
    expect(statusWidget.maxLines, 1);
    expect(statusWidget.softWrap, isFalse);
    expect(statusWidget.overflow, TextOverflow.ellipsis);
    expect(
      (tester.getCenter(find.text(title)).dy -
              tester.getCenter(find.text(status)).dy)
          .abs(),
      lessThan(1),
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('chat chrome keeps the stacked identity at standard text', (
    tester,
  ) async {
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(393, 852),
        enableScaleWH: () => false,
        enableScaleText: () => false,
        builder: (_, __) => MaterialApp(
          home: Scaffold(
            body: ChatConversationHeaderContent(
              avatar: const ColoredBox(color: Colors.green),
              title: '一起吃水果',
              statusLabel: '線上',
              statusColor: Colors.green,
              onBack: () {},
            ),
          ),
        ),
      ),
    );

    expect(
      tester.getCenter(find.text('一起吃水果')).dy,
      lessThan(tester.getCenter(find.text('線上')).dy),
    );
    expect(tester.takeException(), isNull);
  });
}

final class _RoleAwareTextScaler extends TextScaler {
  const _RoleAwareTextScaler();

  @override
  double scale(double fontSize) =>
      fontSize <= 12 ? fontSize * 1.4 : fontSize * 2;

  @override
  double get textScaleFactor => 1.4;
}
