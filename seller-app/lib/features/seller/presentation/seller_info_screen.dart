import 'dart:async';

import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/core/theme/app_colors.dart';
import 'package:hawkr_seller/core/theme/hk_tokens.dart';
import 'package:hawkr_seller/shared/icons/app_icon_colors.dart';
import 'package:hawkr_seller/shared/widgets/hk_button_label.dart';
import 'package:flutter/material.dart';
import 'package:flutter_svg/flutter_svg.dart';
import 'package:flutter_hooks/flutter_hooks.dart';
import 'package:go_router/go_router.dart';
import 'package:hooks_riverpod/hooks_riverpod.dart';

import '../../../core/analytics/analytics_providers.dart';
import '../../../core/router/app_router.dart';
import '../../../core/session/session_provider.dart';
import '../../../shared/utils/floating_toast.dart';
import '../../../shared/widgets/hk_adaptive_page.dart';
import '../../../shared/widgets/hk_content_card.dart';
import '../../../shared/widgets/hk_glass_app_bar.dart';
import '../../../shared/widgets/hk_glass_chrome_surface.dart';
import '../../../shared/widgets/hk_icon.dart';
import '../../../shared/widgets/hk_page_section.dart';
import '../../../shared/widgets/location_icon.dart';
import '../data/account_scope.dart';
import '../data/current_store.dart';
import '../data/seller_agreements_repository.dart';
import '../data/seller_onboarding.dart';
import '../data/version_conflict.dart';
import '../data/measures.dart';
import 'town_picker.dart';
import 'widgets/agreement_checkbox.dart';
import 'widgets/option_row.dart';

class SellerInformationScreen extends HookConsumerWidget {
  const SellerInformationScreen({super.key});

  @override
  Widget build(BuildContext context, WidgetRef ref) {
    final snapshot = ref.watch(myStoreSnapshotProvider);
    final storeNameCtrl = useTextEditingController(
      text: snapshot?.storeName ?? '',
    );
    final addressDetailCtrl = useTextEditingController();
    final canSubmit = useState(false);
    final isSubmitting = useState(false);
    final countyCode = useState('');
    final townCode = useState('');
    final termsAccepted = useState(false);
    final submitRefusal = useState<StoreOpeningBlock?>(null);

    // 有措施紀錄才顯示入口；讀取失敗或沒有資料都不顯示，也不擋開店。
    final accountId = signedInAccountId(ref.watch(sessionProvider));
    final hasMeasures = accountId == null
        ? false
        : ref.watch(accountHasMeasuresProvider(accountId)).value ?? false;

    final needsTerms = snapshot?.hasStore != true;
    // 還沒有店家才問能不能開店（有店家走更新流程）；讀取失敗不擋，送出時由 ApplyStore 的 ErrorReason 後備。
    final opening = needsTerms && accountId != null
        ? ref.watch(storeOpeningBlockForProvider(accountId))
        : null;
    final openingBlock = opening?.value;
    final openingLoading = opening != null && opening.isLoading;
    final terms = needsTerms ? ref.watch(sellerTermsProvider) : null;
    final agreement = terms != null && !terms.isLoading && !terms.hasError
        ? terms.value
        : null;

    bool inputReady() {
      final hasName = storeNameCtrl.text.trim().isNotEmpty;
      final hasAddress =
          townCode.value.isNotEmpty && addressDetailCtrl.text.trim().isNotEmpty;
      final termsReady =
          !needsTerms || (termsAccepted.value && agreement != null);
      return hasName && hasAddress && termsReady;
    }

    // 顯示的拒絕：開店可用性的答案有句子就用它，否則是送出被擋下的回應（兩者都沒有句子時不顯示文字，仍擋送出）。
    final refusal = openingBlock?.message != null
        ? openingBlock
        : submitRefusal.value;
    // 輸入監聽器只在第一次 build 掛上，所以伺服器的開店答案不放進 canSubmit，在按鈕上直接判斷：
    // 讀取中與「不能開店」（不論有沒有認得的原因）都停用。
    final canPress = canSubmit.value && openingBlock == null && !openingLoading;

    void checkInput() => canSubmit.value = inputReady();

    useEffect(() {
      storeNameCtrl.addListener(checkInput);
      addressDetailCtrl.addListener(checkInput);
      return () {
        storeNameCtrl.removeListener(checkInput);
        addressDetailCtrl.removeListener(checkInput);
      };
    }, []);

    useEffect(() {
      scheduleMicrotask(checkInput);
      return null;
    }, [townCode.value, termsAccepted.value, needsTerms, agreement?.version]);

    void showSnack(String msg) {
      if (!context.mounted) return;
      FloatingToast.error(context, msg);
    }

    Future<void> submit({required bool goProfile}) async {
      if (isSubmitting.value) return;
      isSubmitting.value = true;
      try {
        final outcome = await ref
            .read(sellerOnboardingProvider)
            .submit(
              storeName: storeNameCtrl.text.trim(),
              townCode: townCode.value,
              addressDetail: addressDetailCtrl.text.trim(),
              acceptance: agreement == null
                  ? null
                  : AgreementAcceptance(
                      kind: AgreementKind.AGREEMENT_KIND_SELLER_TERMS,
                      version: agreement.version,
                    ),
            );
        switch (outcome) {
          case OnboardingCompleted():
            unawaited(ref.read(analyticsEventsProvider).onboardingComplete());
            if (!context.mounted) return;
            context.go(goProfile ? '/profile' : '/main/dashboard');
          case OnboardingAgreementOutdated():
            termsAccepted.value = false;
            ref.invalidate(sellerAgreementsProvider);
            showSnack(agreementOutdatedMessage);
          case OnboardingConflict(:final message):
            showSnack(message);
          case OnboardingStoreOpeningRefused(:final refusal, :final message):
            submitRefusal.value = StoreOpeningBlock(
              refusal: refusal,
              message: message,
            );
          case OnboardingFailed(:final message):
            showSnack(message);
          case OnboardingTermsRequired():
            final refreshed = await ref
                .read(currentStoreProvider.notifier)
                .refresh();
            if (!refreshed) showSnack(storeStateUnavailableNotice);
        }
      } finally {
        if (context.mounted) isSubmitting.value = false;
      }
    }

    return Scaffold(
      backgroundColor: AppColors.surfacePage,
      appBar: const HkGlassAppBar(
        title: Text('建立店舖'),
        subtitle: '最後一步',
        showBackButton: false,
      ),
      body: HkAdaptivePage(
        maxWidth: 720,
        padding: const EdgeInsets.fromLTRB(16, 20, 16, 32),
        child: SingleChildScrollView(
          keyboardDismissBehavior: ScrollViewKeyboardDismissBehavior.onDrag,
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              HkContentCard(
                elevated: true,
                child: Row(
                  children: [
                    SvgPicture.asset(
                      'assets/brand/logo-mark.svg',
                      width: 48,
                      height: 48,
                      semanticsLabel: 'Hawkr',
                    ),
                    const SizedBox(width: 14),
                    const Expanded(
                      child: Column(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          Text(
                            '準備好開始營運了嗎？',
                            style: TextStyle(
                              color: AppColors.textEmphasis,
                              fontSize: 20,
                              fontWeight: FontWeight.w700,
                            ),
                          ),
                          SizedBox(height: 4),
                          Text(
                            '先完成店名與營運地址，之後可在設定中補齊品牌資料。',
                            style: TextStyle(
                              color: AppColors.textPrimary,
                              fontSize: 14,
                              height: 1.45,
                            ),
                          ),
                        ],
                      ),
                    ),
                  ],
                ),
              ),
              if (refusal case StoreOpeningBlock(message: final text?)) ...[
                const SizedBox(height: HkSpace.lg),
                HkContentCard(
                  key: const Key('store_opening_refusal'),
                  padding: EdgeInsets.zero,
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Padding(
                        padding: const EdgeInsets.all(HkSpace.lg),
                        child: Text(
                          text,
                          style: const TextStyle(
                            color: AppColors.textPrimary,
                            fontSize: 14,
                            height: 1.45,
                          ),
                        ),
                      ),
                      if (refusal.refusal?.offersAppeal ?? false)
                        OptionRow(
                          icon: HkIconName.shield,
                          label: '措施與申覆',
                          onTap: () => context.push(SellerRoutes.measures),
                        ),
                    ],
                  ),
                ),
              ],
              if (hasMeasures) ...[
                const SizedBox(height: HkSpace.lg),
                HkContentCard(
                  padding: EdgeInsets.zero,
                  child: OptionRow(
                    icon: HkIconName.shield,
                    label: '你有措施紀錄，查看與申覆',
                    onTap: () => context.push(SellerRoutes.measures),
                  ),
                ),
              ],
              const SizedBox(height: 24),
              const HkPageSection(
                title: '品牌身份',
                description: '這是買家在店舖、商品與訂單中看到的名稱。',
              ),
              const SizedBox(height: 10),
              HkContentCard(
                child: TextField(
                  controller: storeNameCtrl,
                  textInputAction: TextInputAction.next,
                  autofillHints: const [AutofillHints.organizationName],
                  decoration: const InputDecoration(
                    labelText: '店舖名稱',
                    hintText: '例如：範例果園',
                    prefixIcon: HkIcon(HkIconName.store),
                  ),
                ),
              ),
              const SizedBox(height: 24),
              const HkPageSection(
                title: '營運地址',
                description: '地址會用於物流開通、寄退件與店舖地區顯示。',
              ),
              const SizedBox(height: 10),
              HkContentCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    TownPicker(
                      countyCode: countyCode.value,
                      townCode: townCode.value,
                      onChanged: (county, town) {
                        countyCode.value = county;
                        townCode.value = town;
                      },
                    ),
                    const SizedBox(height: 14),
                    TextField(
                      controller: addressDetailCtrl,
                      textInputAction: TextInputAction.done,
                      autofillHints: const [AutofillHints.fullStreetAddress],
                      decoration: const InputDecoration(
                        labelText: '詳細地址',
                        hintText: '路／街、段、巷、弄、號、樓',
                        prefixIcon: Padding(
                          padding: EdgeInsets.all(16),
                          child: HawkrLocationIcon(
                            size: 20,
                            color: AppIconColors.muted,
                          ),
                        ),
                      ),
                    ),
                  ],
                ),
              ),
              if (needsTerms) ...[
                const SizedBox(height: 20),
                AgreementCheckbox(
                  checkboxKey: const Key('seller_terms_checkbox'),
                  before: '我已閱讀並同意〈',
                  linkText: 'Hawkr 賣家契約',
                  after: '〉',
                  url: agreement?.url,
                  loadFailed: terms?.hasError == true,
                  checked: termsAccepted.value,
                  enabled: !isSubmitting.value,
                  onChanged: (value) => termsAccepted.value = value,
                  onRetry: () => ref.invalidate(sellerAgreementsProvider),
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
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 12),
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                SizedBox(
                  width: double.infinity,
                  height: 50,
                  child: FilledButton(
                    onPressed: canPress && !isSubmitting.value
                        ? () => submit(goProfile: false)
                        : null,
                    child: HkButtonLabel(
                      isSubmitting.value ? '儲存中…' : '儲存並開始使用',
                    ),
                  ),
                ),
                SizedBox(
                  width: double.infinity,
                  height: HkControl.minTap,
                  child: TextButton(
                    onPressed: canPress && !isSubmitting.value
                        ? () => submit(goProfile: true)
                        : null,
                    child: const HkButtonLabel('儲存後前往完整設定'),
                  ),
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
