import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_api/go_text.dart';
import 'package:hawkr_api/transport.dart' show Code, ConnectException;
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/l10n/ui_locale.dart';
import '../../../core/networking/connect_provider.dart';
import '../../../core/router/app_router.dart';
import '../../../core/session/session_provider.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/utils/hk_log.dart';
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
import '../data/inquiries.dart';
import '../data/inquiry_display.dart';
import 'widgets/evidence_photos.dart';

const _submittedNotice = '回覆已送出';
const _closedTitle = '這則詢問目前不能回覆';
const _closedDescription = '請回到平台詢問查看目前狀態。';

void _leaveReply(BuildContext context, String inquiryId) {
  if (context.canPop()) {
    context.pop();
  } else {
    context.go(SellerRoutes.inquiryDetailFor(inquiryId));
  }
}

/// 回覆詢問的全螢幕表單（路由 [SellerRoutes.inquiryReply]）：內容 1–4000 字，附件最多 `reply_attachment_max` 張（機敏上傳，
/// 送出後賣家只看得到數量）。詢問用 [inquiryByIdProvider] 取得；伺服器的 `can_reply` 為假時顯示說明，不顯示表單。
class SellerInquiryReplyScreen extends ConsumerWidget {
  const SellerInquiryReplyScreen({
    super.key,
    required this.inquiryId,
    this.pickEvidence,
  });

  final String inquiryId;

  /// 測試用：換掉相機／相簿。
  final EvidencePicker? pickEvidence;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final accountId = signedInAccountId(ref.watch(sessionProvider));
    Widget shell(Widget child) => Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: HkGlassAppBar(
        title: const Text(inquiryReplyTitle),
        onBackPressed: () => _leaveReply(context, inquiryId),
      ),
      body: HkAdaptivePage(
        child: ListView(
          padding: const EdgeInsets.symmetric(vertical: HkSpace.lg),
          children: [child],
        ),
      ),
    );
    if (accountId == null) {
      return Scaffold(
        backgroundColor: AppColors.surfacePage,
        body: const TruckLoadingOverlay(),
      );
    }
    final provider = inquiryByIdProvider(accountId, inquiryId);
    final async = ref.watch(provider);
    final loaded = async.value;
    if (loaded == null) {
      if (async.hasError && !async.isLoading) {
        final code = ConnectException.from(async.error!).code;
        if (code == Code.notFound || code == Code.permissionDenied) {
          return shell(
            HkAsyncStatePanel(
              kind: HkAsyncStateKind.empty,
              title: inquiryNotFoundTitle,
              description: inquiryNotFoundDescription,
              actionLabel: inquiryNotFoundAction,
              actionIcon: HkIconName.back,
              onAction: () => context.go(SellerRoutes.inquiries),
            ),
          );
        }
        return shell(
          HkAsyncStatePanel(
            kind: HkAsyncStateKind.error,
            title: inquiriesLoadFailedTitle,
            description: inquiriesLoadFailedDescription,
            actionLabel: '重試',
            onAction: () => ref.invalidate(provider),
          ),
        );
      }
      return Scaffold(
        backgroundColor: AppColors.surfacePage,
        appBar: HkGlassAppBar(
          title: const Text(inquiryReplyTitle),
          onBackPressed: () => _leaveReply(context, inquiryId),
        ),
        body: const TruckLoadingOverlay(),
      );
    }
    if (!loaded.canReply) {
      return shell(
        HkAsyncStatePanel(
          kind: HkAsyncStateKind.empty,
          title: _closedTitle,
          description: _closedDescription,
        ),
      );
    }
    return _ReplyBody(inquiry: loaded, pickEvidence: pickEvidence);
  }
}

class _ReplyBody extends ConsumerStatefulWidget {
  const _ReplyBody({required this.inquiry, this.pickEvidence});

  final Inquiry inquiry;
  final EvidencePicker? pickEvidence;

  @override
  ConsumerState<_ReplyBody> createState() => _ReplyBodyState();
}

class _ReplyBodyState extends ConsumerState<_ReplyBody> {
  final _bodyCtrl = TextEditingController();
  final _photos = <EvidencePhoto>[];
  var _nextPhotoId = 0;
  var _busy = false;
  String? _notice;
  String? _bodyError;
  String? _evidenceError;

  int get _maxEvidence => widget.inquiry.replyAttachmentMax;

  @override
  void dispose() {
    _bodyCtrl.dispose();
    super.dispose();
  }

  bool get _uploading =>
      _photos.any((photo) => photo.token == null && photo.error == null);

  bool get _hasFailedPhoto => _photos.any((photo) => photo.error != null);

  int get _bodyLength => goTrimmedRuneCount(_bodyCtrl.text);

  bool get _bodyTooLong => _bodyLength > inquiryReplyMaxLength;

  bool get _canSubmit =>
      _bodyLength > 0 &&
      !_bodyTooLong &&
      !_uploading &&
      !_hasFailedPhoto &&
      !_busy;

  Future<void> _addPhoto() async {
    if (_photos.length >= _maxEvidence || _busy) return;
    final picked =
        await (widget.pickEvidence ??
            (context) => pickEvidenceFromDevice(
              context,
              filenamePrefix: 'inquiry',
            ))(context);
    if (picked == null || !mounted || _busy) return;
    if (_photos.length >= _maxEvidence) return;
    final photo = EvidencePhoto(_nextPhotoId++, picked.bytes);
    setState(() {
      _photos.add(photo);
      _evidenceError = null;
    });
    final String token;
    try {
      token = await ref.read(privateDocumentUploaderProvider)(
        picked.bytes,
        inquiryEvidenceDocType,
        picked.filename,
      );
    } catch (error) {
      HKLog.logW('[InquiryReply] evidence upload failed: $error');
      if (!mounted || !_photos.contains(photo)) return;
      setState(() => _photos.remove(photo));
      FloatingToast.error(context, evidenceUploadFailedNotice);
      return;
    }
    if (!mounted || !_photos.contains(photo)) return;
    setState(() => photo.token = token);
  }

  void _removePhoto(EvidencePhoto photo) {
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

  void _adopt(Inquiry latest) {
    final accountId = signedInAccountId(ref.read(sessionProvider));
    if (accountId == null) return;
    final list = myInquiriesProvider(accountId);
    if (ref.exists(list)) ref.read(list.notifier).adopt(latest);
    final detail = inquiryByIdProvider(accountId, latest.id);
    if (ref.exists(detail)) ref.invalidate(detail);
    ref.invalidate(pendingInquiryCountProvider(accountId));
  }

  Future<void> _submit() async {
    if (!_canSubmit) return;
    setState(() {
      _busy = true;
      _notice = null;
      _bodyError = null;
      _evidenceError = null;
    });
    final inquiry = widget.inquiry;
    final result = await submitInquiryReply(
      ref.read(sellerClientProvider),
      inquiry: inquiry,
      body: _bodyCtrl.text,
      evidenceTokens: [for (final photo in _photos) photo.token!],
      defaults: ref.read(defaultErrorTextProvider),
    );
    if (!mounted) return;
    switch (result) {
      case InquiryReplySubmitted(inquiry: final latest):
        _adopt(latest);
        FloatingToast.success(context, _submittedNotice);
        _leaveReply(context, inquiry.id);
      case InquiryReplyClosed(:final latest, :final message):
        if (latest != null) _adopt(latest);
        FloatingToast.error(context, message);
        _leaveReply(context, inquiry.id);
      case InquiryReplyNotFound(:final message):
        FloatingToast.error(context, message);
        context.go(SellerRoutes.inquiries);
      case InquiryReplyBodyInvalid(:final message):
        setState(() {
          _bodyError = message;
          _busy = false;
        });
      case InquiryReplyEvidenceInvalid(
        :final byIndex,
        :final evidenceMessage,
        :final bodyMessage,
      ):
        final submitted = List.of(_photos);
        setState(() {
          for (final MapEntry(key: index, value: message) in byIndex.entries) {
            if (index >= submitted.length) continue;
            submitted[index]
              ..token = null
              ..error = message;
          }
          _bodyError = bodyMessage;
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
      case InquiryReplyFailed(:final message):
        setState(() {
          _notice = message;
          _busy = false;
        });
    }
  }

  Widget _label(String text) => Text(
    text,
    style: AppTypography.meta.copyWith(
      color: AppColors.textEmphasis,
      fontWeight: FontWeight.w700,
    ),
  );

  @override
  Widget build(BuildContext context) {
    final inquiry = widget.inquiry;
    return PopScope(
      canPop: !_busy,
      child: Scaffold(
        backgroundColor: AppColors.surfacePage,
        appBar: HkGlassAppBar(
          title: const Text(inquiryReplyTitle),
          onBackPressed: _busy ? null : () => _leaveReply(context, inquiry.id),
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
                  child: Text(inquiry.message, style: AppTypography.meta),
                ),
                const SizedBox(height: HkSpace.xl),
                _label('回覆內容 *'),
                const SizedBox(height: HkSpace.sm),
                TextField(
                  key: const ValueKey('inquiry-reply-input'),
                  controller: _bodyCtrl,
                  enabled: !_busy,
                  minLines: 5,
                  maxLines: 8,
                  keyboardType: TextInputType.multiline,
                  onChanged: (_) => setState(() => _bodyError = null),
                  decoration: InputDecoration(
                    hintText: '請依平台的要求提供資料或說明。',
                    errorText:
                        _bodyError ??
                        (_bodyTooLong
                            ? '回覆內容最多 $inquiryReplyMaxLength 字'
                            : null),
                  ),
                ),
                const SizedBox(height: HkSpace.xs),
                Align(
                  alignment: Alignment.centerRight,
                  child: Text(
                    '$_bodyLength / $inquiryReplyMaxLength',
                    key: const ValueKey('inquiry-reply-counter'),
                    style: AppTypography.caption.copyWith(
                      color: _bodyTooLong
                          ? AppColors.danger
                          : AppColors.textMuted,
                    ),
                  ),
                ),
                if (_maxEvidence > 0) ...[
                  const SizedBox(height: HkSpace.xl),
                  _label('附件圖片（選填）'),
                  const SizedBox(height: HkSpace.sm),
                  Wrap(
                    spacing: HkSpace.sm,
                    runSpacing: HkSpace.sm,
                    children: [
                      for (final photo in _photos)
                        EvidencePhotoTile(
                          photo: photo,
                          onRemove: _busy ? null : () => _removePhoto(photo),
                        ),
                      if (_photos.length < _maxEvidence)
                        AddEvidencePhotoTile(onTap: _busy ? null : _addPhoto),
                    ],
                  ),
                  const SizedBox(height: HkSpace.xs),
                  Text(
                    '最多 $_maxEvidence 張照片，送出後只有平台看得到',
                    style: AppTypography.caption.copyWith(
                      color: AppColors.textMuted,
                    ),
                  ),
                ],
                if (_evidenceError != null) ...[
                  const SizedBox(height: HkSpace.xs),
                  Semantics(
                    liveRegion: true,
                    child: Text(
                      _evidenceError!,
                      key: const ValueKey('inquiry-evidence-error'),
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
                      key: const ValueKey('inquiry-reply-notice'),
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
                        onPressed: _busy
                            ? null
                            : () => _leaveReply(context, inquiry.id),
                        child: const HkButtonLabel('返回'),
                      ),
                    ),
                  ),
                  const SizedBox(width: HkSpace.md),
                  Expanded(
                    child: SizedBox(
                      height: HkControl.minTap,
                      child: FilledButton(
                        key: const ValueKey('inquiry-reply-submit'),
                        onPressed: _canSubmit ? _submit : null,
                        child: HkButtonLabel(_busy ? '回覆送出中…' : '送出回覆'),
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
}
