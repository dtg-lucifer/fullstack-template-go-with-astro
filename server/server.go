// Package server wires every subsystem together and manages the full application lifecycle.
// Each subsystem has its own setup method; Shutdown() tears them down in reverse order.
package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/your-username/go-mux-backend-template/server/config"
	"github.com/your-username/go-mux-backend-template/server/internal/core/cache"
	"github.com/your-username/go-mux-backend-template/server/internal/core/events"
	"github.com/your-username/go-mux-backend-template/server/internal/core/queue"
	"github.com/your-username/go-mux-backend-template/server/internal/core/realtime"
	"github.com/your-username/go-mux-backend-template/server/internal/core/workers"
	"github.com/your-username/go-mux-backend-template/server/internal/db"
	"github.com/your-username/go-mux-backend-template/server/internal/db/repository"
	"github.com/your-username/go-mux-backend-template/server/internal/middlewares"
	"github.com/your-username/go-mux-backend-template/server/internal/modules"
	"github.com/your-username/go-mux-backend-template/server/pkg"
)

// Server owns every long-lived resource in the application.
// A nil subsystem field means that subsystem is disabled via config.yaml.
type Server struct {
	cfg        *config.Config
	logger     *pkg.Logger
	startTime  time.Time
	webHandler http.Handler // frontend handler; nil disables static file serving

	pool    *pgxpool.Pool
	redis   cache.Cache
	bus     *events.Bus
	hub     *realtime.Hub
	qmgr    *queue.Manager
	httpSrv *http.Server
	router  *mux.Router
}

// New creates a Server. Call Setup() then Start().
// Pass the web handler from WebFS() in main; pass nil to disable static file
// serving (useful in API-only mode).
func New(cfg *config.Config, logger *pkg.Logger, webHandler http.Handler) *Server {
	return &Server{
		cfg:        cfg,
		logger:     logger,
		startTime:  time.Now(),
		webHandler: webHandler,
	}
}

// Setup initialises every subsystem in the correct order.
func (s *Server) Setup(ctx context.Context) error {
	s.logger.Info("[SERVER] Environment: " + s.cfg.Server.Environment)
	s.logger.Info("[SERVER] API prefix:  " + s.cfg.Server.APIPrefix)

	if err := s.setupDatabase(ctx); err != nil {
		return err
	}
	if err := s.setupRedis(ctx); err != nil {
		return err
	}

	s.setupEventBus()
	s.setupRealtime()

	if err := s.setupQueue(ctx); err != nil {
		return err
	}

	s.setupEventHandlers()
	s.setupRouter()

	s.logger.Info("[SERVER] Setup completed successfully")
	return nil
}

func (s *Server) setupDatabase(ctx context.Context) error {
	s.logger.Info("[DATABASE] Connecting…")

	pool, err := db.Connect(ctx, s.cfg.DSN(), int32(s.cfg.Database.PoolSize))
	if err != nil {
		return fmt.Errorf("database connection failed: %w", err)
	}

	s.pool = pool
	s.logger.Info("[DATABASE] PostgreSQL connected",
		"host", s.cfg.Database.Host,
		"pool_size", s.cfg.Database.PoolSize,
	)
	return nil
}

func (s *Server) setupRedis(ctx context.Context) error {
	if !s.cfg.Redis.Enabled {
		s.logger.Info("[REDIS] Redis disabled in config.yaml")
		return nil
	}

	s.logger.Info("[REDIS] Connecting…")

	client, err := cache.New(ctx, cache.ClientConfig{
		Addr:     s.cfg.RedisAddr(),
		Password: s.cfg.RedisPassword(),
		DB:       s.cfg.Redis.DB,
		PoolSize: s.cfg.Redis.PoolSize,
	})
	if err != nil {
		return fmt.Errorf("redis connection failed: %w", err)
	}

	s.redis = client
	s.logger.Info("[REDIS] Connected", "addr", s.cfg.RedisAddr(), "db", s.cfg.Redis.DB)
	return nil
}

func (s *Server) setupEventBus() {
	s.bus = events.New()
	s.logger.Info("[EVENTS] Domain event bus initialised")
}

func (s *Server) setupRealtime() {
	if !s.cfg.Realtime.WebSocket.Enabled {
		s.logger.Info("[WS] WebSocket disabled in config.yaml")
		return
	}

	s.hub = realtime.NewHub(
		s.bus,
		s.cfg.Security.CORS.Origins,
		s.cfg.Realtime.WebSocket.ReadBufferSize,
		s.cfg.Realtime.WebSocket.WriteBufferSize,
		s.logger,
	)
	s.logger.Info("[WS] WebSocket hub ready", "path", s.cfg.Realtime.WebSocket.Path)
}

func (s *Server) setupQueue(ctx context.Context) error {
	if !s.cfg.Queue.RabbitMQ.Enabled {
		s.logger.Info("[QUEUE] RabbitMQ disabled in config.yaml")
		return nil
	}

	backoff, err := time.ParseDuration(s.cfg.Queue.RabbitMQ.DefaultBackoff)
	if err != nil {
		backoff = time.Second
	}

	mgr, err := queue.New(queue.ManagerConfig{
		URL:             s.cfg.AMQPUrl(),
		DefaultAttempts: s.cfg.Queue.RabbitMQ.DefaultAttempts,
		DefaultBackoff:  backoff,
	}, s.logger)
	if err != nil {
		return fmt.Errorf("queue setup failed: %w", err)
	}
	s.qmgr = mgr

	if s.cfg.Workers.Process.Enabled && s.cfg.Workers.NotificationJobs.Enabled {
		if err := s.qmgr.ConsumeWelcomeEmails(ctx, s.cfg.Workers.NotificationJobs.Concurrency, workers.WelcomeEmailHandler(s.logger)); err != nil {
			return fmt.Errorf("starting email consumer: %w", err)
		}
		s.logger.Info("[QUEUE] Email job consumer started",
			"concurrency", s.cfg.Workers.NotificationJobs.Concurrency,
		)
	} else {
		s.logger.Info("[QUEUE] Worker process disabled in config.yaml")
	}

	return nil
}

func (s *Server) setupEventHandlers() {
	// ── Async audit log writer ─────────────────────────────────────────────────
	// The listener spawns a goroutine so the emitting handler never waits for
	// the DB write. If the write fails it is logged but not retried — audit logs
	// are best-effort and must never block the request path.
	repo := repository.New(s.pool)
	s.bus.OnAuditLog(func(p events.AuditLogPayload) {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			var actorID pgtype.UUID
			if p.ActorUserID != "" {
				_ = actorID.Scan(p.ActorUserID)
			}

			var ip, ua pgtype.Text
			_ = ip.Scan(p.IP)
			_ = ua.Scan(p.UserAgent)

			_, err := repo.CreateAuditLog(ctx, repository.CreateAuditLogParams{
				ActorUserID: actorID,
				Action:      p.Action,
				Entity:      p.Entity,
				Metadata:    p.Metadata,
				Ip:          ip,
				UserAgent:   ua,
			})
			if err != nil {
				s.logger.Error("[AUDIT] Failed to write audit log", "error", err,
					"action", p.Action, "actor", p.ActorUserID)
			}
		}()
	})
	s.logger.Info("[AUDIT] Async audit log listener registered")

	// ── Queue-backed event handlers ────────────────────────────────────────────
	if s.qmgr == nil {
		s.logger.Info("[EVENTS] Queue-backed event handlers are disabled (no queue)")
		return
	}

	s.bus.OnUserRegistered(func(p events.UserRegisteredPayload) {
		job := queue.WelcomeEmailJob{UserID: p.UserID, Email: p.Email}
		if err := s.qmgr.Publish(context.Background(), queue.EmailQueue, job); err != nil {
			s.logger.Error("[EVENTS] Failed to enqueue welcome email", "error", err)
			return
		}
		s.logger.Info("[EVENTS] Welcome email job enqueued", "user_id", p.UserID)
		s.bus.EmitJobEnqueued(events.JobEnqueuedPayload{
			Queue:   queue.EmailQueue,
			JobName: "send-welcome-email",
			JobID:   "welcome-email:" + p.UserID,
		})
	})

	s.logger.Info("[EVENTS] Domain event handlers registered")
}

func (s *Server) setupRouter() {
	router := mux.NewRouter()

	if s.cfg.Middlewares.RequestID.Enabled {
		router.Use(middlewares.RequestIDMiddleware)
	}
	if s.cfg.Security.CORS.Enabled {
		router.Use(middlewares.CORSMiddleware)
	}
	if s.cfg.Security.RateLimit.Enabled {
		rl := middlewares.NewRateLimiter(
			s.cfg.Security.RateLimit.MaxRequests,
			time.Duration(s.cfg.Security.RateLimit.WindowSeconds)*time.Second,
		)
		router.Use(rl.Middleware)
	}
	if s.cfg.Middlewares.Logger.Enabled {
		router.Use(middlewares.LoggerMiddleware(s.cfg.Server.Environment))
	}

	apiRouter := router.PathPrefix(s.cfg.Server.APIPrefix).Subrouter()
	modules.Register(apiRouter, s.pool, s.redis, s.bus, s.startTime)

	s.logger.Info("[ROUTES] Mounted under " + s.cfg.Server.APIPrefix)

	if s.hub != nil {
		router.Handle(s.cfg.Realtime.WebSocket.Path, s.hub)
		s.logger.Info("[WS] WebSocket endpoint mounted", "path", s.cfg.Realtime.WebSocket.Path)
	}

	// ── Frontend (SPA fallback) ───────────────────────────────────────────────
	// In production the handler serves from the embedded FS; in dev it reads
	// from web/dist on disk. Either way, unknown paths fall back to index.html
	// so client-side routing works correctly.
	if s.webHandler != nil {
		router.PathPrefix("/").Handler(s.webHandler)
		s.logger.Info("[WEB] Frontend handler mounted at /")
	}

	s.router = router
	s.httpSrv = &http.Server{
		Addr:         s.cfg.Addr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

// Start begins listening for HTTP connections. It blocks until SIGINT or SIGTERM
// is received, then calls Shutdown().
func (s *Server) Start() {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		s.logger.Info("[SERVER] Started", "addr", s.cfg.Addr())
		if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			s.logger.Error("[SERVER] HTTP server error", "error", err)
			quit <- syscall.SIGTERM
		}
	}()

	<-quit
	s.logger.Info("[SERVER] Shutdown signal received, draining connections…")
	s.Shutdown()
}

// Shutdown tears down every subsystem in reverse setup order.
func (s *Server) Shutdown() {
	if s.httpSrv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := s.httpSrv.Shutdown(ctx); err != nil {
			s.logger.Error("[SERVER] HTTP shutdown error", "error", err)
		} else {
			s.logger.Info("[SERVER] HTTP server closed")
		}
	}

	if s.qmgr != nil {
		s.qmgr.Close()
	}

	if s.redis != nil {
		if err := s.redis.Close(); err != nil {
			s.logger.Error("[REDIS] Close error", "error", err)
		} else {
			s.logger.Info("[REDIS] Connection closed")
		}
	}

	if s.pool != nil {
		s.pool.Close()
		s.logger.Info("[DATABASE] Connection pool closed")
	}

	s.logger.Info("[SERVER] Shutdown complete")
}
