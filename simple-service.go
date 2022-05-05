package webservice

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/beanox/webservice/authorization"
	"github.com/beanox/webservice/logging"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// SimpleService ...
type SimpleService interface {
	Start() (err error)                                                                           // Start service
	SetTimeouts(writeTimeout time.Duration, readTimeout time.Duration, idleTimeout time.Duration) // set timeouts. 0 will use default values. It must be called before start
}

// SimpleServiceObject ...
type SimpleServiceObject interface {
}

// NewSimpleService creates new simple service object
func NewSimpleService(obj SimpleServiceObject) SimpleService {
	return &SimpleServiceBase{
		obj:          obj,
		writeTimeout: time.Second * 15,
		readTimeout:  time.Second * 15,
		idleTimeout:  time.Second * 60,
	}
}

// SimpleServiceGetHTTPHandlerObsolete ...
type SimpleServiceGetHTTPHandlerObsolete interface {
	GetHTTPHandler() (handler http.Handler, err error)
}

// ConfigureRouterHandler ...
type ConfigureRouterHandler interface {
	ConfigureRouter(router *mux.Router) (handler http.Handler, err error)
}

// SimpleServicePreparePFlags ...
type SimpleServicePreparePFlags interface {
	PreparePFlags() (err error)
}

// SimpleServiceBeforeStart ...
type SimpleServiceBeforeStart interface {
	BeforeStart() (err error)
}

// SimpleServiceBeforeEnd ...
type SimpleServiceBeforeEnd interface {
	BeforeEnd()
}

// SimpleServiceGetStatusHandler ...
type SimpleServiceGetStatusHandler interface {
	GetServerStatus() (status interface{})
}

// SimpleServiceBase ...
type SimpleServiceBase struct {
	obj          SimpleServiceObject
	writeTimeout time.Duration
	readTimeout  time.Duration
	idleTimeout  time.Duration
}

// Start starts service
func (s *SimpleServiceBase) Start() (err error) {

	viper.SetDefault("log_level", "warning")
	viper.SetDefault("listen_address", ":8080")

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	pflag.String("log_level", "warning", "Log level")
	pflag.String("listen_address", ":8080", "Listen address")
	pflag.Bool("cors.enabled", false, "Enable cors")
	pflag.String("strip_path", "/", "Strip path from requests (e.g. /api -> http://my.server.com/api/request -> /request)")

	if preparePFlags, ok := s.obj.(SimpleServicePreparePFlags); ok {
		err = preparePFlags.PreparePFlags()
		if err != nil {
			return
		}
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	err = viper.ReadInConfig()

	logFormat := viper.GetString("log_format")
	if logFormat != "" {
		if logFormat == "json" {
			logrus.SetFormatter(&logrus.JSONFormatter{})
		} else if logFormat == "color" {
			logrus.SetFormatter(&logrus.TextFormatter{ForceColors: true})
		} else if logFormat == "gelf" {
			host, hasHost := os.LookupEnv("LOG_GELF_HOST")
			if !hasHost {
				host, _ = os.Hostname()
			}
			logrus.SetFormatter(&GelfFormatter{host: host})
		}
	}

	// Convert all environment variables with JSON_VAR_ prefix into configuration
	// E.g. JSON_VAR_={USER:MyUser, PASS:MyPass} -> db.user=MyUser; db.pass=MyPass
	const jsonMergePrefix = "JSON_VAR_"
	envVars := os.Environ()
	for _, envContent := range envVars {
		if strings.HasPrefix(envContent, jsonMergePrefix) && len(jsonMergePrefix) > 5 {
			variable := strings.Split(envContent, "=")
			configName := variable[0]

			mergeErr := mergeEnvJsonInConfig(configName, configName[len(jsonMergePrefix):])
			if mergeErr != nil {
				logrus.WithError(mergeErr).WithField("var", configName).Warn("error merging env variable in config")
			}
		}
	}

	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logrus.WithError(err).Print("No config file is loaded. Using all default values")
			err = nil
		} else {
			logrus.WithError(err).Error("Unable to load config")
			return
		}
	} else {
		logrus.WithField("config_file", viper.ConfigFileUsed()).Printf("Using config file")
	}

	logLevel, err := logrus.ParseLevel(viper.GetString("log_level"))
	logrus.WithField("log_level", logLevel).Print("Log level set")
	logrus.SetLevel(logLevel)

	if beforeStart, ok := s.obj.(SimpleServiceBeforeStart); ok {
		err = beforeStart.BeforeStart()
		if err != nil {
			return
		}
	}

	var handler http.Handler

	router := mux.NewRouter()
	stripPath := viper.GetString("strip_path")
	if stripPath != "" && stripPath != "/" {
		router = router.PathPrefix(stripPath).Subrouter()
	}

	if _, ok := s.obj.(SimpleServiceGetHTTPHandlerObsolete); ok {
		logrus.Fatal("using obsolete version of GetHTTPHandler - new definition is: ConfigureRouter(router *mux.Router) (handler http.Handler, err error) ")
	}

	if getServerStatusHandler, ok := s.obj.(SimpleServiceGetStatusHandler); ok {
		router.Handle("/status", AppHandler(func(w http.ResponseWriter, r *http.Request) error {
			return json.NewEncoder(w).Encode(getServerStatusHandler.GetServerStatus())
		})).Methods("GET")
	} else {
		router.Handle("/status", AppHandler(func(w http.ResponseWriter, r *http.Request) error {
			return json.NewEncoder(w).Encode(NewServerStatus())
		})).Methods("GET")
	}

	if getHTTPHandler, ok := s.obj.(ConfigureRouterHandler); ok {
		handler, err = getHTTPHandler.ConfigureRouter(router)
		if err != nil {
			logrus.WithError(err).Errorf("unable to start service")
			return
		}
	} else {
		handler = router
	}

	// Prometheus metrics
	disablePrometheus := viper.GetBool("disable_prometheus_metrics")
	if !disablePrometheus {
		router.Handle("/metrics", promhttp.Handler()).Methods("GET")
	}

	enableCors := viper.GetBool("cors.enabled")

	if enableCors {

		corsOrigins := viper.GetStringSlice("cors.allowed_origins")
		if len(corsOrigins) == 0 {
			corsOrigins = []string{"*"}
			logrus.Warnf("cors are enable for all domains")
		}

		corsHeaders := viper.GetStringSlice("cors.allowed_headers")
		if len(corsHeaders) == 0 {
			corsHeaders = []string{"*"}
		}

		corsMethods := viper.GetStringSlice("cors.allowed_methods")
		if len(corsMethods) == 0 {
			corsMethods = []string{"HEAD", "GET", "POST", "PUT", "DELETE"}
		}

		c := cors.New(cors.Options{
			AllowedOrigins:   corsOrigins,
			AllowedHeaders:   corsHeaders,
			AllowedMethods:   corsMethods,
			AllowCredentials: true,
			Debug:            false,
		})
		handler = c.Handler(handler)
	}

	// Add logger
	handler = logging.New(logrus.WithField("facility", "webservice")).Middleware(handler)

	// Authorization
	authMw := authorization.New(authorization.OptionsFromViper("authorization."))
	handler = authMw.Middleware(handler)
	err = authMw.Validate()
	if err != nil {
		logrus.WithError(err).Errorf("unable to validate authorization settings")
		return
	}

	srv := &http.Server{
		Addr: viper.GetString("listen_address"),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: s.writeTimeout,
		ReadTimeout:  s.readTimeout,
		IdleTimeout:  s.idleTimeout,
		Handler:      handler,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil {
			if err != http.ErrServerClosed {
				logrus.Fatal(err)
			}
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	logrus.WithField("addr", srv.Addr).Print("Service is ready for requests")
	// Block until we receive our signal.
	<-c

	logrus.Print("Received request for shutdown")

	if beforeEnd, ok := s.obj.(SimpleServiceBeforeEnd); ok {
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
	logrus.Println("Shutting down")
	os.Exit(0)
	return
}

func (s *SimpleServiceBase) SetTimeouts(writeTimeout time.Duration, readTimeout time.Duration, idleTimeout time.Duration) {

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
