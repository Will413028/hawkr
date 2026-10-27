import 'package:hawkr_buyer/features/chat/presentation/chat_focus_policy.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('chat navigation dismisses the previous route keyboard', (
    tester,
  ) async {
    final sourceFocusNode = FocusNode();
    addTearDown(sourceFocusNode.dispose);

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: TextField(focusNode: sourceFocusNode)),
      ),
    );
    await tester.showKeyboard(find.byType(TextField));
    expect(sourceFocusNode.hasFocus, isTrue);
    expect(tester.testTextInput.isVisible, isTrue);

    ChatFocusPolicy.dismissKeyboard();
    await tester.pump();

    expect(sourceFocusNode.hasFocus, isFalse);
    expect(tester.testTextInput.isVisible, isFalse);
  });

  testWidgets('chat composer stays closed on its first destination frame', (
    tester,
  ) async {
    final inputFocusNode = FocusNode();
    addTearDown(inputFocusNode.dispose);

    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(body: TextField(focusNode: inputFocusNode)),
      ),
    );
    await tester.showKeyboard(find.byType(TextField));
    expect(inputFocusNode.hasFocus, isTrue);

    ChatFocusPolicy.keepClosedOnEntry(inputFocusNode);
    await tester.pump();

    expect(inputFocusNode.hasFocus, isFalse);
    expect(tester.testTextInput.isVisible, isFalse);
  });
}
