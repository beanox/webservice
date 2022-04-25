package webservice

import (
	"encoding/json"
	"fmt"
	"math"
	"os"

	"github.com/sirupsen/logrus"
)

// GelfFormatter ...
type GelfFormatter struct {
	host string
}

type fields map[string]interface{}

// Format formats the log entry to GELF JSON
func (f *GelfFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	data := make(fields, len(entry.Data)+6)
	blacklist := []string{"_id", "id", "timestamp", "version", "level", "host"}

	for k, v := range entry.Data {

		if contains(k, blacklist) {
			continue
		}

		switch v := v.(type) {
		case error:
			// Otherwise errors are ignored by `encoding/json`
			data["_"+k] = v.Error()
		default:
			data["_"+k] = v
		}
	}

	data["version"] = "1.1"
	data["host"] = f.host
	data["short_message"] = entry.Message
	data["timestamp"] = round((float64(entry.Time.UnixNano())/float64(1000000))/float64(1000), 4)
	data["level"] = uint32(entry.Level)
	data["level_name"] = entry.Level.String()
	data["_pid"] = os.Getpid()

	serialized, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal fields to JSON, %v", err)
	}

	return append(serialized, '\n'), nil
}

func contains(needle string, haystack []string) bool {
	for _, a := range haystack {
		if needle == a {
			return true
		}
	}
	return false
}

func round(val float64, places int) float64 {
	shift := math.Pow(10, float64(places))
	return math.Floor((val*shift)+.5) / shift
}
