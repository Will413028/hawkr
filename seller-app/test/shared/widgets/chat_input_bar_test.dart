import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/shared/widgets/chat_input_bar.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

Future<void> _pumpBar(
  WidgetTester tester, {
  required TextEditingController controller,
  required FocusNode focus,
  required VoidCallback onSend,
  bool sending = false,
  Size size = const Size(390, 844),
  double textScale = 1,
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.resetPhysicalSize);
  addTearDown(tester.view.resetDevicePixelRatio);
  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(390, 844),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      builder: (_, _) => MaterialApp(
        theme: AppTheme.light,
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(
            context,
          ).copyWith(textScaler: TextScaler.linear(textScale)),
          child: child!,
        ),
        home: Scaffold(
          body: Align(
            alignment: Alignment.bottomCenter,
            child: ChatInputBar(
              controller: controller,
              focusNode: focus,
              onSend: onSend,
              onPlusPressed: () {},
              sending: sending,
            ),
          ),
        ),
      ),
    ),
  );
  await tester.pump();
}

void main() {
  testWidgets('send validates blank and busy drafts for tap and keyboard', (
    tester,
  ) async {
    final controller = TextEditingController(text: '  ');
    final focus = FocusNode();
    addTearDown(controller.dispose);
    addTearDown(focus.dispose);
    var sent = 0;
    await _pumpBar(
      tester,
      controller: controller,
      focus: focus,
      onSend: () => sent++,
    );
    await tester.tap(find.byTooltip('發送訊息'));
    tester.widget<TextField>(find.byType(TextField)).onSubmitted!('  ');
    expect(sent, 0);

    controller.text = '有內容的訊息';
    await tester.pump();
    await tester.tap(find.byTooltip('發送訊息'));
    expect(sent, 1);

    await _pumpBar(
      tester,
      controller: controller,
      focus: focus,
      onSend: () => sent++,
      sending: true,
    );
    await tester.tap(find.byTooltip('發送訊息'));
    tester.widget<TextField>(find.byType(TextField)).onSubmitted!(
      controller.text,
    );
    expect(sent, 1);
    expect(controller.text, '有內容的訊息');
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });

  testWidgets('desktop send click keeps the composer focus', (tester) async {
    final controller = TextEditingController(text: '詢問商品');
    final focus = FocusNode();
    addTearDown(controller.dispose);
    addTearDown(focus.dispose);
    var sent = 0;
    await _pumpBar(
      tester,
      controller: controller,
      focus: focus,
      onSend: () => sent++,
    );
    focus.requestFocus();
    await tester.pump();
    final pointer = await tester.createGesture(kind: PointerDeviceKind.mouse);
    await pointer.down(tester.getCenter(find.byTooltip('發送訊息')));
    await pointer.up();
    await tester.pump();
    expect(sent, 1);
    expect(focus.hasFocus, isTrue);
  });

  testWidgets('320px multiline composer supports 1.4x text without overflow', (
    tester,
  ) async {
    final controller = TextEditingController(text: '第一行\n第二行\n第三行\n第四行\n第五行');
    final focus = FocusNode();
    addTearDown(controller.dispose);
    addTearDown(focus.dispose);
    await _pumpBar(
      tester,
      controller: controller,
      focus: focus,
      onSend: () {},
      size: const Size(320, 720),
      textScale: 1.4,
    );
    final field = tester.getRect(find.byType(TextField));
    final send = tester.getRect(find.byTooltip('發送訊息'));
    expect(field.right, lessThan(send.left));
    expect(send.size, const Size(48, 48));
    expect(send.right, lessThanOrEqualTo(320));
    expect(tester.takeException(), isNull);
  });
}
