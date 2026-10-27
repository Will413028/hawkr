import 'dart:convert';
import 'dart:typed_data';
import 'dart:ui' as ui;

import 'package:hawkr_seller/core/networking/dio_provider.dart';
import 'package:hawkr_seller/core/theme/app_theme.dart';
import 'package:hawkr_seller/features/order/presentation/order_list_screen.dart';
import 'package:hawkr_seller/shared/widgets/green_header_bar.dart';
import 'package:dio/dio.dart';
import 'package:flutter/material.dart';
import 'package:flutter/rendering.dart';
import 'package:flutter/services.dart' show SystemChrome, SystemUiOverlayStyle;
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_screenutil/flutter_screenutil.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final device in const [
    (width: 320.0, top: 20.0),
    (width: 402.0, top: 62.0),
    (width: 440.0, top: 62.0),
  ]) {
    for (final back in [false, true]) {
      testWidgets(
        'orders share one top surface at ${device.width}, back=$back',
        (tester) async {
          tester.view
            ..devicePixelRatio = 3
            ..physicalSize = Size(device.width * 3, 874 * 3)
            ..padding = FakeViewPadding(top: device.top * 3, bottom: 34 * 3)
            ..viewPadding = FakeViewPadding(
              top: device.top * 3,
              bottom: 34 * 3,
            );
          addTearDown(tester.view.reset);
          final adapter = _EmptyOrderAdapter();
          final dio = Dio(BaseOptions(baseUrl: 'https://fixture.invalid'))
            ..httpClientAdapter = adapter;
          addTearDown(() => dio.close(force: true));
          const captureKey = Key('orders-capture');
          await tester.pumpWidget(
            ProviderScope(
              overrides: [dioProvider.overrideWithValue(dio)],
              child: ScreenUtilInit(
                designSize: const Size(414, 896),
                enableScaleWH: () => false,
                enableScaleText: () => false,
                builder: (_, _) => MaterialApp(
                  theme: AppTheme.light,
                  builder: (context, child) => MediaQuery(
                    data: MediaQuery.of(context).copyWith(
                      textScaler: const TextScaler.linear(1.4),
                      disableAnimations: true,
                    ),
                    child: child!,
                  ),
                  // Simulate returning from a dark-cover page. The order header
                  // must own its status contrast all the way to screen y=0.
                  home: AnnotatedRegion<SystemUiOverlayStyle>(
                    value: SystemUiOverlayStyle.light,
                    child: RepaintBoundary(
                      key: captureKey,
                      child: OrderListScreen(showBackButton: back),
                    ),
                  ),
                ),
              ),
            ),
          );
          await tester.pump();
          await tester.pump(const Duration(milliseconds: 250));
          final surface = find.byKey(
            const ValueKey('seller-glass-header-surface'),
          );
          final rect = tester.getRect(surface);
          expect(rect.top, 0);
          expect(rect.height, device.top + 56);
          expect(
            tester.getRect(find.text('我的訂單')).top,
            greaterThanOrEqualTo(device.top),
          );
          expect(
            tester
                .widget<GreenHeaderBar>(find.byType(GreenHeaderBar))
                .includeTopSafeArea,
            isTrue,
          );
          expect(
            SystemChrome.latestStyle?.statusBarBrightness,
            Brightness.light,
          );
          expect(adapter.paths, contains('/api/v1/app/orders'));

          // Compare actual painted pixels across the former safe-area seam.
          // The glass gradient may change gradually, but cannot restart here.
          final boundary = tester.renderObject<RenderRepaintBoundary>(
            find.byKey(captureKey),
          );
          final pixels = await tester.runAsync(() async {
            final image = await boundary.toImage(pixelRatio: 1);
            final data = await image.toByteData(
              format: ui.ImageByteFormat.rawRgba,
            );
            final bytes = Uint8List.fromList(data!.buffer.asUint8List());
            image.dispose();
            return bytes;
          });
          final x = (device.width / 2).floor();
          final above =
              ((device.top.toInt() - 1) * device.width.toInt() + x) * 4;
          final below = (device.top.toInt() * device.width.toInt() + x) * 4;
          for (var channel = 0; channel < 3; channel++) {
            expect(
              (pixels![above + channel] - pixels[below + channel]).abs(),
              lessThanOrEqualTo(1),
              reason: 'No color/shadow seam at safe inset',
            );
          }
          expect(tester.takeException(), isNull);
          await tester.pumpWidget(const SizedBox.shrink());
        },
        variant: const TargetPlatformVariant({TargetPlatform.iOS}),
      );
    }
  }
}

class _EmptyOrderAdapter implements HttpClientAdapter {
  final paths = <String>[];

  @override
  Future<ResponseBody> fetch(
    RequestOptions options,
    Stream<Uint8List>? requestStream,
    Future<void>? cancelFuture,
  ) async {
    if (options.method != 'GET' ||
        !{
          '/api/v1/app/orders',
          '/api/v1/app/seller/after_sales',
        }.contains(options.path)) {
      throw StateError(
        'Unexpected fixture request: ${options.method} ${options.path}',
      );
    }
    paths.add(options.path);
    return ResponseBody.fromString(
      jsonEncode({
        'code': 0,
        'data': {'records': <Object>[], 'total': 0},
      }),
      200,
      headers: {
        Headers.contentTypeHeader: [Headers.jsonContentType],
      },
    );
  }

  @override
  void close({bool force = false}) {}
}
