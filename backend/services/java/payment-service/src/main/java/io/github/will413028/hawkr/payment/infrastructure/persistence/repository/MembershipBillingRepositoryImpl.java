package io.github.will413028.hawkr.payment.infrastructure.persistence.repository;

import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommand;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.BillingCommandStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipChargeAttempt;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipBillingRepository;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.MembershipProviderSubscription;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeResult;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderChargeStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderAccountProfile;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionCreated;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.ProviderSubscriptionStatus;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubscriptionSubject;
import io.github.will413028.hawkr.payment.domain.subscriptionbilling.SubjectType;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.BillingCommandEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.BillingProviderAccountEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.MembershipChargeAttemptEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.entity.MembershipProviderSubscriptionEntity;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.BillingCommandMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.BillingProviderAccountMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.MembershipChargeAttemptMapper;
import io.github.will413028.hawkr.payment.infrastructure.persistence.mapper.MembershipProviderSubscriptionMapper;
import java.util.Optional;
import java.util.UUID;
import java.time.Duration;
import java.time.LocalDateTime;
import org.springframework.stereotype.Repository;

/** MyBatis implementation of the shared billing persistence port. */
@Repository
public class MembershipBillingRepositoryImpl implements MembershipBillingRepository {

    private static final Duration PROVIDER_CALL_LEASE = Duration.ofMinutes(5);

    private final BillingProviderAccountMapper accountMapper;
    private final BillingCommandMapper commandMapper;
    private final MembershipProviderSubscriptionMapper subscriptionMapper;
    private final MembershipChargeAttemptMapper chargeAttemptMapper;

    public MembershipBillingRepositoryImpl(
            BillingProviderAccountMapper accountMapper,
            BillingCommandMapper commandMapper,
            MembershipProviderSubscriptionMapper subscriptionMapper,
            MembershipChargeAttemptMapper chargeAttemptMapper
    ) {
        this.accountMapper = accountMapper;
        this.commandMapper = commandMapper;
        this.subscriptionMapper = subscriptionMapper;
        this.chargeAttemptMapper = chargeAttemptMapper;
    }

    @Override
    public Optional<ProviderAccountProfile> findProviderAccount(String accountRef) {
        return Optional.ofNullable(accountMapper.findByAccountRef(accountRef)).map(this::toDomain);
    }

    @Override
    public Optional<BillingCommand> findCommand(String operation, String idempotencyKey) {
        return Optional.ofNullable(commandMapper.findByOperationAndKey(operation, idempotencyKey))
                .map(this::toDomain);
    }

    @Override
    public Optional<BillingCommand> findCreateCommandByQuoteId(String billingQuoteId) {
        return Optional.ofNullable(commandMapper.findCreateByQuoteId(billingQuoteId))
                .map(this::toDomain);
    }

    @Override
    public Optional<BillingCommand> findActiveCommandBySubscriptionId(long subscriptionId) {
        return Optional.ofNullable(commandMapper.findActiveBySubscriptionId(subscriptionId))
                .map(this::toDomain);
    }

    @Override
    public boolean insertCommand(BillingCommand command) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        BillingCommandEntity entity = new BillingCommandEntity();
        entity.setId(command.id());
        entity.setOperation(command.operation());
        entity.setSubjectType(command.subject().subjectType().name());
        entity.setSubjectId(command.subject().subjectId());
        entity.setIdempotencyKey(command.idempotencyKey());
        entity.setRequestHash(command.requestHash());
        entity.setBillingQuoteId(command.billingQuoteId());
        entity.setResultSubscriptionId(command.resultSubscriptionId());
        entity.setStatus(command.status().name());
        entity.setErrorCode(command.errorCode());
        entity.setErrorMessage(command.errorMessage());
        entity.setResultProviderSubscriptionId(command.resultProviderSubscriptionId());
        entity.setResultRedirectUrl(command.resultRedirectUrl());
        entity.setResultMerTradeNo(command.resultMerTradeNo());
        entity.setResultStatus(command.resultStatus());
        entity.setActionReason(command.actionReason());
        entity.setLeaseExpiresAt(isInFlight(command.status()) ? now.plus(PROVIDER_CALL_LEASE) : null);
        return commandMapper.insertIfAbsent(entity) == 1;
    }

    @Override
    public void bindCommandSubscription(UUID commandId, long subscriptionId) {
        int updated = commandMapper.bindSubscription(commandId, subscriptionId, DateTimeUtils.nowUtc());
        if (updated != 1) {
            throw new IllegalStateException(
                    "billing command subscription binding was lost: " + commandId);
        }
    }

    @Override
    public boolean claimUnknownCommand(UUID commandId) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        return commandMapper.claimUnknown(commandId, now, now.plus(PROVIDER_CALL_LEASE)) == 1;
    }

    @Override
    public boolean claimStaleProcessingCommand(UUID commandId) {
        LocalDateTime now = DateTimeUtils.nowUtc();
        return commandMapper.claimStaleProcessing(commandId, now, now.plus(PROVIDER_CALL_LEASE)) == 1;
    }

    private static boolean isInFlight(BillingCommandStatus status) {
        return status == BillingCommandStatus.PROCESSING || status == BillingCommandStatus.UNKNOWN;
    }

    @Override
    public void markCommandSucceeded(
            UUID commandId,
            long subscriptionId,
            String providerSubscriptionId,
            String redirectUrl,
            String merTradeNo,
            String status
    ) {
        int updated = commandMapper.markSucceeded(commandId, subscriptionId, providerSubscriptionId, redirectUrl,
                merTradeNo, status, DateTimeUtils.nowUtc());
        requireSingleCommandUpdate(updated, commandId, "SUCCEEDED");
    }

    @Override
    public void markCommandFailed(UUID commandId, String errorCode, String errorMessage) {
        int updated = commandMapper.markFailed(commandId, errorCode, errorMessage, DateTimeUtils.nowUtc());
        requireSingleCommandUpdate(updated, commandId, "FAILED");
    }

    @Override
    public void markCommandUnknown(UUID commandId, String errorCode, String errorMessage) {
        int updated = commandMapper.markUnknown(commandId, errorCode, errorMessage, DateTimeUtils.nowUtc());
        requireSingleCommandUpdate(updated, commandId, "UNKNOWN");
    }

    private static void requireSingleCommandUpdate(int updated, UUID commandId, String targetStatus) {
        if (updated != 1) {
            throw new IllegalStateException(
                    "billing command transition to " + targetStatus + " lost its PROCESSING claim: " + commandId);
        }
    }

    @Override
    public Optional<MembershipProviderSubscription> findSubscriptionByQuoteId(String quoteId) {
        return Optional.ofNullable(subscriptionMapper.findByQuoteId(quoteId)).map(this::toDomain);
    }

    @Override
    public Optional<MembershipProviderSubscription> findSubscriptionById(long subscriptionId) {
        return Optional.ofNullable(subscriptionMapper.findByIdForUpdate(subscriptionId)).map(this::toDomain);
    }

    @Override
    public Optional<MembershipProviderSubscription> findActiveSubscription(SubscriptionSubject subject) {
        return Optional.ofNullable(subscriptionMapper.findActive(
                subject.subjectType().name(), subject.subjectId())).map(this::toDomain);
    }

    @Override
    public int nextSubscriptionSequence(SubscriptionSubject subject) {
        Integer next = subscriptionMapper.nextSequence(subject.subjectType().name(), subject.subjectId());
        return next == null ? 1 : next;
    }

    @Override
    public void insertSubscription(MembershipProviderSubscription subscription) {
        MembershipProviderSubscriptionEntity entity = new MembershipProviderSubscriptionEntity();
        entity.setId(subscription.id());
        entity.setSubjectType(subscription.subject().subjectType().name());
        entity.setSubjectId(subscription.subject().subjectId());
        entity.setMembershipId(subscription.membershipId());
        entity.setMembershipChangeRequestId(subscription.membershipChangeRequestId());
        entity.setBillingQuoteId(subscription.billingQuoteId());
        entity.setSubscriptionKind(subscription.subscriptionKind());
        entity.setProviderAccountRef(subscription.providerAccountRef());
        entity.setProvider(subscription.provider());
        entity.setProviderSubscriptionId(subscription.providerSubscriptionId());
        entity.setSubscriptionSeq(subscription.subscriptionSeq());
        entity.setMerTradeNo(subscription.merTradeNo());
        entity.setOfferId(subscription.offerId());
        entity.setOfferVersionNo(subscription.offerVersionNo());
        entity.setQuoteSnapshotHash(subscription.quoteSnapshotHash());
        entity.setQuoteExpiresAt(subscription.quoteExpiresAt());
        entity.setCurrency(subscription.currency());
        entity.setUnitAmountMinor(subscription.unitAmountMinor());
        entity.setIntervalUnit(subscription.intervalUnit());
        entity.setIntervalCount(subscription.intervalCount());
        entity.setFirstPeriodDate(subscription.firstPeriodDate());
        entity.setProviderAnchorAt(subscription.providerAnchorAt());
        entity.setStatus(subscription.status().name());
        subscriptionMapper.insert(entity);
    }

    @Override
    public void attachProviderSubscription(long subscriptionId, ProviderSubscriptionCreated created) {
        String status = created.status() == null || created.status().isBlank()
                ? ProviderSubscriptionStatus.PENDING.name() : created.status();
        subscriptionMapper.attachProviderSubscription(
                subscriptionId,
                null,
                created.providerSubscriptionId(),
                created.firstPeriodDate(),
                created.providerAnchorAt(),
                status);
    }

    @Override
    public void updateSubscriptionStatus(long subscriptionId, ProviderSubscriptionStatus status) {
        subscriptionMapper.updateStatus(subscriptionId, status.name());
    }

    @Override
    public Optional<MembershipProviderSubscription> findSubscriptionByProviderIdentity(
            String provider, String providerSubscriptionId) {
        return Optional.ofNullable(subscriptionMapper.findByProviderIdentityForUpdate(
                provider, providerSubscriptionId)).map(this::toDomain);
    }

    @Override
    public Optional<MembershipChargeAttempt> findChargeAttemptByProviderEventKey(String providerEventKey) {
        return Optional.ofNullable(chargeAttemptMapper.findByProviderEventKey(providerEventKey))
                .map(this::toChargeAttempt);
    }

    @Override
    public Optional<MembershipChargeAttempt> findAuthoritativeChargeAttempt(
            long providerSubscriptionRef, String providerPeriodId, String periodOrderNo) {
        return Optional.ofNullable(chargeAttemptMapper.findAuthoritative(
                providerSubscriptionRef, providerPeriodId, periodOrderNo)).map(this::toChargeAttempt);
    }

    @Override
    public void insertChargeAttempt(MembershipChargeAttempt attempt) {
        MembershipChargeAttemptEntity entity = new MembershipChargeAttemptEntity();
        ProviderChargeResult result = attempt.result();
        entity.setId(attempt.id());
        entity.setProviderSubscriptionRef(attempt.providerSubscriptionRef());
        entity.setSubjectType(attempt.subject().subjectType().name());
        entity.setSubjectId(attempt.subject().subjectId());
        entity.setMembershipId(attempt.membershipId());
        entity.setMembershipChangeRequestId(attempt.membershipChangeRequestId());
        entity.setBillingQuoteId(result.billingQuoteId());
        entity.setQuoteSnapshotHash(result.quoteSnapshotHash());
        entity.setProviderPeriodId(result.providerPeriodId());
        entity.setPeriodOrderNo(result.periodOrderNo());
        entity.setThisPeriod(result.thisPeriod());
        entity.setTradeNo(result.tradeNo());
        entity.setAmountMinor(result.amountMinor());
        entity.setCurrency(result.currency());
        entity.setProviderEventId(result.providerEventId());
        entity.setProviderEventKey(result.providerEventKey());
        entity.setStatus(result.status().name());
        entity.setResCode(result.resCode());
        entity.setResCodeMsg(result.resCodeMsg());
        entity.setValidFrom(result.validFrom());
        entity.setValidUntil(result.validUntil());
        entity.setCycleStartedAt(result.cycleStartedAt());
        entity.setCycleUntil(result.cycleUntil());
        entity.setIsInitialAuthorization(result.initialAuthorization());
        entity.setIsAuthoritative(attempt.authoritative());
        entity.setCanonicalEventId(attempt.canonicalEventId());
        entity.setReceivedAt(attempt.receivedAt());
        chargeAttemptMapper.insert(entity);
    }

    private ProviderAccountProfile toDomain(BillingProviderAccountEntity entity) {
        return new ProviderAccountProfile(
                entity.getProvider(), entity.getAccountRef(), entity.getEnvironment(),
                entity.getCredentialRef(), entity.getCapabilities(), entity.getStatus());
    }

    private BillingCommand toDomain(BillingCommandEntity entity) {
        SubjectType subjectType = SubjectType.valueOf(entity.getSubjectType());
        return new BillingCommand(
                entity.getId(),
                entity.getOperation(),
                new SubscriptionSubject(subjectType, entity.getSubjectId()),
                entity.getIdempotencyKey(),
                entity.getRequestHash(),
                entity.getBillingQuoteId(),
                entity.getResultSubscriptionId(),
                BillingCommandStatus.valueOf(entity.getStatus()),
                entity.getErrorCode(),
                entity.getErrorMessage(),
                entity.getResultProviderSubscriptionId(),
                entity.getResultRedirectUrl(),
                entity.getResultMerTradeNo(),
                entity.getResultStatus(),
                entity.getActionReason());
    }

    private MembershipProviderSubscription toDomain(MembershipProviderSubscriptionEntity entity) {
        return new MembershipProviderSubscription(
                entity.getId(),
                new SubscriptionSubject(SubjectType.valueOf(entity.getSubjectType()), entity.getSubjectId()),
                entity.getMembershipId(),
                entity.getMembershipChangeRequestId(),
                entity.getBillingQuoteId(),
                entity.getSubscriptionKind(),
                entity.getProviderAccountRef(),
                entity.getProvider(),
                entity.getProviderSubscriptionId(),
                valueOrZero(entity.getSubscriptionSeq()),
                entity.getMerTradeNo(),
                entity.getOfferId(),
                entity.getOfferVersionNo(),
                entity.getQuoteSnapshotHash(),
                entity.getQuoteExpiresAt(),
                entity.getCurrency(),
                valueOrZero(entity.getUnitAmountMinor()),
                entity.getIntervalUnit(),
                valueOrZero(entity.getIntervalCount()),
                entity.getFirstPeriodDate(),
                entity.getProviderAnchorAt(),
                ProviderSubscriptionStatus.valueOf(entity.getStatus()));
    }

    private MembershipChargeAttempt toChargeAttempt(MembershipChargeAttemptEntity entity) {
        MembershipProviderSubscription subscription = Optional.ofNullable(
                subscriptionMapper.findByIdForUpdate(entity.getProviderSubscriptionRef()))
                .map(this::toDomain)
                .orElseThrow(() -> new IllegalStateException(
                        "charge attempt subscription is missing: " + entity.getProviderSubscriptionRef()));
        ProviderChargeResult result = new ProviderChargeResult(
                subscription.provider(),
                subscription.providerAccountRef(),
                subscription.providerSubscriptionId(),
                entity.getProviderPeriodId(),
                String.valueOf(subscription.id()),
                entity.getPeriodOrderNo(),
                entity.getTradeNo(),
                entity.getAmountMinor() == null ? 0L : entity.getAmountMinor(),
                entity.getCurrency() == null ? subscription.currency() : entity.getCurrency(),
                entity.getThisPeriod(),
                ProviderChargeStatus.valueOf(entity.getStatus()),
                entity.getResCode(),
                entity.getResCodeMsg(),
                entity.getValidFrom(),
                entity.getValidUntil(),
                entity.getCycleStartedAt(),
                entity.getCycleUntil(),
                Boolean.TRUE.equals(entity.getIsInitialAuthorization()),
                entity.getBillingQuoteId(),
                entity.getQuoteSnapshotHash(),
                entity.getProviderEventId(),
                entity.getProviderEventKey());
        return new MembershipChargeAttempt(
                entity.getId(), entity.getProviderSubscriptionRef(), subscription.subject(),
                entity.getMembershipId(), entity.getMembershipChangeRequestId(), result,
                Boolean.TRUE.equals(entity.getIsAuthoritative()), entity.getCanonicalEventId(), entity.getReceivedAt());
    }

    private static int valueOrZero(Integer value) {
        return value == null ? 0 : value;
    }

    private static long valueOrZero(Long value) {
        return value == null ? 0L : value;
    }
}
