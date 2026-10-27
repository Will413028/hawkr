import 'package:hawkr_buyer/shared/widgets/keyboard_dismiss_scope.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('touching outside an input dismisses the mobile keyboard focus', (
    tester,
  ) async {
    final focusNode = FocusNode();
    addTearDown(focusNode.dispose);

    await tester.pumpWidget(
      MaterialApp(
        home: KeyboardDismissScope(
          child: Scaffold(
            body: Column(
              children: [
                TextField(focusNode: focusNode),
                const Expanded(
                  child: ColoredBox(
                    key: ValueKey('keyboard-dismiss-empty-area'),
                    color: Colors.white,
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );

    await tester.tap(find.byType(TextField));
    await tester.pump();
    expect(focusNode.hasFocus, isTrue);

    final emptyArea = tester.getRect(
      find.byKey(const ValueKey('keyboard-dismiss-empty-area')),
    );
    await tester.tapAt(emptyArea.topLeft + const Offset(24, 24));
    await tester.pump();

    expect(focusNode.hasFocus, isFalse);
  });
}
