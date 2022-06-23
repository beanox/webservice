package main

import (
	"os"
	"strings"

	"github.com/beanox/webservice"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

func main() {

	logger := logrus.New()

	// Set default values
	viper.SetDefault("log_level", "debug")
	viper.SetDefault("listen_address", ":8080")
	viper.SetDefault("cors.enabled", false)
	viper.SetDefault("authorization.jwks", "https://www.googleapis.com/oauth2/v3/certs")

	viper.SetConfigName("config") // name of the config file
	viper.AddConfigPath(".")      // Path where to search for config file
	viper.AutomaticEnv()          // merge environment variables into config

	// define command line parameters
	pflag.String("log_level", "warning", "Log level")
	pflag.String("listen_address", ":8080", "Listen address")
	pflag.String("strip_path", "/", "Strip path from requests (e.g. /api -> http://my.server.com/api/request -> /request)")

	// Init viper and read config
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	pflag.Parse()
	viper.BindPFlags(pflag.CommandLine)
	err := viper.ReadInConfig()

	logFormat := viper.GetString("log_format")
	if logFormat != "" {
		if logFormat == "json" {
			logger.SetFormatter(&logrus.JSONFormatter{})
		} else if logFormat == "color" {
			logger.SetFormatter(&logrus.TextFormatter{ForceColors: true})
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

			mergeErr := webservice.MergeEnvJsonInConfig(configName, configName[len(jsonMergePrefix):])
			if mergeErr != nil {
				logger.WithError(mergeErr).WithField("var", configName).Warn("error merging env variable in config")
			}
		}
	}

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

	logLevel, _ := logrus.ParseLevel(viper.GetString("log_level"))
	logger.WithField("log_level", logLevel).Print("Log level set")
	logger.SetLevel(logLevel)

	svc := webservice.New(&service{})
	svc.SetLogger(logrus.StandardLogger())
	logrus.SetLevel(logrus.TraceLevel)

	// Configure web service
	svc.SetListenAddress(viper.GetString("listen_address"))

	svc.EnableCors(webservice.CorsOptionsFromViper("cors."))
	svc.StripPath(viper.GetString("strip_path"))
	svc.SetLogger(logger)
	svc.EnablePrometheusMetrics(!viper.GetBool("disable_prometheus_metrics"))
	svc.EnableAuthorization(webservice.AuthorizationOptionsFromViper("authorization."))

	// Start web service
	svc.Start()
}
