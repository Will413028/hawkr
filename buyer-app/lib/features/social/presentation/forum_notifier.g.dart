// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'forum_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 一家店的討論區：項目以 id 存一份，回覆以 parentId 分組；讀取與回覆展開在 [ThreadNotifier]，寫入在這裡（沒有按讚）。

@ProviderFor(Forum)
final forumProvider = ForumFamily._();

/// 一家店的討論區：項目以 id 存一份，回覆以 parentId 分組；讀取與回覆展開在 [ThreadNotifier]，寫入在這裡（沒有按讚）。
final class ForumProvider extends $NotifierProvider<Forum, ForumState> {
  /// 一家店的討論區：項目以 id 存一份，回覆以 parentId 分組；讀取與回覆展開在 [ThreadNotifier]，寫入在這裡（沒有按讚）。
  ForumProvider._({
    required ForumFamily super.from,
    required String super.argument,
  }) : super(
         retry: null,
         name: r'forumProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$forumHash();

  @override
  String toString() {
    return r'forumProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  Forum create() => Forum();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ForumState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ForumState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is ForumProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$forumHash() => r'52de006b8d31eea24e5bb0929de53b2269d21dd2';

/// 一家店的討論區：項目以 id 存一份，回覆以 parentId 分組；讀取與回覆展開在 [ThreadNotifier]，寫入在這裡（沒有按讚）。

final class ForumFamily extends $Family
    with
        $ClassFamilyOverride<
          Forum,
          ForumState,
          ForumState,
          ForumState,
          String
        > {
  ForumFamily._()
    : super(
        retry: null,
        name: r'forumProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 一家店的討論區：項目以 id 存一份，回覆以 parentId 分組；讀取與回覆展開在 [ThreadNotifier]，寫入在這裡（沒有按讚）。

  ForumProvider call(String storeId) =>
      ForumProvider._(argument: storeId, from: this);

  @override
  String toString() => r'forumProvider';
}

/// 一家店的討論區：項目以 id 存一份，回覆以 parentId 分組；讀取與回覆展開在 [ThreadNotifier]，寫入在這裡（沒有按讚）。

abstract class _$Forum extends $Notifier<ForumState> {
  late final _$args = ref.$arg as String;
  String get storeId => _$args;

  ForumState build(String storeId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<ForumState, ForumState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<ForumState, ForumState>,
              ForumState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
