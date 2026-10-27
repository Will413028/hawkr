import 'dart:typed_data';

import 'package:riverpod_annotation/riverpod_annotation.dart';

import 'protected_media_remote_service.dart';

part 'protected_media_repository.g.dart';

abstract interface class ProtectedMediaRepository {
  Future<Uint8List> download(String url);
}

class RemoteProtectedMediaRepository implements ProtectedMediaRepository {
  const RemoteProtectedMediaRepository(this._remoteService);

  final ProtectedMediaRemoteService _remoteService;

  @override
  Future<Uint8List> download(String url) async {
    final bytes = await _remoteService.fetchBytes(url);
    return Uint8List.fromList(bytes);
  }
}

@riverpod
ProtectedMediaRepository protectedMediaRepository(Ref ref) {
  return RemoteProtectedMediaRepository(
    ref.watch(protectedMediaRemoteServiceProvider),
  );
}
