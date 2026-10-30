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
	httpagent "github.com/Will413028/hawkr/backend/internal/httpapi/agent"
	httpbuyer "github.com/Will413028/hawkr/backend/internal/httpapi/buyer"
	"github.com/Will413028/hawkr/backend/internal/httpapi/rpc"
	httpseller "github.com/Will413028/hawkr/backend/internal/httpapi/seller"
	"github.com/Will413028/hawkr/backend/internal/legacy/accountservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/buyerconsumer"
	"github.com/Will413028/hawkr/backend/internal/legacy/buyerrelay"
	"github.com/Will413028/hawkr/backend/internal/legacy/buyerserver"
	"github.com/Will413028/hawkr/backend/internal/legacy/crmservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/delegationservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/gatewayproxy"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/account/accountconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/buyer/buyerconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/gen/seller/sellerconnect"
	"github.com/Will413028/hawkr/backend/internal/legacy/mediaservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/orderservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/productservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/rabbitmq"
	"github.com/Will413028/hawkr/backend/internal/legacy/reviewservice"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerconsumer"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerrelay"
	"github.com/Will413028/hawkr/backend/internal/legacy/sellerserver"
	"github.com/Will413028/hawkr/backend/internal/modules/accountdeletion"
	"github.com/Will413028/hawkr/backend/internal/modules/accountsuspension"
	"github.com/Will413028/hawkr/backend/internal/modules/buyer"
	"github.com/Will413028/hawkr/backend/internal/modules/content"
	"github.com/Will413028/hawkr/backend/internal/modules/delegation"
	"github.com/Will413028/hawkr/backend/internal/modules/reviewqueue"
	"github.com/Will413028/hawkr/backend/internal/platform/config"
	"github.com/Will413028/hawkr/backend/internal/platform/db"
	"github.com/Will413028/hawkr/backend/internal/platform/events"
	"github.com/Will413028/hawkr/backend/internal/platform/jobs"
	"github.com/Will413028/hawkr/backend/internal/platform/migrate"
	"github.com/Will413028/hawkr/backend/internal/platform/revalidate"
)

// shutdownTimeout 是收到停止訊號後，給進行中請求收尾的時間。
const shutdownTimeout = 30 * time.Second

const usage = "可用的子命令：serve、worker、migrate、dev-token、dev-seed、reannounce-exposures、delegation-eligibility、admin-role、social-recount"

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
		return runDevToken(ctx, args[1:], os.Stdout)
	case "dev-seed":
		return runDevSeed(ctx, args[1:], os.Stdout)
	case "reannounce-exposures":
		return runReannounceExposures(ctx, args[1:], os.Stdout)
	case "delegation-eligibility":
		return runDelegationEligibility(ctx, args[1:])
	case "admin-role":
		return runAdminRole(ctx, args[1:], os.Stdout)
	case "social-recount":
		return runSocialRecount(ctx, os.Stdout)
	default:
		return fmt.Errorf("未知的子命令 %q（%s）", args[0], usage)
	}
}

// dbConfig 讀取資料庫設定。
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
	// 舊服務的 gRPC 以服務間簽章認證；沒設定時不簽章，只有對方的 INTERNAL_AUTH_MODE 不是 enforcing 時能用。
	internalSecret := e.StringOr("INTERNAL_SERVICE_HMAC_SECRET", "")
	// media 在遷移計畫 B1 之前還是 Java 的 media-service（gRPC）。
	mediaCfg := mediaservice.Config{Addr: e.String("MEDIA_SERVICE_ADDR"), Secret: internalSecret}
	// 對外圖片的網域（image gateway），client 不持有它（ADR 2026-09-25-api-image-uri-template）。
	mediaOrigin := e.String("MEDIA_ORIGIN")
	// 網站的網域：API 回給 client 的網站網址（入駐文件）一律是絕對網址，由 internal/httpapi 以它組成。
	webOrigin := e.String("PUBLIC_WEB_ORIGIN")
	// 單體是對外的入口（遷移計畫 A2）：還沒搬的路徑轉給 gateway。
	gatewayURL := e.String("GATEWAY_URL")
	// 單體替舊服務提供 account-service 的 gRPC（h2c），舊服務的 ACCOUNT_SERVICE_* 指向這裡。
	legacyAddr := e.String("LEGACY_GRPC_ADDR")
	productAddr := e.String("PRODUCT_SERVICE_ADDR")
	orderAddr := e.String("ORDER_SERVICE_ADDR")
	crmAddr := e.String("CRM_SERVICE_ADDR")
	// 店家評價統計（店面頁、推薦排行）：review 還在舊服務（遷移計畫 A6）。
	reviewAddr := e.String("REVIEW_SERVICE_ADDR")
	// 直接呼叫單體的 web 的 origin（CORS 與 CSRF；後台經自己的 BFF 呼叫，不在這裡）；限流的身分（origin-auth 的金鑰、
	// CDN 帶的 client IP header）。
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
	kyc := readKYCKeys(e)
	pay := readPayUni(e, log)
	registry := readGCIS(e)
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
	med, err := mediaservice.New(mediaCfg)
	if err != nil {
		return err
	}
	products, err := productservice.New(productAddr, internalSecret)
	if err != nil {
		return err
	}
	orders, err := orderservice.New(orderAddr, internalSecret)
	if err != nil {
		return err
	}
	crm, err := crmservice.New(crmAddr, internalSecret)
	if err != nil {
		return err
	}
	reviews, err := reviewservice.New(reviewAddr, internalSecret)
	if err != nil {
		return err
	}
	// 刪除帳號、確認代行的寫入時，以同一個 transaction 排入送給舊服務的事件（RabbitMQ 的 account.deleted、
	// delegation.operation.completed），由 worker 送出。
	bus := events.NewBus()
	enqueueLegacy := func(ctx context.Context, a rabbitmq.PublishArgs) error {
		return jc.Enqueue(ctx, a, rabbitmq.InsertOpts())
	}
	events.Subscribe(bus, accountservice.OnAccountDeleted(enqueueLegacy))
	events.Subscribe(bus, delegationservice.OnDelegationOperationCompleted(enqueueLegacy))
	subscribeBuyerEvents(bus, jc)
	id := newIdentity(d, bus, jc, idc, log)
	// 保存的 Apple token 用到的金鑰都要在 IDENTITY_TOKEN_KEYS 裡，缺了就不啟動。
	if err := id.CheckAppleKeys(ctx); err != nil {
		return err
	}
	buyerModule := buyer.New(buyer.Deps{
		DB: d, Events: bus, Jobs: jc,
		Media: med, Now: time.Now, Logger: log,
	})
	subscribeBuyerLifecycle(bus, buyerModule)
	sellerModule := newSeller(d, bus, jc, id, med, orders, sellerProducts{client: products}, kyc, pay, registry, log)
	// 資料庫參照的金鑰（KYC 的密文、blind index、處分的身分快照）都要在設定的金鑰環裡，缺了就不啟動。
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
	socialModule := newSocial(d, bus, jc, buyerModule, sellerModule, med, socialProducts{client: products}, log)
	agentModule := newAgent(d, sellerModule)
	subscribeAgent(bus, agentModule)
	discoveryModule := newDiscovery(d, orders, products, reviews, sellerModule, log)
	gateway, err := gatewayproxy.New(gatewayURL, func(ctx context.Context) (gatewayproxy.Caller, bool) {
		p, ok := rpc.PrincipalFrom(ctx)
		return gatewayproxy.Caller{Principal: p}, ok
	}, httpagent.SessionOf(agentModule), buyerModule, sellerModule, nil, log)
	if err != nil {
		return err
	}
	// 店面頁、賣家自己的店與所有清單上的店家卡片共用同一份統計組裝（粉絲來自 buyer，評價來自還在舊服務的 review）。
	stats, err := httpseller.NewStatsSource(buyerModule, sellerRatings{reviews}, log)
	if err != nil {
		return err
	}
	deletion := accountdeletion.New(accountdeletion.Deps{DB: d, Buyer: buyerModule, Seller: sellerModule, Identity: id, Orders: orders, Now: time.Now, NewID: uuid.NewV7, Logger: log})
	suspension := accountsuspension.New(accountsuspension.Deps{DB: d, Events: bus, Identity: id, Seller: sellerModule, Logger: log})
	del := newDelegation(bus, delegation.Deps{
		DB: d, Identity: id, Seller: sellerModule,
		NewID: uuid.NewV7, Now: time.Now, Logger: log,
	})

	h, err := httpapi.NewHandler(httpapi.Deps{
		Ready:  d.Ping,
		Social: socialModule,
		Content: content.New(content.Deps{
			DB:     d,
			Media:  med,
			Jobs:   jc,
			NewID:  uuid.NewV7,
			Now:    time.Now,
			Logger: log,
		}),
		Identity:        id,
		Buyer:           buyerModule,
		Seller:          sellerModule,
		Agent:           agentModule,
		Discovery:       discoveryModule,
		ReviewQueue:     reviewqueue.New(reviewqueue.Deps{DB: d}),
		Stats:           stats,
		DashboardOrders: sellerDashboardOrders{orders},
		BuyerPorts:      httpbuyer.Ports{Products: buyerProducts{products}, Stores: sellerModule, Orders: buyerOrders{orders}, CRM: crm},
		Deletion:        deletion,
		Delegation:      del,
		Images:          images,
		WebOrigin:       web,
		Origins:         origins,
		OriginAuth:      originAuth,
		ClientIP:        clientIP,
		Legacy:          gateway,
		LegacyPrefixes:  gatewayproxy.Prefixes,
		Now:             time.Now,
		Logger:          log,
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
	legacyMux.Handle(accountconnect.NewAccountServiceHandler(accountservice.New(id, suspension, log)))
	legacyMux.Handle(buyerconnect.NewBuyerServiceHandler(buyerserver.New(buyerModule)))
	legacyMux.Handle(sellerconnect.NewSellerServiceHandler(sellerserver.New(sellerModule, legacySellerRatings{reviews}, buyerModule, agentModule, log)))
	legacyMux.Handle(accountconnect.NewDelegationServiceHandler(delegationservice.New(del, log)))
	legacy := &http.Server{Handler: legacyMux, ReadHeaderTimeout: 10 * time.Second, Protocols: new(http.Protocols)}
	legacy.Protocols.SetHTTP1(true)
	legacy.Protocols.SetUnencryptedHTTP2(true)
	return serveAll(ctx, listening{ln, &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}}, listening{legacyLn, legacy})
}

// workerConcurrency 是 default queue 同時執行的 job 上限。
const workerConcurrency = 10

func runWorker(ctx context.Context) error {
	e := config.NewEnv()
	dbc := dbConfig(e)
	// website 的快取失效沒有設定時不送出，但照常啟動（本機通常沒有 website；§9.5）。
	rv := revalidate.Config{
		BaseURL: e.StringOr("WEB_REVALIDATE_BASE_URL", ""),
		Secret:  e.StringOr("WEB_REVALIDATE_SECRET", ""),
	}
	// 送給舊服務的事件（RabbitMQ）：還在 RabbitMQ 上的消費者搬進單體之前需要。
	rabbitURL := e.String("RABBITMQ_URL")
	orderAddr := e.String("ORDER_SERVICE_ADDR")
	productAddr := e.String("PRODUCT_SERVICE_ADDR")
	reviewAddr := e.String("REVIEW_SERVICE_ADDR")
	internalSecret := e.StringOr("INTERNAL_SERVICE_HMAC_SECRET", "")
	mediaCfg := mediaservice.Config{Addr: e.String("MEDIA_SERVICE_ADDR"), Secret: internalSecret}
	log := newLogger()
	verifierCtx, stopVerifiers := context.WithCancel(context.WithoutCancel(ctx))
	defer stopVerifiers()
	idc := readIdentityConfig(verifierCtx, e)
	kyc := readKYCKeys(e)
	pay := readPayUni(e, log)
	registry := readGCIS(e)
	if err := e.Err(); err != nil {
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
	products, err := productservice.New(productAddr, internalSecret)
	if err != nil {
		return err
	}
	reviews, err := reviewservice.New(reviewAddr, internalSecret)
	if err != nil {
		return err
	}

	// identity 的 job（停用、刪除 Clerk 的使用者，撤銷 Apple 的授權）在 worker 執行；排入在 serve，撤銷 Apple 的第二段
	// 由 worker 自己排入。
	jc, err := jobs.NewInsertOnly(d)
	if err != nil {
		return err
	}
	bus := events.NewBus()
	subscribeBuyerEvents(bus, jc)
	id := newIdentity(d, bus, jc, idc, log)
	// 保存的 Apple token 用到的金鑰都要在 IDENTITY_TOKEN_KEYS 裡，缺了就不啟動。
	if err := id.CheckAppleKeys(ctx); err != nil {
		return err
	}
	med, err := mediaservice.New(mediaCfg)
	if err != nil {
		return err
	}
	buyerModule := buyer.New(buyer.Deps{
		DB: d, Events: bus, Jobs: jc,
		Media: med, Now: time.Now, Logger: log,
	})
	subscribeBuyerLifecycle(bus, buyerModule)
	sellerModule := newSeller(d, bus, jc, id, med, orders, sellerProducts{client: products}, kyc, pay, registry, log)
	// 資料庫參照的金鑰（KYC 的密文、blind index、處分的身分快照）都要在設定的金鑰環裡，缺了就不啟動。
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
	socialModule := newSocial(d, bus, jc, buyerModule, sellerModule, med, socialProducts{client: products}, log)
	discoveryModule := newDiscovery(d, orders, products, reviews, sellerModule, log)
	agentModule := newAgent(d, sellerModule)
	subscribeAgent(bus, agentModule)
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
	contentModule := content.New(content.Deps{DB: d, Media: med, Jobs: jc, NewID: uuid.NewV7, Now: time.Now, Logger: log})
	workers := river.NewWorkers()
	river.AddWorker(workers, revalidate.NewWorker(rv, log))
	id.AddWorkers(workers)
	contentModule.AddWorkers(workers)
	buyerModule.AddWorkers(workers)
	sellerModule.AddWorkers(workers)
	socialModule.AddWorkers(workers)
	discoveryModule.AddWorkers(workers)
	agentModule.AddWorkers(workers)
	deletion.AddWorkers(workers)
	consumer.AddWorkers(workers)
	river.AddWorker(workers, rabbitmq.NewWorker(pub))
	buyerrelay.NewWorker(buyerModule, products, sellerModule, jc, log).AddWorkers(workers)
	sellerrelay.NewWorker(sellerModule, jc, log).AddWorkers(workers)
	periodic := append(id.PeriodicJobs(), buyerModule.PeriodicJobs()...)
	periodic = append(periodic, deletion.PeriodicJobs()...)
	periodic = append(periodic, sellerModule.PeriodicJobs()...)
	periodic = append(periodic, socialModule.PeriodicJobs()...)
	periodic = append(periodic, discoveryModule.PeriodicJobs()...)
	periodic = append(periodic, agentModule.PeriodicJobs()...)
	periodic = append(periodic, contentModule.PeriodicJobs()...)
	c, err := jobs.NewWorker(d, workers, map[string]int{river.QueueDefault: workerConcurrency}, periodic)
	if err != nil {
		return err
	}
	consumerCtx, stopConsumer := context.WithCancel(ctx)
	defer stopConsumer()
	// 單體自己的 queue（hawkr.monolith 前綴）各一個 consumer：買家的訂單完成與補貨、seller 的 PayUni 審核通知。
	consumerDone := make(chan struct{}, 2)
	go func() {
		defer func() { consumerDone <- struct{}{} }()
		consumer.Run(consumerCtx, rabbitURL, log)
	}()
	go func() {
		defer func() { consumerDone <- struct{}{} }()
		sellerconsumer.Consumer{Seller: sellerModule}.Run(consumerCtx, rabbitURL, log)
	}()
	err = c.Run(ctx)
	stopConsumer()
	<-consumerDone
	<-consumerDone
	return err
}

// newLogger 是各角色共用的 log：JSON 寫到 stderr。
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
	return migrate.Apply(ctx, d, modules())
}

// serve 在 ctx 結束前持續服務；ctx 結束後停止接新連線，並等進行中的請求收尾。
func serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	return serveAll(ctx, listening{ln, &http.Server{Handler: h, ReadHeaderTimeout: 10 * time.Second}})
}

// listening 是一個在 listener 上服務的 server。
type listening struct {
	ln  net.Listener
	srv *http.Server
}

// serveAll 讓每個 server 在自己的 listener 上服務，直到 ctx 結束或其中一個失敗；之後全部停止接新連線，並等
// 進行中的請求收尾。
func serveAll(ctx context.Context, servers ...listening) error {
	errc := make(chan error, len(servers))
	for _, s := range servers {
		go func() { errc <- s.srv.Serve(s.ln) }()
	}
	var first error
	select {
	case first = <-errc:
	case <-ctx.Done():
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
