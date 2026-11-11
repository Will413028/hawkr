import 'package:flutter_test/flutter_test.dart';
import 'package:hawkr_api/gen/hawkr/seller/v1alpha1/seller.pb.dart';
import 'package:hawkr_seller/features/seller/data/onboarding_tasks.dart';
import 'package:hawkr_seller/shared/widgets/hk_status_badge.dart';
import 'package:hawkr_ui_text/hawkr_ui_text.dart';

import '../../../support/my_store_fixtures.dart';
import '../../../support/onboarding_fixtures.dart';

const _kyc = SellerRequirement.SELLER_REQUIREMENT_ONBOARDING_COMPLETE;
const _notSuspended = SellerRequirement.SELLER_REQUIREMENT_NOT_SUSPENDED;

const _profile = OnboardingTaskId.ONBOARDING_TASK_ID_PROFILE;
const _review = OnboardingTaskId.ONBOARDING_TASK_ID_PLATFORM_REVIEW;
const _payment = OnboardingTaskId.ONBOARDING_TASK_ID_PAYMENT_ACCOUNT;
const _completed = OnboardingTaskState.ONBOARDING_TASK_STATE_COMPLETED;
const _actionRequired =
    OnboardingTaskState.ONBOARDING_TASK_STATE_ACTION_REQUIRED;
const _inProgress = OnboardingTaskState.ONBOARDING_TASK_STATE_IN_PROGRESS;
const _locked = OnboardingTaskState.ONBOARDING_TASK_STATE_LOCKED;
const _unknown = OnboardingTaskState.ONBOARDING_TASK_STATE_UNKNOWN;

OnboardingStep _task(List<OnboardingStep> tasks, String id) =>
    tasks.singleWhere((task) => task.id == id);

List<String> _ids(List<OnboardingStep> tasks, OnboardingTaskGroup group) => [
  for (final task in tasks)
    if (task.group == group) task.id,
];

List<OnboardingStep> _required(GetMyOnboardingResponse? summary) =>
    buildOnboardingSteps(
      l: UiLocale.zhTW,
      my: myStore(publicListing: publicListingMissing()),
      summary: summary,
    ).where((step) => step.group == OnboardingTaskGroup.beforePublish).toList();

void main() {
  group('公開前必須只由伺服器的開通摘要推導', () {
    test('必要任務依摘要的順序，標題與狀態句是伺服器的句子，說明句取自共用文案目錄', () {
      final steps = _required(
        onboardingSummary([
          serverTask(
            _profile,
            _actionRequired,
            action: OnboardingAction.ONBOARDING_ACTION_EDIT_PROFILE,
            title: '商店資料',
            status: '缺少：logo',
          ),
          serverTask(
            _review,
            _inProgress,
            action: OnboardingAction.ONBOARDING_ACTION_REFRESH_PAYMENT_ACCOUNT,
            title: '平台審核',
            status: '審核中',
          ),
          serverTask(_payment, _locked, title: '收款帳戶', status: '審核通過後開放'),
        ]),
      );
      expect(
        [for (final s in steps) s.id],
        [
          requiredStepId(_profile),
          requiredStepId(_review),
          requiredStepId(_payment),
        ],
      );
      expect([for (final s in steps) s.title], ['商店資料', '平台審核', '收款帳戶']);
      expect(
        [for (final s in steps) s.statusLabel],
        ['缺少：logo', '審核中', '審核通過後開放'],
      );
      expect(
        [for (final s in steps) s.description],
        [
          _profile.description(UiLocale.zhTW),
          _review.description(UiLocale.zhTW),
          _payment.description(UiLocale.zhTW),
        ],
      );
      expect(steps.every((s) => s.description != null), isTrue);
    });

    test('完成與鎖定只看 state：COMPLETED 完成、LOCKED 鎖定，其餘都不是', () {
      final steps = _required(
        onboardingSummary([
          serverTask(_profile, _completed),
          serverTask(_review, _actionRequired),
          serverTask(_payment, _locked),
        ]),
      );
      expect([for (final s in steps) s.completed], [true, false, false]);
      expect([for (final s in steps) s.locked], [false, false, true]);
      expect(steps.first.tone, HkStatusTone.success);
      expect(steps.last.tone, HkStatusTone.neutral);
    });

    test('state 是 UNKNOWN（來源讀不到）不算完成、也不鎖定', () {
      final steps = _required(
        onboardingSummary([
          serverTask(_profile, _unknown),
          serverTask(_review, _unknown),
          serverTask(_payment, _unknown),
        ]),
      );
      expect(steps, hasLength(3));
      expect(steps.any((s) => s.completed), isFalse);
      expect(steps.any((s) => s.locked), isFalse);
    });

    test('沒有摘要（沒讀到或失敗）：沒有必要任務，不用本地資料補', () {
      expect(_required(null), isEmpty);
      final steps = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc, _notSuspended])),
      );
      expect(_ids(steps, OnboardingTaskGroup.beforePublish), isEmpty);
    });

    test('UNSPECIFIED 的任務略過', () {
      final steps = _required(
        onboardingSummary([
          serverTask(
            OnboardingTaskId.ONBOARDING_TASK_ID_UNSPECIFIED,
            _completed,
          ),
          serverTask(_review, _completed),
        ]),
      );
      expect([for (final s in steps) s.id], [requiredStepId(_review)]);
    });

    test('標題或狀態句是空的：用目錄的「未知」，不顯示空字串', () {
      final steps = _required(
        onboardingSummary([
          OnboardingTask(id: _review, state: _actionRequired),
        ]),
      );
      expect(steps.single.title, commonUnknown(UiLocale.zhTW));
      expect(steps.single.statusLabel, commonUnknown(UiLocale.zhTW));
    });

    test('停權、關店與其他公開條件不再是任務列（由公告頁處理）', () {
      final steps = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(
          publicListing: publicListingMissing([
            _notSuspended,
            SellerRequirement.SELLER_REQUIREMENT_STORE_ACTIVE,
            SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
          ]),
        ),
        summary: completedSummary(),
      );
      expect(_ids(steps, OnboardingTaskGroup.beforePublish), [
        requiredStepId(_profile),
        requiredStepId(_review),
        requiredStepId(_payment),
      ]);
    });

    test('按鈕文字取自目錄、去處依 action；沒有文字的 action 沒有按鈕', () {
      final expected = {
        OnboardingAction.ONBOARDING_ACTION_EDIT_PROFILE:
            OnboardingStepTarget.editProfile,
        OnboardingAction.ONBOARDING_ACTION_SUBMIT_CERTIFICATION:
            OnboardingStepTarget.certification,
        OnboardingAction.ONBOARDING_ACTION_START_PAYMENT_ONBOARDING:
            OnboardingStepTarget.certification,
        OnboardingAction.ONBOARDING_ACTION_CONTACT_SUPPORT:
            OnboardingStepTarget.support,
        OnboardingAction.ONBOARDING_ACTION_REFRESH_PAYMENT_ACCOUNT:
            OnboardingStepTarget.refreshPayment,
      };
      for (final entry in expected.entries) {
        final step = _required(
          onboardingSummary([
            serverTask(_review, _actionRequired, action: entry.key),
          ]),
        ).single;
        expect(step.target, entry.value, reason: '${entry.key}');
        expect(
          step.cta,
          entry.key.button(UiLocale.zhTW),
          reason: '${entry.key}',
        );
        expect(step.cta, isNotNull, reason: '${entry.key}');
      }
      for (final action in [
        OnboardingAction.ONBOARDING_ACTION_NONE,
        OnboardingAction.ONBOARDING_ACTION_UNSPECIFIED,
      ]) {
        final step = _required(
          onboardingSummary([serverTask(_review, _completed, action: action)]),
        ).single;
        expect(step.target, OnboardingStepTarget.none, reason: '$action');
        expect(step.cta, isNull, reason: '$action');
      }
    });

    test('COMPLETED 的任務 action 是 NONE，沒有按鈕', () {
      final step = _required(
        onboardingSummary([serverTask(_review, _completed)]),
      ).single;
      expect(step.cta, isNull);
    });
  });

  group('建議完成', () {
    test('分組與順序', () {
      final tasks = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
      );
      expect(_ids(tasks, OnboardingTaskGroup.recommended), [
        shippingStepId,
        productStepId,
        descriptionStepId,
      ]);
    });

    test('物流設定在 SHIPPING_SETUP 缺 ONBOARDING_COMPLETE 時鎖定，與流程狀態無關', () {
      for (final summary in [
        null,
        reviewPendingSummary(),
        completedSummary(),
      ]) {
        final tasks = buildOnboardingSteps(
          l: UiLocale.zhTW,
          my: myStore(
            publicListing: publicListingMissing([_kyc]),
            shippingSetup: shippingSetupMissing([_kyc]),
          ),
          summary: summary,
        );
        final shipping = _task(tasks, shippingStepId);
        expect(shipping.locked, isTrue, reason: '$summary');
        expect(shipping.statusLabel, '需先通過認證');
      }
      expect(
        _task(
          buildOnboardingSteps(l: UiLocale.zhTW, my: null),
          shippingStepId,
        ).locked,
        isTrue,
      );
    });

    test('物流設定只看 SHIPPING_SETUP：PUBLIC_LISTING 缺 KYC 也照 SHIPPING_SETUP 開放', () {
      final tasks = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc])),
      );
      expect(_task(tasks, shippingStepId).locked, isFalse);
    });

    test('物流設定缺停權或等待刪除：鎖定並顯示對應的既有文字', () {
      final suspended = _task(
        buildOnboardingSteps(
          l: UiLocale.zhTW,
          my: myStore(shippingSetup: shippingSetupMissing([_notSuspended])),
        ),
        shippingStepId,
      );
      expect(suspended.locked, isTrue);
      expect(suspended.statusLabel, '未營業');

      final pending = _task(
        buildOnboardingSteps(
          l: UiLocale.zhTW,
          my: myStore(
            shippingSetup: shippingSetupMissing([
              SellerRequirement.SELLER_REQUIREMENT_NOT_PENDING_DELETION,
            ]),
          ),
        ),
        shippingStepId,
      );
      expect(pending.locked, isTrue);
      expect(pending.statusLabel, '未營業');
    });

    test('物流解鎖只看伺服器的 SHIPPING_SETUP，與認證列各讀各的', () {
      final tasks = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
      );
      expect(_task(tasks, shippingStepId).locked, isFalse);
    });

    test('列的語氣只分完成與未完成', () {
      final tasks = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing([_kyc, _notSuspended])),
      );
      for (final task in tasks) {
        expect(
          task.tone,
          task.completed ? HkStatusTone.success : HkStatusTone.neutral,
          reason: task.id,
        );
      }
    });

    test('APPROVED 之後依物流開通狀態顯示', () {
      OnboardingStep shippingOf(ShippingProgress progress) => _task(
        buildOnboardingSteps(
          l: UiLocale.zhTW,
          my: myStore(publicListing: publicListingMissing()),
          shipping: progress,
        ),
        shippingStepId,
      );

      final todo = shippingOf(const ShippingProgress());
      expect(todo.locked, isFalse);
      expect(todo.statusLabel, '待設定');
      expect(todo.cta, '設定物流');

      final processing = shippingOf(const ShippingProgress(processing: true));
      expect(processing.statusLabel, '審核中');
      expect(processing.cta, '查看審核進度');

      final done = shippingOf(const ShippingProgress(done: true));
      expect(done.completed, isTrue);
      expect(done.statusLabel, '已開通');
      expect(done.cta, '查看物流設定');
    });

    test('物流列的 payuniSubmitStatus：4303 完成、4302 處理中、其餘都不算', () {
      expect(
        ShippingProgress.fromRows([
          {'payuniSubmitStatus': '4303'},
        ]).done,
        isTrue,
      );
      expect(
        ShippingProgress.fromRows([
          {'payuniSubmitStatus': '4302'},
        ]).processing,
        isTrue,
      );
      final other = ShippingProgress.fromRows([
        {'payuniSubmitStatus': '4301'},
        {'payuniSubmitStatus': '4304'},
        {'payuniSubmitStatus': ''},
        {},
      ]);
      expect(other.done, isFalse);
      expect(other.processing, isFalse);
    });

    test('商品與商店介紹', () {
      final empty = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
      );
      expect(_task(empty, productStepId).statusLabel, '待上架');
      expect(_task(empty, productStepId).cta, '新增商品');
      expect(_task(empty, descriptionStepId).statusLabel, '待完成');

      final filled = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(
          publicListing: publicListingMissing(),
          description: '  產地直送  ',
        ),
        productCount: 3,
      );
      expect(_task(filled, productStepId).completed, isTrue);
      expect(_task(filled, productStepId).statusLabel, '已上架');
      expect(_task(filled, descriptionStepId).completed, isTrue);
      expect(_task(filled, descriptionStepId).statusLabel, '已完成');
      expect(
        _task(
          buildOnboardingSteps(
            l: UiLocale.zhTW,
            my: myStore(
              publicListing: publicListingMissing(),
              description: '   ',
            ),
          ),
          descriptionStepId,
        ).completed,
        isFalse,
      );
    });

    test('商品數讀不到（null）當成還沒有商品', () {
      final tasks = buildOnboardingSteps(
        l: UiLocale.zhTW,
        my: myStore(publicListing: publicListingMissing()),
        productCount: null,
      );
      expect(_task(tasks, productStepId).completed, isFalse);
    });
  });
}
