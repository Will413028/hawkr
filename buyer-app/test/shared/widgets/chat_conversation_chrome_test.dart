import 'package:hawkr_buyer/core/theme/app_colors.dart';
import 'package:hawkr_buyer/shared/widgets/chat_conversation_chrome.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  testWidgets('inherits the app nonlinear scaler and keeps large text inline', (
    tester,
  ) async {
    const scaler = _RoleAwareTextScaler();
    const title = '買家小幫手的完整服務名稱';
    const status = 'AI 客服正在提供服務';

    await tester.pumpWidget(
      _TestApp(
        textScaler: scaler,
        child: ChatConversationHeaderContent(
          avatar: const ColoredBox(color: AppColors.primary),
          title: title,
          statusLabel: status,
          statusColor: AppColors.success,
          onBack: _noop,
        ),
      ),
    );

    final titleParagraph = tester.renderObject<RenderParagraph>(
      find.text(title),
    );
    final statusParagraph = tester.renderObject<RenderParagraph>(
      find.text(status),
    );
    expect(titleParagraph.textScaler, same(scaler));
    expect(statusParagraph.textScaler, same(scaler));

    final titleWidget = tester.widget<Text>(find.text(title));
    final statusWidget = tester.widget<Text>(find.text(status));
    expect(titleWidget.maxLines, 1);
    expect(titleWidget.softWrap, isFalse);
    expect(titleWidget.overflow, TextOverflow.ellipsis);
    expect(statusWidget.maxLines, 1);
    expect(statusWidget.softWrap, isFalse);
    expect(statusWidget.overflow, TextOverflow.ellipsis);

    expect(
      (tester.getCenter(find.text(title)).dy -
              tester.getCenter(find.text(status)).dy)
          .abs(),
      lessThan(1),
    );
    expect(tester.takeException(), isNull);
  });

  testWidgets('keeps the two-line identity at the standard text size', (
    tester,
  ) async {
    const scaler = TextScaler.linear(1);

    await tester.pumpWidget(
      _TestApp(
        textScaler: scaler,
        child: const ChatConversationHeaderContent(
          avatar: ColoredBox(color: AppColors.primary),
          title: '買家小幫手',
          statusLabel: 'AI 服務中',
          statusColor: AppColors.success,
          onBack: _noop,
        ),
      ),
    );

    expect(
      tester.getCenter(find.text('買家小幫手')).dy,
      lessThan(tester.getCenter(find.text('AI 服務中')).dy),
    );
    expect(tester.takeException(), isNull);
  });
}

void _noop() {}

final class _TestApp extends StatelessWidget {
  const _TestApp({required this.textScaler, required this.child});

  final TextScaler textScaler;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return ScreenUtilInit(
      designSize: const Size(390, 844),
      enableScaleWH: () => false,
      enableScaleText: () => false,
      minTextAdapt: false,
      builder: (_, __) => MaterialApp(
        home: MediaQuery(
          data: MediaQueryData(
            size: const Size(320, 844),
            textScaler: textScaler,
          ),
          child: Scaffold(
            body: Align(
              alignment: Alignment.topLeft,
              child: SizedBox(width: 320, child: child),
            ),
          ),
        ),
      ),
    );
  }
}

final class _RoleAwareTextScaler extends TextScaler {
  const _RoleAwareTextScaler();

  @override
  double scale(double fontSize) =>
      fontSize <= 12 ? fontSize * 1.4 : fontSize * 2;

  @override
  double get textScaleFactor => 1.4;
}
