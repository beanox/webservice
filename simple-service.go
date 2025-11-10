package webservice

import (
	"context"
	"encoding/json"
	"fmt"
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

var (
	Version   string
	BuildTime string
	CommitSHA string
)

// WebService ...
type WebService interface {
	Start() (err error) // Start service
}

// WebServiceObject ...
type WebServiceObject interface {
}

// BuildInfo represents build information for the service
type BuildInfo struct {
	Name          string `json:"name,omitempty"`
	Version       string `json:"version,omitempty"`
	BuildTime     string `json:"build_time,omitempty"`
	CommitSHA     string `json:"commit_sha,omitempty"`
	PreferLdflags bool   `json:"-"`
}

type Timeouts struct {
	WriteTimeout time.Duration
	ReadTimeout  time.Duration
	IdleTimeout  time.Duration
}

// Config holds all configuration for WebService
type Config struct {
	timeouts  Timeouts
	buildInfo BuildInfo
}

// WebServiceOption defines a function type for setting options
type WebServiceOption func(*Config)

// WithWriteTimeout sets the write timeout
func WithWriteTimeout(timeout time.Duration) WebServiceOption {
	return func(c *Config) {
		c.timeouts.WriteTimeout = timeout
	}
}

// WithReadTimeout sets the read timeout
func WithReadTimeout(timeout time.Duration) WebServiceOption {
	return func(c *Config) {
		c.timeouts.ReadTimeout = timeout
	}
}

// WithIdleTimeout sets the idle timeout
func WithIdleTimeout(timeout time.Duration) WebServiceOption {
	return func(c *Config) {
		c.timeouts.IdleTimeout = timeout
	}
}

// WithTimeouts sets all timeout values at once
func WithTimeouts(write, read, idle time.Duration) WebServiceOption {
	return func(c *Config) {
		c.timeouts.WriteTimeout = write
		c.timeouts.ReadTimeout = read
		c.timeouts.IdleTimeout = idle
	}
}

// WithBuildInfo sets the build information
func WithBuildInfo(bi BuildInfo) WebServiceOption {
	return func(c *Config) {
		if bi.PreferLdflags {

			if c.buildInfo.Name == "" && bi.Name != "" {
				c.buildInfo.Name = bi.Name
			}
			if c.buildInfo.Version == "" && bi.Version != "" {
				c.buildInfo.Version = bi.Version
			}
			if c.buildInfo.BuildTime == "" && bi.BuildTime != "" {
				c.buildInfo.BuildTime = bi.BuildTime
			}
			if c.buildInfo.CommitSHA == "" && bi.CommitSHA != "" {
				c.buildInfo.CommitSHA = bi.CommitSHA
			}
		} else {

			if bi.Name != "" {
				c.buildInfo.Name = bi.Name
			}
			if bi.Version != "" {
				c.buildInfo.Version = bi.Version
			}
			if bi.BuildTime != "" {
				c.buildInfo.BuildTime = bi.BuildTime
			}
			if bi.CommitSHA != "" {
				c.buildInfo.CommitSHA = bi.CommitSHA
			}
		}
	}
}

// New creates new web service object
func New(obj WebServiceObject, options ...WebServiceOption) WebService {
	config := Config{
		timeouts: Timeouts{
			WriteTimeout: time.Second * 15, // default
			ReadTimeout:  time.Second * 15, // default
			IdleTimeout:  time.Second * 60, // default
		},
		buildInfo: BuildInfo{
			Version:   Version,
			BuildTime: BuildTime,
			CommitSHA: CommitSHA,
		},
	}

	// Apply all options
	for _, opt := range options {
		opt(&config)
	}

	return &WebServiceBase{
		obj:    obj,
		config: config,
	}
}

// WebServiceGetHTTPHandlerObsolete ...
type WebServiceGetHTTPHandlerObsolete interface {
	GetHTTPHandler() (handler http.Handler, err error)
}

// ConfigureRouterHandler ...
type ConfigureRouterHandler interface {
	ConfigureRouter(router *mux.Router) (handler http.Handler, err error)
}

// WebServicePreparePFlags ...
type WebServicePreparePFlags interface {
	PreparePFlags() (err error)
}

// WebServiceBeforeStart ...
type WebServiceBeforeStart interface {
	BeforeStart() (err error)
}

// WebServiceBeforeEnd ...
type WebServiceBeforeEnd interface {
	BeforeEnd()
}

// WebServiceGetStatusHandler ...
type WebServiceGetStatusHandler interface {
	GetServerStatus() (status interface{})
}

type WebServiceUserValidator interface {
	ValidateUser(userInfo *authorization.UserInfo) (valid bool)
}

// WebServiceBase ...
type WebServiceBase struct {
	obj    WebServiceObject
	config Config
}

// Start starts service
func (s *WebServiceBase) Start() (err error) {

	viper.SetDefault("log_level", "warning")
	viper.SetDefault("listen_address", ":8080")

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	pflag.String("log_level", "warning", "Log level")
	pflag.String("listen_address", ":8080", "Listen address")
	pflag.Bool("cors.enabled", false, "Enable cors")
	pflag.String("strip_path", "/", "Strip path from requests (e.g. /api -> http://my.server.com/api/request -> /request)")

	if s.config.buildInfo.Version != "" {
		pflag.Bool("version", false, "Show current version")
	}

	if preparePFlags, ok := s.obj.(WebServicePreparePFlags); ok {
		err = preparePFlags.PreparePFlags()
		if err != nil {
			return
		}
	}
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	err = viper.ReadInConfig()

	if viper.GetBool("version") {

		if s.config.buildInfo.Name != "" {
			fmt.Println(s.config.buildInfo.Name)
		}

		if s.config.buildInfo.Version != "" {
			fmt.Println("  Version    : " + s.config.buildInfo.Version)
		} else {
			fmt.Println("  Version    : (unknown)")
		}

		if s.config.buildInfo.BuildTime != "" {
			fmt.Println("  Build time : " + s.config.buildInfo.BuildTime)
		}

		if s.config.buildInfo.CommitSHA != "" {
			fmt.Println("  Commit SHA : " + s.config.buildInfo.CommitSHA)
		}

		return
	}

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

	if s.config.buildInfo.Version != "" {
		logrus.WithField("version", s.config.buildInfo.Version).Info("Version")
	}
	if s.config.buildInfo.BuildTime != "" {
		logrus.WithField("build_time", s.config.buildInfo.BuildTime).Debug("Build time")
	}
	if s.config.buildInfo.CommitSHA != "" {
		logrus.WithField("commit_sha", s.config.buildInfo.CommitSHA).Debug("Commit SHA")
	}

	if beforeStart, ok := s.obj.(WebServiceBeforeStart); ok {
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

	if _, ok := s.obj.(WebServiceGetHTTPHandlerObsolete); ok {
		logrus.Fatal("using obsolete version of GetHTTPHandler - new definition is: ConfigureRouter(router *mux.Router) (handler http.Handler, err error) ")
	}

	if getServerStatusHandler, ok := s.obj.(WebServiceGetStatusHandler); ok {
		router.Handle("/status", AppHandler(func(w http.ResponseWriter, r *http.Request) error {
			return json.NewEncoder(w).Encode(getServerStatusHandler.GetServerStatus())
		})).Methods("GET")
	} else {
		router.Handle("/status", AppHandler(func(w http.ResponseWriter, r *http.Request) error {
			return json.NewEncoder(w).Encode(NewServerStatus(s.config.buildInfo))
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

		corsMaxAge := viper.GetInt("cors.max_age")

		c := cors.New(cors.Options{
			AllowedOrigins:   corsOrigins,
			AllowedHeaders:   corsHeaders,
			AllowedMethods:   corsMethods,
			AllowCredentials: true,
			Debug:            false,
			MaxAge:           corsMaxAge,
		})
		handler = c.Handler(handler)
	}

	// Add logger
	handler = logging.New(logrus.WithField("facility", "webservice")).Middleware(handler)

	// Authorization

	var userValidatorFunc func(userInfo *authorization.UserInfo) bool
	if userValidator, ok := s.obj.(WebServiceUserValidator); ok {
		userValidatorFunc = userValidator.ValidateUser
	}

	authMw := authorization.New(authorization.OptionsFromViper("authorization."), userValidatorFunc)
	handler = authMw.Middleware(handler)
	err = authMw.Validate()
	if err != nil {
		logrus.WithError(err).Errorf("unable to validate authorization settings")
		return
	}

	srv := &http.Server{
		Addr: viper.GetString("listen_address"),
		// Good practice to set timeouts to avoid Slowloris attacks.
		WriteTimeout: s.config.timeouts.WriteTimeout,
		ReadTimeout:  s.config.timeouts.ReadTimeout,
		IdleTimeout:  s.config.timeouts.IdleTimeout,
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

	if beforeEnd, ok := s.obj.(WebServiceBeforeEnd); ok {
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
