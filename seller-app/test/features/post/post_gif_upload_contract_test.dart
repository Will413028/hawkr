import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_seller/features/media/data/media_model.dart';
import 'package:hawkr_seller/features/media/data/media_repository.dart';
import 'package:hawkr_seller/features/post/data/post_repository.dart';
import 'package:dio/dio.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('post GIF upload contract', () {
    test('PostRepository disables compression for GIF MIME types', () async {
      final mediaRepository = _RecordingMediaRepository();
      final container = _containerWith(mediaRepository);
      addTearDown(container.dispose);

      final token = await container
          .read(_postRepositoryProvider)
          .uploadMedia(
            sellerId: '1',
            bytes: _gifBytes,
            filename: 'animated.gif',
            mimeType: ' IMAGE/GIF; charset=binary ',
          );

      expect(token, 'staged-token');
      expect(mediaRepository.lastCompress, isFalse);
    });

    test(
      'PostRepository keeps compression enabled for static images',
      () async {
        final mediaRepository = _RecordingMediaRepository();
        final container = _containerWith(mediaRepository);
        addTearDown(container.dispose);

        await container
            .read(_postRepositoryProvider)
            .uploadMedia(
              sellerId: '1',
              bytes: _jpegBytes,
              filename: 'photo.jpg',
              mimeType: 'image/jpeg',
            );

        expect(mediaRepository.lastCompress, isTrue);
      },
    );
  });

  test('replacement pickers avoid an explicit image-quality transcode', () {
    for (final path in const [
      'lib/features/post/presentation/post_compose_screen.dart',
      'lib/features/post/presentation/post_edit_screen.dart',
    ]) {
      final source = File(path).readAsStringSync();
      expect(
        source,
        isNot(contains('imageQuality:')),
        reason: '$path must not request an extra quality transcode',
      );
    }
  });

  test(
    'PhotoManager keeps GIF originals without changing static image flow',
    () {
      final source = File(
        'lib/features/post/presentation/post_media_picker_screen.dart',
      ).readAsStringSync();

      expect(source, contains('asset.originBytes'));
      expect(source, contains('await asset.mimeTypeAsync'));
      expect(
        source,
        contains('return _readOriginalAssetBytes(asset);'),
        reason: 'GIF assets must keep their original animated bytes',
      );
      expect(
        source,
        contains('final file = await asset.file;'),
        reason:
            'static assets must retain the platform-safe HEIC conversion path',
      );
      expect(
        RegExp(
          r'mediaBytes = await _readAssetImageBytes\(_selected!\);',
        ).allMatches(source),
        hasLength(2),
        reason: 'both Next and AI image paths must preserve GIF conditionally',
      );
    },
  );

  test('all non-frame-aware post editors preserve GIF and warn', () {
    for (final path in const [
      'lib/features/post/presentation/post_compose_screen.dart',
      'lib/features/post/presentation/post_edit_screen.dart',
      'lib/features/post/presentation/post_media_picker_screen.dart',
    ]) {
      final source = File(path).readAsStringSync();
      final detector = source.indexOf('ImageFormatDetector.detect');
      final warning = source.indexOf('GIF 動圖暫不支援編輯，已保留動圖格式');
      final editorNavigation = source.indexOf("'/photo-editor'");

      expect(detector, greaterThanOrEqualTo(0), reason: path);
      expect(warning, greaterThan(detector), reason: path);
      expect(editorNavigation, greaterThan(warning), reason: path);
      expect(
        source.substring(warning, editorNavigation),
        contains('return;'),
        reason: '$path must return before the non-frame-aware editor opens',
      );
    }
  });
}

final _postRepositoryProvider = Provider<PostRepository>(
  (ref) => PostRepository(Dio(), ref),
);

ProviderContainer _containerWith(MediaRepository mediaRepository) {
  return ProviderContainer(
    overrides: [mediaRepositoryProvider.overrideWithValue(mediaRepository)],
  );
}

class _RecordingMediaRepository extends MediaRepository {
  _RecordingMediaRepository() : super(Dio());

  bool? lastCompress;

  @override
  Future<MediaUploadResult> uploadWithSession({
    required Uint8List bytes,
    required String filename,
    bool compress = true,
  }) async {
    lastCompress = compress;
    return const MediaUploadResult(
      objectKey: 'public/post/animated.gif',
      size: 6,
      stagedToken: 'staged-token',
    );
  }
}

final _gifBytes = Uint8List.fromList(const [
  0x47,
  0x49,
  0x46,
  0x38,
  0x39,
  0x61,
  0,
  0,
  0,
  0,
  0,
  0,
]);

final _jpegBytes = Uint8List.fromList(const [
  0xff,
  0xd8,
  0xff,
  0,
  0,
  0,
  0,
  0,
  0,
  0,
  0,
  0,
]);
