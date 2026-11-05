// GENERATED CODE - DO NOT MODIFY BY HAND

part of 'contact_email.dart';

// **************************************************************************
// RiverpodGenerator
// **************************************************************************

// GENERATED CODE - DO NOT MODIFY BY HAND
// ignore_for_file: type=lint, type=warning

@ProviderFor(contactEmailRepository)
final contactEmailRepositoryProvider = ContactEmailRepositoryProvider._();

final class ContactEmailRepositoryProvider
    extends
        $FunctionalProvider<
          ContactEmailRepository,
          ContactEmailRepository,
          ContactEmailRepository
        >
    with $Provider<ContactEmailRepository> {
  ContactEmailRepositoryProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'contactEmailRepositoryProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$contactEmailRepositoryHash();

  @$internal
  @override
  $ProviderElement<ContactEmailRepository> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  ContactEmailRepository create(Ref ref) {
    return contactEmailRepository(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(ContactEmailRepository value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<ContactEmailRepository>(value),
    );
  }
}

String _$contactEmailRepositoryHash() =>
    r'd68a27ca5d477a0fb796b4c25e8d48a9e8b35ec3';

/// [accountId] 這個帳號的聯絡信箱。讀取失敗是 error 狀態，不當成沒有信箱。

@ProviderFor(accountContactEmail)
final accountContactEmailProvider = AccountContactEmailFamily._();

/// [accountId] 這個帳號的聯絡信箱。讀取失敗是 error 狀態，不當成沒有信箱。

final class AccountContactEmailProvider
    extends
        $FunctionalProvider<
          AsyncValue<AccountContactEmail?>,
          AccountContactEmail?,
          FutureOr<AccountContactEmail?>
        >
    with
        $FutureModifier<AccountContactEmail?>,
        $FutureProvider<AccountContactEmail?> {
  /// [accountId] 這個帳號的聯絡信箱。讀取失敗是 error 狀態，不當成沒有信箱。
  AccountContactEmailProvider._({
    required AccountContactEmailFamily super.from,
    required String super.argument,
  }) : super(
         retry: noRetry,
         name: r'accountContactEmailProvider',
         isAutoDispose: false,
         dependencies: null,
         $allTransitiveDependencies: null,
       );

  @override
  String debugGetCreateSourceHash() => _$accountContactEmailHash();

  @override
  String toString() {
    return r'accountContactEmailProvider'
        ''
        '($argument)';
  }

  @$internal
  @override
  $FutureProviderElement<AccountContactEmail?> $createElement(
    $ProviderPointer pointer,
  ) => $FutureProviderElement(pointer);

  @override
  FutureOr<AccountContactEmail?> create(Ref ref) {
    final argument = this.argument as String;
    return accountContactEmail(ref, argument);
  }

  @override
  bool operator ==(Object other) {
    return other is AccountContactEmailProvider && other.argument == argument;
  }

  @override
  int get hashCode {
    return argument.hashCode;
  }
}

String _$accountContactEmailHash() =>
    r'160618b4412a576e773a8c7c319f97905561ade1';

/// [accountId] 這個帳號的聯絡信箱。讀取失敗是 error 狀態，不當成沒有信箱。

final class AccountContactEmailFamily extends $Family
    with $FunctionalFamilyOverride<FutureOr<AccountContactEmail?>, String> {
  AccountContactEmailFamily._()
    : super(
        retry: noRetry,
        name: r'accountContactEmailProvider',
        dependencies: null,
        $allTransitiveDependencies: null,
        isAutoDispose: false,
      );

  /// [accountId] 這個帳號的聯絡信箱。讀取失敗是 error 狀態，不當成沒有信箱。

  AccountContactEmailProvider call(String accountId) =>
      AccountContactEmailProvider._(argument: accountId, from: this);

  @override
  String toString() => r'accountContactEmailProvider';
}

/// 目前登入帳號的聯絡信箱；沒有登入時是 null（外層）。

@ProviderFor(currentContactEmail)
final currentContactEmailProvider = CurrentContactEmailProvider._();

/// 目前登入帳號的聯絡信箱；沒有登入時是 null（外層）。

final class CurrentContactEmailProvider
    extends
        $FunctionalProvider<
          AsyncValue<AccountContactEmail?>?,
          AsyncValue<AccountContactEmail?>?,
          AsyncValue<AccountContactEmail?>?
        >
    with $Provider<AsyncValue<AccountContactEmail?>?> {
  /// 目前登入帳號的聯絡信箱；沒有登入時是 null（外層）。
  CurrentContactEmailProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'currentContactEmailProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$currentContactEmailHash();

  @$internal
  @override
  $ProviderElement<AsyncValue<AccountContactEmail?>?> $createElement(
    $ProviderPointer pointer,
  ) => $ProviderElement(pointer);

  @override
  AsyncValue<AccountContactEmail?>? create(Ref ref) {
    return currentContactEmail(ref);
  }

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(AsyncValue<AccountContactEmail?>? value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<AsyncValue<AccountContactEmail?>?>(
        value,
      ),
    );
  }
}

String _$currentContactEmailHash() =>
    r'ec93c0f26a98cc62779ee0551ef3328ded4e7f01';

/// 重讀目前帳號的聯絡信箱。

@ProviderFor(ContactEmailReader)
final contactEmailReaderProvider = ContactEmailReaderProvider._();

/// 重讀目前帳號的聯絡信箱。
final class ContactEmailReaderProvider
    extends $NotifierProvider<ContactEmailReader, void> {
  /// 重讀目前帳號的聯絡信箱。
  ContactEmailReaderProvider._()
    : super(
        from: null,
        argument: null,
        retry: null,
        name: r'contactEmailReaderProvider',
        isAutoDispose: false,
        dependencies: null,
        $allTransitiveDependencies: null,
      );

  @override
  String debugGetCreateSourceHash() => _$contactEmailReaderHash();

  @$internal
  @override
  ContactEmailReader create() => ContactEmailReader();

  /// {@macro riverpod.override_with_value}
  Override overrideWithValue(void value) {
    return $ProviderOverride(
      origin: this,
      providerOverride: $SyncValueProvider<void>(value),
    );
  }
}

String _$contactEmailReaderHash() =>
    r'7f8f18e4f6510d042587e7dcb80cf6292610b7ca';

/// 重讀目前帳號的聯絡信箱。

abstract class _$ContactEmailReader extends $Notifier<void> {
  void build();
  @$mustCallSuper
  @override
  void runBuild() {
    final ref = this.ref as $Ref<void, void>;
    final element =
        ref.element
            as $ClassProviderElement<
              AnyNotifier<void, void>,
              void,
              Object?,
              Object?
            >;
    element.handleCreate(ref, build);
  }
}
