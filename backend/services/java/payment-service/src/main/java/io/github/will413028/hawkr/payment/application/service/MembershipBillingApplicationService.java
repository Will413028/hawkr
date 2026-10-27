package io.github.will413028.hawkr.payment.application.service;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.TsidUtil;
import io.github.will413028.hawkr.common.utils.code.MembershipBillingErrorCode;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommand;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommandIdentity;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommandRules;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommandStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommandHasher;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingIntervalUnit;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingQuote;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingQuoteValidator;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingValidationException;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipBillingRepository;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipProviderSubscription;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderAccountProfile;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderActionResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionCreated;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionOperation;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderAction;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderAdapter;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderCommand;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionProviderRegistry;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionSubject;
import java.time.LocalDateTime;
import java.util.List;
import java.util.Optional;
import java.util.UUID;
import java.util.function.Supplier;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.stereotype.Service;
import org.springframework.transaction.PlatformTransactionManager;
import org.springframework.transaction.support.TransactionTemplate;

/**
 * Shared provider subscription application service. It owns command idempotency and payment-side
 * lifecycle only; buyer/seller membership periods are never mutated here.
 */
@Service
public class MembershipBillingApplicationService {

    private static final String BASE_MEMBERSHIP = "BASE_MEMBERSHIP";

    private final MembershipBillingRepository repository;
    private final SubscriptionProviderRegistry providerRegistry;
    private final Supplier<LocalDateTime> nowUtc;
    private final Supplier<Long> idGenerator;
    private final TransactionTemplate transactionTemplate;

    /** Spring composition constructor; all registered providers share one lifecycle kernel. */
    @Autowired
    public MembershipBillingApplicationService(
            MembershipBillingRepository repository,
            List<SubscriptionProviderAdapter> providerAdapters,
            PlatformTransactionManager transactionManager
    ) {
        this(repository, new SubscriptionProviderRegistry(providerAdapters), DateTimeUtils::nowUtc,
                TsidUtil::generate, transactionManager);
    }

    /** Convenience constructor retained for focused domain/application tests. */
    public MembershipBillingApplicationService(
            MembershipBillingRepository repository,
            SubscriptionProviderAdapter providerAdapter
    ) {
        this(repository, new SubscriptionProviderRegistry(List.of(providerAdapter)), DateTimeUtils::nowUtc,
                TsidUtil::generate, null);
    }

    /** Constructor with a clock supplier keeps all expiry decisions deterministic in unit tests. */
    public MembershipBillingApplicationService(
            MembershipBillingRepository repository,
            SubscriptionProviderAdapter providerAdapter,
            Supplier<LocalDateTime> nowUtc
    ) {
        this(repository, new SubscriptionProviderRegistry(List.of(providerAdapter)), nowUtc,
                TsidUtil::generate, null);
    }

    public MembershipBillingApplicationService(
            MembershipBillingRepository repository,
            SubscriptionProviderAdapter providerAdapter,
            Supplier<LocalDateTime> nowUtc,
            Supplier<Long> idGenerator
    ) {
        this(repository, new SubscriptionProviderRegistry(List.of(providerAdapter)), nowUtc, idGenerator, null);
    }

    private MembershipBillingApplicationService(
            MembershipBillingRepository repository,
            SubscriptionProviderRegistry providerRegistry,
            Supplier<LocalDateTime> nowUtc,
            Supplier<Long> idGenerator,
            PlatformTransactionManager transactionManager
    ) {
        this.repository = repository;
        this.providerRegistry = providerRegistry;
        this.nowUtc = nowUtc;
        this.idGenerator = idGenerator;
        this.transactionTemplate = transactionManager == null ? null : new TransactionTemplate(transactionManager);
    }

    public CreateResult create(CreateCommand command) {
        validateCreateCommand(command);

        SubscriptionOperation operation = SubscriptionOperation.CREATE_SUBSCRIPTION;
        String requestHash = BillingCommandHasher.hashCreate(
                command.subject(), command.membershipId(), command.membershipChangeRequestId(), command.quote());
        Optional<BillingCommand> existing = inTransaction(
                () -> repository.findCommand(operation.value(), command.requestId()));
        if (existing.isPresent()) {
            return replayOrResumeCreate(existing.get(), operation, command.requestId(), requestHash);
        }
        BillingQuoteValidator.validate(command.quote(), nowUtc.get());

        CreatePreparation preparation = inTransaction(() -> prepareCreate(command, operation, requestHash));
        if (preparation.replayCommand() != null) {
            return replayOrResumeCreate(preparation.replayCommand(), operation, command.requestId(), requestHash);
        }
        if (preparation.replaySubscription() != null) {
            return replayQuoteBoundCreate(preparation.replaySubscription(), command, operation, requestHash);
        }

        try {
            ProviderSubscriptionCreated created = preparation.providerAdapter().create(new SubscriptionProviderCommand(
                    command.quote(), command.membershipId(), command.membershipChangeRequestId(),
                    preparation.subscription().merTradeNo(), preparation.account(),
                    preparation.command().id().toString()));
            ProviderSubscriptionStatus status = parseCreateStatus(created);
            inTransaction(() -> {
                repository.attachProviderSubscription(preparation.subscription().id(), created);
                repository.markCommandSucceeded(preparation.command().id(), preparation.subscription().id(),
                        nullToEmpty(created.providerSubscriptionId()), nullToEmpty(created.redirectUrl()),
                        preparation.subscription().merTradeNo(), status.name());
                return null;
            });
            return new CreateResult(
                    String.valueOf(preparation.subscription().id()),
                    nullToEmpty(created.providerSubscriptionId()),
                    nullToEmpty(created.redirectUrl()),
                    preparation.subscription().merTradeNo(),
                    status.name());
        } catch (BillingValidationException e) {
            inTransaction(() -> {
                repository.markCommandFailed(preparation.command().id(), e.errorCode().code(), e.getMessage());
                repository.updateSubscriptionStatus(preparation.subscription().id(), ProviderSubscriptionStatus.ENDED);
                return null;
            });
            throw e;
        } catch (RuntimeException e) {
            // A transport failure is an unknown provider outcome, not a provider rejection. Keep
            // the command/subscription retryable and require the provider idempotency key above
            // for reconciliation; ending it here could create a second subscription on retry.
            inTransaction(() -> {
                repository.markCommandUnknown(preparation.command().id(),
                        MembershipBillingErrorCode.BILLING_UNAVAILABLE.code(),
                        "provider outcome is unknown; retry with the same request_id");
                return null;
            });
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "provider outcome is unknown; retry with the same request_id");
        }
    }

    public ActionResult suspend(ActionCommand command) {
        return action(command, SubscriptionOperation.SUSPEND_SUBSCRIPTION);
    }

    public ActionResult restart(ActionCommand command) {
        return action(command, SubscriptionOperation.RESTART_SUBSCRIPTION);
    }

    public ActionResult end(ActionCommand command) {
        return action(command, SubscriptionOperation.END_SUBSCRIPTION);
    }

    private ActionResult action(ActionCommand command, SubscriptionOperation operation) {
        if (command == null) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "subscription action command is required");
        }
        requireNonBlank(command.requestId(), MembershipBillingErrorCode.BILLING_COMMAND_CONFLICT,
                "action request_id is required");
        long subscriptionId = parseSubscriptionId(command == null ? null : command.periodSubscriptionId());
        String reason = command.reason() == null ? "" : command.reason();
        String requestHash = BillingCommandHasher.hashAction(operation, String.valueOf(subscriptionId), reason);
        Optional<BillingCommand> existing = inTransaction(
                () -> repository.findCommand(operation.value(), command.requestId()));
        if (existing.isPresent()) {
            return replayOrResumeAction(existing.get(), operation, command.requestId(), requestHash);
        }

        ActionPreparation preparation = inTransaction(
                () -> prepareAction(command, operation, requestHash, subscriptionId, reason));
        if (preparation.replayCommand() != null) {
            return replayOrResumeAction(preparation.replayCommand(), operation, command.requestId(), requestHash);
        }

        try {
            // An uncompleted hosted checkout has no provider identity. Ending it locally is safe;
            // suspend/restart require a provider subscription and must never invent one. Keep this
            // check inside the command failure boundary so a rejected action cannot leave a
            // PROCESSING idempotency row behind.
            MembershipProviderSubscription subscription = preparation.subscription();
            if (isBlank(subscription.providerSubscriptionId()) && operation != SubscriptionOperation.END_SUBSCRIPTION) {
                throw invalid(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED,
                        "provider subscription is not ready for this action");
            }
            SubscriptionProviderAdapter providerAdapter = isBlank(subscription.providerSubscriptionId())
                    ? null : requireAdapterForSubscription(subscription, operation);
            ProviderActionResult providerResult = null;
            if (!isBlank(subscription.providerSubscriptionId())) {
                SubscriptionProviderAction providerAction = new SubscriptionProviderAction(
                        subscription, operation, reason, preparation.command().id().toString());
                providerResult = switch (operation) {
                    case SUSPEND_SUBSCRIPTION -> providerAdapter.suspend(providerAction);
                    case RESTART_SUBSCRIPTION -> providerAdapter.restart(providerAction);
                    case END_SUBSCRIPTION -> providerAdapter.end(providerAction);
                    case CREATE_SUBSCRIPTION -> throw new IllegalStateException("create is not an action");
                };
            }
            ProviderSubscriptionStatus status = targetStatus(operation, providerResult);
            inTransaction(() -> {
                repository.updateSubscriptionStatus(subscriptionId, status);
                repository.markCommandSucceeded(preparation.command().id(), subscriptionId,
                        nullToEmpty(subscription.providerSubscriptionId()), "", subscription.merTradeNo(), status.name());
                return null;
            });
            return new ActionResult(String.valueOf(subscriptionId),
                    nullToEmpty(subscription.providerSubscriptionId()), status.name());
        } catch (BillingValidationException e) {
            inTransaction(() -> {
                repository.markCommandFailed(preparation.command().id(), e.errorCode().code(), e.getMessage());
                return null;
            });
            throw e;
        } catch (RuntimeException e) {
            inTransaction(() -> {
                repository.markCommandUnknown(preparation.command().id(),
                        MembershipBillingErrorCode.BILLING_UNAVAILABLE.code(),
                        "provider outcome is unknown; retry with the same request_id");
                return null;
            });
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "provider outcome is unknown; retry with the same request_id");
        }
    }

    private CreateResult replayOrResumeCreate(
            BillingCommand existing,
            SubscriptionOperation operation,
            String requestId,
            String requestHash
    ) {
        assertSameCommand(existing, operation, requestId, requestHash);
        if (existing.status() == BillingCommandStatus.FAILED) {
            throw replayError(existing);
        }
        if (existing.status() == BillingCommandStatus.SUCCEEDED) {
            return replayCreateSnapshot(existing);
        }
        claimForReconciliation(existing);
        if (existing.resultSubscriptionId() == null) {
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "billing command has no persisted subscription for reconciliation");
        }
        MembershipProviderSubscription subscription = repository.findSubscriptionById(existing.resultSubscriptionId())
                .orElseThrow(() -> invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                        "idempotent result subscription is missing"));
        ProviderAccountProfile account = repository.findProviderAccount(subscription.providerAccountRef())
                .filter(ProviderAccountProfile::isActive)
                .orElseThrow(() -> invalid(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED,
                        "provider account is missing or disabled"));
        SubscriptionProviderAdapter providerAdapter = requireAdapter(account, operation);
        try {
            ProviderSubscriptionCreated created = providerAdapter.create(new SubscriptionProviderCommand(
                    quoteFromSubscription(subscription), subscription.membershipId(),
                    subscription.membershipChangeRequestId(), subscription.merTradeNo(), account,
                    existing.id().toString()));
            ProviderSubscriptionStatus status = parseCreateStatus(created);
            inTransaction(() -> {
                repository.attachProviderSubscription(subscription.id(), created);
                repository.markCommandSucceeded(existing.id(), subscription.id(),
                        nullToEmpty(created.providerSubscriptionId()), nullToEmpty(created.redirectUrl()),
                        subscription.merTradeNo(), status.name());
                return null;
            });
            return new CreateResult(String.valueOf(subscription.id()), nullToEmpty(created.providerSubscriptionId()),
                    nullToEmpty(created.redirectUrl()), subscription.merTradeNo(), status.name());
        } catch (BillingValidationException e) {
            inTransaction(() -> {
                repository.markCommandFailed(existing.id(), e.errorCode().code(), e.getMessage());
                repository.updateSubscriptionStatus(subscription.id(), ProviderSubscriptionStatus.ENDED);
                return null;
            });
            throw e;
        } catch (RuntimeException e) {
            inTransaction(() -> {
                repository.markCommandUnknown(existing.id(), MembershipBillingErrorCode.BILLING_UNAVAILABLE.code(),
                        "provider outcome is unknown; retry with the same request_id");
                return null;
            });
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "provider outcome is unknown; retry with the same request_id");
        }
    }

    private ActionResult replayOrResumeAction(
            BillingCommand existing,
            SubscriptionOperation operation,
            String requestId,
            String requestHash
    ) {
        assertSameCommand(existing, operation, requestId, requestHash);
        if (existing.status() == BillingCommandStatus.FAILED) {
            throw replayError(existing);
        }
        if (existing.status() == BillingCommandStatus.SUCCEEDED) {
            return replayActionSnapshot(existing);
        }
        claimForReconciliation(existing);
        if (existing.resultSubscriptionId() == null) {
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE, "idempotent action result is missing");
        }
        MembershipProviderSubscription subscription = repository.findSubscriptionById(existing.resultSubscriptionId())
                .orElseThrow(() -> invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                        "idempotent result subscription is missing"));
        String reason = nullToEmpty(existing.actionReason());
        try {
            if (isBlank(subscription.providerSubscriptionId()) && operation != SubscriptionOperation.END_SUBSCRIPTION) {
                throw invalid(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED,
                        "provider subscription is not ready for this action");
            }
            ProviderActionResult providerResult = null;
            if (!isBlank(subscription.providerSubscriptionId())) {
                SubscriptionProviderAdapter providerAdapter = requireAdapterForSubscription(subscription, operation);
                SubscriptionProviderAction providerAction = new SubscriptionProviderAction(
                        subscription, operation, reason, existing.id().toString());
                providerResult = switch (operation) {
                    case SUSPEND_SUBSCRIPTION -> providerAdapter.suspend(providerAction);
                    case RESTART_SUBSCRIPTION -> providerAdapter.restart(providerAction);
                    case END_SUBSCRIPTION -> providerAdapter.end(providerAction);
                    case CREATE_SUBSCRIPTION -> throw new IllegalStateException("create is not an action");
                };
            }
            ProviderSubscriptionStatus status = targetStatus(operation, providerResult);
            inTransaction(() -> {
                repository.updateSubscriptionStatus(subscription.id(), status);
                repository.markCommandSucceeded(existing.id(), subscription.id(),
                        nullToEmpty(subscription.providerSubscriptionId()), "", subscription.merTradeNo(), status.name());
                return null;
            });
            return new ActionResult(String.valueOf(subscription.id()),
                    nullToEmpty(subscription.providerSubscriptionId()), status.name());
        } catch (BillingValidationException e) {
            inTransaction(() -> {
                repository.markCommandFailed(existing.id(), e.errorCode().code(), e.getMessage());
                return null;
            });
            throw e;
        } catch (RuntimeException e) {
            inTransaction(() -> {
                repository.markCommandUnknown(existing.id(), MembershipBillingErrorCode.BILLING_UNAVAILABLE.code(),
                        "provider outcome is unknown; retry with the same request_id");
                return null;
            });
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "provider outcome is unknown; retry with the same request_id");
        }
    }

    private CreatePreparation prepareCreate(
            CreateCommand command,
            SubscriptionOperation operation,
            String requestHash
    ) {
        Optional<BillingCommand> existing = repository.findCommand(operation.value(), command.requestId());
        if (existing.isPresent()) {
            return new CreatePreparation(null, null, null, null, existing.get(), null);
        }
        Optional<MembershipProviderSubscription> sameQuote = repository.findSubscriptionByQuoteId(
                command.quote().quoteId());
        if (sameQuote.isPresent()) {
            return new CreatePreparation(null, null, null, null, null, sameQuote.get());
        }
        if (repository.findActiveSubscription(command.subject()).isPresent()) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "one active BASE_MEMBERSHIP subscription per subject is allowed");
        }

        ProviderAccountProfile account = repository.findProviderAccount(command.quote().providerAccountRef())
                .filter(ProviderAccountProfile::isActive)
                .orElseThrow(() -> invalid(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED,
                        "provider account is missing or disabled"));
        SubscriptionProviderAdapter providerAdapter = requireAdapter(account, operation);
        int sequence = repository.nextSubscriptionSequence(command.subject());
        if (sequence <= 0) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "subscription sequence must be positive");
        }
        long subscriptionId = nextSubscriptionId();
        MembershipProviderSubscription subscription = new MembershipProviderSubscription(
                subscriptionId, command.subject(), command.membershipId(), command.membershipChangeRequestId(),
                command.quote().quoteId(), BASE_MEMBERSHIP, account.accountRef(), account.provider(), null,
                sequence, merTradeNo(command.subject(), sequence), command.quote().offerId(),
                command.quote().offerVersionNo(), command.quote().snapshotHash(), command.quote().expiresAt(),
                command.quote().currency(), command.quote().unitAmountMinor(),
                command.quote().intervalUnit().name(), command.quote().intervalCount(),
                command.quote().firstPeriodDate(), null, ProviderSubscriptionStatus.PENDING);
        UUID commandId = UUID.randomUUID();
        // Claim the idempotency key before inserting the subscription. The command row keeps a
        // nullable result reference until the parent subscription exists; binding it below keeps
        // the database FK immediate without sacrificing the command-first race boundary.
        BillingCommand pendingCommand = new BillingCommand(
                commandId, operation.value(), command.subject(), command.requestId(), requestHash,
                command.quote().quoteId(), null, BillingCommandStatus.PROCESSING, null,
                null, null, null, null, null, null);
        if (!repository.insertCommand(pendingCommand)) {
            BillingCommand raced = repository.findCommand(operation.value(), command.requestId())
                    .orElseThrow(() -> invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                            "idempotency command was claimed but its row is not readable"));
            return new CreatePreparation(null, null, null, null, raced, null);
        }
        repository.insertSubscription(subscription);
        repository.bindCommandSubscription(pendingCommand.id(), subscription.id());
        return new CreatePreparation(pendingCommand, subscription, account, providerAdapter, null, null);
    }

    private ActionPreparation prepareAction(
            ActionCommand command,
            SubscriptionOperation operation,
            String requestHash,
            long subscriptionId,
            String reason
    ) {
        Optional<BillingCommand> existing = repository.findCommand(operation.value(), command.requestId());
        if (existing.isPresent()) {
            return new ActionPreparation(existing.get());
        }
        MembershipProviderSubscription subscription = repository.findSubscriptionById(subscriptionId)
                .orElseThrow(() -> invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                        "period_subscription_id does not exist"));
        validateActionState(subscription, operation);
        if (repository.findActiveCommandBySubscriptionId(subscriptionId).isPresent()) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "subscription already has a lifecycle command in progress");
        }
        UUID commandId = UUID.randomUUID();
        BillingCommand pendingCommand = new BillingCommand(
                commandId, operation.value(), subscription.subject(), command.requestId(), requestHash, null,
                subscriptionId, BillingCommandStatus.PROCESSING, null, null, null, null, null, null, reason);
        if (!repository.insertCommand(pendingCommand)) {
            Optional<BillingCommand> raced = repository.findCommand(operation.value(), command.requestId());
            if (raced.isPresent()) {
                return new ActionPreparation(raced.get());
            }
            if (repository.findActiveCommandBySubscriptionId(subscriptionId).isPresent()) {
                throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                        "subscription already has a lifecycle command in progress");
            }
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "idempotency command was claimed but its row is not readable");
        }
        return new ActionPreparation(subscription, pendingCommand);
    }

    private CreateResult replayQuoteBoundCreate(
            MembershipProviderSubscription subscription,
            CreateCommand command,
            SubscriptionOperation operation,
            String requestHash
    ) {
        validateQuoteReplay(subscription, command);
        BillingCommand original = inTransaction(
                () -> repository.findCreateCommandByQuoteId(command.quote().quoteId()))
                .orElseThrow(() -> invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                        "quote is already bound but its original billing command is missing"));
        // The quote hash intentionally excludes request_id. Reuse the original command identity
        // so a different caller key still replays/resumes the same provider idempotency boundary.
        return replayOrResumeCreate(original, operation, original.idempotencyKey(), requestHash);
    }

    private void validateCreateCommand(CreateCommand command) {
        if (command == null || command.subject() == null) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBJECT_INVALID, "create command subject is required");
        }
        requireNonBlank(command.requestId(), MembershipBillingErrorCode.BILLING_QUOTE_INVALID,
                "create request_id is required");
        validateQuoteBinding(command.subject(), command.membershipId(), command.membershipChangeRequestId(), command.quote());
    }

    private CreateResult replayCreateSnapshot(BillingCommand command) {
        return new CreateResult(
                String.valueOf(command.resultSubscriptionId()),
                nullToEmpty(command.resultProviderSubscriptionId()),
                nullToEmpty(command.resultRedirectUrl()),
                nullToEmpty(command.resultMerTradeNo()),
                nullToEmpty(command.resultStatus()));
    }

    private ActionResult replayActionSnapshot(BillingCommand command) {
        return new ActionResult(
                String.valueOf(command.resultSubscriptionId()),
                nullToEmpty(command.resultProviderSubscriptionId()),
                nullToEmpty(command.resultStatus()));
    }

    private BillingValidationException replayError(BillingCommand command) {
        MembershipBillingErrorCode code;
        try {
            code = MembershipBillingErrorCode.fromCode(command.errorCode());
        } catch (IllegalArgumentException ignored) {
            code = MembershipBillingErrorCode.BILLING_UNAVAILABLE;
        }
        String message = isBlank(command.errorMessage())
                ? "the idempotent billing command previously failed"
                : command.errorMessage();
        return invalid(code, message);
    }

    private void claimForReconciliation(BillingCommand command) {
        if (command.status() == BillingCommandStatus.PROCESSING) {
            if (!inTransaction(() -> repository.claimStaleProcessingCommand(command.id()))) {
                throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                        "billing command is already in progress; retry with the same request_id");
            }
            return;
        }
        if (command.status() == BillingCommandStatus.UNKNOWN
                && !inTransaction(() -> repository.claimUnknownCommand(command.id()))) {
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "billing command reconciliation is already claimed; retry with the same request_id");
        }
    }

    private BillingQuote quoteFromSubscription(MembershipProviderSubscription subscription) {
        return new BillingQuote(
                subscription.billingQuoteId(), subscription.subject(), subscription.membershipId(),
                subscription.membershipChangeRequestId(), subscription.offerId(), subscription.offerVersionNo(),
                subscription.unitAmountMinor(), subscription.currency(),
                BillingIntervalUnit.valueOf(subscription.intervalUnit()), subscription.intervalCount(),
                subscription.providerAccountRef(), subscription.firstPeriodDate(), subscription.quoteExpiresAt(),
                subscription.quoteSnapshotHash());
    }

    private <T> T inTransaction(Supplier<T> action) {
        if (transactionTemplate == null) {
            return action.get();
        }
        return transactionTemplate.execute(status -> action.get());
    }

    private void assertSameCommand(
            BillingCommand existing,
            SubscriptionOperation operation,
            String requestId,
            String requestHash
    ) {
        var decision = BillingCommandRules.decide(
                new BillingCommandIdentity(existing.operation(), existing.idempotencyKey(), existing.requestHash()),
                new BillingCommandIdentity(operation.value(), requestId, requestHash));
        if (decision == io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommandDecision.CONFLICT) {
            throw invalid(MembershipBillingErrorCode.BILLING_COMMAND_CONFLICT,
                    "same idempotency key was used with a different request hash");
        }
    }

    private void validateQuoteBinding(
            SubscriptionSubject subject,
            String membershipId,
            String changeRequestId,
            BillingQuote quote
    ) {
        if (quote == null || quote.subject() == null || !subject.equals(quote.subject())
                || !equalsNonNull(membershipId, quote.membershipId())
                || !equalsNonNull(changeRequestId, quote.membershipChangeRequestId())) {
            throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID,
                    "quote subject and membership change binding do not match the command");
        }
    }

    /**
     * A quote id is a single-use immutable commercial snapshot. A retry with a different request
     * id may reuse that quote only when every persisted binding and term is byte-for-byte equal.
     */
    private void validateQuoteReplay(
            MembershipProviderSubscription subscription,
            CreateCommand command
    ) {
        BillingQuote quote = command.quote();
        boolean matches = subscription.subject().equals(command.subject())
                && equalsNonNull(subscription.membershipId(), command.membershipId())
                && equalsNonNull(subscription.membershipChangeRequestId(), command.membershipChangeRequestId())
                && equalsNonNull(subscription.billingQuoteId(), quote.quoteId())
                && equalsNonNull(subscription.offerId(), quote.offerId())
                && equalsNonNull(subscription.offerVersionNo(), quote.offerVersionNo())
                && subscription.unitAmountMinor() == quote.unitAmountMinor()
                && equalsNonNull(subscription.currency(), quote.currency())
                && equalsNonNull(subscription.intervalUnit(), quote.intervalUnit().name())
                && subscription.intervalCount() == quote.intervalCount()
                && equalsNonNull(subscription.providerAccountRef(), quote.providerAccountRef())
                && equalsNonNull(subscription.quoteSnapshotHash(), quote.snapshotHash())
                && sameTime(subscription.quoteExpiresAt(), quote.expiresAt())
                && sameTime(subscription.firstPeriodDate(), quote.firstPeriodDate());
        if (!matches) {
            throw invalid(MembershipBillingErrorCode.BILLING_QUOTE_INVALID,
                    "billing_quote_id is already bound to a different immutable snapshot");
        }
    }

    private static boolean sameTime(LocalDateTime left, LocalDateTime right) {
        return left == null ? right == null : left.equals(right);
    }

    private static void requireNonBlank(String value, MembershipBillingErrorCode code, String message) {
        if (value == null || value.isBlank()) {
            throw invalid(code, message);
        }
    }

    private SubscriptionProviderAdapter requireAdapter(ProviderAccountProfile account,
                                                       SubscriptionOperation operation) {
        if (account == null) {
            throw invalid(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED,
                    "provider account is required");
        }
        if (!account.isActive() || !account.supports(operation.requiredCapability())) {
            throw invalid(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED,
                    "provider account does not support " + operation.requiredCapability());
        }
        return providerRegistry.require(account.provider());
    }

    private SubscriptionProviderAdapter requireAdapterForSubscription(
            MembershipProviderSubscription subscription,
            SubscriptionOperation operation
    ) {
        ProviderAccountProfile account = repository.findProviderAccount(subscription.providerAccountRef())
                .orElseThrow(() -> invalid(MembershipBillingErrorCode.PROVIDER_CAPABILITY_UNSUPPORTED,
                        "provider account is missing or disabled"));
        return requireAdapter(account, operation);
    }

    private ProviderSubscriptionStatus parseCreateStatus(ProviderSubscriptionCreated created) {
        if (created == null) {
            throw invalid(MembershipBillingErrorCode.PROVIDER_REJECTED, "provider returned no create result");
        }
        if (isBlank(created.status())) {
            return ProviderSubscriptionStatus.PENDING;
        }
        try {
            ProviderSubscriptionStatus status = ProviderSubscriptionStatus.valueOf(created.status());
            if (status == ProviderSubscriptionStatus.ENDED || status == ProviderSubscriptionStatus.SUSPENDED) {
                throw invalid(MembershipBillingErrorCode.PROVIDER_REJECTED,
                        "provider create returned an invalid lifecycle status");
            }
            if (status == ProviderSubscriptionStatus.ACTIVE && isBlank(created.providerSubscriptionId())) {
                throw invalid(MembershipBillingErrorCode.PROVIDER_REJECTED,
                        "active provider subscription must have a provider subscription id");
            }
            return status;
        } catch (IllegalArgumentException e) {
            throw invalid(MembershipBillingErrorCode.PROVIDER_REJECTED, "provider create status is invalid");
        }
    }

    private ProviderSubscriptionStatus targetStatus(
            SubscriptionOperation operation,
            ProviderActionResult result
    ) {
        ProviderSubscriptionStatus expected = switch (operation) {
            case SUSPEND_SUBSCRIPTION -> ProviderSubscriptionStatus.SUSPENDED;
            case RESTART_SUBSCRIPTION -> ProviderSubscriptionStatus.ACTIVE;
            case END_SUBSCRIPTION -> ProviderSubscriptionStatus.ENDED;
            case CREATE_SUBSCRIPTION -> throw new IllegalStateException("create is not an action");
        };
        if (result == null || isBlank(result.status())) {
            return expected;
        }
        try {
            ProviderSubscriptionStatus providerStatus = ProviderSubscriptionStatus.valueOf(result.status());
            if (providerStatus != expected) {
                throw invalid(MembershipBillingErrorCode.PROVIDER_REJECTED,
                        "provider action returned unexpected status");
            }
            return providerStatus;
        } catch (IllegalArgumentException e) {
            throw invalid(MembershipBillingErrorCode.PROVIDER_REJECTED, "provider action status is invalid");
        }
    }

    private void validateActionState(MembershipProviderSubscription subscription, SubscriptionOperation operation) {
        ProviderSubscriptionStatus status = subscription.status();
        boolean allowed = switch (operation) {
            case SUSPEND_SUBSCRIPTION -> status == ProviderSubscriptionStatus.PENDING
                    || status == ProviderSubscriptionStatus.ACTIVE;
            case RESTART_SUBSCRIPTION -> status == ProviderSubscriptionStatus.SUSPENDED;
            case END_SUBSCRIPTION -> status != ProviderSubscriptionStatus.ENDED;
            case CREATE_SUBSCRIPTION -> false;
        };
        if (!allowed) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "subscription lifecycle does not allow " + operation.value());
        }
    }

    private static CreateResult toCreateResult(MembershipProviderSubscription subscription, String redirectUrl) {
        return new CreateResult(
                String.valueOf(subscription.id()),
                nullToEmpty(subscription.providerSubscriptionId()),
                redirectUrl,
                subscription.merTradeNo(),
                subscription.status().name());
    }

    private static long parseSubscriptionId(String value) {
        if (value == null || !value.matches("[1-9][0-9]*")) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "period_subscription_id must be a positive decimal TSID");
        }
        try {
            return Long.parseLong(value);
        } catch (NumberFormatException e) {
            throw invalid(MembershipBillingErrorCode.BILLING_SUBSCRIPTION_CONFLICT,
                    "period_subscription_id is out of range");
        }
    }

    private static String merTradeNo(SubscriptionSubject subject, int sequence) {
        return "M" + subject.subjectId() + "-S" + sequence;
    }

    private long nextSubscriptionId() {
        Long value = idGenerator.get();
        if (value == null || value <= 0) {
            throw invalid(MembershipBillingErrorCode.BILLING_UNAVAILABLE,
                    "subscription id generator returned an invalid value");
        }
        return value;
    }

    private static boolean equalsNonNull(String left, String right) {
        return left != null && left.equals(right);
    }

    private static boolean isBlank(String value) {
        return value == null || value.isBlank();
    }

    private static String nullToEmpty(String value) {
        return value == null ? "" : value;
    }

    private static BillingValidationException invalid(MembershipBillingErrorCode code, String message) {
        return new BillingValidationException(code, message);
    }

    public record CreateCommand(
            SubscriptionSubject subject,
            String membershipId,
            String membershipChangeRequestId,
            BillingQuote quote,
            String requestId
    ) {
    }

    public record ActionCommand(String periodSubscriptionId, String requestId, String reason) {
    }

    public record CreateResult(
            String periodSubscriptionId,
            String providerSubscriptionId,
            String redirectUrl,
            String merTradeNo,
            String status
    ) {
    }

    public record ActionResult(String periodSubscriptionId, String providerSubscriptionId, String status) {
        /** Source compatibility for callers compiled against the first draft of the contract. */
        public ActionResult(String periodSubscriptionId, String status) {
            this(periodSubscriptionId, "", status);
        }
    }

    private record CreatePreparation(
            BillingCommand command,
            MembershipProviderSubscription subscription,
            ProviderAccountProfile account,
            SubscriptionProviderAdapter providerAdapter,
            BillingCommand replayCommand,
            MembershipProviderSubscription replaySubscription
    ) {
    }

    private record ActionPreparation(
            MembershipProviderSubscription subscription,
            BillingCommand command,
            BillingCommand replayCommand
    ) {
        private ActionPreparation(MembershipProviderSubscription subscription, BillingCommand command) {
            this(subscription, command, null);
        }

        private ActionPreparation(BillingCommand replayCommand) {
            this(null, null, replayCommand);
        }
    }
}
