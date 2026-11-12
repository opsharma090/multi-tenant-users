package main

import (
	"context"
	"errors"
	"log"
	"multi-tenant-users/internal/audit"
	"multi-tenant-users/internal/auth"
	"multi-tenant-users/internal/middleware"
	"multi-tenant-users/internal/platform"
	"multi-tenant-users/internal/role"
	"multi-tenant-users/internal/tenant"
	"multi-tenant-users/internal/user"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return errors.New("DATABASE_URL is required")
	}

	connectCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	db, err := platform.OpenDB(connectCtx, databaseURL)
	cancel()
	if err != nil {
		return err
	}
	defer db.Close()

	jwtSecret := os.Getenv("JWT_SECRET")
	if len(jwtSecret) < 32 {
		return errors.New("JWT_SECRET must contain at least 32 characters")
	}

	authRepo := auth.NewRepository(db)
	authService := auth.NewService(authRepo, jwtSecret)
	authHandler := auth.NewHandler(authService)

	mux := http.NewServeMux()
	mux.HandleFunc("POST /auth/login", authHandler.Login)
	mux.HandleFunc("POST /auth/refresh", authHandler.Refresh)
	authMiddleware := middleware.NewAuth(db, jwtSecret)

	userRepo := user.NewRepository(db)
	userService := user.NewService(userRepo)
	userHandler := user.NewHandler(userService)

	mux.Handle("GET /users", authMiddleware.Authenticate(
		authMiddleware.Require("user:read",
			http.HandlerFunc(userHandler.List),
		),
	))

	mux.Handle("GET /users/{id}", authMiddleware.Authenticate(
		authMiddleware.Require("user:read",
			http.HandlerFunc(userHandler.Get),
		),
	))

	mux.Handle("POST /users", authMiddleware.Authenticate(
		authMiddleware.Require("user:create",
			http.HandlerFunc(userHandler.Create),
		),
	))

	mux.Handle("PATCH /users/{id}", authMiddleware.Authenticate(
		authMiddleware.Require("user:update",
			http.HandlerFunc(userHandler.Update),
		),
	))

	mux.Handle("DELETE /users/{id}", authMiddleware.Authenticate(
		authMiddleware.Require("user:delete",
			http.HandlerFunc(userHandler.Delete),
		),
	))

	roleRepo := role.NewRepository(db)
	roleService := role.NewService(roleRepo)
	roleHandler := role.NewHandler(roleService)

	mux.Handle("POST /roles", authMiddleware.Authenticate(
		authMiddleware.Require("role:create",
			http.HandlerFunc(roleHandler.Create),
		),
	))

	mux.Handle("GET /roles", authMiddleware.Authenticate(
		authMiddleware.Require("role:read",
			http.HandlerFunc(roleHandler.List),
		),
	))
	mux.Handle("GET /permissions", authMiddleware.Authenticate(
		authMiddleware.Require("permission:read",
			http.HandlerFunc(roleHandler.Permissions),
		),
	))

	mux.Handle("PUT /roles/{id}/permissions", authMiddleware.Authenticate(
		authMiddleware.Require("permission:assign",
			http.HandlerFunc(roleHandler.SetPermissions),
		),
	))

	mux.Handle("PUT /users/{id}/roles/{roleId}", authMiddleware.Authenticate(
		authMiddleware.Require("role:assign",
			http.HandlerFunc(roleHandler.Assign),
		),
	))
	tenantRepo := tenant.NewRepository(db)
	tenantService := tenant.NewService(tenantRepo)
	tenantHandler := tenant.NewHandler(tenantService)

	mux.Handle("GET /tenant", authMiddleware.Authenticate(
		authMiddleware.Require("tenant:read",
			http.HandlerFunc(tenantHandler.Get),
		),
	))

	mux.Handle("PATCH /tenant", authMiddleware.Authenticate(
		authMiddleware.Require("tenant:update",
			http.HandlerFunc(tenantHandler.Update),
		),
	))
	mux.Handle("POST /tenants", authMiddleware.Authenticate(
		authMiddleware.Require("tenant:create",
			http.HandlerFunc(tenantHandler.Create),
		),
	))
	auditRepo := audit.NewRepository(db)
	auditService := audit.NewService(auditRepo)
	auditHandler := audit.NewHandler(auditService)

	mux.Handle("GET /audit-logs", authMiddleware.Authenticate(
		authMiddleware.Require("audit:read",
			http.HandlerFunc(auditHandler.List),
		),
	))
	mux.Handle("GET /me", authMiddleware.Authenticate(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := middleware.CurrentIdentity(r.Context())
			if !ok {
				platform.Error(w, 401, "unauthorized", "Authentication required")
				return
			}

			platform.JSON(w, http.StatusOK, identity)
		}),
	))

	mux.Handle("GET /permission-check", authMiddleware.Authenticate(
		authMiddleware.Require("user:create",
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				platform.JSON(w, http.StatusOK, map[string]string{
					"message": "You have user:create permission",
				})
			}),
		),
	))

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		checkCtx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := db.Ping(checkCtx); err != nil {
			platform.Error(
				w,
				http.StatusServiceUnavailable,
				"database_unavailable",
				"Database is unavailable",
			)
			return
		}

		platform.JSON(w, http.StatusOK, map[string]string{
			"status":   "ok",
			"database": "connected",
		})
	})

	server := &http.Server{
		Addr:              ":8080",
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErrors := make(chan error, 1)

	go func() {
		log.Println("API running at http://localhost:8080")
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err

	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(
			context.Background(),
			10*time.Second,
		)
		defer cancel()

		return server.Shutdown(shutdownCtx)
	}
}
