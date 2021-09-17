package webservice

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gorilla/mux"
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

// SimpleServiceGetHTTPHandler ...
type SimpleServiceGetHTTPHandler interface {
	GetHTTPHandler() (handler http.Handler, err error)
}

// SimpleServicePreparePFlags ...
type SimpleServicePreparePFlags interface {
	PreparePFlags() (err error)
}

// SimpleServiceBeforeStart ...
type SimpleServiceBeforeStart interface {
	BeforeStart() (err error)
}

// SimpleServiceBase ...
type SimpleServiceBase struct {
	obj          SimpleServiceObject
	writeTimeout time.Duration
	readTimeout  time.Duration
	idleTimeout  time.Duration
}

var logger *logrus.Entry

// Start starts service
func (s *SimpleServiceBase) Start() (err error) {

	viper.SetDefault("log_level", "warning")
	viper.SetDefault("listen_address", ":8080")

	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.AutomaticEnv()

	pflag.String("log_level", "warning", "Log level")
	pflag.String("listen_address", ":8080", "Listen address")

	if preparePFlags, ok := s.obj.(SimpleServicePreparePFlags); ok {
		err = preparePFlags.PreparePFlags()
		if err != nil {
			return
		}
	}

	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)

	logFormat, logFormatDefined := os.LookupEnv("LOG_FORMAT")
	if logFormatDefined {
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

	logger = logrus.WithField("facility", "microservice")

	err = viper.ReadInConfig()

	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			logger.WithError(err).Print("No config file is loaded. Using all default values")
			err = nil
		} else {
			logger.WithError(err).Error("Unable to load config")
			return
		}
	} else {
		logger.WithField("config_file", viper.ConfigFileUsed()).Printf("Using config file")
	}

	logLevel, err := logrus.ParseLevel(viper.GetString("log_level"))
	logger.WithField("log_level", logLevel).Print("Log level set")
	logrus.SetLevel(logLevel)
	logger.Logger.SetLevel(logLevel)

	if beforeStart, ok := s.obj.(SimpleServiceBeforeStart); ok {
		err = beforeStart.BeforeStart()
		if err != nil {
			return
		}
	}

	var handler http.Handler

	if getHTTPHandler, ok := s.obj.(SimpleServiceGetHTTPHandler); ok {
		handler, err = getHTTPHandler.GetHTTPHandler()
		if err != nil {
			return
		}
	} else {
		handler = mux.NewRouter()
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
			logger.Fatal(err)
		}
	}()

	c := make(chan os.Signal, 1)
	// We'll accept graceful shutdowns when quit via SIGINT (Ctrl+C)
	// SIGKILL, SIGQUIT or SIGTERM (Ctrl+/) will not be caught.
	signal.Notify(c, os.Interrupt)

	logger.WithField("addr", srv.Addr).Print("Service is ready for requests")
	// Block until we receive our signal.
	<-c

	logger.Print("Received request for shutdown")

	// Create a deadline to wait for.
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()
	// Doesn't block if no connections, but will otherwise wait
	// until the timeout deadline.
	srv.Shutdown(ctx)
	// Optionally, you could run srv.Shutdown in a goroutine and block on
	// <-ctx.Done() if your application should wait for other services
	// to finalize based on context cancellation.
	logger.Println("Shutting down")
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
