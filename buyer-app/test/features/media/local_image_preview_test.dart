import 'dart:convert';
import 'dart:io';

import 'package:cached_network_image/cached_network_image.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_buyer/config/flavor.dart';
import 'package:hawkr_buyer/config/image_config.dart';
import 'package:hawkr_buyer/config/media_image.dart';
import 'package:hawkr_buyer/features/media/data/local_image_preview.dart';
import 'package:hawkr_buyer/shared/widgets/oss_image.dart';

const _png =
    'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==';

void main() {
  setUpAll(() {
    if (!F.isInitialized) F.init(Flavor.dev);
  });

  test('local file becomes an inline data URI with the file bytes', () async {
    final dir = await Directory.systemTemp.createTemp('preview');
    addTearDown(() => dir.delete(recursive: true));
    final file = File('${dir.path}/a.png')
      ..writeAsBytesSync(base64Decode(_png));

    final uri = await localImagePreviewDataUri(file);

    expect(isInlineImageData(uri), isTrue);
    expect(base64Decode(uri.split(',').last), base64Decode(_png));
  });

  test('unreadable file gives an empty preview', () async {
    expect(await localImagePreviewDataUri(File('/nonexistent/x.png')), '');
  });

  testWidgets('OssImage renders an inline data URI from memory', (
    tester,
  ) async {
    await tester.pumpWidget(
      const MaterialApp(
        home: OssImage(
          imageSource: 'data:image/jpeg;base64,$_png',
          scene: ImageScene.reviewPhoto,
          width: 50,
          height: 50,
        ),
      ),
    );

    expect(find.byType(Image), findsOneWidget);
    expect(find.byType(CachedNetworkImage), findsNothing);
  });
}
