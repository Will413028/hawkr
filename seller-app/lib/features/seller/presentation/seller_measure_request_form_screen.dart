import 'dart:io';
import 'dart:typed_data';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/go_text.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';
import 'package:image_picker/image_picker.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/router/app_router.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_adaptive_page.dart';
import '../../../shared/widgets/hk_async_state_panel.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_glass_chrome_surface.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/truck_loading_overlay.dart';
import '../../media/data/private_document_uploader.dart';
import '../data/account_scope.dart';
import '../data/measure_display.dart';
import '../data/measures.dart';

const _uploadFailedNotice = '上傳失敗，請再試一次';
const _pickFailedNotice = '選取圖片失敗';
const _closedDescription = '請回到措施與申覆查看目前狀態。';

/// 兩種請求各自的文字（申覆與請求解除）。
extension on MeasureRequestAction {
  String get formTitle => switch (this) {
    MeasureRequestAction.appeal => '提出申覆',
    MeasureRequestAction.remediation => '請求解除措施',
  };
  String get reasonPlaceholder => switch (this) {
    MeasureRequestAction.appeal => '請說明此次措施有誤的理由，平台會盡快協助複核。',
    MeasureRequestAction.remediation => '請說明已完成的改善，平台會盡快複核。',
  };
  String get reasonLabel => switch (this) {
    MeasureRequestAction.appeal => '申覆理由 *',
    MeasureRequestAction.remediation => '改善說明 *',
  };
  String get reasonTooLongNotice => switch (this) {
    MeasureRequestAction.appeal => '申覆理由最多 $requestReasonMaxLength 字',
    MeasureRequestAction.remediation => '改善說明最多 $requestReasonMaxLength 字',
  };
  String get submittedNotice => switch (this) {
    MeasureRequestAction.appeal => '申覆已送出，請等候審核',
    MeasureRequestAction.remediation => '解除請求已送出，請等候審核',
  };
  String get submitLabel => switch (this) {
    MeasureRequestAction.appeal => '送出申覆',
    MeasureRequestAction.remediation => '送出請求',
  };
  String get submittingLabel => switch (this) {
    MeasureRequestAction.appeal => '申覆送出中…',
    MeasureRequestAction.remediation => '請求送出中…',
  };
  String closedNotice(Measure measure) => switch (this) {
    MeasureRequestAction.appeal => appealClosedNotice(measure),
    MeasureRequestAction.remediation => remediationClosedNotice(measure),
  };
}

/// 選好的一張佐證圖片（位元組與檔名）。
typedef PickedEvidence = ({Uint8List bytes, String filename});

/// 從相機或相簿選一張佐證圖片；取消時回 null。
typedef EvidencePicker = Future<PickedEvidence?> Function(BuildContext context);

class _Photo {
  _Photo(this.id, this.bytes);

  final int id;
  final Uint8List bytes;

  /// 上傳完成才有；沒有 token 也沒有 [error] 的照片還在上傳。
  String? token;

  /// 伺服器指出這張的 token 不能用（過期或被拒）的說明；有值表示要移除後重新新增。
  String? error;
}

/// 提出申覆或請求解除措施的全螢幕表單（website `features/seller/measures/` 的申覆與解除請求頁的 App 版），路由 [SellerRoutes.measureRequest] 以措施 id 開啟。
///
/// 措施用 [measureByIdProvider] 取得（列表已讀過就用列表的，否則 `GetMyMeasure`），所以網址就能開啟，狀態還原也不會掉資料。
/// 讀到的措施已經不能送出這種請求（伺服器的 `can_appeal`／`can_request_remediation`）時顯示原因，不顯示表單。
class SellerMeasureRequestFormScreen extends ConsumerWidget {
  const SellerMeasureRequestFormScreen({
    super.key,
    required this.measureId,
    this.kind = MeasureRequestAction.appeal,
    this.pickEvidence,
  });

  final String measureId;

  /// 申覆或請求解除；路由由查詢參數決定（[SellerRoutes.measureRemediationFor]）。
  final MeasureRequestAction kind;

  /// 測試用：換掉相機／相簿。
  final EvidencePicker? pickEvidence;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accountId = signedInAccountId(ref.watch(sessionProvider));
    if (accountId == null) {
      return _FormShell(kind: kind, child: const TruckLoadingOverlay());
    }
    final measureProvider = measureByIdProvider(accountId, measureId);
    final measure = ref.watch(measureProvider);
    // 有讀到的值就用它：重新讀取失敗（帶著上一次的值）不能讓表單被卸載而丟掉已輸入的理由與已上傳的照片（和措施列表一樣）。
    final loaded = measure.value;
    if (loaded == null) {
      // 重試中（error 狀態被 invalidate 後是讀取中）顯示讀取中，不繼續停在錯誤面板。
      if (measure.hasError && !measure.isLoading) {
        // 不存在或不是自己的措施（網址帶了別的 id）重讀也一樣：終態，回列表，沒有重試；暫時性的錯誤才可重試。
        final code = ConnectException.from(measure.error!).code;
        if (code == Code.notFound || code == Code.permissionDenied) {
          return _FormShell(
            kind: kind,
            child: _PanelPage(
              HkAsyncStatePanel(
                kind: HkAsyncStateKind.empty,
                title: measureNotFoundTitle,
                description: measureNotFoundDescription,
                actionLabel: measureNotFoundAction,
                actionIcon: HkIconName.back,
                onAction: () => context.go(SellerRoutes.measures),
              ),
            ),
          );
        }
        return _FormShell(
          kind: kind,
          child: _PanelPage(
            HkAsyncStatePanel(
              kind: HkAsyncStateKind.error,
              title: measuresLoadFailedTitle,
              description: measuresLoadFailedDescription,
              actionLabel: '重試',
              onAction: () => ref.invalidate(measureProvider),
            ),
          ),
        );
      }
      return _FormShell(kind: kind, child: const TruckLoadingOverlay());
    }
    return kind.allowedOn(loaded)
        ? _RequestFormBody(
            kind: kind,
            measure: loaded,
            pickEvidence: pickEvidence,
          )
        : _FormShell(
            kind: kind,
            child: _PanelPage(
              HkAsyncStatePanel(
                kind: HkAsyncStateKind.empty,
                title: kind.closedNotice(loaded),
                description: _closedDescription,
              ),
            ),
          );
  }
}

/// 離開表單：有上一頁就返回；網址直接開啟（沒有上一頁）時改去措施與申覆列表。
void _leaveForm(BuildContext context) {
  if (context.canPop()) {
    context.pop();
  } else {
    context.go(SellerRoutes.measures);
  }
}

/// 讀取中、讀取失敗、不能送出這種請求時的外框（和表單同一個標題列）。
class _FormShell extends StatelessWidget {
  const _FormShell({required this.kind, required this.child});

  final MeasureRequestAction kind;
  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: HkGlassAppBar(
        title: Text(kind.formTitle),
        onBackPressed: () => _leaveForm(context),
      ),
      body: HkAdaptivePage(child: child),
    );
  }
}

class _PanelPage extends StatelessWidget {
  const _PanelPage(this.panel);

  final Widget panel;

  @override
  Widget build(BuildContext context) => ListView(
    padding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
    children: [panel],
  );
}

class _RequestFormBody extends ConsumerStatefulWidget {
  const _RequestFormBody({
    required this.kind,
    required this.measure,
    this.pickEvidence,
  });

  final MeasureRequestAction kind;
  final Measure measure;
  final EvidencePicker? pickEvidence;

  @override
  ConsumerState<_RequestFormBody> createState() => _RequestFormBodyState();
}

class _RequestFormBodyState extends ConsumerState<_RequestFormBody> {
  final _reasonCtrl = TextEditingController();
  final _photos = <_Photo>[];
  MeasureRequestAction get _kind => widget.kind;
  late Measure _measure = widget.measure;
  var _nextPhotoId = 0;
  var _busy = false;
  String? _notice;
  String? _reasonError;
  String? _evidenceError;

  @override
  void didUpdateWidget(_RequestFormBody oldWidget) {
    super.didUpdateWidget(oldWidget);
    // 重新讀到的措施（版本、狀態變了）換掉表單依據的那份；輸入的理由與照片不動。
    if (oldWidget.measure != widget.measure) _measure = widget.measure;
  }

  @override
  void dispose() {
    _reasonCtrl.dispose();
    super.dispose();
  }

  /// 送出前的警告：伺服器給了 `rejection_cooldown`（下一個請求若被駁回會開始的冷卻期）才有；申覆與請求解除兩種表單都顯示。
  String? get _cooldownWarning =>
      rejectionCooldownWarning(_measure, context.uiLocale);

  bool get _uploading =>
      _photos.any((photo) => photo.token == null && photo.error == null);

  bool get _hasFailedPhoto => _photos.any((photo) => photo.error != null);

  int get _reasonLength => goTrimmedRuneCount(_reasonCtrl.text);

  bool get _reasonTooLong => _reasonLength > requestReasonMaxLength;

  bool get _canSubmit =>
      _reasonLength > 0 &&
      !_reasonTooLong &&
      !_uploading &&
      !_hasFailedPhoto &&
      !_busy;

  Future<void> _addPhoto() async {
    if (_photos.length >= requestMaxEvidence || _busy) return;
    final picked = await (widget.pickEvidence ?? _pickFromDevice)(context);
    // 選圖器開著的時候可能已經送出：送出中不再加照片（送出的清單已經定了）。
    if (picked == null || !mounted || _busy) return;
    if (_photos.length >= requestMaxEvidence) return;
    final photo = _Photo(_nextPhotoId++, picked.bytes);
    setState(() {
      _photos.add(photo);
      _evidenceError = null;
    });
    final String token;
    try {
      token = await ref.read(privateDocumentUploaderProvider)(
        picked.bytes,
        requestEvidenceDocType,
        picked.filename,
      );
    } catch (error) {
      HKLog.logW('[MeasureRequestForm] evidence upload failed: $error');
      // 上傳中被移除的照片，結果不再有人要：失敗也不提示。
      if (!mounted || !_photos.contains(photo)) return;
      setState(() => _photos.remove(photo));
      FloatingToast.error(context, _uploadFailedNotice);
      return;
    }
    if (!mounted || !_photos.contains(photo)) return;
    setState(() => photo.token = token);
  }

  /// 賣家移除照片：整個佐證欄位的錯誤跟著照片清單失效；該張照片的失敗提示（[_notice] 取自它的 error）也一併清掉，
  /// 但其他張還在失敗且是同一句時保留。
  void _removePhoto(_Photo photo) {
    setState(() {
      _photos.remove(photo);
      _evidenceError = null;
      final notice = photo.error;
      if (notice != null &&
          _notice == notice &&
          !_photos.any((other) => other.error == notice)) {
        _notice = null;
      }
    });
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() {
      _busy = true;
      _notice = null;
      _reasonError = null;
      _evidenceError = null;
    });
    final result = await submitMeasureRequest(
      ref.read(sellerClientProvider),
      kind: _kind,
      measure: _measure,
      reason: _reasonCtrl.text,
      evidenceTokens: [for (final photo in _photos) photo.token!],
      defaults: ref.read(defaultErrorTextProvider),
    );
    if (!mounted) return;
    switch (result) {
      case MeasureRequestSubmitted(:final measure):
        _adoptIntoLists(measure);
        FloatingToast.success(context, _kind.submittedNotice);
        _leaveForm(context);
      case MeasureRequestClosed(:final latest, :final notice):
        _adoptIntoLists(latest);
        FloatingToast.error(context, notice);
        _leaveForm(context);
      case MeasureRequestRefreshed(:final latest, :final notice):
        _adoptIntoLists(latest);
        setState(() {
          _measure = latest;
          _notice = notice;
          _busy = false;
        });
      case MeasureRequestReasonInvalid(:final message):
        setState(() {
          _reasonError = message;
          _busy = false;
        });
      case MeasureRequestEvidenceInvalid(
        :final byIndex,
        :final evidenceMessage,
        :final reasonMessage,
      ):
        // 送出的 token 清單與 _photos 同序（送出時每張都有 token）。
        final submitted = List.of(_photos);
        setState(() {
          for (final MapEntry(key: index, value: message) in byIndex.entries) {
            if (index >= submitted.length) continue;
            submitted[index]
              ..token = null
              ..error = message;
          }
          _reasonError = reasonMessage;
          // 整個欄位的錯誤（沒有索引）顯示在佐證區，不標任何一張。
          _evidenceError = evidenceMessage;
          _notice = byIndex.isEmpty
              ? null
              : byIndex.entries
                    .firstWhere(
                      (entry) => entry.key < submitted.length,
                      orElse: () => byIndex.entries.first,
                    )
                    .value;
          _busy = false;
        });
      case MeasureRequestFailed(:final message):
        setState(() {
          _notice = message;
          _busy = false;
        });
    }
  }

  /// 伺服器剛回的這一筆寫進已讀到的列表（全部、生效中；沒讀過的列表就不動，見 [MyMeasures.adopt]）。
  void _adoptIntoLists(Measure latest) {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    if (accountId == null) return;
    for (final filter in adoptingMeasureStates) {
      final list = myMeasuresProvider(accountId, filter);
      if (ref.exists(list)) ref.read(list.notifier).adopt(latest);
    }
  }

  @override
  Widget build(BuildContext context) {
    return PopScope(
      canPop: !_busy,
      child: Scaffold(
        backgroundColor: AppColors.surfacePage,
        appBar: HkGlassAppBar(
          title: Text(_kind.formTitle),
          onBackPressed: _busy ? null : () => _leaveForm(context),
        ),
        body: HkAdaptivePage(
          maxWidth: HkControl.chromeMaxWidth,
          padding: const EdgeInsets.fromLTRB(
            HkSpace.lg,
            HkSpace.xl,
            HkSpace.lg,
            HkSpace.section,
          ),
          child: SingleChildScrollView(
            keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.stretch,
              children: [
                HkContentCard(
                  child: Text(
                    _measure.statement.isNotEmpty
                        ? _measure.statement
                        : measureTitle(_measure, context.uiLocale),
                    style: AppTypography.meta,
                  ),
                ),
                if (_cooldownWarning != null) ...[
                  const SizedBox(height: HkSpace.md),
                  Text(
                    _cooldownWarning!,
                    key: const ValueKey('rejection-cooldown-warning'),
                    style: AppTypography.caption.copyWith(
                      color: AppColors.danger,
                      fontWeight: FontWeight.w700,
                    ),
                  ),
                ],
                const SizedBox(height: HkSpace.xl),
                _label(_kind.reasonLabel),
                const SizedBox(height: HkSpace.sm),
                TextField(
                  key: const ValueKey('request-reason'),
                  controller: _reasonCtrl,
                  enabled: !_busy,
                  minLines: 5,
                  maxLines: 8,
                  keyboardType: TextInputType.multiline,
                  onChanged: (_) => setState(() => _reasonError = null),
                  decoration: InputDecoration(
                    hintText: _kind.reasonPlaceholder,
                    errorText:
                        _reasonError ??
                        (_reasonTooLong ? _kind.reasonTooLongNotice : null),
                  ),
                ),
                const SizedBox(height: HkSpace.xs),
                Align(
                  alignment: Alignment.centerRight,
                  child: Text(
                    '$_reasonLength / $requestReasonMaxLength',
                    key: const ValueKey('request-reason-counter'),
                    style: AppTypography.caption.copyWith(
                      color: _reasonTooLong
                          ? AppColors.danger
                          : AppColors.textMuted,
                    ),
                  ),
                ),
                const SizedBox(height: HkSpace.xl),
                _label('佐證圖片（選填）'),
                const SizedBox(height: HkSpace.sm),
                Wrap(
                  spacing: HkSpace.sm,
                  runSpacing: HkSpace.sm,
                  children: [
                    for (final photo in _photos)
                      _PhotoTile(
                        photo: photo,
                        onRemove: _busy ? null : () => _removePhoto(photo),
                      ),
                    if (_photos.length < requestMaxEvidence)
                      _AddPhotoTile(onTap: _busy ? null : _addPhoto),
                  ],
                ),
                const SizedBox(height: HkSpace.xs),
                Text(
                  '最多 $requestMaxEvidence 張照片',
                  style: AppTypography.caption.copyWith(
                    color: AppColors.textMuted,
                  ),
                ),
                if (_evidenceError != null) ...[
                  const SizedBox(height: HkSpace.xs),
                  Semantics(
                    liveRegion: true,
                    child: Text(
                      _evidenceError!,
                      key: const ValueKey('request-evidence-error'),
                      style: AppTypography.caption.copyWith(
                        color: AppColors.danger,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                ],
                if (_notice != null) ...[
                  const SizedBox(height: HkSpace.lg),
                  Semantics(
                    liveRegion: true,
                    child: Text(
                      _notice!,
                      key: const ValueKey('request-notice'),
                      style: AppTypography.caption.copyWith(
                        color: AppColors.danger,
                        fontWeight: FontWeight.w700,
                      ),
                    ),
                  ),
                ],
              ],
            ),
          ),
        ),
        bottomNavigationBar: HkGlassChromeSurface(
          showBottomBorder: false,
          child: SafeArea(
            top: false,
            child: Padding(
              padding: const EdgeInsets.symmetric(
                horizontal: HkSpace.lg,
                vertical: HkSpace.md,
              ),
              child: Row(
                children: [
                  Expanded(
                    child: SizedBox(
                      height: HkControl.minTap,
                      child: OutlinedButton(
                        onPressed: _busy ? null : () => _leaveForm(context),
                        child: const HkButtonLabel('返回'),
                      ),
                    ),
                  ),
                  const SizedBox(width: HkSpace.md),
                  Expanded(
                    child: SizedBox(
                      height: HkControl.minTap,
                      child: FilledButton(
                        onPressed: _canSubmit ? _submit : null,
                        child: HkButtonLabel(
                          _busy ? _kind.submittingLabel : _kind.submitLabel,
                        ),
                      ),
                    ),
                  ),
                ],
              ),
            ),
          ),
        ),
      ),
    );
  }

  Widget _label(String text) => Text(
    text,
    style: AppTypography.meta.copyWith(
      color: AppColors.textEmphasis,
      fontWeight: FontWeight.w700,
    ),
  );

  Future<PickedEvidence?> _pickFromDevice(BuildContext context) async {
    final source = await showModalBottomSheet<ImageSource>(
      context: context,
      builder: (sheetContext) => SafeArea(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            ListTile(
              leading: const HkIcon(HkIconName.photoAdd),
              title: const Text('拍照'),
              onTap: () => Navigator.pop(sheetContext, ImageSource.camera),
            ),
            ListTile(
              leading: const HkIcon(HkIconName.photoAdd),
              title: const Text('從相簿選擇'),
              onTap: () => Navigator.pop(sheetContext, ImageSource.gallery),
            ),
            ListTile(
              leading: const HkIcon(HkIconName.close),
              title: const Text('取消'),
              onTap: () => Navigator.pop(sheetContext),
            ),
          ],
        ),
      ),
    );
    if (source == null) return null;
    try {
      final image = await ImagePicker().pickImage(
        source: source,
        imageQuality: 80,
      );
      if (image == null) return null;
      final bytes = await File(image.path).readAsBytes();
      return (
        bytes: bytes,
        filename: 'appeal_${DateTime.now().millisecondsSinceEpoch}.jpg',
      );
    } catch (error) {
      HKLog.logW('[MeasureRequestForm] pick image failed: $error');
      if (context.mounted) FloatingToast.error(context, _pickFailedNotice);
      return null;
    }
  }
}

class _PhotoTile extends StatelessWidget {
  const _PhotoTile({required this.photo, required this.onRemove});

  final _Photo photo;
  final VoidCallback? onRemove;

  /// 縮圖的邊長（沒有對應的共用尺寸，只有這個表單用）。
  static const _size = 72.0;
  static const _spinner = 20.0;
  static const _removeIcon = 14.0;

  @override
  Widget build(BuildContext context) {
    final failed = photo.error != null;
    final uploading = photo.token == null && !failed;
    return SizedBox.square(
      dimension: _size,
      child: Stack(
        fit: StackFit.expand,
        children: [
          ClipRRect(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            child: Image.memory(photo.bytes, fit: BoxFit.cover),
          ),
          if (uploading)
            ColoredBox(
              color: AppColors.textEmphasis.withValues(alpha: 0.4),
              child: const Center(
                child: SizedBox.square(
                  dimension: _spinner,
                  child: CircularProgressIndicator(strokeWidth: 2),
                ),
              ),
            ),
          if (failed)
            Semantics(
              label: photo.error,
              child: DecoratedBox(
                decoration: BoxDecoration(
                  color: AppColors.danger.withValues(alpha: 0.35),
                  borderRadius: BorderRadius.circular(HkRadius.sm),
                  border: Border.all(color: AppColors.danger),
                ),
              ),
            ),
          if (onRemove != null)
            Positioned(
              top: 0,
              right: 0,
              child: Semantics(
                button: true,
                label: '移除照片',
                child: InkWell(
                  onTap: onRemove,
                  child: const Padding(
                    padding: EdgeInsets.all(HkSpace.xs),
                    child: DecoratedBox(
                      decoration: BoxDecoration(
                        color: AppColors.textEmphasis,
                        shape: BoxShape.circle,
                      ),
                      child: Padding(
                        padding: EdgeInsets.all(HkSpace.xs / 2),
                        child: HkIcon(
                          HkIconName.close,
                          size: _removeIcon,
                          color: AppIconColors.onBrand,
                        ),
                      ),
                    ),
                  ),
                ),
              ),
            ),
        ],
      ),
    );
  }
}

class _AddPhotoTile extends StatelessWidget {
  const _AddPhotoTile({required this.onTap});

  final VoidCallback? onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      label: '新增照片',
      child: InkWell(
        onTap: onTap,
        borderRadius: BorderRadius.circular(HkRadius.sm),
        child: Container(
          width: _PhotoTile._size,
          height: _PhotoTile._size,
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(HkRadius.sm),
            border: Border.all(color: AppColors.border),
          ),
          child: const Center(
            child: HkIcon(HkIconName.photoAdd, size: HkControl.icon),
          ),
        ),
      ),
    );
  }
}
