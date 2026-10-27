// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'member_card_controller.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(MemberCardsController)
final memberCardsControllerProvider = MemberCardsControllerProvider._();

final class MemberCardsControllerProvider
    extends
        $AsyncNotifierProvider<
          MemberCardsController,
          ({bool hasMore, List<MemberCard> records})
        > {
  MemberCardsControllerProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'memberCardsControllerProvider',
        isAutoDispose: true,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$memberCardsControllerHash();

  @$internal
  @override
  MemberCardsController create() => MemberCardsController();
}

String _$memberCardsControllerHash() =>
    r'b023d87f91dc0fa6f5e686130e6690dfaeec08b1';

abstract class _$MemberCardsController
    extends $AsyncNotifier<({bool hasMore, List<MemberCard> records})> {
  FutureOr<({bool hasMore, List<MemberCard> records})> build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<({bool hasMore, List<MemberCard> records})>,
              ({bool hasMore, List<MemberCard> records})
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<({bool hasMore, List<MemberCard> records})>,
                ({bool hasMore, List<MemberCard> records})
              >,
              AsyncValue<({bool hasMore, List<MemberCard> records})>,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}

@ProviderFor(SpendingRecordsController)
final spendingRecordsControllerProvider = SpendingRecordsControllerFamily._();

final class SpendingRecordsControllerProvider
    extends
        $AsyncNotifierProvider<
          SpendingRecordsController,
          ({
            bool hasMore,
            List<SpendingRecord> records,
            int totalRefund,
            int totalSpending,
          })
        > {
  SpendingRecordsControllerProvider._({
    required SpendingRecordsControllerFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'spendingRecordsControllerProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$spendingRecordsControllerHash();

  @override
  String toString() {
    return r'spendingRecordsControllerProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  SpendingRecordsController create() => SpendingRecordsController();

  @override
  bool operator ==(Object other) {
    return other is SpendingRecordsControllerProvider &&
        other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$spendingRecordsControllerHash() =>
    r'ee79dd41e53a11ba66eb16775acc2c8633dfb4ce';

final class SpendingRecordsControllerFamily extends $Family
    with
        $ClassFamilyOverride<
          SpendingRecordsController,
          AsyncValue<
            ({
              bool hasMore,
              List<SpendingRecord> records,
              int totalRefund,
              int totalSpending,
            })
          >,
          ({
            bool hasMore,
            List<SpendingRecord> records,
            int totalRefund,
            int totalSpending,
          }),
          FutureOr<
            ({
              bool hasMore,
              List<SpendingRecord> records,
              int totalRefund,
              int totalSpending,
            })
          >,
          String
        > {
  SpendingRecordsControllerFamily._()
    : super(
        retry: null,
        name: r'spendingRecordsControllerProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  SpendingRecordsControllerProvider call(String sellerId) =>
      SpendingRecordsControllerProvider._(argument: sellerId, from: this);

  @override
  String toString() => r'spendingRecordsControllerProvider';
}

abstract class _$SpendingRecordsController
    extends
        $AsyncNotifier<
          ({
            bool hasMore,
            List<SpendingRecord> records,
            int totalRefund,
            int totalSpending,
          })
        > {
  late final _$args = ref.$arg as String;
  String get sellerId => _$args;

  FutureOr<
    ({
      bool hasMore,
      List<SpendingRecord> records,
      int totalRefund,
      int totalSpending,
    })
  >
  build(String sellerId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref =
        this.ref
            as $Ref<
              AsyncValue<
                ({
                  bool hasMore,
                  List<SpendingRecord> records,
                  int totalRefund,
                  int totalSpending,
                })
              >,
              ({
                bool hasMore,
                List<SpendingRecord> records,
                int totalRefund,
                int totalSpending,
              })
            >;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<
                AsyncValue<
                  ({
                    bool hasMore,
                    List<SpendingRecord> records,
                    int totalRefund,
                    int totalSpending,
                  })
                >,
                ({
                  bool hasMore,
                  List<SpendingRecord> records,
                  int totalRefund,
                  int totalSpending,
                })
              >,
              AsyncValue<
                ({
                  bool hasMore,
                  List<SpendingRecord> records,
                  int totalRefund,
                  int totalSpending,
                })
              >,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
