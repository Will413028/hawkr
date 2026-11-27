// Command hawkr 是後端單體唯一的 binary，以子命令決定執行的角色：
//
//	serve        對外 HTTP API（入口：還沒搬的路徑轉給 gateway）
//	worker       執行非同步 job（River）
//	migrate      套用資料庫 migration（部署時在 serve、worker 之前執行）
//	dev-token    本機開發用：顧客在資料庫建 dev session、後台以本機的金鑰簽 JWT，不連外部 provider（見 identity.go）
//	delegation-eligibility 開通或撤回管理員的代行資格
//	admin-role   授予或撤銷管理員的角色（必填理由，寫入 identity.admin_role_events）
//	social-recount 對全部可重算的社群計數跑重算與覆寫，印出漂移（手動全量重算）
//
// 這裡是組裝點：設定、資料庫、各模組與它們之間的接線都在這裡完成，其他地方不自己組。
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	"uuid"

	"github.com/riverqueue/river"

	"github.com/Will413028/hawkr/backend/internal/httpapi"
	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/legacy/accountservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/bizcalendarrelay"
	"github.com/Will413028/hawkr/backend/internal/legacy/buyerconsumer"
	"github.com/Will413028/hawkr/backend/internal/legacy/buyerrelay"
	"github.com/Will413028/hawkr/backend/internal/legacy/buyerserver"
	"github.com/Will413028/hawkr/backend/internal/legacy/crmconsumer"
	"github.com/Will413028/hawkr/backend/internal/legacy/delegationservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/gatewayproxy"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/account/accountconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/buyer/buyerconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/payment/paymentconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/seller/sellerconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/invoiceservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/paymentserver"
	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/legacy/reviewrelay"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerrelay"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerserver"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/assistant"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/imageai"
	"github.com/Will413028/hawkr/backend/internal/modules/media"
	"github.com/Will413028/hawkr/backend/internal/modules/moderation"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/platform/adminaudit"
	"github.com/Will413028/hawkr/backend/internal/platform/bizcalendar"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/llm"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

const shutdownTimeout = 30 * time.Second

const usage = "可用的子命令：serve、worker、migrate、dev-token、dev-seed、reannounce-exposures、delegation-eligibility、admin-role、admin-audit、social-recount"

func main() {
	if code := runMain(); code != 0 {
		os.Exit(code)
	}
}

func runMain() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "hawkr:", err)
		return 1
	}
	return 0
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return errors.New("缺少子命令（" + usage + "）")
	}
	switch args[0] {
	case "serve":
		return runServe(ctx)
	case "worker":
		return runWorker(ctx)
	case "migrate":
		return runMigrate(ctx)
	case "dev-token":
		return runDevToken(ctx, args[1:], os.Stdout, os.Stderr)
	case "dev-seed":
		return runDevSeed(ctx, args[1:], os.Stdout)
	case "reannounce-exposures":
		return runReannounceExposures(ctx, args[1:], os.Stdout)
	case "delegation-eligibility":
		return runDelegationEligibility(ctx, args[1:])
	case "admin-role":
		return runAdminRole(ctx, args[1:], os.Stdout)
	case "admin-audit":
		return runAdminAudit(ctx, args[1:], os.Stdout)
	case "social-recount":
		return runSocialRecount(ctx, os.Stdout)
	default:
		return fmt.Errorf("未知的子命令 %q（%s）", args[0], usage)
	}
}

func dbConfig(e *config.Env) db.Config {
	return db.Config{
		URL:                e.String("DATABASE_URL"),
		MaxConns:           e.Int32("DB_MAX_CONNS", 10),
		ConnectRetryBudget: e.Duration("DB_CONNECT_RETRY_BUDGET", 60*time.Second),
	}
}

func runServe(ctx context.Context) error {
	e := config.NewEnv()
	addr := e.StringOr("HTTP_ADDR", ":8080")
	dbc := dbConfig(e)
	internalSecret := e.StringOr("INTERNAL_SERVICE_HMAC_SECRET", "")
	mediaOrigin := e.String("MEDIA_ORIGIN")
	webOrigin := e.String("PUBLIC_WEB_ORIGIN")
	gatewayURL := e.String("GATEWAY_URL")
	legacyAddr := e.String("LEGACY_GRPC_ADDR")
	orderAddr := e.String("ORDER_SERVICE_ADDR")
	origins := e.List("CORS_ALLOWED_ORIGINS")
	originAuth := httpapi.OriginAuth{
		Main: []byte(e.StringOr("ORIGIN_AUTH_SECRET", "")),
		Prev: []byte(e.StringOr("ORIGIN_AUTH_SECRET_PREV", "")),
		CI:   []byte(e.StringOr("ORIGIN_AUTH_CI_SECRET", "")),
	}
	clientIP := httpapi.ClientIP{Header: e.StringOr("CLIENT_IP_HEADER", "")}
	log := newLogger()
	verifierCtx, stopVerifiers := context.WithCancel(context.WithoutCancel(ctx))
	defer stopVerifiers()
	idc := readIdentityConfig(verifierCtx, e)
	kycMAC := readKYCHashKeys(e)
	paymentProvider := readPayment(e, log)
	chargeProvider := readCharge(e, log)
	paymentPublishableKey := e.StringOr("STRIPE_PUBLISHABLE_KEY", "")
	notificationConfig := readNotificationProviders(ctx, e, log)
	registry := readGCIS(e)
	mediaCfg := readMedia(e)
	llmConfig := llm.ReadConfig(e)
	imageAIConfig := imageai.ReadConfig(e)
	if err := e.Err(); err != nil {
		return err
	}
	images, err := rpc.NewImages(mediaOrigin)
	if err != nil {
		return err
	}
	web, err := rpc.NewWebOrigin(webOrigin)
	if err != nil {
		return err
	}
	d, err := db.Open(ctx, dbc)
	if err != nil {
		return err
	}
	defer d.Close()
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		return err
	}
	med, err := newMedia(mediaCfg, d, jc, log)
	if err != nil {
		return err
	}
	orders, err := orderservice.New(orderAddr, internalSecret)
	if err != nil {
		return err
	}
	bus := events.NewBus()
	calendar := bizcalendar.New(d, adminaudit.New(d, time.Now), bus)
	subscribeBuyerEvents(bus, jc)
	id := newIdentity(d, bus, idc, log)
	webLogin, err := newWebLogin(verifierCtx, id, web, idc, log)
	if err != nil {
		return err
	}
	appLogin, err := newAppLogin(verifierCtx, id, idc)
	if err != nil {
		return err
	}
	buyerModule := buyer.New(buyer.Deps{
		DB: d, Events: bus,
		Media: med, Now: time.Now, Logger: log,
	})
	subscribeBuyerLifecycle(bus, buyerModule)
	catalogModule := newCatalog(d, bus, med, buyerModule)
	subscribeCatalogFavorites(bus, catalogModule)
	box := newInbox(d, jc, log)
	payments, owners := newPayment(d, bus, jc, box, paymentProvider, chargeProvider, paymentOrders{client: orders}, log)
	sellerModule := newSeller(d, bus, jc, id, med, orders, sellerProducts{catalog: catalogModule}, kycMAC, payments, registry, log)
	owners.bind(sellerModule)
	subscribeSellerPayout(bus, sellerModule)
	if err := sellerModule.CheckKYCKeys(ctx); err != nil {
		return err
	}
	if n, err := sellerModule.RehashKYCIdentities(ctx); err != nil {
		return err
	} else if n > 0 {
		log.Info("KYC 身分比對 hash 已補算金鑰輪替後缺的列", "applications", n)
	}
	subscribeSellerViews(bus, sellerModule)
	subscribeSellerRelay(bus, jc)
	subscribePaymentRelay(bus, jc)
	subscribeBizCalendarRelay(bus, jc)
	delegationDeps := delegation.Deps{DB: d, Identity: id, Seller: sellerModule, NewID: uuid.NewV7, Now: time.Now, Logger: log}
	socialModule := newSocial(d, bus, jc, buyerModule, sellerModule, med, socialProducts{catalog: catalogModule}, delegation.NewScheduleAuthorizer(delegationDeps), log)
	reviewModule := newReview(d, bus, jc, buyerModule, sellerModule, catalogModule, med, orders, log)
	imModule := newIM(d, bus, jc, buyerModule, sellerModule, med, orders, log)
	crmModule := newCRM(d, bus, jc, buyerModule, sellerModule, log)
	delegationDeps.Social = socialModule
	suspension := accountsuspension.New(accountsuspension.Deps{DB: d, Events: bus, Identity: id, Seller: sellerModule, Logger: log})
	moderationModule := newModeration(d, bus, socialModule, reviewModule, imModule, sellerModule, buyerModule, suspension, med, log)
	subscribeProcurement(bus, d, log)
	imageAIModule := newImageAI(d, imageAIConfig, sellerModule, llm.NewImage(llmConfig, log).Generator(), med, log)
	subscribeImageAI(bus, imageAIModule)
	procurementModule := newProcurement(d, jc, buyerModule, sellerModule, bus, log)
	stopProcurementHub := startProcurementHub(ctx, procurementModule, log)
	defer stopProcurementHub()
	subscribeModeration(bus, moderationModule.Retention())
	couponModule := newCoupon(d, sellerModule, buyerModule)
	notificationModule := newNotification(d, id, buyerModule, sellerModule, couponModule, moderationModule, jc, box, notificationConfig, images, log)
	subscribeNotification(bus, notificationModule)
	discoveryModule := newDiscovery(d, orders, catalogModule, reviewModule, sellerModule, log)
	gateway, err := gatewayproxy.New(gatewayURL, func(ctx context.Context) (gatewayproxy.Caller, bool) {
		p, ok := rpc.PrincipalFrom(ctx)
		return gatewayproxy.Caller{Principal: p}, ok
	}, buyerModule, sellerModule, nil, log)
	if err != nil {
		return err
	}
	stats, err := httpseller.NewStatsSource(buyerModule, sellerRatings{reviewModule}, log)
	if err != nil {
		return err
	}
	deletion := accountdeletion.New(accountdeletion.Deps{DB: d, Buyer: buyerModule, Seller: sellerModule, Identity: id, Orders: orders, Now: time.Now, NewID: uuid.NewV7, Logger: log})
	del := newDelegation(bus, delegationDeps)
	assistantModule := assistant.New(assistant.Deps{DB: d, LLM: llm.New(llm.Assistant, llmConfig, log).LLM(), Orders: assistantOrders{orders}, Now: time.Now})
	stopAssistant := context.AfterFunc(ctx, func() {
		sc, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		_ = assistantModule.Shutdown(sc)
	})
	defer stopAssistant()

	h, err := httpapi.NewHandler(httpapi.Deps{
		Assistant:    assistantModule,
		Procurement:  procurementModule,
		ImageAI:      imageAIModule,
		Ready:        d.Ping,
		Social:       socialModule,
		Review:       reviewModule,
		IM:           imModule,
		CRM:          crmModule,
		Coupon:       couponModule,
		Moderation:   moderationModule,
		Notification: notificationModule,
		Content: content.New(content.Deps{
			DB:       d,
			Social:   socialModule,
			Products: catalogModule,
			Media:    med,
			NewID:    uuid.NewV7,
			Now:      time.Now,
			Logger:   log,
		}),
		Media:                 med,
		Identity:              id,
		WebLogin:              webLogin,
		AppLogin:              appLogin,
		Buyer:                 buyerModule,
		Seller:                sellerModule,
		Payment:               payments,
		PaymentPublishableKey: paymentPublishableKey,
		BizCalendar:           calendar,
		Discovery:             discoveryModule,
		ReviewQueue:           reviewqueue.New(reviewqueue.Deps{DB: d}),
		Stats:                 stats,
		DashboardOrders:       sellerDashboardOrders{orders},
		SocialProducts:        socialCards{catalogModule},
		BuyerPorts:            httpbuyer.Ports{Products: buyerProducts{catalogModule}, Skus: buyerSkus{catalogModule}, Stores: sellerModule, Orders: buyerOrders{orders}, CRM: crmModule},
		Deletion:              deletion,
		Delegation:            del,
		Images:                images,
		WebOrigin:             web,
		Origins:               origins,
		OriginAuth:            originAuth,
		ClientIP:              clientIP,
		Legacy:                gateway,
		LegacyPrefixes:        gatewayproxy.Prefixes,
		Now:                   time.Now,
		Logger:                log,
	})
	if err != nil {
		return err
	}
	ln, err := new(net.ListenConfig).Listen(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	legacyLn, err := new(net.ListenConfig).Listen(ctx, "tcp", legacyAddr)
	if err != nil {
		return err
	}
	legacyMux := http.NewServeMux()
	legacyMux.Handle(accountconnect.NewAccountServiceHandler(accountservice.New(id, log)))
	legacyMux.Handle(buyerconnect.NewBuyerServiceHandler(buyerserver.New(buyerModule)))
	legacyMux.Handle(paymentconnect.NewPaymentServiceHandler(paymentserver.New(payments)))
	legacyMux.Handle(sellerconnect.NewSellerServiceHandler(sellerserver.New(sellerModule, legacySellerRatings{reviewModule}, buyerModule, log)))
	legacyMux.Handle(accountconnect.NewDelegationServiceHandler(delegationservice.New(del, log)))
	serveErr := serveWithRun(ctx, joinRuns(imModule.Run, notificationModule.Run), listening{ln, newServer(h)}, listening{legacyLn, newServer(legacyMux)})
	sc, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	return errors.Join(serveErr, assistantModule.Shutdown(sc))
}

const workerConcurrency = 10

func runWorker(ctx context.Context) error {
	e := config.NewEnv()
	dbc := dbConfig(e)
	rv := revalidate.Config{
		BaseURL: e.StringOr("WEB_REVALIDATE_BASE_URL", ""),
		Secret:  e.StringOr("WEB_REVALIDATE_SECRET", ""),
	}
	rabbitURL := e.String("RABBITMQ_URL")
	orderAddr := e.String("ORDER_SERVICE_ADDR")
	invoiceAddr := e.String("INVOICE_SERVICE_ADDR")
	internalSecret := e.StringOr("INTERNAL_SERVICE_HMAC_SECRET", "")
	log := newLogger()
	verifierCtx, stopVerifiers := context.WithCancel(context.WithoutCancel(ctx))
	defer stopVerifiers()
	idc := readIdentityConfig(verifierCtx, e)
	kycMAC := readKYCHashKeys(e)
	paymentProvider := readPayment(e, log)
	chargeProvider := readCharge(e, log)
	notificationConfig := readNotificationProviders(ctx, e, log)
	mediaOrigin := e.String("MEDIA_ORIGIN")
	registry := readGCIS(e)
	mediaCfg := readMedia(e)
	llmConfig := llm.ReadConfig(e)
	imageAIConfig := imageai.ReadConfig(e)
	if err := e.Err(); err != nil {
		return err
	}
	images, err := rpc.NewImages(mediaOrigin)
	if err != nil {
		return err
	}
	d, err := db.Open(ctx, dbc)
	if err != nil {
		return err
	}
	defer d.Close()
	pub, err := rabbitmq.New(rabbitURL)
	if err != nil {
		return err
	}
	defer func() {
		if err := pub.Close(); err != nil {
			log.ErrorContext(ctx, "rabbitmq：關閉 publisher 失敗", "error", err)
		}
	}()
	orders, err := orderservice.New(orderAddr, internalSecret)
	if err != nil {
		return err
	}
	invoices, err := invoiceservice.New(invoiceAddr, internalSecret)
	if err != nil {
		return err
	}

	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		return err
	}
	bus := events.NewBus()
	subscribeBuyerEvents(bus, jc)
	calendar := bizcalendar.New(d, adminaudit.New(d, time.Now), bus)
	id := newIdentity(d, bus, idc, log)
	med, err := newMedia(mediaCfg, d, jc, log)
	if err != nil {
		return err
	}
	buyerModule := buyer.New(buyer.Deps{
		DB: d, Events: bus,
		Media: med, Now: time.Now, Logger: log,
	})
	subscribeBuyerLifecycle(bus, buyerModule)
	catalogModule := newCatalog(d, bus, med, buyerModule)
	subscribeCatalogFavorites(bus, catalogModule)
	box := newInbox(d, jc, log)
	payments, owners := newPayment(d, bus, jc, box, paymentProvider, chargeProvider, paymentOrders{client: orders}, log)
	sellerModule := newSeller(d, bus, jc, id, med, orders, sellerProducts{catalog: catalogModule}, kycMAC, payments, registry, log)
	owners.bind(sellerModule)
	subscribeSellerPayout(bus, sellerModule)
	if err := sellerModule.CheckKYCKeys(ctx); err != nil {
		return err
	}
	if n, err := sellerModule.RehashKYCIdentities(ctx); err != nil {
		return err
	} else if n > 0 {
		log.Info("KYC 身分比對 hash 已補算金鑰輪替後缺的列", "applications", n)
	}
	subscribeSellerViews(bus, sellerModule)
	subscribeSellerRelay(bus, jc)
	subscribePaymentRelay(bus, jc)
	delegationDeps := delegation.Deps{DB: d, Identity: id, Seller: sellerModule, NewID: uuid.NewV7, Now: time.Now, Logger: log}
	socialModule := newSocial(d, bus, jc, buyerModule, sellerModule, med, socialProducts{catalog: catalogModule}, delegation.NewScheduleAuthorizer(delegationDeps), log)
	reviewModule := newReview(d, bus, jc, buyerModule, sellerModule, catalogModule, med, orders, log)
	imModule := newIM(d, bus, jc, buyerModule, sellerModule, med, orders, log)
	crmModule := newCRM(d, bus, jc, buyerModule, sellerModule, log)
	delegationDeps.Social = socialModule
	newDelegation(bus, delegationDeps)
	discoveryModule := newDiscovery(d, orders, catalogModule, reviewModule, sellerModule, log)
	deletion := accountdeletion.New(accountdeletion.Deps{DB: d, Buyer: buyerModule, Seller: sellerModule, Identity: id, Orders: orders, Now: time.Now, NewID: uuid.NewV7, Logger: log})
	consumer := buyerconsumer.Consumer{
		Buyer: buyerModule,
		Complete: func(ctx context.Context, accountID int64) error {
			_, err := deletion.CompletePending(ctx, accountID)
			return err
		},
		StoreAccount: func(ctx context.Context, storeID int64) (int64, error) {
			s, err := sellerModule.Store(ctx, storeID)
			return s.AccountID, err
		},
		AdvanceStore: sellerModule.AdvanceTerminations,
		Jobs:         jc,
	}
	moderationModule := newModerationRetention(d, buyerModule, med, log)
	subscribeProcurement(bus, d, log)
	imageAIModule := newImageAI(d, imageAIConfig, sellerModule, llm.DisabledImage().Generator(), med, log)
	subscribeImageAI(bus, imageAIModule)
	subscribeModeration(bus, moderationModule)
	notificationModule := newNotification(d, id, buyerModule, sellerModule, newCoupon(d, sellerModule, buyerModule), moderation.NewCaseReader(d), jc, box, notificationConfig, images, log)
	subscribeNotification(bus, notificationModule)
	workers := river.NewWorkers()
	river.AddWorker(workers, revalidate.NewWorker(rv, log))
	med.AddWorkers(workers)
	procurementModule := newProcurementWorker(d, jc, buyerModule, bus, llm.New(llm.Procurement, llmConfig, log), catalogModule, sellerModule, procurementOrders{orders}, log)
	addProcurementWorkers(workers, procurementModule)
	buyerModule.AddWorkers(workers)
	sellerModule.AddWorkers(workers)
	payments.AddWorkers(workers)
	box.AddWorkers(workers)
	socialModule.AddWorkers(workers)
	reviewModule.AddWorkers(workers)
	imModule.AddWorkers(workers)
	crmModule.AddWorkers(workers)
	moderationModule.AddWorkers(workers)
	notificationModule.AddWorkers(workers)
	discoveryModule.AddWorkers(workers)
	deletion.AddWorkers(workers)
	consumer.AddWorkers(workers)
	crmConsumer := crmconsumer.Consumer{CRM: crmModule, Orders: orders, Jobs: jc}
	crmConsumer.AddWorkers(workers)
	publisher := rabbitmq.NewWorker(pub)
	river.AddWorker(workers, publisher)
	buyerrelay.NewWorker(jc).AddWorkers(workers)
	sellerrelay.NewWorker(sellerModule, jc, log).AddWorkers(workers)
	reviewrelay.NewWorker(reviewModule, jc, log).AddWorkers(workers)
	bizcalendarrelay.NewWorker(calendar, invoices, time.Now, log).AddWorkers(workers)
	periodic := append(id.PeriodicJobs(), buyerModule.PeriodicJobs()...)
	periodic = append(periodic, deletion.PeriodicJobs()...)
	periodic = append(periodic, sellerModule.PeriodicJobs()...)
	periodic = append(periodic, payments.PeriodicJobs()...)
	periodic = append(periodic, box.PeriodicJobs()...)
	periodic = append(periodic, socialModule.PeriodicJobs()...)
	periodic = append(periodic, reviewModule.PeriodicJobs()...)
	periodic = append(periodic, imModule.PeriodicJobs()...)
	periodic = append(periodic, crmModule.PeriodicJobs()...)
	periodic = append(periodic, moderationModule.PeriodicJobs()...)
	periodic = append(periodic, notificationModule.PeriodicJobs()...)
	periodic = append(periodic, discoveryModule.PeriodicJobs()...)
	periodic = append(periodic, med.PeriodicJobs()...)
	periodic = append(periodic, procurementModule.PeriodicJobs()...)
	periodic = append(periodic, procurementModule.OrderPeriodicJobs()...)
	periodic = append(periodic, bizcalendarrelay.PeriodicJobs()...)
	queues, err := workerQueues(workerConcurrency, media.Queues())
	if err != nil {
		return err
	}
	c, err := jobs.NewWorker(d, workers, queues, periodic)
	if err != nil {
		return err
	}
	consumerCtx, stopConsumer := context.WithCancel(ctx)
	defer stopConsumer()
	consumerDone := make(chan struct{})
	crmDone := make(chan struct{})
	go func() {
		defer close(consumerDone)
		consumer.Run(consumerCtx, rabbitURL, log)
	}()
	go func() {
		defer close(crmDone)
		crmConsumer.Run(consumerCtx, rabbitURL, log)
	}()
	err = c.Run(ctx)
	stopConsumer()
	<-consumerDone
	<-crmDone
	return err
}

func newLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, nil))
}

func runMigrate(ctx context.Context) error {
	e := config.NewEnv()
	dbc := dbConfig(e)
	if err := e.Err(); err != nil {
		return err
	}
	d, err := db.Open(ctx, dbc)
	if err != nil {
		return err
	}
	defer d.Close()

	if err := jobs.Migrate(ctx, d); err != nil {
		return err
	}
	return migrate.Apply(ctx, d, migrations())
}

func serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	return serveAll(ctx, listening{ln, newServer(h)})
}

func newServer(h http.Handler) *http.Server {
	srv := &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second, Protocols: new(http.Protocols)}
	srv.Protocols.SetHTTP1(true)
	srv.Protocols.SetUnencryptedHTTP2(true)
	return srv
}

func serveWithRun(ctx context.Context, run func(context.Context) error, servers ...listening) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	runDone := make(chan error, 1)
	go func() {
		err := run(ctx)
		switch {
		case ctx.Err() != nil && (err == nil || errors.Is(err, context.Canceled)):
			err = nil
		case err == nil:
			err = errors.New("背景元件在 ctx 結束前返回")
		}
		if err != nil {
			cancel()
		}
		runDone <- err
	}()
	serveErr := serveAllThen(ctx, cancel, servers...)
	cancel()
	return errors.Join(serveErr, <-runDone)
}

func joinRuns(runs ...func(context.Context) error) func(context.Context) error {
	return func(ctx context.Context) error {
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		errs := make(chan error, len(runs))
		for _, run := range runs {
			go func() {
				err := run(ctx)
				if err == nil && ctx.Err() == nil {
					err = errors.New("背景元件在 ctx 結束前返回")
				}
				cancel()
				errs <- err
			}()
		}
		var all []error
		for range runs {
			if err := <-errs; err != nil && !errors.Is(err, context.Canceled) {
				all = append(all, err)
			}
		}
		return errors.Join(all...)
	}
}

type listening struct {
	ln  net.Listener
	srv *http.Server
}

func serveAll(ctx context.Context, servers ...listening) error {
	return serveAllThen(ctx, nil, servers...)
}

func serveAllThen(ctx context.Context, beforeShutdown func(), servers ...listening) error {
	errc := make(chan error, len(servers))
	for _, s := range servers {
		go func() { errc <- s.srv.Serve(s.ln) }()
	}
	var first error
	select {
	case first = <-errc:
	case <-ctx.Done():
	}
	if beforeShutdown != nil {
		beforeShutdown()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	errs := []error{first}
	for _, s := range servers {
		errs = append(errs, s.srv.Shutdown(shutdownCtx))
	}
	remaining := len(servers)
	if first != nil {
		remaining--
	}
	for range remaining {
		if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
			errs = append(errs, err)
		}
	}
	if errors.Is(first, http.ErrServerClosed) {
		errs[0] = nil
	}
	return errors.Join(errs...)
}
