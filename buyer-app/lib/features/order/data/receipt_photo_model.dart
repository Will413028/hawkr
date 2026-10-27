import 'package:flutter/foundation.dart';

enum ReceiptPhotoSource { backend, local, none }

@immutable
final class ReceiptPhotoState {
  ReceiptPhotoState({
    required List<String> photos,
    required this.source,
    this.problem,
  }) : photos = List.unmodifiable(photos);

  static final empty = ReceiptPhotoState(
    photos: const [],
    source: ReceiptPhotoSource.none,
  );

  final List<String> photos;
  final ReceiptPhotoSource source;
  final Object? problem;
}

@immutable
final class ReceiptPhotoWriteResult {
  const ReceiptPhotoWriteResult._({required this.isSuccess, this.problem});

  const ReceiptPhotoWriteResult.success() : this._(isSuccess: true);

  const ReceiptPhotoWriteResult.failure(Object error)
    : this._(isSuccess: false, problem: error);

  final bool isSuccess;
  final Object? problem;
}
