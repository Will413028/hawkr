import 'package:hawkr_seller/shared/widgets/hk_tap_target.dart';
import 'package:hawkr_seller/shared/widgets/rich_text_editor_text_block.dart';
import 'package:hawkr_seller/shared/widgets/rich_text_editor_toolbar.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const labels = <String>[
    '新增內容區塊',
    '粗體',
    '斜體',
    '底線',
    '插入連結',
    '文字顏色',
    '靠左對齊',
    '置中對齊',
    '靠右對齊',
    '文字樣式',
    '插入圖片',
    '插入表格',
    '插入項目符號清單',
    '插入編號清單',
    '撤銷',
    '重做',
    '隱藏鍵盤',
  ];

  Future<void> pumpToolbar(WidgetTester tester) async {
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => MaterialApp(
          home: Scaffold(
            body: Align(
              alignment: Alignment.bottomCenter,
              child: EditorToolbar(
                onAddBlock: () {},
                onBold: () {},
                onItalic: () {},
                onUnderline: () {},
                onLink: () {},
                onColorPicker: () {},
                onAlignLeft: () {},
                onAlignCenter: () {},
                onAlignRight: () {},
                onTextStyle: () {},
                onInsertImage: () {},
                onInsertTable: () {},
                onInsertBulletList: () {},
                onInsertNumberedList: () {},
                onUndo: () {},
                isBoldActive: true,
                isColorActive: true,
                currentLength: 18,
                maxLength: 5000,
                selectedStyle: const TextBlockStyle(
                  alignment: TextAlign.center,
                ),
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pump();
  }

  testWidgets('all 17 toolbar controls have one labeled 48dp target', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await pumpToolbar(tester);

    expect(find.byType(HkTapTarget), findsNWidgets(labels.length));
    for (var index = 0; index < labels.length; index++) {
      expect(
        find.bySemanticsLabel(labels[index]),
        findsOneWidget,
        reason: 'missing or duplicate semantic label: ${labels[index]}',
      );
      final size = tester.getSize(find.byType(HkTapTarget).at(index));
      expect(
        size.width,
        greaterThanOrEqualTo(48),
        reason: '${labels[index]} width',
      );
      expect(
        size.height,
        greaterThanOrEqualTo(48),
        reason: '${labels[index]} height',
      );
    }

    expect(
      tester.getSemantics(find.bySemanticsLabel('粗體')),
      isSemantics(
        label: '粗體',
        hasToggledState: true,
        isToggled: true,
        isButton: true,
        hasEnabledState: true,
        isEnabled: true,
        hasTapAction: true,
      ),
    );
    expect(
      tester.getSemantics(find.bySemanticsLabel('置中對齊')),
      isSemantics(
        label: '置中對齊',
        hasSelectedState: true,
        isSelected: true,
        isButton: true,
        hasEnabledState: true,
        isEnabled: true,
        hasTapAction: true,
      ),
    );
    expect(
      tester.getSemantics(find.bySemanticsLabel('重做')),
      isSemantics(
        label: '重做',
        isButton: true,
        hasEnabledState: true,
        isEnabled: false,
        hasTapAction: false,
      ),
    );
    semantics.dispose();
  });

  testWidgets('toolbar passes Android, iOS, and labeled-target guidelines', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await pumpToolbar(tester);

    await expectLater(tester, meetsGuideline(androidTapTargetGuideline));
    await expectLater(tester, meetsGuideline(iOSTapTargetGuideline));
    await expectLater(tester, meetsGuideline(labeledTapTargetGuideline));
    semantics.dispose();
  });
}
