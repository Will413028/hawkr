package io.github.will413028.hawkr.payment.adapter.grpc;

import io.github.will413028.hawkr.common.grpc.GrpcAuthContext;
import io.github.will413028.hawkr.common.obs.grpc.GrpcErrorHandler;
import io.github.will413028.hawkr.common.obs.log.ObsLogger;
import io.github.will413028.hawkr.common.utils.DateTimeUtils;
import io.github.will413028.hawkr.common.utils.code.AccountType;
import io.github.will413028.hawkr.common.utils.code.GrpcResponseCode;
import io.github.will413028.hawkr.common.utils.code.LogisticsReceivableStatus;
import io.github.will413028.hawkr.payment.grpc.AdminFulfillmentCaseResolutionRecord;
import io.github.will413028.hawkr.payment.grpc.AdminFulfillmentExceptionCaseRecord;
import io.github.will413028.hawkr.payment.grpc.AdminLogisticsAdjustmentRecord;
import io.github.will413028.hawkr.payment.grpc.AdminLogisticsCollectionRecord;
import io.github.will413028.hawkr.payment.grpc.AdminLogisticsReceivableDetail;
import io.github.will413028.hawkr.payment.grpc.AdminLogisticsReceivableSummary;
import io.github.will413028.hawkr.payment.grpc.AdminProviderSettlementLineRecord;
import io.github.will413028.hawkr.payment.grpc.AdminProviderStatementDetail;
import io.github.will413028.hawkr.payment.grpc.AdminProviderStatementSummary;
import io.github.will413028.hawkr.payment.grpc.AdminSellerLogisticsAccount;
import io.github.will413028.hawkr.payment.grpc.GetFulfillmentExceptionCaseForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.GetFulfillmentExceptionCaseForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.GetLogisticsReceivableForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.GetLogisticsReceivableForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.GetProviderStatementForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.GetProviderStatementForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.GetSellerLogisticsFinanceSummaryRequest;
import io.github.will413028.hawkr.payment.grpc.GetSellerLogisticsFinanceSummaryResponse;
import io.github.will413028.hawkr.payment.grpc.ListFulfillmentExceptionCasesForAdminData;
import io.github.will413028.hawkr.payment.grpc.ListFulfillmentExceptionCasesForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ListFulfillmentExceptionCasesForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsAdjustmentsForAdminData;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsAdjustmentsForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsAdjustmentsForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsCollectionsForAdminData;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsCollectionsForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsCollectionsForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsReceivablesForAdminData;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsReceivablesForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ListLogisticsReceivablesForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.ListProviderStatementsForAdminData;
import io.github.will413028.hawkr.payment.grpc.ListProviderStatementsForAdminRequest;
import io.github.will413028.hawkr.payment.grpc.ListProviderStatementsForAdminResponse;
import io.github.will413028.hawkr.payment.grpc.ListSellerLogisticsLedgerEntriesData;
import io.github.will413028.hawkr.payment.grpc.ListSellerLogisticsLedgerEntriesRequest;
import io.github.will413028.hawkr.payment.grpc.ListSellerLogisticsLedgerEntriesResponse;
import io.github.will413028.hawkr.payment.grpc.ListSellerLogisticsReceivablesData;
import io.github.will413028.hawkr.payment.grpc.ListSellerLogisticsReceivablesRequest;
import io.github.will413028.hawkr.payment.grpc.ListSellerLogisticsReceivablesResponse;
import io.github.will413028.hawkr.payment.grpc.LogisticsCollectionStatusCount;
import io.github.will413028.hawkr.payment.grpc.LogisticsFinanceServiceGrpc;
import io.github.will413028.hawkr.payment.grpc.LogisticsLedgerEntryRecord;
import io.github.will413028.hawkr.payment.grpc.LogisticsLedgerTxnRecord;
import io.github.will413028.hawkr.payment.grpc.SellerLogisticsFinanceSummary;
import io.github.will413028.hawkr.payment.grpc.SellerLogisticsReceivableRecord;
import io.github.will413028.hawkr.payment.logistics.domain.ResponsibilityImpactPreviewCodec;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.AdminCaseView;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.AdminReceivableDetail;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.AdminStatementDetail;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.LedgerTxnView;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.Paged;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.SellerFinanceSummary;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.SellerReceivableView;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.StatementLineView;
import io.github.will413028.hawkr.payment.logistics.application.LogisticsFinanceQueryService.StatusCount;
import io.github.will413028.hawkr.payment.logistics.repository.entity.FulfillmentCaseResolutionEntity;
import com.fasterxml.jackson.databind.ObjectMapper;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LedgerEntryEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsAdjustmentEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsCollectionAttemptEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.LogisticsReceivableEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.ProviderSettlementLineEntity;
import io.github.will413028.hawkr.payment.logistics.repository.entity.SellerLogisticsAccountEntity;
import io.github.will413028.hawkr.payment.logistics.repository.projection.LogisticsFinanceProjections.StatementAggregate;
import io.grpc.Status;
import io.grpc.StatusRuntimeException;
import io.grpc.stub.StreamObserver;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.List;
import net.devh.boot.grpc.server.service.GrpcService;

/**
 * 物流財務（線 B）的 gRPC 出口。<b>全部唯讀</b>。
 *
 * <h2>授權軸</h2>
 * <ul>
 *   <li><b>賣家面</b>（{@code getSellerLogisticsFinanceSummary} 等三支）：
 *       {@code requireAccountType(SELLER)} ＋ request 帶 Gateway 解出的現任 {@code store_id}，
 *       服務端以它強制過濾。🔴 {@code store_id <= 0} 一律 {@code UNAUTHENTICATED} 而<b>不是</b>
 *       空結果——空結果看起來就像「這家店沒有物流費」，Gateway 接線壞掉會一路沒人發現
 *       （比照 {@code PaymentGrpcService.getSellerPaymentHistory} 的同一個判斷）。</li>
 *   <li><b>Admin 面</b>（七支 {@code *ForAdmin}）：{@code requireAccountType(ADMIN)}。</li>
 * </ul>
 * 🔴 兩組<b>不共用任何一支 RPC</b>（Proto 管理規範 §5.1）：不得以 flag 或特殊值分流。
 *
 * <h2>型別邊界</h2>
 * 金額、計數、TSID 一律在這裡轉成「元」整數的數字字串（10 檔 §4）。
 * 🔴 <b>NULL 不是 0</b> 的欄位（{@code actual_cost}／{@code actual_fee}／{@code net_proceeds}／
 * {@code residual}／{@code fund_date}）一律以 proto3 {@code optional}「不設值」表達，
 * <b>不得</b>填 "0"：填了之後「還沒算出來」與「算出來是 0」在 wire 上就分不開了。
 *
 * <h2>🔴 Seller Subsidy（維度 6/11）永遠不設值</h2>
 * payment-service 不是那個維度的來源（V9 九張表零命中，它是 true-up 的瞬時輸入）。
 * 讀取模型裡也沒有那個欄位，所以「不小心填 0」在結構上不可能發生。
 */
@GrpcService
public class LogisticsFinanceGrpcService
        extends LogisticsFinanceServiceGrpc.LogisticsFinanceServiceImplBase {

    private static final ObsLogger log = ObsLogger.forClass(LogisticsFinanceGrpcService.class);

    private final LogisticsFinanceQueryService queryService;
    /** 只用於解 {@code impact_preview} JSON（唯讀投影）。 */
    private final ObjectMapper objectMapper;

    public LogisticsFinanceGrpcService(LogisticsFinanceQueryService queryService,
                                       ObjectMapper objectMapper) {
        this.queryService = queryService;
        this.objectMapper = objectMapper;
    }

    // =========================================================================
    // 賣家面
    // =========================================================================

    @Override
    public void getSellerLogisticsFinanceSummary(
            GetSellerLogisticsFinanceSummaryRequest request,
            StreamObserver<GetSellerLogisticsFinanceSummaryResponse> observer) {
        try {
            long storeId = requireSellerScope(request.getStoreId(), "getSellerLogisticsFinanceSummary");
            SellerFinanceSummary s = queryService.sellerSummary(storeId,
                    parseTime(request.getCreatedFrom(), "created_from"),
                    parseTime(request.getCreatedTo(), "created_to"));
            observer.onNext(GetSellerLogisticsFinanceSummaryResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(toSummary(s))
                    .build());
            observer.onCompleted();
        } catch (StatusRuntimeException e) {
            observer.onNext(GetSellerLogisticsFinanceSummaryResponse.newBuilder()
                    .setCode(mapStatus(e)).setMessage(describe(e)).build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    GetSellerLogisticsFinanceSummaryResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "getSellerLogisticsFinanceSummary");
        }
    }

    @Override
    public void listSellerLogisticsReceivables(
            ListSellerLogisticsReceivablesRequest request,
            StreamObserver<ListSellerLogisticsReceivablesResponse> observer) {
        try {
            long storeId = requireSellerScope(request.getStoreId(), "listSellerLogisticsReceivables");
            Paged<SellerReceivableView> paged = queryService.sellerReceivables(storeId,
                    parseStatuses(request.getStatus()),
                    parseTime(request.getCreatedFrom(), "created_from"),
                    parseTime(request.getCreatedTo(), "created_to"),
                    parseInt(request.getPage(), 1, "page"),
                    parseInt(request.getPageSize(), 20, "page_size"));

            ListSellerLogisticsReceivablesData.Builder data = ListSellerLogisticsReceivablesData.newBuilder()
                    .setTotal(str(paged.total()))
                    .setPage(str(paged.page()))
                    .setPageSize(str(paged.pageSize()));
            for (SellerReceivableView v : paged.records()) {
                data.addRecords(toSellerRecord(v));
            }
            observer.onNext(ListSellerLogisticsReceivablesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (StatusRuntimeException e) {
            observer.onNext(ListSellerLogisticsReceivablesResponse.newBuilder()
                    .setCode(mapStatus(e)).setMessage(describe(e)).build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ListSellerLogisticsReceivablesResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "listSellerLogisticsReceivables");
        }
    }

    @Override
    public void listSellerLogisticsLedgerEntries(
            ListSellerLogisticsLedgerEntriesRequest request,
            StreamObserver<ListSellerLogisticsLedgerEntriesResponse> observer) {
        try {
            long storeId = requireSellerScope(request.getStoreId(), "listSellerLogisticsLedgerEntries");
            Paged<LedgerTxnView> paged = queryService.sellerLedger(storeId,
                    parseInt(request.getPage(), 1, "page"),
                    parseInt(request.getPageSize(), 20, "page_size"));

            ListSellerLogisticsLedgerEntriesData.Builder data =
                    ListSellerLogisticsLedgerEntriesData.newBuilder()
                            .setTotal(str(paged.total()))
                            .setPage(str(paged.page()))
                            .setPageSize(str(paged.pageSize()));
            for (LedgerTxnView v : paged.records()) {
                data.addRecords(toLedgerTxn(v));
            }
            observer.onNext(ListSellerLogisticsLedgerEntriesResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (StatusRuntimeException e) {
            observer.onNext(ListSellerLogisticsLedgerEntriesResponse.newBuilder()
                    .setCode(mapStatus(e)).setMessage(describe(e)).build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ListSellerLogisticsLedgerEntriesResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "listSellerLogisticsLedgerEntries");
        }
    }

    // =========================================================================
    // Admin 面
    // =========================================================================

    @Override
    public void listLogisticsReceivablesForAdmin(
            ListLogisticsReceivablesForAdminRequest request,
            StreamObserver<ListLogisticsReceivablesForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Paged<LogisticsReceivableEntity> paged = queryService.adminReceivables(
                    parseOptionalLong(request.getStoreId(), "store_id"),
                    parseStatuses(request.getStatus()),
                    blankToNull(request.getProvider()),
                    blankToNull(request.getOrderNo()),
                    blankToNull(request.getProviderDeliveryRef()),
                    parseTime(request.getCreatedFrom(), "created_from"),
                    parseTime(request.getCreatedTo(), "created_to"),
                    request.getUnreconciledOnly(),
                    parseLongOrZero(request.getMinAgingDays(), "min_aging_days"),
                    parseInt(request.getPage(), 1, "page"),
                    parseInt(request.getPageSize(), 20, "page_size"));

            ListLogisticsReceivablesForAdminData.Builder data =
                    ListLogisticsReceivablesForAdminData.newBuilder()
                            .setTotal(str(paged.total()))
                            .setPage(str(paged.page()))
                            .setPageSize(str(paged.pageSize()));
            for (LogisticsReceivableEntity r : paged.records()) {
                data.addRecords(toAdminReceivable(r));
            }
            observer.onNext(ListLogisticsReceivablesForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ListLogisticsReceivablesForAdminResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "listLogisticsReceivablesForAdmin");
        }
    }

    @Override
    public void getLogisticsReceivableForAdmin(
            GetLogisticsReceivableForAdminRequest request,
            StreamObserver<GetLogisticsReceivableForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            long id = parseRequiredLong(request.getReceivableNo(), "receivable_no");
            AdminReceivableDetail detail = queryService.adminReceivable(id).orElse(null);
            if (detail == null) {
                observer.onNext(GetLogisticsReceivableForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("logistics receivable not found")
                        .build());
                observer.onCompleted();
                return;
            }
            AdminLogisticsReceivableDetail.Builder b = AdminLogisticsReceivableDetail.newBuilder()
                    .setReceivable(toAdminReceivable(detail.receivable()))
                    .setSellerAccount(toAccount(detail.receivable().getStoreId(), detail.account()));
            for (LogisticsCollectionAttemptEntity a : detail.collections()) {
                b.addCollections(toAdminCollection(a, detail.receivable().getStoreId()));
            }
            for (LogisticsAdjustmentEntity adj : detail.adjustments()) {
                b.addAdjustments(toAdminAdjustment(adj));
            }
            observer.onNext(GetLogisticsReceivableForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(b)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    GetLogisticsReceivableForAdminResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "getLogisticsReceivableForAdmin");
        }
    }

    @Override
    public void listLogisticsCollectionsForAdmin(
            ListLogisticsCollectionsForAdminRequest request,
            StreamObserver<ListLogisticsCollectionsForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Paged<LogisticsCollectionAttemptEntity> paged = queryService.adminCollections(
                    parseOptionalLong(request.getReceivableNo(), "receivable_no"),
                    blankToNull(request.getTradeNo()),
                    parseOptionalLong(request.getStoreId(), "store_id"),
                    blankToNull(request.getSubmitStatus()),
                    request.getPendingFundDateOnly(),
                    request.getResidualOnly(),
                    parseTime(request.getCreatedFrom(), "created_from"),
                    parseTime(request.getCreatedTo(), "created_to"),
                    parseInt(request.getPage(), 1, "page"),
                    parseInt(request.getPageSize(), 20, "page_size"));

            ListLogisticsCollectionsForAdminData.Builder data =
                    ListLogisticsCollectionsForAdminData.newBuilder()
                            .setTotal(str(paged.total()))
                            .setPage(str(paged.page()))
                            .setPageSize(str(paged.pageSize()));
            for (LogisticsCollectionAttemptEntity a : paged.records()) {
                // store_id 不在 attempt 上（它只存在於應收）；列表頁以 receivable_no 串回去。
                data.addRecords(toAdminCollection(a, null));
            }
            observer.onNext(ListLogisticsCollectionsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ListLogisticsCollectionsForAdminResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "listLogisticsCollectionsForAdmin");
        }
    }

    @Override
    public void listLogisticsAdjustmentsForAdmin(
            ListLogisticsAdjustmentsForAdminRequest request,
            StreamObserver<ListLogisticsAdjustmentsForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Paged<LogisticsAdjustmentEntity> paged = queryService.adminAdjustments(
                    parseOptionalLong(request.getStoreId(), "store_id"),
                    parseOptionalLong(request.getReceivableNo(), "receivable_no"),
                    blankToNull(request.getKind()),
                    blankToNull(request.getResponsibilityCause()),
                    parseOptionalLong(request.getStatementLineId(), "statement_line_id"),
                    parseTime(request.getCreatedFrom(), "created_from"),
                    parseTime(request.getCreatedTo(), "created_to"),
                    parseInt(request.getPage(), 1, "page"),
                    parseInt(request.getPageSize(), 20, "page_size"));

            ListLogisticsAdjustmentsForAdminData.Builder data =
                    ListLogisticsAdjustmentsForAdminData.newBuilder()
                            .setTotal(str(paged.total()))
                            .setPage(str(paged.page()))
                            .setPageSize(str(paged.pageSize()));
            for (LogisticsAdjustmentEntity adj : paged.records()) {
                data.addRecords(toAdminAdjustment(adj));
            }
            observer.onNext(ListLogisticsAdjustmentsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ListLogisticsAdjustmentsForAdminResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "listLogisticsAdjustmentsForAdmin");
        }
    }

    @Override
    public void listProviderStatementsForAdmin(
            ListProviderStatementsForAdminRequest request,
            StreamObserver<ListProviderStatementsForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Paged<StatementAggregate> paged = queryService.adminStatements(
                    blankToNull(request.getProvider()),
                    blankToNull(request.getStatementPeriod()),
                    request.getUnsettledOnly(),
                    parseInt(request.getPage(), 1, "page"),
                    parseInt(request.getPageSize(), 20, "page_size"));

            ListProviderStatementsForAdminData.Builder data =
                    ListProviderStatementsForAdminData.newBuilder()
                            .setTotal(str(paged.total()))
                            .setPage(str(paged.page()))
                            .setPageSize(str(paged.pageSize()));
            for (StatementAggregate s : paged.records()) {
                data.addRecords(toStatement(s));
            }
            observer.onNext(ListProviderStatementsForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ListProviderStatementsForAdminResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "listProviderStatementsForAdmin");
        }
    }

    @Override
    public void getProviderStatementForAdmin(
            GetProviderStatementForAdminRequest request,
            StreamObserver<GetProviderStatementForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            // 🔴 provider 必填：statement_ref 只在同一個 provider 內唯一，不帶會跨 provider 撈錯帳單。
            if (request.getProvider().isBlank() || request.getStatementRef().isBlank()) {
                throw Status.INVALID_ARGUMENT
                        .withDescription("provider 與 statement_ref 皆為必填")
                        .asRuntimeException();
            }
            AdminStatementDetail detail = queryService
                    .adminStatement(request.getProvider(), request.getStatementRef()).orElse(null);
            if (detail == null) {
                observer.onNext(GetProviderStatementForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.NOT_FOUND.code())
                        .setMessage("provider statement not found")
                        .build());
                observer.onCompleted();
                return;
            }
            AdminProviderStatementDetail.Builder b = AdminProviderStatementDetail.newBuilder()
                    .setStatement(toStatement(detail.statement()))
                    .setAwaitingReviewCount(str(detail.awaitingReviewCount()));
            for (StatementLineView line : detail.lines()) {
                b.addLines(toStatementLine(line));
            }
            observer.onNext(GetProviderStatementForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(b)
                    .build());
            observer.onCompleted();
        } catch (StatusRuntimeException e) {
            observer.onNext(GetProviderStatementForAdminResponse.newBuilder()
                    .setCode(mapStatus(e)).setMessage(describe(e)).build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    GetProviderStatementForAdminResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "getProviderStatementForAdmin");
        }
    }

    @Override
    public void listFulfillmentExceptionCasesForAdmin(
            ListFulfillmentExceptionCasesForAdminRequest request,
            StreamObserver<ListFulfillmentExceptionCasesForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            Paged<AdminCaseView> paged = queryService.adminExceptionCases(
                    splitCsv(request.getStatus()),
                    blankToNull(request.getSeverity()),
                    blankToNull(request.getCategory()),
                    blankToNull(request.getSubjectType()),
                    blankToNull(request.getSubjectRef()),
                    parseOptionalLong(request.getStoreId(), "store_id"),
                    request.getAwaitingReviewOnly(),
                    parseInt(request.getPage(), 1, "page"),
                    parseInt(request.getPageSize(), 20, "page_size"));

            ListFulfillmentExceptionCasesForAdminData.Builder data =
                    ListFulfillmentExceptionCasesForAdminData.newBuilder()
                            .setTotal(str(paged.total()))
                            .setPage(str(paged.page()))
                            .setPageSize(str(paged.pageSize()));
            for (AdminCaseView v : paged.records()) {
                data.addRecords(toAdminCase(v));
            }
            observer.onNext(ListFulfillmentExceptionCasesForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(data)
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    ListFulfillmentExceptionCasesForAdminResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "listFulfillmentExceptionCasesForAdmin");
        }
    }

    @Override
    public void getFulfillmentExceptionCaseForAdmin(
            GetFulfillmentExceptionCaseForAdminRequest request,
            StreamObserver<GetFulfillmentExceptionCaseForAdminResponse> observer) {
        try {
            GrpcAuthContext.requireAccountType(AccountType.ADMIN);
            String caseNo = blankToNull(request.getCaseNo());
            if (caseNo == null) {
                // 🔴 空 case_no 不得退化成「查全部然後取第一筆」——那會回一張隨機案件。
                observer.onNext(GetFulfillmentExceptionCaseForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.EXCEPTION_CASE_NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.EXCEPTION_CASE_NOT_FOUND.message())
                        .build());
                observer.onCompleted();
                return;
            }
            AdminCaseView view = queryService.adminExceptionCaseByNo(caseNo);
            if (view == null) {
                observer.onNext(GetFulfillmentExceptionCaseForAdminResponse.newBuilder()
                        .setCode(GrpcResponseCode.EXCEPTION_CASE_NOT_FOUND.code())
                        .setMessage(GrpcResponseCode.EXCEPTION_CASE_NOT_FOUND.message())
                        .build());
                observer.onCompleted();
                return;
            }
            observer.onNext(GetFulfillmentExceptionCaseForAdminResponse.newBuilder()
                    .setCode(GrpcResponseCode.SUCCESS.code())
                    .setMessage(GrpcResponseCode.SUCCESS.message())
                    .setData(toAdminCase(view))
                    .build());
            observer.onCompleted();
        } catch (Exception e) {
            GrpcErrorHandler.handleException(observer,
                    GetFulfillmentExceptionCaseForAdminResponse.newBuilder(), e,
                    "LogisticsFinanceGrpcService", "getFulfillmentExceptionCaseForAdmin");
        }
    }

    // =========================================================================
    // 映射：讀取模型 → proto
    // =========================================================================

    /**
     * 從 {@code impact_preview} JSON 取出退款金額，轉成元的整數字串。
     *
     * <p>🔴 <b>解不開時回空字串，不回 "0"</b>：ADM 要分得出「這個提案沒有影響預覽」
     * （非責任判定型別本來就沒有）與「預覽算出來是 0 元」（{@code BUYER_CAUSED} 的正常結果）。
     * 兩者在畫面上的意義完全相反——後者正是「買家拿不到退款」這個決定本身。
     *
     * <p>⚠️ 這裡刻意<b>不</b>讓解碼失敗上拋：本方法只服務唯讀的列表／詳情端點，
     * 一列壞掉的 JSON 不該讓整個 ADM 佇列查不出來。真正會依賴這個值做財務決定的是
     * {@code finalizeResponsibility} 的 stale guard，而它走的是嚴格解碼（解不開就拋）。
     */
    private String previewRefundAmount(String previewJson) {
        if (previewJson == null || previewJson.isBlank()) {
            return "";
        }
        try {
            return String.valueOf(ResponsibilityImpactPreviewCodec
                    .fromJson(objectMapper, previewJson).refundAmount());
        } catch (RuntimeException e) {
            log.warn("LogisticsFinanceGrpcService.impactPreview.undecodable",
                    "影響預覽 JSON 解不開，該欄回空字串（不影響佇列查詢）：" + e.getMessage());
            return "";
        }
    }

    /**
     * 十一個維度的彙總。
     *
     * <p>🔴 {@code seller_subsidy}（6/11）<b>刻意不設值</b>：本服務不是它的來源。
     * 🔴 {@code seller_credit}（11/11）與 {@code receivable_balance} 分兩欄設，永不合成淨額。
     */
    private static SellerLogisticsFinanceSummary toSummary(SellerFinanceSummary s) {
        SellerLogisticsFinanceSummary.Builder b = SellerLogisticsFinanceSummary.newBuilder()
                .setStoreId(str(s.storeId()))
                .setCreatedFrom(iso(s.createdFrom()))
                .setCreatedTo(iso(s.createdTo()))
                .setEstimatedProviderCost(str(s.estimatedProviderCost()))
                .setActualProviderCost(str(s.actualProviderCost()))
                .setUnreconciledCount(str(s.unreconciledCount()))
                .setSellerLogisticsCharge(str(s.sellerLogisticsCharge()))
                .setPayuniCollectionFee(str(s.payuniCollectionFee()))
                .setPendingFeeQueryCount(str(s.pendingFeeQueryCount()))
                .setPlatformSubsidy(str(s.platformSubsidy()))
                .setResponsibilityAdjustment(str(s.responsibilityAdjustment()))
                .setReconciliationAdjustment(str(s.reconciliationAdjustment()))
                .setCollectionResidual(str(s.collectionResidual()))
                .setSellerCredit(str(s.sellerCredit()))
                .setReceivableBalance(str(s.receivableBalance()))
                .setLastActivityAt(iso(s.lastActivityAt()))
                .setAgingDays(str(s.agingDays()))
                .setHasAccount(s.hasAccount());
        for (StatusCount c : s.collectionStatusCounts()) {
            b.addCollectionStatusCounts(LogisticsCollectionStatusCount.newBuilder()
                    .setStatus(c.status().value())
                    // label 取自 CodeMeta enum，不在任何地方硬寫（10 檔 §8）。
                    .setStatusLabel(c.status().label())
                    .setCount(str(c.count())));
        }
        return b.build();
    }

    private SellerLogisticsReceivableRecord toSellerRecord(SellerReceivableView v) {
        LogisticsReceivableEntity r = v.receivable();
        LogisticsCollectionAttemptEntity a = v.latestAttempt();
        SellerLogisticsReceivableRecord.Builder b = SellerLogisticsReceivableRecord.newBuilder()
                .setReceivableNo(str(r.getId()))
                .setOrderNo(nullToEmpty(r.getOrderNo()))
                .setShipmentId(str(r.getShipmentId()))
                .setProvider(nullToEmpty(r.getProvider()))
                .setProviderDeliveryRef(nullToEmpty(r.getProviderDeliveryRef()))
                .setEstimatedProviderCost(str(r.getEstimatedCost()))
                .setSellerLogisticsCharge(str(r.getTargetNet()))
                .setPlatformSubsidy(str(r.getPlatformSubsidy()))
                .setResponsibilityAdjustment(str(v.responsibilityAdjustment()))
                .setReconciliationAdjustment(str(v.reconciliationAdjustment()))
                .setCollectionStatus(nullToEmpty(r.getStatus()))
                .setCollectionStatusLabel(statusLabel(r.getStatus()))
                .setSellerCredit(str(v.sellerCredit()))
                .setCarriedResidual(str(r.getCarriedResidual()))
                .setChargeBasisFrozenAt(iso(r.getChargeBasisFrozenAt()))
                .setInvoiceDocumentId(str(r.getInvoiceDocumentId()))
                .setStatementLineId(str(r.getStatementLineId()))
                .setCreatedAt(iso(r.getCreatedAt()))
                .setUpdatedAt(iso(r.getUpdatedAt()))
                .setAgingDays(str(queryService.agingDaysOf(r)));
        // 🔴 三個「NULL 不是 0」的維度：有值才設，沒值就讓 optional 維持未設。
        if (r.getActualCost() != null) {
            b.setActualProviderCost(str(r.getActualCost()));
        }
        if (a != null && a.getActualFee() != null) {
            b.setPayuniCollectionFee(str(a.getActualFee()));
        }
        if (a != null && a.getResidual() != null) {
            b.setCollectionResidual(str(a.getResidual()));
        }
        return b.build();
    }

    private static LogisticsLedgerTxnRecord toLedgerTxn(LedgerTxnView v) {
        LogisticsLedgerTxnRecord.Builder b = LogisticsLedgerTxnRecord.newBuilder()
                .setTxnNo(nullToEmpty(v.txn().getTxnNo()))
                .setTxnType(nullToEmpty(v.txn().getTxnType()))
                .setOrderNo(nullToEmpty(v.txn().getOrderNo()))
                .setSettlementPeriod(nullToEmpty(v.txn().getSettlementPeriod()))
                .setSourceType(nullToEmpty(v.txn().getSourceType()))
                .setSourceRef(nullToEmpty(v.txn().getSourceRef()))
                .setPostedAt(iso(v.txn().getPostedAt()));
        for (LedgerEntryEntity e : v.entries()) {
            b.addEntries(LogisticsLedgerEntryRecord.newBuilder()
                    .setLineSeq(e.getLineSeq() == null ? "" : String.valueOf(e.getLineSeq()))
                    .setAccountCode(nullToEmpty(e.getAccountCode()))
                    .setOwnerType(nullToEmpty(e.getOwnerType()))
                    .setOwnerRefId(str(e.getOwnerRefId()))
                    // 🔴 amount 恆為正，正負由 direction 表達——不得在這裡折成帶正負的數字。
                    .setDirection(nullToEmpty(e.getDirection()))
                    .setAmount(str(e.getAmount()))
                    .setMemo(nullToEmpty(e.getMemo())));
        }
        return b.build();
    }

    private AdminLogisticsReceivableSummary toAdminReceivable(LogisticsReceivableEntity r) {
        AdminLogisticsReceivableSummary.Builder b = AdminLogisticsReceivableSummary.newBuilder()
                .setReceivableNo(str(r.getId()))
                .setStoreId(str(r.getStoreId()))
                .setOrderNo(nullToEmpty(r.getOrderNo()))
                .setShipmentId(str(r.getShipmentId()))
                .setProvider(nullToEmpty(r.getProvider()))
                .setProviderDeliveryRef(nullToEmpty(r.getProviderDeliveryRef()))
                .setEstimatedCost(str(r.getEstimatedCost()))
                .setPlatformSubsidy(str(r.getPlatformSubsidy()))
                .setTargetNet(str(r.getTargetNet()))
                .setCarriedResidual(str(r.getCarriedResidual()))
                .setStatus(nullToEmpty(r.getStatus()))
                .setStatusLabel(statusLabel(r.getStatus()))
                .setStatementLineId(str(r.getStatementLineId()))
                .setInvoiceDocumentId(str(r.getInvoiceDocumentId()))
                .setChargeBasisFrozenAt(iso(r.getChargeBasisFrozenAt()))
                .setCreatedAt(iso(r.getCreatedAt()))
                .setUpdatedAt(iso(r.getUpdatedAt()))
                .setAgingDays(str(queryService.agingDaysOf(r)))
                .setSubmitLeaseUntil(iso(r.getSubmitLeaseUntil()));
        if (r.getActualCost() != null) {
            b.setActualCost(str(r.getActualCost()));
        }
        return b.build();
    }

    private AdminSellerLogisticsAccount toAccount(Long storeId, SellerLogisticsAccountEntity a) {
        if (a == null) {
            // 🔴 沒有帳戶列 ⇒ 兩個餘額回 "0" 但 exists=false：那是「還沒有帳戶」不是「結清了」。
            return AdminSellerLogisticsAccount.newBuilder()
                    .setStoreId(str(storeId))
                    .setReceivableBalance("0")
                    .setCreditBalance("0")
                    .setAgingDays("0")
                    .setExists(false)
                    .build();
        }
        return AdminSellerLogisticsAccount.newBuilder()
                .setStoreId(str(a.getStoreId()))
                .setReceivableBalance(str(a.getReceivableBalance()))
                .setCreditBalance(str(a.getCreditBalance()))
                .setLastActivityAt(iso(a.getLastActivityAt()))
                .setAgingDays(str(queryService.agingDaysOfAccount(a)))
                .setExists(true)
                .build();
    }

    private static AdminLogisticsCollectionRecord toAdminCollection(LogisticsCollectionAttemptEntity a,
                                                                    Long storeId) {
        AdminLogisticsCollectionRecord.Builder b = AdminLogisticsCollectionRecord.newBuilder()
                .setCollectionNo(str(a.getId()))
                .setReceivableNo(str(a.getReceivableId()))
                .setStoreId(str(storeId))
                .setTradeNo(nullToEmpty(a.getTradeNo()))
                .setGrossAmount(str(a.getGrossAmount()))
                .setSubmitStatus(nullToEmpty(a.getSubmitStatus()))
                .setSubmitMessage(nullToEmpty(a.getSubmitMessage()))
                .setSubmittedAt(iso(a.getSubmittedAt()))
                .setExpectedFundDate(date(a.getExpectedFundDate()))
                .setQueryLeaseUntil(iso(a.getQueryLeaseUntil()))
                .setCreatedAt(iso(a.getCreatedAt()))
                .setUpdatedAt(iso(a.getUpdatedAt()));
        // 🔴 四個「NULL 不是 0／不是空」的欄位：未查詢時一律維持未設。
        if (a.getNetProceeds() != null) {
            b.setNetProceeds(str(a.getNetProceeds()));
        }
        if (a.getResidual() != null) {
            b.setResidual(str(a.getResidual()));
        }
        if (a.getActualFee() != null) {
            b.setActualFee(str(a.getActualFee()));
        }
        if (a.getFundDate() != null) {
            b.setFundDate(date(a.getFundDate()));
        }
        return b.build();
    }

    private static AdminLogisticsAdjustmentRecord toAdminAdjustment(LogisticsAdjustmentEntity adj) {
        return AdminLogisticsAdjustmentRecord.newBuilder()
                .setAdjustmentNo(str(adj.getId()))
                .setReceivableNo(str(adj.getReceivableId()))
                .setStoreId(str(adj.getStoreId()))
                .setProviderDeliveryRef(nullToEmpty(adj.getProviderDeliveryRef()))
                .setKind(nullToEmpty(adj.getKind()))
                // ⚠️ amount 允許負數（業務調整單的正負就是它的語意）。
                .setAmount(str(adj.getAmount()))
                .setResponsibilityCause(nullToEmpty(adj.getResponsibilityCause()))
                .setStatementLineId(str(adj.getStatementLineId()))
                .setCreatedBy(str(adj.getCreatedBy()))
                .setReason(nullToEmpty(adj.getReason()))
                .setCreatedAt(iso(adj.getCreatedAt()))
                .build();
    }

    private static AdminProviderStatementSummary toStatement(StatementAggregate s) {
        return AdminProviderStatementSummary.newBuilder()
                .setProvider(nullToEmpty(s.provider()))
                .setStatementRef(nullToEmpty(s.statementRef()))
                .setStatementPeriod(nullToEmpty(s.statementPeriod()))
                .setLineCount(str(s.lineCount()))
                .setTotalAmount(str(s.totalAmount()))
                .setMatchedCount(str(s.matchedCount()))
                .setUnmatchedCount(str(s.unmatchedCount()))
                .setDisputedCount(str(s.disputedCount()))
                .setWrittenOffCount(str(s.writtenOffCount()))
                // 推導值，不是 recon_status 的碼值（V9 沒有 MISMATCH）。
                .setMismatchCount(str(s.mismatchCount()))
                .setMismatchAmount(str(s.mismatchAmount()))
                .setFirstImportedAt(iso(s.firstImportedAt()))
                .setLastImportedAt(iso(s.lastImportedAt()))
                .build();
    }

    private static AdminProviderSettlementLineRecord toStatementLine(StatementLineView v) {
        ProviderSettlementLineEntity l = v.line();
        return AdminProviderSettlementLineRecord.newBuilder()
                .setLineId(str(l.getId()))
                .setProvider(nullToEmpty(l.getProvider()))
                .setStatementRef(nullToEmpty(l.getStatementRef()))
                .setStatementPeriod(nullToEmpty(l.getStatementPeriod()))
                .setProviderShipmentRef(nullToEmpty(l.getProviderShipmentRef()))
                .setOrderNo(nullToEmpty(l.getOrderNo()))
                .setFeeType(nullToEmpty(l.getFeeType()))
                .setAmount(str(l.getAmount()))
                .setReconStatus(nullToEmpty(l.getReconStatus()))
                .setLedgerTxnNo(nullToEmpty(l.getLedgerTxnNo()))
                .setExceptionCaseId(str(l.getExceptionCaseId()))
                .setImportedBy(str(l.getImportedBy()))
                .setImportedAt(iso(l.getImportedAt()))
                .setHasReconciliationAdjustment(v.reconciliationAdjustmentCount() > 0)
                .setReconciliationAdjustmentAmount(str(v.reconciliationAdjustmentAmount()))
                .build();
    }

    private AdminFulfillmentExceptionCaseRecord toAdminCase(AdminCaseView v) {
        var c = v.exceptionCase();
        AdminFulfillmentExceptionCaseRecord.Builder b = AdminFulfillmentExceptionCaseRecord.newBuilder()
                .setCaseNo(nullToEmpty(c.getCaseNo()))
                .setCaseId(str(c.getId()))
                .setSourceService(nullToEmpty(c.getSourceService()))
                .setCategory(nullToEmpty(c.getCategory()))
                .setSeverity(nullToEmpty(c.getSeverity()))
                .setSubjectType(nullToEmpty(c.getSubjectType()))
                .setSubjectRef(nullToEmpty(c.getSubjectRef()))
                .setStoreId(str(c.getStoreId()))
                .setStatus(nullToEmpty(c.getStatus()))
                .setStatusEnteredAt(iso(c.getStatusEnteredAt()))
                .setOwnerAccountId(str(c.getOwnerAccountId()))
                .setOwnerAssignedAt(iso(c.getOwnerAssignedAt()))
                .setResolutionCode(nullToEmpty(c.getResolutionCode()))
                .setResolutionNote(nullToEmpty(c.getResolutionNote()))
                .setResolvedByAccountId(str(c.getResolvedByAccountId()))
                .setResolvedAt(iso(c.getResolvedAt()))
                // 🔴 來源端發生時間，與 created_at 是兩個時間，不得互相冒充。
                .setOccurredAt(iso(c.getOccurredAt()))
                .setCreatedAt(iso(c.getCreatedAt()))
                // 🔴 責任判定軸的 immutable event context（V14）：建案當下凍結。
                //    N17 與 N16 都從這一列取值 ⇒ 兩筆事件不可能帶不同的 order_no，
                //    而 order_no 正是 order-service 35→54 CAS 的錨（36 §3.3.1）。
                //    ⚠️ generic case 這三欄本來就是 null ⇒ 一律吐空字串，不是缺陷。
                .setOrderNo(nullToEmpty(c.getOrderNo()))
                .setShipmentId(str(c.getShipmentId()))
                .setShippingMethod(nullToEmpty(c.getShippingMethod()));
        for (FulfillmentCaseResolutionEntity r : v.resolutions()) {
            b.addResolutions(AdminFulfillmentCaseResolutionRecord.newBuilder()
                    .setResolutionId(str(r.getId()))
                    .setResolutionType(nullToEmpty(r.getResolutionType()))
                    .setReason(nullToEmpty(r.getReason()))
                    .setStatus(nullToEmpty(r.getStatus()))
                    .setRequiresReview(Boolean.TRUE.equals(r.getRequiresReview()))
                    .setProposedByAccountId(str(r.getProposedByAccountId()))
                    .setProposedAt(iso(r.getProposedAt()))
                    .setReviewedByAccountId(str(r.getReviewedByAccountId()))
                    .setReviewComment(nullToEmpty(r.getReviewComment()))
                    .setReviewedAt(iso(r.getReviewedAt()))
                    // 🔴 maker 主張的成因（V13）：approve 時原封不動成為 N16 的
                    //    responsibility_cause，覆核者改不了它。非責任型別為 null ⇒ 空字串。
                    .setProposedResponsibilityCause(nullToEmpty(r.getProposedResponsibilityCause()))
                    // 影響預覽快照。🔴 只有退款金額一個維度——platformFeeDelta／
                    //    sellerNetDelta 在現行商流下語意不成立，填 0 會是假結論。
                    .setImpactPreviewRefundAmount(previewRefundAmount(r.getImpactPreview()))
                    .setImpactComputedAt(iso(r.getImpactComputedAt())));
        }
        return b.build();
    }

    // =========================================================================
    // 授權與轉型
    // =========================================================================

    /**
     * 賣家面的身分守門。
     *
     * <p>🔴 {@code storeId <= 0} 是<b>契約違反</b>而不是越權：它代表 Gateway 沒有把解析後的
     * store_id 帶進來。刻意 fail loud 而不是回空結果——空結果會讓接線錯誤在 prod 才被發現
     * （比照 {@code PaymentGrpcService.getSellerPaymentHistory} 的 missingStoreId 分支）。
     */
    private static long requireSellerScope(long storeId, String method) {
        GrpcAuthContext.requireAccountType(AccountType.SELLER);
        if (storeId <= 0L) {
            log.warn("LogisticsFinanceGrpcService." + method + ".missingStoreId",
                    "賣家視角呼叫但未帶 Gateway 解析出的 store_id（Gateway 契約違反）",
                    "storeId", String.valueOf(storeId));
            throw Status.UNAUTHENTICATED.withDescription("請先登入").asRuntimeException();
        }
        return storeId;
    }

    /**
     * 逗號分隔的應收狀態 → CodeMeta enum。
     *
     * <p>🔴 認不出來的值一律 {@code INVALID_ARGUMENT}，<b>不是</b>靜默忽略：忽略之後
     * 「打錯字」與「這個狀態沒有資料」回一樣的空清單，而 ADM 會以為真的沒有。
     */
    private static List<LogisticsReceivableStatus> parseStatuses(String csv) {
        List<String> raw = splitCsv(csv);
        if (raw.isEmpty()) {
            return List.of();
        }
        List<LogisticsReceivableStatus> out = new ArrayList<>(raw.size());
        for (String s : raw) {
            // ⚠️ 先 isValidValue：fromValue 對未知值丟 IllegalArgumentException，
            //    那會被外層 catch 成 500，而這是呼叫端打錯字 ⇒ 應該是 400。
            if (!LogisticsReceivableStatus.isValidValue(s)) {
                throw Status.INVALID_ARGUMENT
                        .withDescription("無效的 status: " + s)
                        .asRuntimeException();
            }
            out.add(LogisticsReceivableStatus.fromValue(s));
        }
        return out;
    }

    private static List<String> splitCsv(String csv) {
        if (csv == null || csv.isBlank()) {
            return List.of();
        }
        return Arrays.stream(csv.split(","))
                .map(String::trim)
                .filter(s -> !s.isEmpty())
                .toList();
    }

    private static String statusLabel(String value) {
        if (value == null) {
            return "";
        }
        // ⚠️ 認不出來就回空字串而不是丟例外：label 是顯示用的，不該讓整支查詢 500。
        return LogisticsReceivableStatus.isValidValue(value)
                ? LogisticsReceivableStatus.fromValue(value).label() : "";
    }

    private static LocalDateTime parseTime(String iso, String field) {
        if (iso == null || iso.isBlank()) {
            return null;
        }
        LocalDateTime t = DateTimeUtils.parseIso8601(iso);
        if (t == null) {
            throw Status.INVALID_ARGUMENT
                    .withDescription(field + " 必須是 ISO 8601 UTC（YYYY-MM-DDTHH:mm:ssZ）")
                    .asRuntimeException();
        }
        return t;
    }

    private static int parseInt(String raw, int fallback, String field) {
        if (raw == null || raw.isBlank()) {
            return fallback;
        }
        try {
            return Integer.parseInt(raw.trim());
        } catch (NumberFormatException e) {
            throw Status.INVALID_ARGUMENT.withDescription(field + " 必須是整數").asRuntimeException();
        }
    }

    private static Long parseOptionalLong(String raw, String field) {
        if (raw == null || raw.isBlank()) {
            return null;
        }
        try {
            return Long.parseLong(raw.trim());
        } catch (NumberFormatException e) {
            throw Status.INVALID_ARGUMENT.withDescription(field + " 必須是整數").asRuntimeException();
        }
    }

    private static long parseLongOrZero(String raw, String field) {
        Long v = parseOptionalLong(raw, field);
        return v == null ? 0L : v;
    }

    private static long parseRequiredLong(String raw, String field) {
        Long v = parseOptionalLong(raw, field);
        if (v == null) {
            throw Status.INVALID_ARGUMENT.withDescription(field + " 為必填").asRuntimeException();
        }
        return v;
    }

    private static String blankToNull(String s) {
        return s == null || s.isBlank() ? null : s;
    }

    private static String nullToEmpty(String s) {
        return s == null ? "" : s;
    }

    /** 數值 → 數字字串；null 一律空字串（10 檔 §4：對外數值全部 string）。 */
    private static String str(Long v) {
        return v == null ? "" : String.valueOf(v);
    }

    private static String str(long v) {
        return String.valueOf(v);
    }

    private static String str(int v) {
        return String.valueOf(v);
    }

    private static String iso(LocalDateTime t) {
        return t == null ? "" : DateTimeUtils.formatIso8601(t);
    }

    private static String date(LocalDate d) {
        return d == null ? "" : d.toString();
    }

    private static int mapStatus(StatusRuntimeException e) {
        return switch (e.getStatus().getCode()) {
            case UNAUTHENTICATED -> GrpcResponseCode.UNAUTHORIZED.code();
            case PERMISSION_DENIED -> GrpcResponseCode.PERMISSION_DENIED.code();
            case NOT_FOUND -> GrpcResponseCode.NOT_FOUND.code();
            case INVALID_ARGUMENT -> GrpcResponseCode.INVALID_PARAM.code();
            default -> GrpcResponseCode.INTERNAL_ERROR.code();
        };
    }

    private static String describe(StatusRuntimeException e) {
        String d = e.getStatus().getDescription();
        return d == null ? GrpcResponseCode.INTERNAL_ERROR.message() : d;
    }
}
