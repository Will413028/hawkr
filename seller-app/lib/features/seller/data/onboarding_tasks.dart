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

part 'onboarding_tasks.g.dart';

enum OnboardingTaskGroup {
  beforePublish('公開前必須'),
  recommended('建議完成');

  const OnboardingTaskGroup(this.label);

  final String label;
}

/// 進度卡一列的按鈕去哪裡（路由是 client 的，位置依伺服器的 `OnboardingAction` 或建議任務決定）。
enum OnboardingStepTarget {
  /// 沒有按鈕。
  none,

  /// 商店資料（商店設定頁）。
  editProfile,

  /// 商家認證與收款帳戶（錢包頁）。
  certification,

  /// 聯絡客服（說明頁）。
  support,

  /// 物流設定。
  shipping,

  /// 新增商品。
  product,
}

const shippingStepId = 'shipping';
const productStepId = 'product';
const descriptionStepId = 'description';

/// 伺服器的必要任務在進度卡的列 id；伺服器新增的任務 id 同樣有自己的列 id。
String requiredStepId(OnboardingTaskId id) => 'required_${id.value}';

/// 開店進度的一列。必要任務（[OnboardingTaskGroup.beforePublish]）的狀態、下一步與標題、狀態句取自伺服器的開通摘要，
/// 說明句與按鈕文字取自共用文案目錄；建議任務由 client 組。
final class OnboardingStep {
  const OnboardingStep({
    required this.id,
    required this.group,
    required this.title,
    required this.statusLabel,
    required this.completed,
    this.description,
    this.cta,
    this.target = OnboardingStepTarget.none,
    this.locked = false,
  });

  final String id;
  final OnboardingTaskGroup group;
  final String title;
  final String? description;
  final String statusLabel;
  final bool completed;

  /// 按鈕文字；這項沒有要辦的事時是 null。
  final String? cta;

  /// 按鈕去哪裡；[cta] 是 null 時是 [OnboardingStepTarget.none]。
  final OnboardingStepTarget target;

  /// 還不能做（必要任務的 `LOCKED`；物流設定在伺服器的 `SHIPPING_SETUP` 啟用之前）。
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

OnboardingStepTarget _targetOf(OnboardingAction action) => switch (action) {
  OnboardingAction.ONBOARDING_ACTION_EDIT_PROFILE =>
    OnboardingStepTarget.editProfile,
  OnboardingAction.ONBOARDING_ACTION_SUBMIT_CERTIFICATION ||
  OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING =>
    OnboardingStepTarget.certification,
  OnboardingAction.ONBOARDING_ACTION_CONTACT_SUPPORT =>
    OnboardingStepTarget.support,
  _ => OnboardingStepTarget.none,
};

String _textOrUnknown(String text, UiLocale l) =>
    text.isEmpty ? commonUnknown(l) : text;

OnboardingStep? _requiredStep(UiLocale l, OnboardingTask task) {
  if (task.id == OnboardingTaskId.ONBOARDING_TASK_ID_UNSPECIFIED) return null;
  final cta = task.action.button(l);
  final target = cta == null
      ? OnboardingStepTarget.none
      : _targetOf(task.action);
  return OnboardingStep(
    id: requiredStepId(task.id),
    group: OnboardingTaskGroup.beforePublish,
    title: _textOrUnknown(task.title.message, l),
    description: task.id.description(l),
    statusLabel: _textOrUnknown(task.status.message, l),
    completed:
        task.state == OnboardingTaskState.ONBOARDING_TASK_STATE_COMPLETED,
    locked: task.state == OnboardingTaskState.ONBOARDING_TASK_STATE_LOCKED,
    cta: target == OnboardingStepTarget.none ? null : cta,
    target: target,
  );
}

/// 開店進度的列。「公開前必須」只是伺服器開通摘要（`GetMyOnboarding`）的任務依序呈現，client 不自己判斷狀態；沒有摘要
/// （還沒讀到或讀取失敗）時沒有必要任務。「建議完成」（物流、商品、簡介）由 client 組。[l] 是 UI 語系。
List<OnboardingStep> buildOnboardingSteps({
  required UiLocale l,
  required MyStore? my,
  GetMyOnboardingResponse? summary,
  ShippingProgress shipping = const ShippingProgress(),
  int? productCount,
}) {
  final tasks = <OnboardingStep>[
    for (final task in summary?.tasks ?? const <OnboardingTask>[])
      ?_requiredStep(l, task),
  ];

  final shippingGate = my?.shippingGate ?? ShippingGate.unknown;
  final shippingOpen = shippingGate == ShippingGate.open;
  final productsDone = (productCount ?? 0) > 0;
  final hasDescription = my?.description != null;

  tasks.addAll([
    OnboardingStep(
      id: shippingStepId,
      group: OnboardingTaskGroup.recommended,
      title: '物流設定',
      description: '設定配送與退貨資訊。',
      target: OnboardingStepTarget.shipping,
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
    OnboardingStep(
      id: productStepId,
      group: OnboardingTaskGroup.recommended,
      title: '上架商品',
      description: '建立第一個可購買商品。',
      target: OnboardingStepTarget.product,
      cta: '新增商品',
      completed: productsDone,
      statusLabel: productsDone ? '已上架' : '待上架',
    ),
    OnboardingStep(
      id: descriptionStepId,
      group: OnboardingTaskGroup.recommended,
      title: '商店介紹',
      description: '說明你賣什麼、產地與品牌故事。',
      target: OnboardingStepTarget.editProfile,
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
