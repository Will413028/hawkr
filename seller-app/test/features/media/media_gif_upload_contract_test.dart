import 'dart:typed_data';

import 'package:hawkr_seller/features/media/data/media_repository.dart';
import 'package:flutter_test/flutter_test.dart';

void main() {
  group('MediaRepository GIF compression guard', () {
    test('large GIF never enters the JPEG compressor', () {
      final gif = _largeBytesWithHeader(const [
        0x47,
        0x49,
        0x46,
        0x38,
        0x39,
        0x61,
      ]);

      expect(
        MediaRepository.shouldCompressForUpload(
          bytes: gif,
          contentType: 'image/gif',
        ),
        isFalse,
      );
    });

    test(
      'mislabelled GIF follows declared type instead of changing contracts',
      () {
        final gif = _largeBytesWithHeader(const [
          0x47,
          0x49,
          0x46,
          0x38,
          0x37,
          0x61,
        ]);

        expect(
          MediaRepository.shouldCompressForUpload(
            bytes: gif,
            contentType: 'image/jpeg',
          ),
          isTrue,
        );
      },
    );

    test('existing static-image compression policy is unchanged', () {
      final largeJpeg = _largeBytesWithHeader(const [0xff, 0xd8, 0xff]);
      final smallJpeg = Uint8List.fromList(const [
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

      expect(
        MediaRepository.shouldCompressForUpload(
          bytes: largeJpeg,
          contentType: 'image/jpeg',
        ),
        isTrue,
      );
      expect(
        MediaRepository.shouldCompressForUpload(
          bytes: smallJpeg,
          contentType: 'image/jpeg',
        ),
        isFalse,
      );
      expect(
        MediaRepository.shouldCompressForUpload(
          bytes: largeJpeg,
          contentType: 'image/jpeg',
          compress: false,
        ),
        isFalse,
      );
    });
  });
}

Uint8List _largeBytesWithHeader(List<int> header) {
  final bytes = Uint8List(2 * 1024 * 1024 + 1);
  bytes.setRange(0, header.length, header);
  return bytes;
}
