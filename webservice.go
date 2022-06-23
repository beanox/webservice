package webservice

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
)

// WebService ...
type WebService interface {
	Start() (err error)
	SetTimeouts(writeTimeout time.Duration, readTimeout time.Duration, idleTimeout time.Duration)
	SetListenAddress(listenAddress string)
	EnableCors(options *cors.Options)
	StripPath(path string)
	SetLogger(logger *logrus.Logger)
	EnablePrometheusMetrics(enable bool)
	EnableAuthorization(options *AuthorizationOptions)
}

// webservice ...
type webservice struct {
	obj                     WebserviceObject
	writeTimeout            time.Duration
	readTimeout             time.Duration
	idleTimeout             time.Duration
	listenAddress           string
	corsOptions             *cors.Options
	stripPath               string
	logger                  *logrus.Logger
	enablePrometheusMetrics bool
	authorizationOptions    *AuthorizationOptions
}

// WebserviceObject ...
type WebserviceObject interface {
}

// New creates new web service
func New(obj WebserviceObject) WebService {
	return &webservice{
		obj:                     obj,
		writeTimeout:            time.Second * 15,
		readTimeout:             time.Second * 15,
		idleTimeout:             time.Second * 60,
		listenAddress:           ":8080",
		corsOptions:             nil, // cors are not enabled
		stripPath:               "",
		logger:                  nil,
		enablePrometheusMetrics: false,
		authorizationOptions:    nil,
	}
}

// ConfigureRouterHandler is an interface to implement to configure routing for web service
type ConfigureRouterHandler interface {
	ConfigureRouter(router *mux.Router) (handler http.Handler, err error)
}

// WebServiceBeforeStartHandler is an interface to implement a callback BeforeStart()
type WebServiceBeforeStartHandler interface {
	BeforeStart() (err error)
}

// WebServiceBeforeEndHandler is an interface to implement a callback BeforeEnd()
type WebServiceBeforeEndHandler interface {
	BeforeEnd()
}

// WebServiceGetStatusHandler is an interface for implementing custom server status - GetServerStatus()
type WebServiceGetStatusHandler interface {
	GetServerStatus() (status interface{})
}

// Start starts service
func (s *webservice) Start() (err error) {

	if beforeStart, ok := s.obj.(WebServiceBeforeStartHandler); ok {
		err = beforeStart.BeforeStart()
		if err != nil {
			return
		}
	}

	var handler http.Handler

	router := mux.NewRouter()
	if s.stripPath != "" && s.stripPath != "/" {
		router = router.PathPrefix(s.stripPath).Subrouter()
	}

	if getServerStatusHandler, ok := s.obj.(WebServiceGetStatusHandler); ok {
		router.Handle("/status", AppHandler(func(w http.ResponseWriter, r *http.Request, userInfo *UserInfo) error {
			return json.NewEncoder(w).Encode(getServerStatusHandler.GetServerStatus())
		}).AllowAnonymous()).Methods("GET")
	} else {
		router.Handle("/status", AppHandler(func(w http.ResponseWriter, r *http.Request, userInfo *UserInfo) error {
			return json.NewEncoder(w).Encode(NewServerStatus())
		}).AllowAnonymous()).Methods("GET")
	}

	if getHTTPHandler, ok := s.obj.(ConfigureRouterHandler); ok {
		handler, err = getHTTPHandler.ConfigureRouter(router)
		if err != nil {
			if s.logger != nil {
				s.logger.WithError(err).Errorf("unable to start service")
			}
			return
		}
		if handler == nil {
			if s.logger != nil {
				s.logger.Fatal("invalid handler retured in ConfigureRouter()")
			} else {
				panic("invalid handler retured in ConfigureRouter()")
			}
		}

	} else {
		handler = router
	}

	// Prometheus metrics
	if s.enablePrometheusMetrics {
		router.Handle("/metrics", promhttp.Handler()).Methods("GET")
	}

	if s.corsOptions != nil {
		c := cors.New(*s.corsOptions)
		handler = c.Handler(handler)
	}

	// Add logger
	if s.logger != nil {
		handler = NewLoggingMiddleware(s.logger).Middleware(handler)
	}

	// Authorization
	if s.authorizationOptions != nil {
		authMw := newAuthorizationMiddleware(s.authorizationOptions, s.logger)
		handler = authMw.Middleware(handler)
		err = authMw.Validate()
		if err != nil {
			if s.logger != nil {
				s.logger.WithError(err).Errorf("unable to validate authorization settings")
			}
			return
		}
	}

	srv := &http.Server{
		Addr: s.listenAddress,
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: s.writeTimeout,
		ReadTimeout:  s.readTimeout,
		IdleTimeout:  s.idleTimeout,
		Handler:      handler,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				if s.logger != nil {
					s.logger.Fatal(err)
				} else {
					panic(err)
				}
			}
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	if s.logger != nil {
		s.logger.WithField("addr", srv.Addr).Print("Service is ready for requests")
	}

	// Block until we receive our signal.
	<-c

	if s.logger != nil {
		s.logger.Print("Received request for shutdown")
	}

	if beforeEnd, ok := s.obj.(WebServiceBeforeEndHandler); ok {
		beforeEnd.BeforeEnd()
	}

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.

	if s.logger != nil {
		s.logger.Println("Shutting down")
	}

	os.Exit(0)
	return
}

// Set timemouts
func (s *webservice) SetTimeouts(writeTimeout time.Duration, readTimeout time.Duration, idleTimeout time.Duration) {

	if writeTimeout > 0 {
		s.writeTimeout = writeTimeout
	}
	if readTimeout > 0 {
		s.readTimeout = readTimeout
	}
	if idleTimeout > 0 {
		s.idleTimeout = idleTimeout
	}
}

// Set listen address - default value is ":8080"
func (s *webservice) SetListenAddress(listenAddress string) {
	s.listenAddress = listenAddress
}

// Enable CORS
func (s *webservice) EnableCors(options *cors.Options) {
	s.corsOptions = options
}

// Strip path  - e.g. if path is /my/root/path and request comes over https://mydomain.com/my/root/path/foo - it will be routed to /foo
func (s *webservice) StripPath(path string) {
	s.stripPath = path
}

// Configure logger
func (s *webservice) SetLogger(logger *logrus.Logger) {
	s.logger = logger
}

// Enable prometheus metrics over GET /metrics
func (s *webservice) EnablePrometheusMetrics(enable bool) {
	s.enablePrometheusMetrics = enable
}

// Enable authorization - for more details check authorization.Options struct
func (s *webservice) EnableAuthorization(options *AuthorizationOptions) {
	s.authorizationOptions = options
}
