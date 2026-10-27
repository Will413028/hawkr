import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/shared/icons/app_icons.dart';
import 'package:hawkr_seller/shared/widgets/chat_input_bar.dart';
import 'package:hawkr_seller/shared/widgets/hk_frosted_icon_button.dart';
import 'package:flutter/material.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:integration_test/integration_test.dart';

/// Native rendering and keyboard smoke for the production shared controls.
/// This isolated fixture never initializes auth, repositories or live APIs.
/// Consumer ownership and request failures are covered by their widget tests.
void main() {
  IntegrationTestWidgetsFlutterBinding.ensureInitialized();

  testWidgets('native more menu opens, cancels and selects once', (
    tester,
  ) async {
    final fixture = await _pumpFixture(tester);
    final more = find.byTooltip('更多操作');
    expect(tester.getSize(more), const Size(48, 48));

    await tester.tap(more);
    await tester.pumpAndSettle();
    expect(find.text('回覆'), findsOneWidget);
    expect(find.text('編輯'), findsOneWidget);
    expect(find.text('刪除'), findsOneWidget);
    expect(fixture.currentState!.selected, isNull);

    // Dismiss by tapping the actual modal barrier, without choosing an action.
    await tester.tapAt(const Offset(20, 240));
    await tester.pumpAndSettle();
    expect(find.text('回覆'), findsNothing);
    expect(fixture.currentState!.selections, 0);

    await tester.tap(more);
    await tester.pumpAndSettle();
    await tester.tap(find.text('編輯'));
    await tester.pumpAndSettle();
    expect(fixture.currentState!.selected, '編輯');
    expect(fixture.currentState!.selections, 1);
    expect(find.byType(PopupMenuItem<String>), findsNothing);
    expect(tester.takeException(), isNull);
  });

  testWidgets(
    'native keyboard send stays visible and busy taps do not repeat',
    (tester) async {
      final fixture = await _pumpFixture(tester);
      final state = fixture.currentState!;
      final send = find.byTooltip('發送訊息');
      expect(tester.getSize(send), const Size(48, 48));
      await tester.tap(send);
      await tester.pump();
      expect(state.sends, 0);

      await tester.tap(find.byType(TextField));
      await _waitForKeyboard(tester);
      await tester.enterText(find.byType(TextField), '原生鍵盤傳送測試');
      await tester.pump();
      expect(state.focus.hasFocus, isTrue);
      final keyboardTop =
          (tester.view.physicalSize.height - tester.view.viewInsets.bottom) /
          tester.view.devicePixelRatio;
      expect(tester.getRect(send).bottom, lessThanOrEqualTo(keyboardTop));

      await tester.tap(send);
      await tester.pump();
      expect(state.sends, 1);
      expect(state.submitted, '原生鍵盤傳送測試');
      expect(state.focus.hasFocus, isTrue);
      expect(find.byType(CircularProgressIndicator), findsOneWidget);
      expect(tester.getSize(send), const Size(48, 48));
      await tester.tap(send);
      await tester.tap(send);
      await tester.pump();
      expect(state.sends, 1);

      state.completeSend();
      await tester.pumpAndSettle();
      expect(state.controller.text, isEmpty);
      await tester.tap(send);
      await tester.pump();
      expect(state.sends, 1);
      expect(tester.takeException(), isNull);
    },
  );

  testWidgets('native attachment tap preserves the active draft and focus', (
    tester,
  ) async {
    final fixture = await _pumpFixture(tester);
    final state = fixture.currentState!;
    await tester.tap(find.byType(TextField));
    await _waitForKeyboard(tester);
    await tester.enterText(find.byType(TextField), '尚未送出的草稿');
    await tester.pump();
    final semantics = tester.ensureSemantics();
    try {
      await tester.pump();
      await tester.tap(find.bySemanticsLabel('新增附件'));
    } finally {
      semantics.dispose();
    }
    await tester.pump();
    expect(state.attachments, 1);
    expect(state.focus.hasFocus, isTrue);
    expect(state.controller.text, '尚未送出的草稿');
    expect(state.sends, 0);
    expect(tester.takeException(), isNull);
  });
}

Future<GlobalKey<_ActionFixtureState>> _pumpFixture(WidgetTester tester) async {
  final fixture = GlobalKey<_ActionFixtureState>();
  await tester.pumpWidget(
    ScreenUtilInit(
      designSize: const Size(390, 844),
      minTextAdapt: true,
      builder: (_, _) => MaterialApp(
        theme: AppTheme.light,
        home: _ActionFixture(key: fixture),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return fixture;
}

Future<void> _waitForKeyboard(WidgetTester tester) async {
  for (var attempt = 0; attempt < 30; attempt++) {
    await tester.pump(const Duration(milliseconds: 100));
    if (tester.view.viewInsets.bottom > 0) return;
  }
  fail('Native software keyboard did not open; check simulator keyboard mode.');
}

class _ActionFixture extends StatefulWidget {
  const _ActionFixture({super.key});

  @override
  State<_ActionFixture> createState() => _ActionFixtureState();
}

class _ActionFixtureState extends State<_ActionFixture> {
  final controller = TextEditingController();
  final focus = FocusNode();
  bool sending = false;
  int sends = 0;
  int sendCallbacks = 0;
  int attachments = 0;
  int selections = 0;
  String? submitted;
  String? selected;

  void _send() {
    sendCallbacks++;
    if (sending || controller.text.trim().isEmpty) return;
    setState(() {
      sending = true;
      sends++;
      submitted = controller.text;
    });
  }

  void completeSend() {
    controller.clear();
    setState(() => sending = false);
  }

  Future<void> _showMore(BuildContext anchor) async {
    final button = anchor.findRenderObject()! as RenderBox;
    final overlay =
        Overlay.of(context).context.findRenderObject()! as RenderBox;
    final origin = button.localToGlobal(Offset.zero, ancestor: overlay);
    final choice = await showMenu<String>(
      context: context,
      position: RelativeRect.fromRect(
        origin & button.size,
        Offset.zero & overlay.size,
      ),
      items: const [
        PopupMenuItem(value: '回覆', child: Text('回覆')),
        PopupMenuItem(value: '編輯', child: Text('編輯')),
        PopupMenuItem(value: '刪除', child: Text('刪除')),
      ],
    );
    if (!mounted || choice == null) return;
    setState(() {
      selected = choice;
      selections++;
    });
  }

  @override
  void dispose() {
    controller.dispose();
    focus.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    body: SafeArea(
      bottom: false,
      child: Column(
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                const Expanded(child: Text('原生互動測試')),
                Builder(
                  builder: (anchor) => HkFrostedIconButton(
                    label: '更多操作',
                    icon: AppIcons.moreHoriz,
                    onPressed: () => _showMore(anchor),
                  ),
                ),
              ],
            ),
          ),
          const Spacer(),
          ChatInputBar(
            controller: controller,
            focusNode: focus,
            onSend: _send,
            onPlusPressed: () => setState(() => attachments++),
            sending: sending,
          ),
        ],
      ),
    ),
  );
}
