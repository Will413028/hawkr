import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/codemeta/code_meta.dart' show PayUniSubmitStatus;
import '../../../core/session/my_store.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../../product/data/product_repository.dart';
import '../../settings/data/shipping_settings_remote_service.dart';
import 'current_store.dart';
import 'payment_account.dart';

part 'onboarding_tasks.g.dart';

enum OnboardingTaskGroup {
  beforePublish('公開前必須'),
  recommended('建議完成');

  const OnboardingTaskGroup(this.label);

  final String label;
}

enum OnboardingTaskId {
  profile,
  platformReview,
  paymentAccount,
  notSuspended,
  storeActive,
  otherRequirement,
  shipping,
  product,
  description,
}

/// 開店進度的一項任務：和 website 的 `onboardingTasks.ts` 同一套語意與文字。
final class OnboardingTask {
  const OnboardingTask({
    required this.id,
    required this.group,
    required this.title,
    required this.description,
    required this.statusLabel,
    required this.completed,
    this.cta,
    this.locked = false,
  });

  final OnboardingTaskId id;
  final OnboardingTaskGroup group;
  final String title;
  final String description;
  final String statusLabel;
  final bool completed;

  /// 按鈕文字；這項沒有要辦的事時是 null。
  final String? cta;

  /// 還不能做（物流設定在伺服器的 `SHIPPING_SETUP` 啟用之前）。
  final bool locked;

  HkStatusTone get tone =>
      completed ? HkStatusTone.success : HkStatusTone.neutral;
}

/// 物流開通的進度：有任何一列送 PayUni 成功算完成，有一列送審中算處理中（`payuniSubmitStatus`）。
final class ShippingProgress {
  const ShippingProgress({this.done = false, this.processing = false});

  factory ShippingProgress.fromRows(Iterable<Map<String, dynamic>> rows) {
    final statuses = [for (final row in rows) '${row['payuniSubmitStatus']}'];
    return ShippingProgress(
      done: statuses.contains(PayUniSubmitStatus.succeeded.value),
      processing: statuses.contains(PayUniSubmitStatus.requested.value),
    );
  }

  final bool done;
  final bool processing;
}

/// 進度卡除了店家與認證之外需要的資料。讀不到的部分保守處理：物流當作沒開通，商品數是 null（當作沒有）。
final class OnboardingInputs {
  const OnboardingInputs({
    this.shipping = const ShippingProgress(),
    this.productCount,
  });

  final ShippingProgress shipping;
  final int? productCount;
}

const _knownRequirements = {
  SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE,
  SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
  SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED,
  SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
  SellerRequirement.SELLER_REQUIREMENT_STORE_NAME,
  SellerRequirement.SELLER_REQUIREMENT_LOGO,
  SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE,
};

/// 商店資料任務涵蓋的公開條件；名詞取自共用文案目錄。
const _profileRequirements = [
  SellerRequirement.SELLER_REQUIREMENT_STORE_NAME,
  SellerRequirement.SELLER_REQUIREMENT_LOGO,
  SellerRequirement.SELLER_REQUIREMENT_TOWN_CODE,
];

/// 「公開前必須」只由伺服器的 `PUBLIC_LISTING` 缺的條件（`missing`）推導，client 不自己判斷：伺服器新增條件時，沒有專屬列的
/// 條件以「其他公開條件」出現。沒有 `PUBLIC_LISTING`（還沒讀到）時每個條件都當成還缺。
///
/// [reviewStatus] 是平台審核的狀態（`GetMyKYC` 的 `KYCApplication.status`），沒有申請時是 null；[reviewReadable] 為 false 表示
/// `GetMyKYC` 還沒讀到或讀取失敗，此時 null 不是「未送出」，狀態文字是目錄的「未知」。[paymentAccount] 是 `GetMyPaymentAccount`
/// （平台審核核准之前沒有）。[l] 是 UI 語系（名詞與狀態文字取自共用文案目錄）。
List<OnboardingTask> buildOnboardingTasks({
  required UiLocale l,
  required MyStore? my,
  required KYCStatus? reviewStatus,
  required PaymentAccount? paymentAccount,
  bool reviewReadable = true,
  ShippingProgress shipping = const ShippingProgress(),
  int? productCount,
}) {
  final missing = my?.missingFor(
    SellerCapability.SELLER_CAPABILITY_PUBLIC_LISTING,
  );
  bool lacks(SellerRequirement requirement) =>
      missing == null || missing.contains(requirement);

  final missingProfile = [
    for (final requirement in _profileRequirements)
      if (lacks(requirement)) requirement.uiText(l) ?? commonUnknown(l),
  ];
  final profileDone = missingProfile.isEmpty;
  // 收款帳戶啟用而且平台審核核准才通過開通閘門（`ONBOARDING_COMPLETE`），這是伺服器的判斷；平台審核另看申請狀態。
  final paymentDone = !lacks(
    SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE,
  );
  final reviewDone =
      paymentDone || reviewStatus == KYCStatus.KYC_STATUS_APPROVED;
  // 伺服器說開通閘門已通過、收款帳戶卻還沒讀到：狀態文字用「已啟用」，不顯示成「準備中」。
  final account = paymentDone && paymentAccount == null
      ? PaymentAccount(
          status: PaymentAccountStatus.PAYMENT_ACCOUNT_STATUS_ENABLED,
        )
      : effectivePaymentAccount(paymentAccount, platformApproved: reviewDone);
  final reviewLabel = reviewDone
      ? KYCStatus.KYC_STATUS_APPROVED.uiText(l)
      : reviewStatus?.uiText(l);

  final tasks = <OnboardingTask>[
    OnboardingTask(
      id: OnboardingTaskId.profile,
      group: OnboardingTaskGroup.beforePublish,
      title: '商店資料',
      description: '店名、logo 與所在地。',
      cta: '去設定',
      completed: profileDone,
      statusLabel: profileDone ? '已完成' : '缺少：${missingProfile.join('、')}',
    ),
    OnboardingTask(
      id: OnboardingTaskId.platformReview,
      group: OnboardingTaskGroup.beforePublish,
      title: '平台審核',
      description: '商家資料與統編的平台審核。',
      // 已通過就沒有要辦的審核了，不再給按鈕。
      cta: reviewDone ? null : '前往商家認證',
      completed: reviewDone,
      statusLabel: reviewStatus == null && !reviewDone && reviewReadable
          ? '未送出'
          : reviewLabel ?? commonUnknown(l),
    ),
    OnboardingTask(
      id: OnboardingTaskId.paymentAccount,
      group: OnboardingTaskGroup.beforePublish,
      title: '收款帳戶',
      description: '向金流商設定身分與銀行資料。',
      // 已啟用就沒有要辦的了；平台審核還沒通過時只有灰色的提示。
      cta: paymentDone || account == null ? null : account.status.action(l),
      completed: paymentDone,
      locked: account == null,
      statusLabel: account == null
          ? '平台審核通過後開放設定'
          : account.status.uiText(l) ?? commonUnknown(l),
    ),
  ];

  if (missing?.contains(SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED) ??
      false) {
    tasks.add(
      const OnboardingTask(
        id: OnboardingTaskId.notSuspended,
        group: OnboardingTaskGroup.beforePublish,
        title: '停權處分',
        description: '帳號目前處於停權。',
        cta: '查看停權說明',
        completed: false,
        statusLabel: '停權中',
      ),
    );
  }
  if ((missing?.contains(SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE) ??
          false) ||
      (missing?.contains(
            SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
          ) ??
          false)) {
    tasks.add(
      const OnboardingTask(
        id: OnboardingTaskId.storeActive,
        group: OnboardingTaskGroup.beforePublish,
        title: '商店狀態',
        description: '商店目前不是營業中。',
        cta: '查看說明',
        completed: false,
        statusLabel: '未營業',
      ),
    );
  }
  if ((missing ?? const <SellerRequirement>[]).any(
    (requirement) => !_knownRequirements.contains(requirement),
  )) {
    tasks.add(
      const OnboardingTask(
        id: OnboardingTaskId.otherRequirement,
        group: OnboardingTaskGroup.beforePublish,
        title: '其他公開條件',
        description: '還有公開條件尚未滿足。',
        completed: false,
        statusLabel: '待完成',
      ),
    );
  }

  final shippingGate = my?.shippingGate ?? ShippingGate.unknown;
  final shippingOpen = shippingGate == ShippingGate.open;
  final productsDone = (productCount ?? 0) > 0;
  final hasDescription = my?.description != null;

  tasks.addAll([
    OnboardingTask(
      id: OnboardingTaskId.shipping,
      group: OnboardingTaskGroup.recommended,
      title: '物流設定',
      description: '設定配送與退貨資訊。',
      cta: shipping.done
          ? '查看物流設定'
          : shipping.processing
          ? '查看審核進度'
          : '設定物流',
      completed: shipping.done,
      locked: !shippingOpen,
      statusLabel: !shippingOpen
          ? (shippingGate == ShippingGate.certification ? '需先通過認證' : '未營業')
          : shipping.done
          ? '已開通'
          : shipping.processing
          ? '審核中'
          : '待設定',
    ),
    OnboardingTask(
      id: OnboardingTaskId.product,
      group: OnboardingTaskGroup.recommended,
      title: '上架商品',
      description: '建立第一個可購買商品。',
      cta: '新增商品',
      completed: productsDone,
      statusLabel: productsDone ? '已上架' : '待上架',
    ),
    OnboardingTask(
      id: OnboardingTaskId.description,
      group: OnboardingTaskGroup.recommended,
      title: '商店介紹',
      description: '說明你賣什麼、產地與品牌故事。',
      cta: '去設定',
      completed: hasDescription,
      statusLabel: hasDescription ? '已完成' : '待完成',
    ),
  ]);
  return tasks;
}

/// 進度卡需要的物流與商品數；各自讀取，失敗的部分保守處理。
@riverpod
Future<OnboardingInputs> onboardingInputs(Ref ref) async {
  final storeId = ref.watch(sellerStoreIdProvider);
  final shippingRows = ref.read(shippingSettingsRemoteServiceProvider);
  final products = ref.read(productRepositoryProvider);

  var shipping = const ShippingProgress();
  try {
    shipping = ShippingProgress.fromRows(await shippingRows.fetchOnboardings());
  } catch (error) {
    HKLog.logW('[Onboarding] shipping onboardings failed: $error');
  }

  int? productCount;
  if (storeId != null) {
    try {
      productCount = (await products.getSellerProducts(storeId)).length;
    } catch (error) {
      HKLog.logW('[Onboarding] product count failed: $error');
    }
  }
  return OnboardingInputs(shipping: shipping, productCount: productCount);
}
