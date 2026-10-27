import 'dart:typed_data';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../data/protected_media_repository.dart';

part 'protected_media_controller.g.dart';

/// Command boundary used by views that need image bytes.
///
/// It is kept alive because callers may issue multiple parallel downloads via
/// `ref.read` without watching command state.
@Riverpod(keepAlive: true)
class ProtectedMediaController extends _$ProtectedMediaController {
  @override
  void build() {}

  Future<Uint8List> download(String url) {
    return ref.read(protectedMediaRepositoryProvider).download(url);
  }
}
