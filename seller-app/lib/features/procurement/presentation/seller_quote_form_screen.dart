import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/codemeta/code_meta.dart';
import '../../../core/theme/app_colors.dart';
import '../../../core/theme/app_typography.dart';
import '../../../core/theme/hk_tokens.dart';
import '../../../shared/icons/app_icon_colors.dart';
import '../../../shared/icons/app_icons.dart';
import '../../../shared/widgets/hk_button_label.dart';
import '../../../shared/widgets/hk_choice_chip.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../data/seller_quote_model.dart';
import 'seller_quote_controller.dart';
import 'seller_quote_labels.dart';
import 'seller_quote_validation.dart';

/// 平台賣家的回價表單（Hawkr Agent 採購線，A14 PLATFORM 半邊）。
///
/// 買家發 RFQ publication → 賣家在這裡回一個 immutable 報價版本 → 買家接受
/// exact version。三條不得違反的產品規則都落在這支畫面上：
///
/// 1. ⛔ 實際品相（[OfferedQuality]）只能由賣家自己填，沒有「帶入上次報價」的
///    捷徑，未填就擋在送出之前。
/// 2. ⛔ 兩個 `responseType` 的必填欄位完全相同；選了
///    `ACCEPT_NON_PRICE_TERMS_WITH_QUOTE` 也不會隱藏或跳過任何金額欄位。
/// 3. ⛔ 畫面只顯示 [SellerFacingPublication.visibleContextFields] 白名單內的
///    需求欄位——買家預算、買家 PII、其他賣家的報價與候選排序永遠不會出現。
class SellerQuoteFormScreen extends ConsumerWidget {
  const SellerQuoteFormScreen({super.key, required this.publicationId});

  final String publicationId;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final asyncState = ref.watch(sellerQuoteFormProvider(publicationId));

    return Scaffold(
      backgroundColor: AppColors.bgApp,
      appBar: AppBar(
        backgroundColor: AppColors.topBarBackground,
        title: const Text('回覆詢價'),
      ),
      body: SafeArea(
        child: asyncState.when(
          loading: () => const Center(child: CircularProgressIndicator()),
          error: (error, _) => _LoadFailure(
            message: error is Exception ? '$error' : '載入詢價內容失敗',
            onRetry: () =>
                ref.invalidate(sellerQuoteFormProvider(publicationId)),
          ),
          data: (state) => _SellerQuoteFormBody(
            key: ValueKey('seller-quote-$publicationId'),
            publicationId: publicationId,
            initial: state,
          ),
        ),
      ),
    );
  }
}

class _LoadFailure extends StatelessWidget {
  const _LoadFailure({required this.message, required this.onRetry});

  final String message;
  final VoidCallback onRetry;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(HkSpace.xl),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(
              message,
              style: AppTypography.body,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: HkSpace.lg),
            FilledButton(
              onPressed: onRetry,
              child: const HkButtonLabel('重新載入'),
            ),
          ],
        ),
      ),
    );
  }
}

class _SellerQuoteFormBody extends HookConsumerWidget {
  const _SellerQuoteFormBody({
    super.key,
    required this.publicationId,
    required this.initial,
  });

  final String publicationId;
  final SellerQuoteFormState initial;

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final state =
        ref.watch(sellerQuoteFormProvider(publicationId)).value ?? initial;
    final notifier = ref.read(sellerQuoteFormProvider(publicationId).notifier);
    final quote = state.quote;
    final validation = validateSellerQuote(quote);
    final showErrors = state.submitAttempted;

    String? errorFor(SellerQuoteField field) =>
        showErrors ? validation.errorFor(field) : null;

    return ListView(
      padding: const EdgeInsets.fromLTRB(
        HkSpace.lg,
        HkSpace.lg,
        HkSpace.lg,
        HkSpace.sectionLarge,
      ),
      children: [
        if (state.notice != null)
          _NoticeBanner(
            notice: state.notice!,
            onReload: notifier.reload,
            onDismiss: notifier.dismissNotice,
          ),
        _PublicationContextCard(payload: state.sellerFacingPayload),
        const SizedBox(height: HkSpace.lg),
        _ResponseTypeCard(
          selected: quote.responseTypeValue,
          error: errorFor(SellerQuoteField.responseType),
          onChanged: (value) =>
              notifier.updateQuote((q) => q.copyWith(responseType: value.code)),
        ),
        const SizedBox(height: HkSpace.lg),
        _QualityCard(
          quality: quote.offeredQuality,
          gradeError: errorFor(SellerQuoteField.qualityGrade),
          notesError: errorFor(SellerQuoteField.qualityInspectionNotes),
          onGrade: (value) => notifier.updateQuote(
            (q) => q.copyWith(
              offeredQuality: q.offeredQuality.copyWith(grade: value),
            ),
          ),
          onNotes: (value) => notifier.updateQuote(
            (q) => q.copyWith(
              offeredQuality: q.offeredQuality.copyWith(inspectionNotes: value),
            ),
          ),
        ),
        const SizedBox(height: HkSpace.lg),
        _QuantityCard(
          quote: quote,
          errorFor: errorFor,
          onChanged: notifier.updateQuote,
        ),
        const SizedBox(height: HkSpace.lg),
        _AmountCard(
          quote: quote,
          validation: validation,
          errorFor: errorFor,
          onChanged: notifier.updateQuote,
        ),
        const SizedBox(height: HkSpace.lg),
        _ScheduleCard(
          quote: quote,
          errorFor: errorFor,
          onChanged: notifier.updateQuote,
        ),
        const SizedBox(height: HkSpace.lg),
        _NotesCard(quote: quote, onChanged: notifier.updateQuote),
        const SizedBox(height: HkSpace.xl),
        _ActionBar(state: state, notifier: notifier),
      ],
    );
  }
}

// -----------------------------------------------------------------------------
// 橫幅
// -----------------------------------------------------------------------------

class _NoticeBanner extends StatelessWidget {
  const _NoticeBanner({
    required this.notice,
    required this.onReload,
    required this.onDismiss,
  });

  final SellerQuoteNotice notice;
  final Future<void> Function() onReload;
  final VoidCallback onDismiss;

  Color get _background => switch (notice.tone) {
    SellerQuoteNoticeTone.success => AppColors.successSoft,
    SellerQuoteNoticeTone.warning => AppColors.warningSoft,
    SellerQuoteNoticeTone.error => AppColors.dangerSoft,
    SellerQuoteNoticeTone.info => AppColors.infoSoft,
  };

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(bottom: HkSpace.lg),
      child: HkContentCard(
        backgroundColor: _background,
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  child: Text(notice.message, style: AppTypography.meta),
                ),
                if (!notice.requiresReload)
                  IconButton(
                    tooltip: '關閉提示',
                    onPressed: onDismiss,
                    icon: const Icon(AppIcons.close),
                  ),
              ],
            ),
            // ⛔ 依據已過期時只給「重新載入」這一條路：不得靜默改成送最新版。
            if (notice.requiresReload) ...[
              const SizedBox(height: HkSpace.sm),
              Align(
                alignment: Alignment.centerLeft,
                child: FilledButton(
                  onPressed: () => onReload(),
                  child: const HkButtonLabel('重新載入最新版本'),
                ),
              ),
            ],
          ],
        ),
      ),
    );
  }
}

// -----------------------------------------------------------------------------
// 需求摘要（白名單）
// -----------------------------------------------------------------------------

class _PublicationContextCard extends StatelessWidget {
  const _PublicationContextCard({required this.payload});

  final SellerFacingPayload payload;

  @override
  Widget build(BuildContext context) {
    final fields = payload.contextFields;
    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('買家需求', style: AppTypography.sectionTitle),
          const SizedBox(height: HkSpace.sm),
          if (fields.isEmpty)
            Text('這則詢價沒有填寫需求說明。', style: AppTypography.metaMuted)
          else
            ...fields.map(
              (field) => Padding(
                padding: const EdgeInsets.only(bottom: HkSpace.sm),
                child: Row(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    SizedBox(
                      width: 96,
                      child: Text(field.label, style: AppTypography.metaMuted),
                    ),
                    Expanded(
                      child: Text(field.value, style: AppTypography.meta),
                    ),
                  ],
                ),
              ),
            ),
          if (payload.hasBuyerNote) ...[
            const SizedBox(height: HkSpace.sm),
            _BuyerNoteBlock(note: payload.buyerNote),
          ],
          const SizedBox(height: HkSpace.sm),
          _QualityKnowledgeNotice(payload: payload),
        ],
      ),
    );
  }
}

/// 買家自己打的一段補充說明，原文照登。
///
/// ⛔ 不進 contextFields：那是「標籤＋短值」的結構化事實，一段兩百字的自由文字
/// 塞進 96px 標籤旁邊的 Expanded 會擠成一團。⛔ 也不是品相宣告——實際品相仍然
/// 只能由賣家自己填，下面那張卡片講的話不因為這裡有字而改變。
class _BuyerNoteBlock extends StatelessWidget {
  const _BuyerNoteBlock({required this.note});

  final String note;

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text('買家的補充說明', style: AppTypography.metaMuted),
        const SizedBox(height: HkSpace.xs),
        Text(note, style: AppTypography.meta),
      ],
    );
  }
}

/// 買家有沒有指定品相。
///
/// ⛔ 空字串是「未知」（這張詢價建立於本欄位加入之前），畫面照實說未知，
/// **不補值也不推斷成「買家沒指定」**。不論哪一種，實際品相都必須由賣家自己填，
/// 所以這張卡片直接把話講到底：不要自己假設一個等級去報價。
class _QualityKnowledgeNotice extends StatelessWidget {
  const _QualityKnowledgeNotice({required this.payload});

  final SellerFacingPayload payload;

  @override
  Widget build(BuildContext context) {
    final known = payload.hasQualityKnowledge;
    return Container(
      width: double.infinity,
      padding: const EdgeInsets.all(HkSpace.md),
      decoration: BoxDecoration(
        color: AppColors.infoSoft,
        borderRadius: BorderRadius.circular(HkRadius.md),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(
            known ? '買家品相說明：${payload.qualityKnowledgeState}' : '買家品相說明：未知',
            style: AppTypography.meta,
          ),
          const SizedBox(height: HkSpace.xs),
          Text(
            known
                ? '這只是買家的說明。實際品相仍要由你自己填，下面兩欄不能省。'
                : '這張詢價沒有買家的品相說明，平台也不會替你猜。'
                      '不要自己假設一個等級去報價——實際品相請照這批貨的真實狀況填。',
            style: AppTypography.metaMuted,
          ),
        ],
      ),
    );
  }
}

// -----------------------------------------------------------------------------
// 回應方式
// -----------------------------------------------------------------------------

class _ResponseTypeCard extends StatelessWidget {
  const _ResponseTypeCard({
    required this.selected,
    required this.error,
    required this.onChanged,
  });

  final ProcurementSellerResponseType? selected;
  final String? error;
  final ValueChanged<ProcurementSellerResponseType> onChanged;

  @override
  Widget build(BuildContext context) {
    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('回應方式', style: AppTypography.sectionTitle),
          const SizedBox(height: HkSpace.xs),
          // ⛔ 兩個值的必填欄位完全相同，UI 不得在選了第二個之後少問任何金額。
          Text(
            '兩種回應都要填完整的品相、計價基準與金額——「接受非價格條款」不是少填金額的捷徑。',
            style: AppTypography.metaMuted,
          ),
          const SizedBox(height: HkSpace.md),
          ...ProcurementSellerResponseType.values.map(
            (type) => _RadioTile(
              label: type.formLabel,
              value: type.code,
              selected: selected == type,
              onTap: () => onChanged(type),
            ),
          ),
          if (error != null) _FieldError(error!),
        ],
      ),
    );
  }
}

class _RadioTile extends StatelessWidget {
  const _RadioTile({
    required this.label,
    required this.value,
    required this.selected,
    required this.onTap,
  });

  final String label;
  final String value;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      button: true,
      selected: selected,
      child: InkWell(
        onTap: onTap,
        child: Padding(
          padding: const EdgeInsets.symmetric(vertical: HkSpace.md),
          child: Row(
            children: [
              Icon(
                selected
                    ? AppIcons.radioButtonChecked
                    : AppIcons.radioButtonUnchecked,
                color: selected ? AppIconColors.active : AppIconColors.muted,
              ),
              const SizedBox(width: HkSpace.md),
              Expanded(child: Text(label, style: AppTypography.body)),
            ],
          ),
        ),
      ),
    );
  }
}

// -----------------------------------------------------------------------------
// 實際品相
// -----------------------------------------------------------------------------

class _QualityCard extends StatelessWidget {
  const _QualityCard({
    required this.quality,
    required this.gradeError,
    required this.notesError,
    required this.onGrade,
    required this.onNotes,
  });

  final OfferedQuality quality;
  final String? gradeError;
  final String? notesError;
  final ValueChanged<String> onGrade;
  final ValueChanged<String> onNotes;

  @override
  Widget build(BuildContext context) {
    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('實際品相與驗收（必填）', style: AppTypography.sectionTitle),
          const SizedBox(height: HkSpace.xs),
          // ⛔ 不得由平台、LLM 或 Pricebook 代填，也沒有「帶入上次報價」。
          Text('這兩欄只能由你自己填寫，平台不會替你帶入上一次報價的品相。', style: AppTypography.metaMuted),
          const SizedBox(height: HkSpace.md),
          _QuoteTextField(
            label: '品相等級',
            hint: '例如：特級、A 級、次級可用',
            initialValue: quality.grade,
            error: gradeError,
            onChanged: onGrade,
          ),
          _QuoteTextField(
            label: '驗收說明',
            hint: '例如：外觀無壓傷，糖度 11 以上，到貨後 24 小時內驗收',
            initialValue: quality.inspectionNotes,
            error: notesError,
            maxLines: 3,
            onChanged: onNotes,
          ),
        ],
      ),
    );
  }
}

// -----------------------------------------------------------------------------
// 數量與計價基準
// -----------------------------------------------------------------------------

typedef _QuoteEdit =
    void Function(SellerQuotePayload Function(SellerQuotePayload));
typedef _FieldErrorLookup = String? Function(SellerQuoteField);

class _QuantityCard extends StatelessWidget {
  const _QuantityCard({
    required this.quote,
    required this.errorFor,
    required this.onChanged,
  });

  final SellerQuotePayload quote;
  final _FieldErrorLookup errorFor;
  final _QuoteEdit onChanged;

  @override
  Widget build(BuildContext context) {
    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('數量與計價基準', style: AppTypography.sectionTitle),
          const SizedBox(height: HkSpace.md),
          _QuoteTextField(
            label: '計價基準數量',
            hint: '例如：1',
            initialValue: quote.unitBasis.quantity,
            error: errorFor(SellerQuoteField.unitBasisQuantity),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            onChanged: (value) => onChanged(
              (q) =>
                  q.copyWith(unitBasis: q.unitBasis.copyWith(quantity: value)),
            ),
          ),
          _QuoteTextField(
            label: '計價基準單位',
            hint: '例如：公斤',
            initialValue: quote.unitBasis.unit,
            error: errorFor(SellerQuoteField.unitBasisUnit),
            onChanged: (value) => onChanged(
              (q) => q.copyWith(unitBasis: q.unitBasis.copyWith(unit: value)),
            ),
          ),
          _QuoteTextField(
            label: '供應數量',
            hint: '例如：500',
            initialValue: quote.offeredQuantity,
            error: errorFor(SellerQuoteField.offeredQuantity),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            onChanged: (value) =>
                onChanged((q) => q.copyWith(offeredQuantity: value)),
          ),
          _QuoteTextField(
            label: '供應數量單位',
            hint: '例如：公斤',
            initialValue: quote.offeredQuantityUnit,
            error: errorFor(SellerQuoteField.offeredQuantityUnit),
            onChanged: (value) =>
                onChanged((q) => q.copyWith(offeredQuantityUnit: value)),
          ),
          _QuoteTextField(
            label: '換算基準（選填）',
            hint: '例如：1 箱 ＝ 10 公斤',
            initialValue: quote.conversionBasis,
            onChanged: (value) =>
                onChanged((q) => q.copyWith(conversionBasis: value)),
          ),
        ],
      ),
    );
  }
}

// -----------------------------------------------------------------------------
// 金額
// -----------------------------------------------------------------------------

class _AmountCard extends StatelessWidget {
  const _AmountCard({
    required this.quote,
    required this.validation,
    required this.errorFor,
    required this.onChanged,
  });

  final SellerQuotePayload quote;
  final SellerQuoteValidation validation;
  final _FieldErrorLookup errorFor;
  final _QuoteEdit onChanged;

  @override
  Widget build(BuildContext context) {
    final computed = validation.computedTotal;
    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Expanded(child: Text('金額', style: AppTypography.sectionTitle)),
              Text('幣別 ${quote.currency}', style: AppTypography.metaMuted),
            ],
          ),
          const SizedBox(height: HkSpace.md),
          _QuoteTextField(
            label: '貨款',
            initialValue: quote.goodsAmount,
            error: errorFor(SellerQuoteField.goodsAmount),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            onChanged: (value) =>
                onChanged((q) => q.copyWith(goodsAmount: value)),
          ),
          _QuoteTextField(
            label: '運費',
            initialValue: quote.shippingAmount,
            error: errorFor(SellerQuoteField.shippingAmount),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            onChanged: (value) =>
                onChanged((q) => q.copyWith(shippingAmount: value)),
          ),
          _QuoteTextField(
            label: '稅額',
            initialValue: quote.taxAmount,
            error: errorFor(SellerQuoteField.taxAmount),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            onChanged: (value) =>
                onChanged((q) => q.copyWith(taxAmount: value)),
          ),
          const SizedBox(height: HkSpace.sm),
          Text('含稅基準', style: AppTypography.metaMuted),
          const SizedBox(height: HkSpace.sm),
          Wrap(
            spacing: HkSpace.sm,
            children: ProcurementTaxInclusionBasis.values
                .map(
                  (basis) => HkChoiceChip(
                    label: basis.formLabel,
                    selected: quote.taxInclusionBasisValue == basis,
                    onTap: () => onChanged(
                      (q) => q.copyWith(taxInclusionBasis: basis.code),
                    ),
                  ),
                )
                .toList(),
          ),
          if (errorFor(SellerQuoteField.taxInclusionBasis) != null)
            _FieldError(errorFor(SellerQuoteField.taxInclusionBasis)!),
          const SizedBox(height: HkSpace.md),
          _QuoteTextField(
            label: '總金額',
            initialValue: quote.totalAmount,
            error: errorFor(SellerQuoteField.totalAmount),
            keyboardType: const TextInputType.numberWithOptions(decimal: true),
            onChanged: (value) =>
                onChanged((q) => q.copyWith(totalAmount: value)),
          ),
          // ⛔ 只提示，不覆寫。使用者填什麼就送什麼，server 才是權威。
          if (computed != null)
            Text(
              validation.amountIdentityHolds
                  ? '貨款＋運費＋稅額 ＝ $computed，與總金額一致。'
                  : '貨款＋運費＋稅額 ＝ $computed，與你填的總金額不一致。',
              style: AppTypography.metaMuted.copyWith(
                color: validation.amountIdentityHolds
                    ? AppColors.success
                    : AppColors.error,
              ),
            ),
        ],
      ),
    );
  }
}

// -----------------------------------------------------------------------------
// 交期與有效期
// -----------------------------------------------------------------------------

class _ScheduleCard extends StatelessWidget {
  const _ScheduleCard({
    required this.quote,
    required this.errorFor,
    required this.onChanged,
  });

  final SellerQuotePayload quote;
  final _FieldErrorLookup errorFor;
  final _QuoteEdit onChanged;

  @override
  Widget build(BuildContext context) {
    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('交期與有效期', style: AppTypography.sectionTitle),
          const SizedBox(height: HkSpace.md),
          _QuoteTextField(
            label: '備貨天數（選填）',
            hint: '例如：3',
            initialValue: quote.leadTimeDays,
            keyboardType: TextInputType.number,
            onChanged: (value) =>
                onChanged((q) => q.copyWith(leadTimeDays: value)),
          ),
          _DateTimeField(
            label: '承諾交貨時間（選填）',
            value: quote.promisedDeliveryAt,
            error: errorFor(SellerQuoteField.promisedDeliveryAt),
            onChanged: (value) =>
                onChanged((q) => q.copyWith(promisedDeliveryAt: value)),
          ),
          _DateTimeField(
            label: '報價有效期限',
            value: quote.validUntil,
            error: errorFor(SellerQuoteField.validUntil),
            onChanged: (value) =>
                onChanged((q) => q.copyWith(validUntil: value)),
          ),
        ],
      ),
    );
  }
}

// -----------------------------------------------------------------------------
// 備註
// -----------------------------------------------------------------------------

class _NotesCard extends StatelessWidget {
  const _NotesCard({required this.quote, required this.onChanged});

  final SellerQuotePayload quote;
  final _QuoteEdit onChanged;

  @override
  Widget build(BuildContext context) {
    return HkContentCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text('條款與偏離揭露', style: AppTypography.sectionTitle),
          const SizedBox(height: HkSpace.md),
          _QuoteTextField(
            label: '條款說明（選填）',
            initialValue: quote.termsNote,
            maxLines: 3,
            onChanged: (value) =>
                onChanged((q) => q.copyWith(termsNote: value)),
          ),
          _QuoteTextField(
            label: '代換／偏離揭露（無偏離請留空）',
            hint: '例如：以同產區同等級品項代換',
            initialValue: quote.deviationDisclosure,
            maxLines: 3,
            onChanged: (value) =>
                onChanged((q) => q.copyWith(deviationDisclosure: value)),
          ),
        ],
      ),
    );
  }
}

// -----------------------------------------------------------------------------
// 動作列
// -----------------------------------------------------------------------------

class _ActionBar extends StatelessWidget {
  const _ActionBar({required this.state, required this.notifier});

  final SellerQuoteFormState state;
  final SellerQuoteForm notifier;

  @override
  Widget build(BuildContext context) {
    final blocked = state.busy || state.reloadRequired;
    final response = state.latestResponse;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (response != null)
          Padding(
            padding: const EdgeInsets.only(bottom: HkSpace.md),
            child: Text(
              '目前是第 ${response.sellerResponseVersion} 版'
              '（${response.statusValue?.label ?? response.status}）'
              '${response.isLatestVersion ? '' : '，已被第 ${response.supersededByVersion} 版取代'}',
              style: AppTypography.metaMuted,
            ),
          ),
        OutlinedButton(
          onPressed: blocked ? null : () => notifier.saveDraft(),
          child: const HkButtonLabel('儲存草稿'),
        ),
        const SizedBox(height: HkSpace.md),
        FilledButton(
          onPressed: blocked ? null : () => notifier.submit(),
          child: HkButtonLabel(state.isRevision ? '送出改版' : '送出正式回價'),
        ),
        const SizedBox(height: HkSpace.md),
        if (response != null &&
            response.statusValue == ProcurementSellerResponseStatus.active)
          OutlinedButton(
            onPressed: blocked
                ? null
                : () => _confirm(
                    context,
                    title: '撤回這一版報價？',
                    body: '撤回後買家就看不到這一版報價。',
                    onConfirm: notifier.withdraw,
                  ),
            child: const HkButtonLabel('撤回這一版報價'),
          ),
        const SizedBox(height: HkSpace.md),
        TextButton(
          onPressed: blocked
              ? null
              : () => _confirm(
                  context,
                  title: '拒絕這則詢價？',
                  body: '拒絕後這則詢價就不再向你收件。',
                  onConfirm: () => notifier.decline(),
                ),
          child: const HkButtonLabel('拒絕這則詢價'),
        ),
      ],
    );
  }

  Future<void> _confirm(
    BuildContext context, {
    required String title,
    required String body,
    required Future<bool> Function() onConfirm,
  }) async {
    final agreed = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: Text(title),
        content: Text(body),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(dialogContext).pop(false),
            child: const HkButtonLabel('取消'),
          ),
          FilledButton(
            onPressed: () => Navigator.of(dialogContext).pop(true),
            child: const HkButtonLabel('確認'),
          ),
        ],
      ),
    );
    if (agreed == true) await onConfirm();
  }
}

// -----------------------------------------------------------------------------
// 欄位元件
// -----------------------------------------------------------------------------

class _QuoteTextField extends HookWidget {
  const _QuoteTextField({
    required this.label,
    required this.initialValue,
    required this.onChanged,
    this.hint,
    this.error,
    this.maxLines = 1,
    this.keyboardType,
  });

  final String label;
  final String initialValue;
  final ValueChanged<String> onChanged;
  final String? hint;
  final String? error;
  final int maxLines;
  final TextInputType? keyboardType;

  @override
  Widget build(BuildContext context) {
    final controller = useTextEditingController(text: initialValue);
    return Padding(
      padding: const EdgeInsets.only(bottom: HkSpace.md),
      child: TextField(
        controller: controller,
        maxLines: maxLines,
        keyboardType: keyboardType,
        inputFormatters: keyboardType == null
            ? null
            : [FilteringTextInputFormatter.allow(RegExp(r'[0-9.]'))],
        decoration: InputDecoration(
          labelText: label,
          hintText: hint,
          errorText: error,
        ),
        onChanged: onChanged,
      ),
    );
  }
}

class _DateTimeField extends StatelessWidget {
  const _DateTimeField({
    required this.label,
    required this.value,
    required this.onChanged,
    this.error,
  });

  final String label;

  /// RFC3339 UTC 字串；空字串 ＝ 未設定。
  final String value;
  final ValueChanged<String> onChanged;
  final String? error;

  @override
  Widget build(BuildContext context) {
    final parsed = value.isEmpty ? null : DateTime.tryParse(value)?.toLocal();
    return Padding(
      padding: const EdgeInsets.only(bottom: HkSpace.md),
      child: InkWell(
        onTap: () => _pick(context, parsed),
        child: InputDecorator(
          decoration: InputDecoration(labelText: label, errorText: error),
          child: Text(
            parsed == null ? '尚未設定' : _formatLocal(parsed),
            style: AppTypography.body,
          ),
        ),
      ),
    );
  }

  static String _formatLocal(DateTime value) {
    String two(int n) => n.toString().padLeft(2, '0');
    return '${value.year}-${two(value.month)}-${two(value.day)} '
        '${two(value.hour)}:${two(value.minute)}';
  }

  Future<void> _pick(BuildContext context, DateTime? current) async {
    final now = DateTime.now();
    final base = current ?? now.add(const Duration(days: 1));
    final date = await showDatePicker(
      context: context,
      initialDate: base,
      firstDate: DateTime(now.year - 1),
      lastDate: DateTime(now.year + 3),
    );
    if (date == null || !context.mounted) return;
    final time = await showTimePicker(
      context: context,
      initialTime: TimeOfDay.fromDateTime(base),
    );
    if (time == null) return;
    onChanged(
      toRfc3339Utc(
        DateTime(date.year, date.month, date.day, time.hour, time.minute),
      ),
    );
  }
}

class _FieldError extends StatelessWidget {
  const _FieldError(this.message);

  final String message;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.only(top: HkSpace.xs),
      child: Text(
        message,
        style: AppTypography.metaMuted.copyWith(color: AppColors.error),
      ),
    );
  }
}
