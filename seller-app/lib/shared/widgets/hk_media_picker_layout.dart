import 'dart:math' as math;

import 'package:flutter/material.dart';

/// Gives the media preview and gallery usable space within the current window.
///
/// Portrait keeps a full-width square when height permits. Short portrait
/// windows reserve at least 40% for the gallery; landscape places the gallery
/// beside a square preview bounded by the available height.
class HkMediaPickerLayout extends StatelessWidget {
  const HkMediaPickerLayout({
    super.key,
    required this.preview,
    required this.grid,
  });

  final Widget preview;
  final Widget grid;

  @override
  Widget build(BuildContext context) {
    return SafeArea(
      top: false,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final width = constraints.maxWidth;
          final height = constraints.maxHeight;
          final gallery = SizedBox(
            key: const ValueKey('media-picker-grid'),
            child: grid,
          );
          if (width >= 480 && width > height) {
            final previewExtent = math.min(height, width * 0.45);
            return Row(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                SizedBox(
                  width: previewExtent,
                  child: Center(
                    child: SizedBox.square(
                      key: const ValueKey('media-picker-preview'),
                      dimension: previewExtent,
                      child: preview,
                    ),
                  ),
                ),
                Expanded(child: gallery),
              ],
            );
          }
          return Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              SizedBox(
                key: const ValueKey('media-picker-preview'),
                height: math.min(width, height * 0.6),
                child: preview,
              ),
              Expanded(child: gallery),
            ],
          );
        },
      ),
    );
  }
}
