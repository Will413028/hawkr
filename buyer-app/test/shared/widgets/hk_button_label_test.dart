import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  const longLabel = '這是一段刻意超過按鈕寬度但絕對不能換行的完整操作文案';

  for (final scale in <double>[0.9, 1, 1.15, 1.4]) {
    testWidgets('keeps a button label on one line at ${scale}x text', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();

      await tester.pumpWidget(
        MaterialApp(
          builder: (context, child) => MediaQuery(
            data: MediaQuery.of(
              context,
            ).copyWith(textScaler: TextScaler.linear(scale)),
            child: child!,
          ),
          theme: ThemeData(
            filledButtonTheme: FilledButtonThemeData(
              style: FilledButton.styleFrom(minimumSize: const Size(48, 48)),
            ),
          ),
          home: Scaffold(
            body: Center(
              child: SizedBox(
                width: 144,
                child: FilledButton(
                  onPressed: _noop,
                  child: const HkButtonLabel(longLabel),
                ),
              ),
            ),
          ),
        ),
      );

      expect(tester.takeException(), isNull);
      final text = tester.widget<Text>(find.text(longLabel));
      expect(text.maxLines, 1);
      expect(text.softWrap, isFalse);
      expect(text.overflow, TextOverflow.ellipsis);

      final paragraph = tester.renderObject<RenderParagraph>(
        find.text(longLabel),
      );
      expect(paragraph.didExceedMaxLines, isTrue);
      expect(tester.getSize(find.byType(FilledButton)).height, 48);
      expect(find.bySemanticsLabel(longLabel), findsOneWidget);
      semantics.dispose();
    });
  }
}

void _noop() {}
