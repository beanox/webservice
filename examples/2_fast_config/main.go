package main

import (
	"github.com/beanox/webservice"
	"github.com/spf13/viper"
)

func main() {

	// Some default settings. Everything could be replaced with config file or environment variable
	viper.SetDefault("authorization.jwks", "https://www.googleapis.com/oauth2/v3/certs")
	viper.SetDefault("cors.enabled", "true")
	viper.SetDefault("log_format", "color")
	viper.SetDefault("log_level", "trace")

	svc := webservice.New(&service{})

	// fast configuration:
	// - configure CORS (origins, headers, methods) but will not enable it
	// - set config name to config and set path to current path (examle ./config.yaml or ./config.json)
	// - enable configuration over environment variable (cors.enabled -> CORS_ENABLED, etc...)
	// - add command line parameters: log_level and listen_address
	// - create logger and use log_format=json|color to set valid format
	// - convert all JSON_VAR_*** variable into configuration - E.g. JSON_VAR_DB={USER:MyUser, PASS:MyPass} -> db.user=MyUser; db.pass=MyPass
	// - configure valid log level
	// - configure strip path
	// - enable prometheus metrics if disable_prometheus_metrics is not set
	// - enable authorization based on ENV variables (autorization.jwks, autorization.disabled, autorization.scope, ...)
	webservice.FastConfig(svc)

	// Start service
	svc.Start()
}
