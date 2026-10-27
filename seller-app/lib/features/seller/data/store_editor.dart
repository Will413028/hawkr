import 'package:hawkr_api/errors.dart'
    show DefaultErrorText, ErrorAction, ErrorFacts, classifyError;
import 'package:hawkr_api/field_mask.dart';
import 'package:hawkr_api/gen/hawkr/common/v1alpha1/error_reason.pbenum.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.connect.client.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/submit.dart';
import 'package:hawkr_api/transport.dart' show ConnectException;
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/session/my_store.dart';
import '../../../shared/utils/hk_log.dart';
import 'current_store.dart';
import 'store_form.dart';
import 'version_conflict.dart';

part 'store_editor.g.dart';

/// 一次寫入店家（`UpdateMyStore`）的結果。
sealed class StoreSaveOutcome {
  const StoreSaveOutcome();
}

final class StoreSaved extends StoreSaveOutcome {
  const StoreSaved();
}

/// 版本衝突：店家已重讀成最新版本（[reloaded]），使用者的輸入由呼叫端保留，確認後再送出。
final class StoreSaveConflict extends StoreSaveOutcome {
  const StoreSaveConflict({required this.reloaded});

  final bool reloaded;
}

/// 欄位錯誤：[fields] 的 key 是請求的 proto 路徑（`store.contact_name`）。
final class StoreSaveFieldErrors extends StoreSaveOutcome {
  const StoreSaveFieldErrors(this.fields, {this.message});

  final Map<String, String> fields;
  final String? message;
}

final class StoreSaveFailed extends StoreSaveOutcome {
  const StoreSaveFailed(this.message);

  final String message;
}

/// 店家資料頁的寫入：帶讀到的 etag 呼叫 `UpdateMyStore`，成功或版本衝突後重讀店家。
class StoreEditor {
  StoreEditor({
    required SellerAppServiceClient client,
    required MyStore? Function() current,
    required Future<bool> Function() reload,
    required void Function(Store written) adopt,
    required DefaultErrorText defaults,
  }) : _client = client,
       _current = current,
       _reload = reload,
       _adopt = adopt,
       _defaults = defaults;

  final SellerAppServiceClient _client;
  final MyStore? Function() _current;
  final Future<bool> Function() _reload;
  final void Function(Store written) _adopt;
  final DefaultErrorText _defaults;

  Future<StoreSaveOutcome> save(StoreChanges changes) async {
    if (changes.isEmpty) return const StoreSaved();
    final etag = _current()?.etag;
    if (etag == null) {
      return const StoreSaveFailed(storeStateUnavailableNotice);
    }
    try {
      final response = await _client.updateMyStore(
        UpdateMyStoreRequest(
          store: changes.toStore(etag),
          updateMask: fieldMask(Store(), changes.paths),
        ),
      );
      if (response.hasStore()) _adopt(response.store);
    } catch (error) {
      return _failure(error);
    }
    await _reload();
    return const StoreSaved();
  }

  Future<StoreSaveOutcome> _failure(Object error) async {
    HKLog.logW('[StoreEditor] write failed: $error');
    final failure = submitError(error, defaults: _defaults);
    switch (failure) {
      case SubmitConflict():
        return StoreSaveConflict(reloaded: await _reload());
      case SubmitFields(:final fields, :final message):
        return StoreSaveFieldErrors(fields, message: message);
      case SubmitMessage(:final message):
        return StoreSaveFailed(message);
    }
  }
}

@Riverpod(keepAlive: true)
StoreEditor storeEditor(Ref ref) => StoreEditor(
  client: ref.watch(sellerClientProvider),
  current: () => ref.read(myStoreSnapshotProvider),
  reload: () => ref.read(currentStoreProvider.notifier).refresh(),
  adopt: (written) =>
      ref.read(currentStoreProvider.notifier).adoptWritten(written),
  defaults: ref.watch(defaultErrorTextProvider),
);

/// 伺服器的 reason 是 [reason] 的失敗（例如 `AGREEMENT_OUTDATED`）。
bool hasErrorReason(Object error, ErrorReason reason) {
  final decision = classifyError(
    ErrorFacts.fromException(ConnectException.from(error)),
  );
  return decision.action == ErrorAction.reason && decision.reason == reason;
}

/// 儲存失敗要顯示的文字，成功是 null。[fieldPaths] 是這個畫面欄位的錯誤路徑，欄位錯誤優先顯示這些路徑的文字。
String? storeSaveMessage(
  StoreSaveOutcome outcome, {
  List<String> fieldPaths = const [],
  String? conflictGuidance,
}) => switch (outcome) {
  StoreSaved() => null,
  StoreSaveConflict(:final reloaded) =>
    reloaded
        ? '${versionConflictHeadline('商店資料')}\n${conflictGuidance ?? versionConflictGuidance()}'
        : versionReloadFailedNotice,
  StoreSaveFieldErrors(:final fields, :final message) =>
    fieldMessage(fields, fieldPaths) ?? message ?? fields.values.first,
  StoreSaveFailed(:final message) => message,
};
