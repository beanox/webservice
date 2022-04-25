package webservice

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/viper"
)

func mergeEnvJsonInConfig(envName string, configName string) (err error) {
	if envName == configName {
		err = fmt.Errorf("environment name is not allowed to be the same as configuration name")
		return
	}
	dbConfig, ok := os.LookupEnv(envName)
	if ok {
		cfg := make(map[string]interface{})
		err = json.Unmarshal([]byte(dbConfig), &cfg)
		if err == nil {
			if configName == "" {
				viper.MergeConfigMap(cfg)
			} else {
				viper.MergeConfigMap(map[string]interface{}{
					configName: cfg,
				})
			}
		}
	}
	return
}
