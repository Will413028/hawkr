import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/imageai/v1alpha1/imageai.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/imageai/v1alpha1/imageai.connect.spec.dart';
import 'package:hawkr_api/gen/hawkr/imageai/v1alpha1/imageai.pb.dart' as pb;
import 'package:hawkr_api/gen/hawkr/imageai/v1alpha1/imageai.pbenum.dart';
import 'package:hawkr_api/testing.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hawkr_seller/features/photo_editor/data/ai_image_service.dart';

import '../../support/connect_errors.dart';
import '../../support/default_error_text.dart';

Future<Uint8List> _passThrough(
  Uint8List bytes, {
  required int maxBytes,
}) async => bytes;

AiImageService _service(FakeTransportBuilder builder, {ImageDownsizer? down}) =>
    AiImageService(
      ImageAiAppServiceClient(builder.build()),
      defaults: zhDefaultErrorText,
      downsize: down ?? _passThrough,
    );

void main() {
  group('fetchStyles', () {
    test('maps styles, default prompt and weekly usage', () async {
      pb.ListAiStylesRequest? seen;
      final service = _service(
        FakeTransportBuilder().unary(ImageAiAppService.listAiStyles, (r, _) {
          seen = r;
          return pb.ListAiStylesResponse(
            styles: [pb.AiStyle(id: 's1', name: 'n', label: 'l', prompt: 'p')],
            defaultPrompt: 'dp',
            weeklyUsage: 2,
            weeklyLimit: 5,
          );
        }),
      );

      final styles = await service.fetchStyles(category: 'post');

      expect(seen!.category, AiStyleCategory.AI_STYLE_CATEGORY_POST);
      expect(styles!.styles.single.id, 's1');
      expect(styles.styles.single.prompt, 'p');
      expect(styles.defaultPrompt, 'dp');
      expect(styles.weeklyUsage, 2);
      expect(styles.weeklyLimit, 5);
    });

    test('returns null when the call fails', () async {
      final service = _service(
        FakeTransportBuilder().unary(ImageAiAppService.listAiStyles, (_, _) {
          throw ConnectException(Code.unavailable, 'down');
        }),
      );
      expect(await service.fetchStyles(), isNull);
    });
  });

  group('transformWithGemini', () {
    test(
      'sends the downsized image with the prompt and returns the result',
      () async {
        pb.TransformImageRequest? seen;
        int? limit;
        final service = _service(
          FakeTransportBuilder().unary(ImageAiAppService.transformImage, (
            r,
            _,
          ) {
            seen = r;
            return pb.TransformImageResponse(imageData: [9, 8, 7]);
          }),
          down: (bytes, {required maxBytes}) async {
            limit = maxBytes;
            return Uint8List.fromList([1, 2]);
          },
        );

        final out = await service.transformWithGemini(
          imageBytes: Uint8List.fromList([5, 5, 5, 5]),
          prompt: 'brighter',
        );

        expect(out, [9, 8, 7]);
        expect(limit, kMaxTransformImageBytes);
        expect(seen!.imageData, [1, 2]);
        expect(seen!.mimeType, 'image/jpeg');
        expect(seen!.prompt, 'brighter');
      },
    );

    test(
      'quota exhausted maps to AiQuotaExceededException with the server text',
      () async {
        final service = _service(
          FakeTransportBuilder().unary(ImageAiAppService.transformImage, (
            _,
            _,
          ) {
            throw reasonError(
              Code.resourceExhausted,
              'IMAGE_QUOTA_EXCEEDED',
              message: 'quota text',
            );
          }),
        );
        await expectLater(
          service.transformWithGemini(imageBytes: Uint8List(1)),
          throwsA(
            isA<AiQuotaExceededException>().having(
              (e) => e.message,
              'message',
              'quota text',
            ),
          ),
        );
      },
    );

    test('other failures map to AiTransformException', () async {
      final service = _service(
        FakeTransportBuilder().unary(ImageAiAppService.transformImage, (_, _) {
          throw reasonError(
            Code.unavailable,
            'IMAGE_NOT_PRODUCED',
            message: 'no image',
          );
        }),
      );
      await expectLater(
        service.transformWithGemini(imageBytes: Uint8List(1)),
        throwsA(
          isA<AiTransformException>().having(
            (e) => e.message,
            'message',
            'no image',
          ),
        ),
      );
    });

    test('an empty image in the response is a failure', () async {
      final service = _service(
        FakeTransportBuilder().unary(
          ImageAiAppService.transformImage,
          (_, _) => pb.TransformImageResponse(),
        ),
      );
      await expectLater(
        service.transformWithGemini(imageBytes: Uint8List(1)),
        throwsA(isA<AiTransformException>()),
      );
    });
  });
}
