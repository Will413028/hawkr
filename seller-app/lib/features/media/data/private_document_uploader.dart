import 'dart:typed_data';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'media_repository.dart';

part 'private_document_uploader.g.dart';

/// 上傳一張私密文件，回 staged token。
typedef PrivateDocumentUploader =
    Future<String> Function(Uint8List bytes, String docType, String filename);

/// 私密文件（認證證件、申訴佐證）走單體的機敏分段上傳，身分由 Bearer 決定；
/// 回傳的 staged token 由送出該文件所屬的請求帶上。
@Riverpod(keepAlive: true)
PrivateDocumentUploader privateDocumentUploader(Ref ref) =>
    (bytes, docType, filename) =>
        ref.read(mediaRepositoryProvider).uploadPrivateDocument(bytes: bytes);
