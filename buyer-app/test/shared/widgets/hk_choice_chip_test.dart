import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/core/theme/hk_tokens.dart';
import 'package:hawkr_buyer/shared/widgets/hk_choice_chip.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

Future<void> _pump(WidgetTester tester, {required bool selected}) {
  return tester.pumpWidget(
    MaterialApp(
      home: Scaffold(
        body: Center(
          child: HkChoiceChip(label: '最新上架', selected: selected, onTap: () {}),
        ),
      ),
    ),
  );
}

/// 實際上色的膠囊（32dp），外層 48dp 只是觸控範圍。
final _pill = find.descendant(
  of: find.byType(HkChoiceChip),
  matching: find.byType(AnimatedContainer),
);

TextStyle? _renderedStyle(WidgetTester tester) =>
    tester.renderObject<RenderParagraph>(find.text('最新上架')).text.style;

void main() {
  testWidgets('uses one compact visual spec with an accessible tap target', (
    tester,
  ) async {
    await _pump(tester, selected: false);

    final chip = find.byType(HkChoiceChip);
    expect(tester.getSize(chip).height, HkControl.minTap);
    // 網頁設計系統 sm 控制項階梯：32px 高、12px 字。
    expect(tester.getSize(_pill).height, HkControl.compactChoiceHeight);
    expect(tester.getSize(_pill).height, 32);
    expect(_renderedStyle(tester)?.fontSize, 12);
  });

  testWidgets('未選中吃淡色實底而不是描邊', (tester) async {
    await _pump(tester, selected: false);

    final decoration =
        tester.widget<AnimatedContainer>(_pill).decoration as ShapeDecoration;
    expect(decoration.color, AppColors.gray100);
    expect(decoration.shape, const StadiumBorder());
  });

  testWidgets('切換選取不改變寬度，Wrap 版面才不會重排', (tester) async {
    await _pump(tester, selected: false);
    final unselected = tester.getSize(find.byType(HkChoiceChip));
    expect(_renderedStyle(tester)?.fontWeight, FontWeight.w500);

    await _pump(tester, selected: true);
    await tester.pumpAndSettle();

    expect(tester.getSize(find.byType(HkChoiceChip)), unselected);
    // 字重不跳（跳了寬度就會變）；選取狀態靠實底與反白文字表達。
    expect(_renderedStyle(tester)?.fontWeight, FontWeight.w500);
    expect(_renderedStyle(tester)?.color, AppColors.textInverse);
  });

  testWidgets('整個 48dp 範圍都能點，不是只有膠囊本身', (tester) async {
    var taps = 0;
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: Align(
            alignment: Alignment.topLeft,
            child: HkChoiceChip(
              label: '最新上架',
              selected: false,
              onTap: () => taps++,
            ),
          ),
        ),
      ),
    );

    final rect = tester.getRect(find.byType(HkChoiceChip));
    expect(rect.height, greaterThanOrEqualTo(HkControl.minTap));
    // 膠囊只有 32dp 高並置中，上下各留 8dp。那 8dp 以前不會反應。
    for (final point in [
      Offset(rect.center.dx, rect.top + 2),
      rect.center,
      Offset(rect.center.dx, rect.bottom - 2),
    ]) {
      await tester.tapAt(point);
      await tester.pump();
    }
    expect(taps, 3);
  });

  testWidgets('文字在膠囊裡垂直置中，且寬度仍收斂成膠囊', (tester) async {
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: SizedBox(
            width: 400,
            child: Wrap(
              children: [
                HkChoiceChip(label: '最新上架', selected: false, onTap: () {}),
              ],
            ),
          ),
        ),
      ),
    );

    // 固定高度會把 tight constraint 傳到 Text，少了 Align 文字會被釘在頂端。
    final pill = tester.getRect(_pill);
    final text = tester.getRect(find.text('最新上架'));
    expect(text.center.dy, moreOrLessEquals(pill.center.dy, epsilon: 0.5));

    // Container 設了 alignment 會撐滿整行，Wrap 就變成一行一顆。
    expect(tester.getSize(find.byType(HkChoiceChip)).width, lessThan(200));
  });
}
