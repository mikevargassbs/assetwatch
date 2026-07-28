package main

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"

	"sbs-bsp-cctv/internal/acceptance"
	"sbs-bsp-cctv/internal/appsettings"
	"sbs-bsp-cctv/internal/audit"
	"sbs-bsp-cctv/internal/auth"
	"sbs-bsp-cctv/internal/config"
	"sbs-bsp-cctv/internal/datamanagement"
	"sbs-bsp-cctv/internal/db"
	"sbs-bsp-cctv/internal/defective"
	"sbs-bsp-cctv/internal/hardware"
	"sbs-bsp-cctv/internal/installation"
	"sbs-bsp-cctv/internal/items"
	"sbs-bsp-cctv/internal/logistics"
	"sbs-bsp-cctv/internal/mailer"
	"sbs-bsp-cctv/internal/metadata"
	"sbs-bsp-cctv/internal/rbac"
	"sbs-bsp-cctv/internal/reporting"
	"sbs-bsp-cctv/internal/sitelocation"
	"sbs-bsp-cctv/internal/version"
	"sbs-bsp-cctv/internal/webserver"
	frontend "sbs-bsp-cctv/web"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx := context.Background()
	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	mailCfg := mailer.Config{
		Host: cfg.SMTPHost, Port: cfg.SMTPPort, Username: cfg.SMTPUsername,
		Password: cfg.SMTPPassword, From: cfg.SMTPFrom,
	}

	tokenIssuer := auth.NewTokenIssuer(cfg.JWTSecret)
	auditSvc := audit.NewService(pool)
	auditHandlers := audit.NewHandlers(auditSvc)
	authSvc := auth.NewService(pool, tokenIssuer, cfg.AllowedEmailDomain, mailCfg)
	authHandlers := auth.NewHandlers(authSvc, auditSvc)

	metadataSvc := metadata.NewService(pool)
	appSettingsSvc := appsettings.NewService(pool)
	appSettingsHandlers := appsettings.NewHandlers(appSettingsSvc)
	hardwareSvc := hardware.NewService(pool, auditSvc, appSettingsSvc)
	hardwareHandlers := hardware.NewHandlers(hardwareSvc, metadataSvc)

	logisticsSvc := logistics.NewService(pool, auditSvc, hardwareSvc)
	logisticsHandlers := logistics.NewHandlers(logisticsSvc)

	installationSvc := installation.NewService(pool, auditSvc, hardwareSvc)
	installationHandlers := installation.NewHandlers(installationSvc)

	acceptanceSvc := acceptance.NewService(pool, auditSvc, hardwareSvc, mailCfg)
	acceptanceHandlers := acceptance.NewHandlers(acceptanceSvc)

	defectiveSvc := defective.NewService(pool, auditSvc, hardwareSvc, mailCfg)
	defectiveHandlers := defective.NewHandlers(defectiveSvc)

	reportingSvc := reporting.NewService(pool)
	reportingHandlers := reporting.NewHandlers(reportingSvc)

	siteLocationSvc := sitelocation.NewService(pool)
	siteLocationHandlers := sitelocation.NewHandlers(siteLocationSvc)

	itemsSvc := items.NewService(pool)
	itemsHandlers := items.NewHandlers(itemsSvc)

	dataMgmtSvc := datamanagement.NewService(pool, auditSvc)
	dataMgmtHandlers := datamanagement.NewHandlers(dataMgmtSvc)

	r := chi.NewRouter()
	r.Use(chimiddleware.RequestID)
	r.Use(chimiddleware.RealIP)
	r.Use(chimiddleware.Logger)
	r.Use(chimiddleware.Recoverer)
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(webserver.SecurityHeaders)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":    "ok",
			"version":   version.Version,
			"commit":    version.Commit,
			"buildDate": version.BuildDate,
		})
	})

	// 10 attempts per minute per source IP against the login endpoint, to
	// blunt online password guessing now that this app is internet-facing.
	loginLimiter := webserver.NewLoginRateLimiter(10, time.Minute)

	r.Route("/api/v1", func(api chi.Router) {
		api.With(loginLimiter.Middleware).Post("/auth/login", authHandlers.Login)
		api.Post("/auth/refresh", authHandlers.Refresh)
		api.Post("/auth/logout", authHandlers.Logout)
		api.Post("/auth/forgot-password", authHandlers.ForgotPassword)
		api.Post("/auth/reset-password", authHandlers.ResetPassword)

		// Public client e-signature page — token-authenticated, no login.
		api.Get("/public/acceptance/{token}", acceptanceHandlers.GetPublicSigningInfo)
		api.Post("/public/acceptance/{token}", acceptanceHandlers.SubmitClientSignature)

		api.Group(func(protected chi.Router) {
			protected.Use(auth.Middleware(tokenIssuer))

			protected.Group(func(adminOnly chi.Router) {
				adminOnly.Use(rbac.RequireRole(rbac.Admin))
				adminOnly.Get("/users", authHandlers.ListUsers)
				adminOnly.Post("/users", authHandlers.CreateUser)
				adminOnly.Put("/users/{id}", authHandlers.UpdateUser)
				adminOnly.Put("/users/{id}/roles", authHandlers.SetUserRoles)
				adminOnly.Put("/users/{id}/password", authHandlers.SetUserPassword)
				adminOnly.Post("/meta-data-fields", hardwareHandlers.CreateMetaDataField)
				adminOnly.Put("/meta-data-fields/{fieldID}", hardwareHandlers.UpdateMetaDataField)
				adminOnly.Delete("/meta-data-fields/{fieldID}", hardwareHandlers.DeactivateMetaDataField)
				adminOnly.Post("/meta-data-fields/{fieldID}/reactivate", hardwareHandlers.ReactivateMetaDataField)
				adminOnly.Post("/site-locations", siteLocationHandlers.Create)
				adminOnly.Put("/site-locations/{id}", siteLocationHandlers.Update)
				adminOnly.Delete("/site-locations/{id}", siteLocationHandlers.Deactivate)
				adminOnly.Post("/site-locations/{id}/reactivate", siteLocationHandlers.Reactivate)
				adminOnly.Post("/items", itemsHandlers.Create)
				adminOnly.Put("/items/{id}", itemsHandlers.Update)
				adminOnly.Delete("/items/{id}", itemsHandlers.Deactivate)
				adminOnly.Post("/items/{id}/reactivate", itemsHandlers.Reactivate)
				adminOnly.Post("/hardware-units/{id}/board-column", hardwareHandlers.MoveBoardColumn)
				adminOnly.Delete("/hardware-units/{id}", hardwareHandlers.DeleteUnit)
				adminOnly.Post("/hardware-units/{id}/restore", hardwareHandlers.RestoreUnit)
				adminOnly.Delete("/hardware-units/{id}/purge", hardwareHandlers.PurgeUnit)
				adminOnly.Get("/audit-log", auditHandlers.List)
				adminOnly.Put("/settings/barcode-label", appSettingsHandlers.UpdateBarcodeLabelSettings)
				adminOnly.Post("/settings/barcode-label/preview", hardwareHandlers.PreviewBarcodeLabel)
				adminOnly.Get("/data-management/export", dataMgmtHandlers.Export)
				adminOnly.Post("/data-management/wipe", dataMgmtHandlers.WipeAll)
			})

			// Readable by anyone authenticated (Kanban board, forms, etc.)
			protected.Get("/hardware-units/device-makes", hardwareHandlers.ListDeviceMakes)
			protected.Get("/hardware-units/device-models", hardwareHandlers.ListDeviceModels)
			protected.Get("/site-locations", siteLocationHandlers.List)
			protected.Get("/items", itemsHandlers.List)
			protected.Get("/hardware-units", hardwareHandlers.ListUnits)
			protected.Get("/hardware-units/{id}", hardwareHandlers.GetUnit)
			protected.Post("/hardware-units/{id}/board-column/sync", hardwareHandlers.SyncBoardColumn)
			protected.Get("/hardware-units/{id}/receiving", hardwareHandlers.GetReceiving)
			protected.Get("/hardware-units/{id}/stage1a", hardwareHandlers.GetStage1A)
			protected.Get("/hardware-units/stage1a/network-defaults", hardwareHandlers.NetworkDefaultsForSite)
			protected.Get("/hardware-units/{id}/stage1a/barcode", hardwareHandlers.PrintBarcodeSticker)
			protected.Get("/hardware-units/{id}/stage1b", hardwareHandlers.GetStage1B)
			protected.Get("/meta-data-fields", hardwareHandlers.ListMetaDataFields)
			protected.Get("/settings/barcode-label", appSettingsHandlers.GetBarcodeLabelSettings)
			protected.Get("/settings/barcode-label/fields", hardwareHandlers.ListBarcodeLabelFields)
			protected.Get("/hardware-units/{id}/logistics", logisticsHandlers.GetUnitLogistics)
			protected.Get("/delivery-dockets", logisticsHandlers.ListDockets)
			protected.Get("/delivery-dockets/{id}", logisticsHandlers.GetDocket)
			protected.Get("/delivery-dockets/{id}/waybill", logisticsHandlers.PrintWaybill)
			protected.Get("/hardware-units/{id}/installation", installationHandlers.GetInstallation)
			protected.Get("/hardware-units/{id}/installation/photos", installationHandlers.ListPhotos)
			protected.Get("/hardware-units/{id}/installation/photos/{photoId}", installationHandlers.DownloadPhoto)
			protected.Get("/hardware-units/{id}/acceptance", acceptanceHandlers.GetAcceptance)
			protected.Get("/hardware-units/{id}/acceptance/document", acceptanceHandlers.DownloadDocument)
			protected.Get("/hardware-units/{id}/defect", defectiveHandlers.GetDefectReport)
			protected.Get("/hardware-units/{id}/defect/report", defectiveHandlers.PrintReport)
			protected.Get("/hardware-units/{id}/report", reportingHandlers.GetUnitInfoSheet)
			protected.Get("/reports/hardware-summary", reportingHandlers.GetHardwareSummary)
			protected.Get("/reports/defects", reportingHandlers.GetDefectsReport)
			protected.Get("/reports/packing-list", reportingHandlers.GetPackingListForSite)

			protected.Group(func(intake chi.Router) {
				intake.Use(rbac.RequireRole(rbac.PMPC, rbac.Encoder))
				intake.Post("/hardware-units", hardwareHandlers.CreateUnit)
			})

			protected.Group(func(receiving chi.Router) {
				receiving.Use(rbac.RequireRole(rbac.PMPC))
				receiving.Post("/hardware-units/{id}/receiving", hardwareHandlers.RecordReceiving)
			})

			protected.Group(func(stage1a chi.Router) {
				stage1a.Use(rbac.RequireRole(rbac.Encoder, rbac.Configurator, rbac.QC))
				stage1a.Put("/hardware-units/{id}/stage1a", hardwareHandlers.UpsertStage1A)
				stage1a.Post("/hardware-units/{id}/stage1a/sign-off", hardwareHandlers.SignOffStage1A)
			})

			protected.Group(func(stage1b chi.Router) {
				stage1b.Use(rbac.RequireRole(rbac.Configurator, rbac.QC))
				stage1b.Put("/hardware-units/{id}/stage1b", hardwareHandlers.UpsertStage1B)
				stage1b.Post("/hardware-units/{id}/stage1b/sign-off", hardwareHandlers.SignOffStage1B)
			})

			protected.Group(func(custody chi.Router) {
				custody.Use(rbac.RequireRole(rbac.PMPC, rbac.Logistics))
				custody.Post("/hardware-units/{id}/store-custody", hardwareHandlers.LogStoreCustody)
			})

			protected.Group(func(logisticsGroup chi.Router) {
				logisticsGroup.Use(rbac.RequireRole(rbac.Logistics))
				logisticsGroup.Post("/delivery-dockets", logisticsHandlers.CreateDocket)
				logisticsGroup.Post("/delivery-dockets/{id}/items", logisticsHandlers.AddItem)
				logisticsGroup.Delete("/delivery-dockets/{id}/items/{itemId}", logisticsHandlers.RemoveItem)
				logisticsGroup.Post("/delivery-dockets/{id}/dispatch", logisticsHandlers.Dispatch)
				logisticsGroup.Post("/delivery-dockets/{id}/receive", logisticsHandlers.Receive)
				logisticsGroup.Post("/delivery-dockets/{id}/tracking-events", logisticsHandlers.AddTrackingEvent)
			})

			protected.Group(func(installationGroup chi.Router) {
				installationGroup.Use(rbac.RequireRole(rbac.FieldTechnician))
				installationGroup.Post("/hardware-units/{id}/installation/site-receipt", installationHandlers.RecordSiteReceipt)
				installationGroup.Put("/hardware-units/{id}/installation", installationHandlers.UpsertInstallation)
				installationGroup.Post("/hardware-units/{id}/installation/contactability-check", installationHandlers.CheckContactability)
				installationGroup.Post("/hardware-units/{id}/installation/sign-off", installationHandlers.SignOffInstallation)
				installationGroup.Post("/hardware-units/{id}/installation/photos", installationHandlers.UploadPhoto)
				installationGroup.Delete("/hardware-units/{id}/installation/photos/{photoId}", installationHandlers.DeletePhoto)
			})

			protected.Group(func(acceptanceGroup chi.Router) {
				acceptanceGroup.Use(rbac.RequireRole(rbac.BSPAcceptanceOfficer))
				acceptanceGroup.Post("/hardware-units/{id}/acceptance/bsp-signoff", acceptanceHandlers.RecordBSPAcceptance)
				acceptanceGroup.Post("/hardware-units/{id}/acceptance/generate-client-link", acceptanceHandlers.GenerateClientSigningLink)
				acceptanceGroup.Post("/hardware-units/{id}/acceptance/email-client-link", acceptanceHandlers.EmailClientSigningLink)
				acceptanceGroup.Post("/hardware-units/{id}/acceptance/generate-head-office-link", acceptanceHandlers.GenerateHeadOfficeSigningLink)
				acceptanceGroup.Post("/hardware-units/{id}/acceptance/email-head-office-link", acceptanceHandlers.EmailHeadOfficeSigningLink)
				acceptanceGroup.Post("/hardware-units/{id}/acceptance/manual-upload", acceptanceHandlers.RecordManualUpload)
			})

			// A defect can be found by anyone operational, at any stage — it's
			// a branch off the main flow, not tied to one role or column.
			protected.Group(func(defectGroup chi.Router) {
				defectGroup.Use(rbac.RequireRole(
					rbac.PMPC, rbac.Encoder, rbac.Configurator, rbac.QC,
					rbac.Logistics, rbac.FieldTechnician, rbac.BSPAcceptanceOfficer,
				))
				defectGroup.Post("/hardware-units/{id}/defect", defectiveHandlers.DeclareDefect)
				defectGroup.Post("/hardware-units/{id}/defect/email-supplier", defectiveHandlers.EmailToSupplier)
				defectGroup.Post("/hardware-units/{id}/defect/shipped-back", defectiveHandlers.MarkShippedBack)
				defectGroup.Post("/hardware-units/{id}/defect/delivered", defectiveHandlers.MarkDelivered)
				defectGroup.Post("/hardware-units/{id}/defect/supplier-received", defectiveHandlers.MarkSupplierReceived)
				defectGroup.Post("/hardware-units/{id}/defect/replacement", defectiveHandlers.RecordReplacement)
				defectGroup.Put("/hardware-units/{id}/meta-data", hardwareHandlers.UpdateUnitMetaData)
				defectGroup.Put("/hardware-units/{id}/identity", hardwareHandlers.UpdateUnitIdentity)
			})
		})
	})

	// Serve the built frontend (web/dist, embedded at compile time) for any
	// route not matched above, with SPA fallback to index.html so client-side
	// routes (e.g. /board, /units/:id) work on a full page load or refresh.
	// This lets the whole app — API and UI — run as a single binary/port.
	if distFS, err := fs.Sub(frontend.DistFS, "dist"); err == nil {
		r.NotFound(webserver.SPAHandler(distFS))
	} else {
		log.Printf("frontend not embedded (run `npm run build` in web/ first): %v", err)
	}

	log.Printf("sbs-api %s (commit %s, built %s) listening on :%s", version.Version, version.Commit, version.BuildDate, cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatal(err)
	}
}
