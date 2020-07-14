package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/jwtauth"
	"github.com/go-chi/render"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"github.com/plumlab/go-chi-sample/internal/handler"
	smart "github.com/plumlab/go-chi-sample/internal/middleware"
	"github.com/plumlab/go-chi-sample/internal/repo"
	"github.com/plumlab/go-chi-sample/internal/service"
	"github.com/sony/gobreaker"
	"github.com/spf13/viper"
)

var tokenAuth *jwtauth.JWTAuth

func init() {
	tokenAuth = jwtauth.New(jwt.SigningMethodHS256.Alg(), []byte(viper.GetString("SIGNING_KEY")), nil)
}

func main() {
	viper.AutomaticEnv()
	dbURL := viper.GetString("DB_URL")
	db, err := sqlx.Connect("mysql", dbURL)
	if err != nil {
		log.Fatalf("Cannot connect to MySQL at %v: %v", dbURL, err)
	}
	defer func() {
		if err = db.Close(); err != nil {
			log.Printf("Error while closing DB connection: %v", err)
		}
	}()
	var st gobreaker.Settings
	st.Name = "MYSQLDB"
	st.ReadyToTrip = func(counts gobreaker.Counts) bool {
		failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
		return counts.Requests >= 3 && failureRatio >= 0.6
	}
	st.Timeout = time.Minute

	cb := gobreaker.NewCircuitBreaker(st)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var srv http.Server
	done := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		// interrupt signal sent from terminal
		signal.Notify(sigint, os.Interrupt)
		// sigterm signal sent from kubernetes
		signal.Notify(sigint, syscall.SIGTERM)

		<-sigint

		// We received an interrupt signal, shut down gracefully
		if err := srv.Shutdown(ctx); err != nil {
			log.Printf("HTTP server Shutdown: %v", err)
		}
		close(done)
	}()

	r := chi.NewRouter()
	cors := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})

	r.Use(cors.Handler)
	r.Use(middleware.RequestID)
	r.Use(middleware.Logger)
	r.Use(middleware.URLFormat)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.ThrottleBacklog(50, 100, 5*time.Second))
	r.Use(render.SetContentType(render.ContentTypeJSON))
	r.Use(middleware.Heartbeat("/ping"))

	r.Mount("/debug", middleware.Profiler())

	userRepo := repo.NewUser(cb, db)
	accountHandler := handler.NewAccount(service.NewAccount(userRepo))
	r.Post("/signin", accountHandler.Login)
	r.Put("/register", accountHandler.Register)
	r.Post("/forgot-password", accountHandler.ForgotPassword)
	r.Post("/reset-password", accountHandler.ResetPassword)
	r.Post("/verify-token", accountHandler.VerifyPasswordResetToken)
	r.Post("/verify-email", accountHandler.VerifyEmailToken)

	r.Group(func(r chi.Router) {
		r.Use(jwtauth.Verifier(tokenAuth))
		r.Use(jwtauth.Authenticator)
		r.Use(smart.Verifier(userRepo))

		r.Post("/signout", accountHandler.Logout)
	})

	srv = http.Server{
		Addr:    ":8080",
		Handler: r,
	}
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("HTTP server ListenAndServe: %v", err)
	}

	<-done
}
