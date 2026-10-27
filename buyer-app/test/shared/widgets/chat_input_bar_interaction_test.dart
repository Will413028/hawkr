import 'package:hawkr_buyer/core/theme/app_theme.dart';
import 'package:hawkr_buyer/shared/widgets/chat_input_bar.dart';
import 'package:hawkr_buyer/shared/widgets/hk_frosted_icon_button.dart';
import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final width in [320.0, 390.0]) {
    testWidgets('${width.toInt()}px大字級的附件與送出可點，滑鼠按下保留焦點，空白和忙碌不送', (
      tester,
    ) async {
      tester.view.devicePixelRatio = 1;
      tester.view.physicalSize = Size(width, 844);
      addTearDown(tester.view.resetDevicePixelRatio);
      addTearDown(tester.view.resetPhysicalSize);
      final controller = TextEditingController();
      final focus = FocusNode();
      addTearDown(controller.dispose);
      addTearDown(focus.dispose);
      var sent = <String>[];
      var attachments = 0;
      var sending = false;
      late StateSetter update;
      await tester.pumpWidget(
        ScreenUtilInit(
          designSize: const Size(390, 844),
          enableScaleWH: () => false,
          enableScaleText: () => false,
          builder: (_, __) => MaterialApp(
            theme: AppTheme.light,
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(
                context,
              ).copyWith(textScaler: const TextScaler.linear(1.4)),
              child: child!,
            ),
            home: Scaffold(
              body: Align(
                alignment: Alignment.bottomCenter,
                child: StatefulBuilder(
                  builder: (context, setState) {
                    update = setState;
                    return ChatInputBar(
                      controller: controller,
                      focusNode: focus,
                      sending: sending,
                      onPlusPressed: () => attachments++,
                      onSend: () {
                        sent.add(controller.text);
                        controller.clear();
                      },
                    );
                  },
                ),
              ),
            ),
          ),
        ),
      );
      final input = find.byType(TextField);
      final send = find.byType(HkFrostedIconButton);
      await tester.enterText(input, '   ');
      await tester.pump();
      await tester.tap(send);
      await tester.testTextInput.receiveAction(TextInputAction.send);
      expect(sent, isEmpty);
      await tester.enterText(input, '第一筆');
      await tester.pump();
      final point = tester.getCenter(send);
      final gesture = await tester.startGesture(
        point,
        kind: PointerDeviceKind.mouse,
      );
      await tester.pump();
      expect(focus.hasFocus, isTrue);
      expect(tester.getCenter(send), point);
      await gesture.up();
      await tester.pump();
      expect(sent, ['第一筆']);
      await tester.tap(find.bySemanticsLabel('新增附件'));
      await tester.pump();
      expect(attachments, 1);
      await tester.enterText(input, '保留下一筆');
      update(() => sending = true);
      await tester.pump();
      await tester.tap(send);
      await tester.testTextInput.receiveAction(TextInputAction.send);
      expect(sent, ['第一筆']);
      expect(controller.text, '保留下一筆');
      update(() => sending = false);
      await tester.pump();
      await tester.showKeyboard(input);
      await tester.testTextInput.receiveAction(TextInputAction.send);
      await tester.pump();
      expect(sent, ['第一筆', '保留下一筆']);
      expect(tester.takeException(), isNull);
    });
  }
}
