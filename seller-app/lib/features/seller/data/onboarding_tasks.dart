import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pbenum.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';
import 'package:riverpod_annotation/riverpod_annotation.dart';

import '../../../core/codemeta/code_meta.dart' show PayUniSubmitStatus;
import '../../../core/router/pending_wallet_focus.dart';
import '../../../core/session/my_store.dart';
import '../../../shared/utils/hk_log.dart';
import '../../../shared/widgets/hk_status_badge.dart';
import '../../product/data/product_repository.dart';
import '../../settings/data/shipping_settings_remote_service.dart';
import 'current_store.dart';
import 'kyc_flow.dart';

part 'onboarding_tasks.g.dart';

enum OnboardingTaskGroup {
  beforePublish('公開前必須'),
  recommended('建議完成');

  const OnboardingTaskGroup(this.label);

  final String label;
}

enum OnboardingTaskId {
  profile,
  cert,
  notSuspended,
  storeActive,
  otherRequirement,
  shipping,
  product,
  food,
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
    this.walletSection,
    this.optional = false,
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

  /// 這項導向錢包時要展開並捲到的區塊（見 [WalletSections]）；導向錢包頂端或不是錢包的任務是 null。
  final String? walletSection;

  /// 非必要。
  final bool optional;

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
  SellerRequirement.SELLER_REQUIREMENT_KYC_APPROVED,
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
/// [flowStatus] 是 `GetMyKYC` 的流程狀態；讀不到時是 null。[l] 是 UI 語系（名詞與狀態文字取自共用文案目錄）。
List<OnboardingTask> buildOnboardingTasks({
  required UiLocale l,
  required MyStore? my,
  required KYCFlowStatus? flowStatus,
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
  final certDone = my?.isCertified ?? false;
  // 讀不到流程狀態時和「尚未開始」一樣引導開始認證（website `kycFlowActionLabel`）。
  final action = kycFlowCopy(
    flowStatus ?? KYCFlowStatus.KYC_FLOW_STATUS_NOT_STARTED,
    l,
  ).action;
  final statusLabel =
      (certDone ? KYCFlowStatus.KYC_FLOW_STATUS_APPROVED : flowStatus)?.uiText(
        l,
      ) ??
      commonUnknown(l);

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
      id: OnboardingTaskId.cert,
      group: OnboardingTaskGroup.beforePublish,
      title: '商家認證',
      description: '身分認證與 PayUni 收款審核。',
      cta: certDone ? null : action,
      completed: certDone,
      statusLabel: statusLabel,
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
  final hasFoodReg = my?.foodRegistrationNumber != null;

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
      id: OnboardingTaskId.food,
      group: OnboardingTaskGroup.recommended,
      title: '食品加工登錄字號',
      description: '販售加工食品時再補上。',
      cta: hasFoodReg ? '查看或更新資料' : '填寫資料',
      walletSection: WalletSections.foodRegistration,
      completed: hasFoodReg,
      optional: true,
      statusLabel: hasFoodReg ? '已登錄' : '尚未登錄',
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
