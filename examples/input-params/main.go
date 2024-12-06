package main

import (
	"strings"

	"github.com/beanox/webservice"

	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

type inputParamService struct {
}

func main() {
	svc := New()
	svc.Start()
}

// New creates App instance
func New() webservice.WebService {
	return webservice.New(
		&inputParamService{},
		webservice.WithBuildInfo(webservice.BuildInfo{
			Version: "1.0.0",
		}),
	)
}

func (s *inputParamService) PreparePFlags() (err error) {
	// new input parameter
	pflag.Bool("enable_cors", false, "Enable cors for all domains")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	// set default value from config / env variable
	viper.SetDefault("sap.werk", "1000") // Environment variable is SAP_WERK
	return
}

func (s *inputParamService) BeforeStart() (err error) {

	sapWerk := viper.GetString("sap.werk")
	logrus.Warnf("SAP-werk : %v", sapWerk)

	// Read data from config.yaml
	sapUser := viper.GetString("sap.user")
	sapPassword := viper.GetString("sap.password")

	logrus.Warnf("SAP User : %v/%v", sapUser, sapPassword)

	return nil
}
