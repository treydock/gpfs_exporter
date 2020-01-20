package collectors

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/treydock/gpfs_exporter/config"
)

var (
	mmpmonMap = map[string]string{
		"_fs_":  "FS",
		"_nn_":  "NodeName",
		"_br_":  "ReadBytes",
		"_bw_":  "WriteBytes",
		"_rdc_": "Reads",
		"_wc_":  "Writes",
		"_oc_":  "Opens",
		"_cc_":  "Closes",
		"_dir_": "ReadDir",
		"_iu_":  "InodeUpdates",
	}
)

type PerfMetrics struct {
	FS           string
	NodeName     string
	ReadBytes    int64
	WriteBytes   int64
	Reads        int64
	Writes       int64
	Opens        int64
	Closes       int64
	ReadDir      int64
	InodeUpdates int64
}

type MmpmonCollector struct {
	target      config.Target
	read_bytes  *prometheus.Desc
	write_bytes *prometheus.Desc
	operations  *prometheus.Desc
}

func NewMmpmonCollector(target config.Target) *MmpmonCollector {
	return &MmpmonCollector{
		read_bytes: prometheus.NewDesc(prometheus.BuildFQName(namespace, "perf", "read_bytes"),
			"GPFS read bytes", []string{"fs", "nodename"}, nil),
		write_bytes: prometheus.NewDesc(prometheus.BuildFQName(namespace, "perf", "write_bytes"),
			"GPFS write bytes", []string{"fs", "nodename"}, nil),
		operations: prometheus.NewDesc(prometheus.BuildFQName(namespace, "perf", "operations"),
			"GPFS operationgs reported by mmpmon", []string{"fs", "nodename", "operation"}, nil),
	}
}

func (c *MmpmonCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.read_bytes
	ch <- c.write_bytes
	ch <- c.operations
}

func (c *MmpmonCollector) Collect(ch chan<- prometheus.Metric) {
	log.Debug("Collecting mmpmon metrics")
	err := c.collect(ch)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmpmon")
	} else {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 0, "mmpmon")
	}
}

func (c *MmpmonCollector) collect(ch chan<- prometheus.Metric) error {
	collectTime := time.Now()
	mmpmon_out, err := mmpmon()
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmpmon")
		return err
	}
	perfs, err := mmpmon_parse(mmpmon_out)
	if err != nil {
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, 1, "mmpmon")
		return err
	}
	for _, perf := range perfs {
		ch <- prometheus.MustNewConstMetric(c.read_bytes, prometheus.CounterValue, float64(perf.ReadBytes), perf.FS, perf.NodeName)
		ch <- prometheus.MustNewConstMetric(c.write_bytes, prometheus.CounterValue, float64(perf.WriteBytes), perf.FS, perf.NodeName)
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.Reads), perf.FS, perf.NodeName, "reads")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.Writes), perf.FS, perf.NodeName, "writes")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.Opens), perf.FS, perf.NodeName, "opens")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.Closes), perf.FS, perf.NodeName, "closes")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.ReadDir), perf.FS, perf.NodeName, "read_dir")
		ch <- prometheus.MustNewConstMetric(c.operations, prometheus.CounterValue, float64(perf.InodeUpdates), perf.FS, perf.NodeName, "inode_updates")
	}
	ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), "mmpmon")
	return nil
}

func mmpmon() (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmpmon", "-s", "-p")
	cmd.Stdin = strings.NewReader("fs_io_s\n")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func mmpmon_parse(out string) ([]PerfMetrics, error) {
	var metrics []PerfMetrics
	lines := strings.Split(out, "\n")
	for _, l := range lines {
		if !strings.HasPrefix(l, "_") {
			continue
		}
		var headers []string
		var values []string
		items := strings.Split(l, " ")
		for _, i := range items[1:] {
			if strings.HasPrefix(i, "_") {
				headers = append(headers, i)
			} else {
				values = append(values, i)
			}
		}
		var perf PerfMetrics
		ps := reflect.ValueOf(&perf) // pointer to struct - addressable
		s := ps.Elem()               // struct
		for i, h := range headers {
			if field, ok := mmpmonMap[h]; ok {
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
		metrics = append(metrics, perf)
	}
	return metrics, nil
}
