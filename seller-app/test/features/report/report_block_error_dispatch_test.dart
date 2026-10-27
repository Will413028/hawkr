import 'package:hawkr_seller/core/exceptions/app_exception.dart';
import 'package:hawkr_seller/core/exceptions/report_block_codes.dart';
import 'package:hawkr_seller/features/report/presentation/report_block_error_dispatch.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

/// 測試用 trigger widget：把 [handleReportBlockApiError] 呼叫延後到
/// onPressed（build 完成後）才執行。
///
/// 不能在 [Builder.builder] 內直接呼叫（如 brief 原始範例），因為
/// `FloatingToast.error` 最終會呼叫 `OverlayState.insert` 觸發
/// `setState()`——在 widget tree 還在 build 階段時呼叫 setState 會被
/// framework 判為非法（"setState() or markNeedsBuild() called during
/// build"）。實際 call site（按鈕 onPressed / catch block）本來就是
/// build 完成後才執行，這裡改用按鈕觸發以貼近真實使用情境並避開此限制。
class _Trigger extends StatelessWidget {
  const _Trigger({required this.onPressed});

  final VoidCallback onPressed;

  @override
  Widget build(BuildContext context) {
    return ElevatedButton(onPressed: onPressed, child: const Text('trigger'));
  }
}

void main() {
  test('ReportBlockCode 常數對應 backend-payuni#645 bizcode', () {
    expect(ReportBlockCode.blockedMessageNotAllowed, 9202);
    expect(ReportBlockCode.moderationRejected, 9210);
  });

  group('handleReportBlockApiError', () {
    testWidgets('非 ApiException 回傳 false', (tester) async {
      late bool result;
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) {
              return _Trigger(
                onPressed: () {
                  result = handleReportBlockApiError(
                    context,
                    Exception('other'),
                  );
                },
              );
            },
          ),
        ),
      );
      await tester.tap(find.byType(ElevatedButton));
      await tester.pump();
      expect(result, isFalse);
    });

    testWidgets('9202 blockedMessageNotAllowed 命中並顯示固定文案', (tester) async {
      late bool result;
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) {
              return _Trigger(
                onPressed: () {
                  result = handleReportBlockApiError(
                    context,
                    const ApiException(
                      code: ReportBlockCode.blockedMessageNotAllowed,
                      message: '已被封鎖',
                    ),
                  );
                },
              );
            },
          ),
        ),
      );
      await tester.tap(find.byType(ElevatedButton));
      await tester.pump();
      expect(result, isTrue);
      expect(find.text('對方已將你封鎖，無法傳送訊息'), findsOneWidget);
      // FloatingToast 排了 2 秒後自動關閉的 Timer，測試結束前要讓它跑完，
      // 否則 flutter_test 會斷言「timer still pending after dispose」。
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('9210 moderationRejected 命中並顯示後端訊息', (tester) async {
      late bool result;
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) {
              return _Trigger(
                onPressed: () {
                  result = handleReportBlockApiError(
                    context,
                    const ApiException(
                      code: ReportBlockCode.moderationRejected,
                      message: '含敏感詞',
                    ),
                  );
                },
              );
            },
          ),
        ),
      );
      await tester.tap(find.byType(ElevatedButton));
      await tester.pump();
      expect(result, isTrue);
      expect(find.text('含敏感詞'), findsOneWidget);
      await tester.pump(const Duration(seconds: 3));
    });

    testWidgets('未知業務碼回傳 false', (tester) async {
      late bool result;
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (context) {
              return _Trigger(
                onPressed: () {
                  result = handleReportBlockApiError(
                    context,
                    const ApiException(code: 4090, message: 'other'),
                  );
                },
              );
            },
          ),
        ),
      );
      await tester.tap(find.byType(ElevatedButton));
      await tester.pump();
      expect(result, isFalse);
    });
  });
}
