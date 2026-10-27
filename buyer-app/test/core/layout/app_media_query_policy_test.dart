import 'package:hawkr_buyer/core/layout/app_media_query_policy.dart';
import 'package:hawkr_buyer/core/layout/app_text_size.dart';
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  for (final scenario in <({double inputScale, double expectedScale})>[
    (inputScale: 0.8, expectedScale: 0.8),
    (inputScale: 1.0, expectedScale: 1.0),
    (inputScale: 1.4, expectedScale: 1.4),
    (inputScale: 2.0, expectedScale: 1.4),
  ]) {
    test('maps ${scenario.inputScale}x system text to '
        '${scenario.expectedScale}x', () {
      final source = MediaQueryData(
        textScaler: TextScaler.linear(scenario.inputScale),
      );

      final result = AppMediaQueryPolicy.apply(source);

      expect(
        result.textScaler.scale(10),
        closeTo(10 * scenario.expectedScale, 0.0001),
      );
    });
  }

  test('preserves nonlinear scaling below the per-font-size ceiling', () {
    const source = MediaQueryData(textScaler: _TestNonlinearTextScaler());

    final result = AppMediaQueryPolicy.apply(source);

    expect(result.textScaler.scale(10), 12);
    expect(result.textScaler.scale(20), 28);
  });

  for (final scenario in <({AppTextSize preference, double expectedScale})>[
    (preference: AppTextSize.small, expectedScale: 0.9),
    (preference: AppTextSize.standard, expectedScale: 1.0),
    (preference: AppTextSize.large, expectedScale: 1.15),
  ]) {
    test(
      '${scenario.preference.name} adjusts text without scaling geometry',
      () {
        const source = MediaQueryData(
          size: Size(390, 844),
          padding: EdgeInsets.only(top: 59, bottom: 34),
          textScaler: TextScaler.linear(1),
        );

        final result = AppMediaQueryPolicy.apply(
          source,
          textSize: scenario.preference,
        );

        expect(
          result.textScaler.scale(20),
          closeTo(20 * scenario.expectedScale, 0.0001),
        );
        expect(result.size, source.size);
        expect(result.padding, source.padding);
      },
    );
  }

  test('preference adjustments stay inside the tested text scale range', () {
    final smallest = AppMediaQueryPolicy.apply(
      const MediaQueryData(textScaler: TextScaler.linear(0.5)),
      textSize: AppTextSize.small,
    );
    final largest = AppMediaQueryPolicy.apply(
      const MediaQueryData(textScaler: TextScaler.linear(2)),
      textSize: AppTextSize.large,
    );

    expect(smallest.textScaler.scale(20), 16);
    expect(largest.textScaler.scale(20), 28);
  });

  test('large preference preserves nonlinear platform scaling', () {
    const source = MediaQueryData(textScaler: _GentleNonlinearTextScaler());

    final result = AppMediaQueryPolicy.apply(
      source,
      textSize: AppTextSize.large,
    );

    expect(result.textScaler.scale(10), closeTo(12, 0.0001));
    expect(result.textScaler.scale(20), closeTo(27, 0.0001));
  });

  for (final device in <({Size size, EdgeInsets viewPadding})>[
    (size: Size(375, 667), viewPadding: EdgeInsets.only(top: 20)),
    (size: Size(375, 812), viewPadding: EdgeInsets.only(top: 44, bottom: 34)),
    (size: Size(390, 844), viewPadding: EdgeInsets.only(top: 59, bottom: 34)),
    (size: Size(430, 932), viewPadding: EdgeInsets.only(top: 59, bottom: 34)),
    (size: Size(844, 390), viewPadding: EdgeInsets.fromLTRB(59, 0, 59, 21)),
    (size: Size(768, 1024), viewPadding: EdgeInsets.only(top: 24, bottom: 20)),
  ]) {
    for (final scenario in <({double inputScale, double expectedScale})>[
      (inputScale: 1.0, expectedScale: 1.0),
      (inputScale: 1.4, expectedScale: 1.4),
      (inputScale: 2.0, expectedScale: 1.4),
    ]) {
      test('preserves ${device.size} geometry at '
          '${scenario.inputScale}x system text', () {
        final source = MediaQueryData(
          size: device.size,
          padding: device.viewPadding,
          viewPadding: device.viewPadding,
          viewInsets: const EdgeInsets.only(bottom: 320),
          textScaler: TextScaler.linear(scenario.inputScale),
        );

        final result = AppMediaQueryPolicy.apply(source);

        expect(result.size, device.size);
        expect(result.padding, device.viewPadding);
        expect(result.viewPadding, device.viewPadding);
        expect(result.viewInsets, const EdgeInsets.only(bottom: 320));
        expect(
          result.textScaler.scale(16),
          closeTo(16 * scenario.expectedScale, 0.0001),
        );
        expect(result.copyWith(textScaler: source.textScaler), source);
      });
    }
  }
}

final class _TestNonlinearTextScaler extends TextScaler {
  const _TestNonlinearTextScaler();

  @override
  double scale(double fontSize) =>
      fontSize <= 16 ? fontSize * 1.2 : fontSize * 1.8;

  @override
  double get textScaleFactor => 1.2;
}

final class _GentleNonlinearTextScaler extends TextScaler {
  const _GentleNonlinearTextScaler();

  @override
  double scale(double fontSize) =>
      fontSize <= 16 ? fontSize * 1.05 : fontSize * 1.2;

  @override
  double get textScaleFactor => 1.05;
}
