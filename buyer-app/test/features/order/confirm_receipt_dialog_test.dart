import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:hawkr_buyer/core/networking/dio_provider.dart';
import 'package:hawkr_buyer/features/order/presentation/confirm_receipt_dialog.dart';
import 'package:hawkr_buyer/shared/widgets/hk_button_label.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:hawkr_buyer/shared/widgets/hk_brand_loader.dart';

void main() {
  testWidgets('320px large text keeps both receipt actions separated', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await _openDialog(tester, size: const Size(320, 568));

    expect(tester.takeException(), isNull);
    expect(
      find.byKey(const ValueKey('confirm-receipt-actions-row')),
      findsOneWidget,
    );

    final submit = find.byKey(const ValueKey('confirm-receipt-submit'));
    final skip = find.byKey(const ValueKey('confirm-receipt-skip'));
    final submitRect = tester.getRect(submit);
    final skipRect = tester.getRect(skip);
    expect(submitRect.height, 48);
    expect(skipRect.height, 48);
    expect(submitRect.right, lessThan(skipRect.left));
    expect(skipRect.left - submitRect.right, greaterThanOrEqualTo(12));

    for (final entry in <(Finder, String)>[(submit, '送出'), (skip, '收貨且略過')]) {
      expect(
        find.descendant(of: entry.$1, matching: find.byType(HkButtonLabel)),
        findsOneWidget,
      );
      final text = tester.widget<Text>(
        find.descendant(of: entry.$1, matching: find.text(entry.$2)),
      );
      expect(text.maxLines, 1);
      expect(text.softWrap, isFalse);
      expect(text.overflow, TextOverflow.ellipsis);
      expect(
        tester
            .renderObject<RenderParagraph>(
              find.descendant(of: entry.$1, matching: find.text(entry.$2)),
            )
            .didExceedMaxLines,
        isFalse,
      );
      expect(find.bySemanticsLabel(entry.$2), findsOneWidget);
    }

    final surface = tester.getRect(
      find.byKey(const ValueKey('confirm-receipt-dialog-surface')),
    );
    expect(surface.left, greaterThanOrEqualTo(0));
    expect(surface.right, lessThanOrEqualTo(320));
    expect(surface.top, greaterThanOrEqualTo(24));
    expect(surface.bottom, lessThanOrEqualTo(568 - 16));

    final close = find.byKey(const ValueKey('confirm-receipt-close'));
    expect(tester.getSize(close), const Size.square(48));
    final closeRect = tester.getRect(close);
    expect(closeRect.top, greaterThanOrEqualTo(24));
    expect(closeRect.right, lessThanOrEqualTo(320));
    await tester.tap(close);
    await tester.pumpAndSettle();
    expect(submit, findsNothing);
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });

  testWidgets('short landscape viewport scrolls to the receipt actions', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await _openDialog(tester, size: const Size(568, 260));

    expect(tester.takeException(), isNull);
    final scroll = find.byKey(const ValueKey('confirm-receipt-dialog-scroll'));
    final scrollable = tester.state<ScrollableState>(
      find.descendant(of: scroll, matching: find.byType(Scrollable)),
    );
    expect(scrollable.position.maxScrollExtent, greaterThan(0));

    final skip = find.byKey(const ValueKey('confirm-receipt-skip'));
    await tester.ensureVisible(skip);
    await tester.pumpAndSettle();

    expect(tester.getSize(skip).height, 48);
    expect(find.bySemanticsLabel('收貨且略過'), findsOneWidget);
    expect(tester.takeException(), isNull);
    semantics.dispose();
  });

  testWidgets('in-flight receipt confirmation cannot be closed or backed out', (
    tester,
  ) async {
    final adapter = _PendingConfirmAdapter();
    final dio = Dio(BaseOptions(baseUrl: 'https://confirm-receipt.test'))
      ..httpClientAdapter = adapter;
    addTearDown(dio.close);
    await _openDialog(
      tester,
      size: const Size(390, 844),
      dio: dio,
      textScale: 1,
    );

    await tester.tap(find.byKey(const ValueKey('confirm-receipt-skip')));
    await tester.pump(const Duration(milliseconds: 10));

    expect(adapter.requests, hasLength(1));
    expect(find.byType(HkBrandLoader), findsOneWidget);
    final closeFinder = find.byKey(const ValueKey('confirm-receipt-close'));
    expect(tester.widget<IconButton>(closeFinder).onPressed, isNull);

    await tester.tap(closeFinder);
    await tester.pump();
    expect(closeFinder, findsOneWidget);

    await tester.binding.handlePopRoute();
    await tester.pump();
    expect(closeFinder, findsOneWidget);

    adapter.completeSuccess();
    await tester.pumpAndSettle();
    expect(closeFinder, findsNothing);
    expect(tester.takeException(), isNull);
  });
}

Future<void> _openDialog(
  WidgetTester tester, {
  required Size size,
  Dio? dio,
  double textScale = 1.4,
}) async {
  tester.view.devicePixelRatio = 1;
  tester.view.physicalSize = size;
  addTearDown(tester.view.resetDevicePixelRatio);
  addTearDown(tester.view.resetPhysicalSize);

  await tester.pumpWidget(
    ProviderScope(
      overrides: [if (dio != null) dioProvider.overrideWithValue(dio)],
      child: MaterialApp(
        builder: (context, child) => MediaQuery(
          data: MediaQuery.of(context).copyWith(
            padding: const EdgeInsets.only(top: 24, bottom: 16),
            textScaler: TextScaler.linear(textScale),
          ),
          child: child!,
        ),
        home: Builder(
          builder: (context) => Scaffold(
            body: Center(
              child: FilledButton(
                onPressed: () =>
                    ConfirmReceiptDialog.show(context, orderSn: 'HK-LAYOUT-1'),
                child: const Text('開啟確認收貨'),
              ),
            ),
          ),
        ),
      ),
    ),
  );

  await tester.tap(find.text('開啟確認收貨'));
  await tester.pumpAndSettle();
}

class _PendingConfirmAdapter implements HttpClientAdapter {
  final requests = <RequestOptions>[];
  final _response = Completer<ResponseBody>();

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) {
    requests.add(options);
    return _response.future;
  }

  void completeSuccess() {
    _response.complete(
      ResponseBody.fromString(
        jsonEncode(const {'code': 0, 'message': 'success', 'data': null}),
        200,
        headers: {
          Headers.contentTypeHeader: [Headers.jsonContentType],
        },
      ),
    );
  }

  @override
  void close({bool force = false}) {}
}
