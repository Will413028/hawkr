import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_seller/shared/widgets/rich_text_editor_text_block.dart';

/// 表格互動（issue #1246 對齊 Notion）：
/// - 刪「這一列/欄」以目前聚焦格為錨點（未聚焦 fallback 刪尾）
/// - 格內貼上 tab/多行資料 → 從錨點格展開填入、自動擴列欄
/// 模擬器無法自動點擊，互動行為由本 widget test 覆蓋。
void main() {
  TextBlockData tableBlock() => TextBlockData(
    id: 't1',
    text: '',
    type: BlockType.table,
    tableWithHeadings: true,
    tableCells: [
      [TableCellData('品名'), TableCellData('產地'), TableCellData('備註')],
      [TableCellData('愛文'), TableCellData('台南'), TableCellData('香甜')],
      [TableCellData('金煌'), TableCellData('屏東'), TableCellData('限定')],
    ],
  );

  Future<TextBlockData> pumpTable(WidgetTester tester) async {
    final data = tableBlock();
    await tester.pumpWidget(
      ScreenUtilInit(
        designSize: const Size(390, 844),
        builder: (_, __) => MaterialApp(
          home: Scaffold(
            body: SingleChildScrollView(
              child: TextBlockWidget(
                data: data,
                index: 0,
                isSelected: true,
                onTap: () {},
                onTextChanged: (_) {},
                onDelete: () {},
              ),
            ),
          ),
        ),
      ),
    );
    await tester.pumpAndSettle();
    return data;
  }

  testWidgets('聚焦中間欄 → 刪此欄：刪的是聚焦欄非最後一欄', (tester) async {
    final data = await pumpTable(tester);
    // 點第二欄的格（產地）
    await tester.tap(find.widgetWithText(TextFormField, '產地'));
    await tester.pumpAndSettle();
    expect(find.text('刪此欄'), findsOneWidget);
    await tester.tap(find.text('刪此欄'));
    await tester.pumpAndSettle();
    expect(data.tableCells![0].map((c) => c.text).toList(), ['品名', '備註']);
    expect(data.tableCells![1].map((c) => c.text).toList(), ['愛文', '香甜']);
  });

  testWidgets('聚焦中間列 → 刪此列：刪的是聚焦列非最後一列', (tester) async {
    final data = await pumpTable(tester);
    await tester.tap(find.widgetWithText(TextFormField, '愛文'));
    await tester.pumpAndSettle();
    expect(find.text('刪此列'), findsOneWidget);
    await tester.tap(find.text('刪此列'));
    await tester.pumpAndSettle();
    expect(data.tableCells!.length, 2);
    expect(data.tableCells![1].map((c) => c.text).toList(), ['金煌', '屏東', '限定']);
  });

  testWidgets('未聚焦 → 刪列/刪欄維持刪尾', (tester) async {
    final data = await pumpTable(tester);
    expect(find.text('刪列'), findsOneWidget);
    await tester.tap(find.text('刪列'));
    await tester.pumpAndSettle();
    expect(data.tableCells!.length, 2);
    expect(data.tableCells!.last.first.text, '愛文');
  });

  testWidgets('格內貼上 tab 資料 → 從錨點展開並自動擴列欄', (tester) async {
    final data = await pumpTable(tester);
    // 錨點 = 尾格（限定，r2c2）；貼 2x2 → 擴成 4 列 4 欄
    await tester.tap(find.widgetWithText(TextFormField, '限定'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.widgetWithText(TextFormField, '限定'),
      '甲\t乙\n丙\t丁',
    );
    await tester.pumpAndSettle();
    expect(data.tableCells!.length, 4);
    expect(data.tableCells![0].length, 4);
    expect(data.tableCells![2][2].text, '甲');
    expect(data.tableCells![2][3].text, '乙');
    expect(data.tableCells![3][2].text, '丙');
    expect(data.tableCells![3][3].text, '丁');
  });

  testWidgets('格內一般打字不受貼上攔截影響', (tester) async {
    final data = await pumpTable(tester);
    await tester.enterText(find.widgetWithText(TextFormField, '香甜'), '香甜多汁');
    await tester.pumpAndSettle();
    expect(data.tableCells![1][2].text, '香甜多汁');
  });
}
