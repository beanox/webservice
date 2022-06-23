package main

import (
	"strings"

	"github.com/beanox/webservice"

	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type inputParamService struct {
}

func main() {

	viper.SetConfigName("config") // name of the config file
	viper.AddConfigPath(".")      // Path where to search for config file
	viper.AutomaticEnv()          // merge environment variables into config

	// define input parameter
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// set default value from config / env variable
	viper.SetDefault("sap.werk", "1000") // Environment varaible is SAP_WERK

	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}

	svc := webservice.New(&inputParamService{})

	svc.Start()
}

func (s *inputParamService) BeforeStart() (err error) {

	sapWerk := viper.GetString("sap.werk")
	logrus.Warnf("SAP-werk : %v", sapWerk)

	// Read data from config.yaml
	sapUser := viper.GetString("sap.user")
	sapPassword := viper.GetString("sap.password")

	if sapUser == "" || sapPassword == "" {
		logrus.Fatal("sap user/password is not configured")
	}

	logrus.Warnf("SAP User : %v/%v", sapUser, sapPassword)

	return nil
}
