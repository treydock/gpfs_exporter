package collectors

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
)

var (
	mmhealthMap = map[string]string{
		"component":  "Component",
		"entityname": "EntityName",
		"entitytype": "EntityType",
		"status":     "Status",
	}
)

type HealthMetric struct {
	Component  string
	EntityName string
	EntityType string
	Status     string
}

type MmhealthCollector struct {
	State *prometheus.Desc
}

func init() {
	registerCollector("mmhealth", false, NewMmhealthCollector)
}

func NewMmhealthCollector() Collector {
	return &MmhealthCollector{
		State: prometheus.NewDesc(prometheus.BuildFQName(namespace, "health", "status"),
			"GPFS health status, 1=healthy 0=not healthy", []string{"component", "entityname", "entitytype", "status"}, nil),
	}
}

func (c *MmhealthCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.State
}

func (c *MmhealthCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting mmhealth metrics")
	err := c.collect(ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmhealth")
	} else {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "mmhealth")
	}
}

func (c *MmhealthCollector) collect(ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
	mmhealth_out, err := mmhealth()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmhealth")
		return err
	}
	metrics, err := mmhealth_parse(mmhealth_out)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmhealth")
		return err
	}
	for _, m := range metrics {
		statusValue := parseMmhealthStatus(m.Status)
		ch <- prometheus.MustNewConstMetric(c.State, prometheus.GaugeValue, statusValue, m.Component, m.EntityName, m.EntityType, m.Status)
	}
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmhealth")
	return nil
}

func mmhealth() (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmhealth", "node", "show", "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func mmhealth_parse(out string) ([]HealthMetric, error) {
	var metrics []HealthMetric
	lines := strings.Split(out, "\n")
	var headers []string
	for _, l := range lines {
		if !strings.HasPrefix(l, "mmhealth") {
			continue
		}
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
		if items[1] != "State" {
			continue
		}
		var values []string
		if items[2] == "HEADER" {
			for _, i := range items {
				headers = append(headers, i)
			}
			continue
		} else {
			for _, i := range items {
				values = append(values, i)
			}
		}
		var metric HealthMetric
		ps := reflect.ValueOf(&metric) // pointer to struct - addressable
		s := ps.Elem()                 // struct
		for i, h := range headers {
			if field, ok := mmhealthMap[h]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(values[i])
				} else if f.Kind() == reflect.Int64 {
					if val, err := strconv.ParseInt(values[i], 10, 64); err == nil {
						f.SetInt(val)
					} else {
						log.Errorf("Error parsing %s value %s: %s", h, values[i], err.Error())
					}
				}
			}
		}
		metrics = append(metrics, metric)
	}
	return metrics, nil
}

func parseMmhealthStatus(status string) float64 {
	if bytes.Equal([]byte(status), []byte("HEALTHY")) {
		return 1
	} else {
		return 0
	}
}
