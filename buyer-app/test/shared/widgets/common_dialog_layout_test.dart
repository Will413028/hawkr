import 'package:hawkr_buyer/shared/widgets/common_dialog.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final viewport in <Size>[const Size(320, 568), const Size(844, 390)]) {
    testWidgets(
      'dialog stays scrollable and actions stay single-line at $viewport',
      (tester) async {
        tester.view.devicePixelRatio = 1;
        tester.view.physicalSize = viewport;
        addTearDown(tester.view.resetDevicePixelRatio);
        addTearDown(tester.view.resetPhysicalSize);

        await tester.pumpWidget(
          MaterialApp(
            builder: (context, child) => MediaQuery(
              data: MediaQuery.of(context).copyWith(
                padding: viewport.width > viewport.height
                    ? const EdgeInsets.fromLTRB(59, 0, 59, 21)
                    : EdgeInsets.zero,
                textScaler: const TextScaler.linear(1.4),
              ),
              child: child!,
            ),
            home: const Scaffold(
              body: CommonDialog(
                title: '這是一段較長的確認標題，用來驗證窄螢幕仍能正常閱讀',
                content:
                    '重要操作說明會保留完整內容並允許捲動。'
                    '即使畫面高度很短，也不能讓下方操作按鈕離開可視範圍。'
                    '這段文字刻意加長，以涵蓋橫向手機與大字級的組合。',
                cancelText: '暫時不要執行這項操作',
                confirmText: '確認並繼續執行這項操作',
              ),
            ),
          ),
        );

        expect(tester.takeException(), isNull);
        expect(find.byType(SingleChildScrollView), findsOneWidget);
        expect(find.byType(HkButtonLabel), findsNWidgets(2));

        for (final label in tester.widgetList<Text>(
          find.descendant(
            of: find.byType(HkButtonLabel),
            matching: find.byType(Text),
          ),
        )) {
          expect(label.maxLines, 1);
          expect(label.softWrap, isFalse);
          expect(label.overflow, TextOverflow.ellipsis);
        }

        for (final button in find.byType(TextButton).evaluate()) {
          expect(tester.getSize(find.byWidget(button.widget)).height, 48);
        }

        final dialogRect = tester.getRect(find.byType(Dialog));
        final actionsRect = tester.getRect(find.byType(TextButton).first);
        expect(actionsRect.bottom, lessThanOrEqualTo(dialogRect.bottom));
      },
    );
  }
}
