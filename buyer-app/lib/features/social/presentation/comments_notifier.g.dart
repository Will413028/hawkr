// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'comments_notifier.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning
/// 一則貼文的留言：留言以 id 存一份，回覆以 parentId 分組，分頁、回覆展開與寫入都在這裡。
///
/// 換觀看者時整份重建清空；每個跨 await 的路徑回來先比對世代，過期就丟棄。重新整理另有一個序號，
/// 讓整理之前發出的分頁與展開不會蓋掉新讀到的第一頁。重讀失敗保留舊值；留言數的變動寫回貼文實體。

@ProviderFor(Comments)
final commentsProvider = CommentsFamily._();

/// 一則貼文的留言：留言以 id 存一份，回覆以 parentId 分組，分頁、回覆展開與寫入都在這裡。
///
/// 換觀看者時整份重建清空；每個跨 await 的路徑回來先比對世代，過期就丟棄。重新整理另有一個序號，
/// 讓整理之前發出的分頁與展開不會蓋掉新讀到的第一頁。重讀失敗保留舊值；留言數的變動寫回貼文實體。
final class CommentsProvider
    extends $NotifierProvider<Comments, CommentsState> {
  /// 一則貼文的留言：留言以 id 存一份，回覆以 parentId 分組，分頁、回覆展開與寫入都在這裡。
  ///
  /// 換觀看者時整份重建清空；每個跨 await 的路徑回來先比對世代，過期就丟棄。重新整理另有一個序號，
  /// 讓整理之前發出的分頁與展開不會蓋掉新讀到的第一頁。重讀失敗保留舊值；留言數的變動寫回貼文實體。
  CommentsProvider._({
    required CommentsFamily super.from,
    required int super.argument,
  }) : super(
         retry: null,
         name: r'commentsProvider',
         isAutoDispose: true,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$commentsHash();

  @override
  String toString() {
    return r'commentsProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  Comments create() => Comments();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(CommentsState value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<CommentsState>(value),
    );
  }

  @override
  bool operator ==(Object other) {
    return other is CommentsProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$commentsHash() => r'76117c3a925e62cd0a7164e6730328627ef91b7c';

/// 一則貼文的留言：留言以 id 存一份，回覆以 parentId 分組，分頁、回覆展開與寫入都在這裡。
///
/// 換觀看者時整份重建清空；每個跨 await 的路徑回來先比對世代，過期就丟棄。重新整理另有一個序號，
/// 讓整理之前發出的分頁與展開不會蓋掉新讀到的第一頁。重讀失敗保留舊值；留言數的變動寫回貼文實體。

final class CommentsFamily extends $Family
    with
        $ClassFamilyOverride<
          Comments,
          CommentsState,
          CommentsState,
          CommentsState,
          int
        > {
  CommentsFamily._()
    : super(
        retry: null,
        name: r'commentsProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: true,
      );

  /// 一則貼文的留言：留言以 id 存一份，回覆以 parentId 分組，分頁、回覆展開與寫入都在這裡。
  ///
  /// 換觀看者時整份重建清空；每個跨 await 的路徑回來先比對世代，過期就丟棄。重新整理另有一個序號，
  /// 讓整理之前發出的分頁與展開不會蓋掉新讀到的第一頁。重讀失敗保留舊值；留言數的變動寫回貼文實體。

  CommentsProvider call(int postId) =>
      CommentsProvider._(argument: postId, from: this);

  @override
  String toString() => r'commentsProvider';
}

/// 一則貼文的留言：留言以 id 存一份，回覆以 parentId 分組，分頁、回覆展開與寫入都在這裡。
///
/// 換觀看者時整份重建清空；每個跨 await 的路徑回來先比對世代，過期就丟棄。重新整理另有一個序號，
/// 讓整理之前發出的分頁與展開不會蓋掉新讀到的第一頁。重讀失敗保留舊值；留言數的變動寫回貼文實體。

abstract class _$Comments extends $Notifier<CommentsState> {
  late final _$args = ref.$arg as int;
  int get postId => _$args;

  CommentsState build(int postId);
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<CommentsState, CommentsState>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<CommentsState, CommentsState>,
              CommentsState,
              Object?,
              Object?
            >;
    element.handleCreate(ref, () => build(_$args));
  }
}
