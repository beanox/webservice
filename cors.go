package webservice

import (
	"github.com/rs/cors"
	"github.com/spf13/viper"
)

func CorsOptionsFromViper(prefix string) (options *cors.Options) {

	if !viper.GetBool(prefix + "enabled") {
		return nil
	}

	options = &cors.Options{
		AllowedOrigins:   viper.GetStringSlice(prefix + "allowed_origins"),
		AllowedMethods:   viper.GetStringSlice(prefix + "allowed_methods"),
		AllowedHeaders:   viper.GetStringSlice(prefix + "allowed_headers"),
		AllowCredentials: true,
	}

	if len(options.AllowedMethods) == 0 {
		options.AllowedMethods = []string{"HEAD", "GET", "POST", "PUT", "DELETE"}
	}

	if len(options.AllowedOrigins) == 0 {
		options.AllowedOrigins = []string{"*"}
	}

	if len(options.AllowedHeaders) == 0 {
		options.AllowedHeaders = []string{"*"}
	}

	return
}
