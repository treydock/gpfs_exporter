package main

import (
    "bytes"
	"net/http"
	"os/exec"
    "reflect"
	"strings"
    "strconv"
	"sync"

    linuxproc "github.com/c9s/goprocinfo/linux"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

const (
	namespace = "gpfs"
)

var (
	listenAddr    = kingpin.Flag("listen", "Address on which to expose metrics.").Default(":9303").String()
	fsMounts  = kingpin.Flag("fs.mounts", "Comma delimited list of filesystem paths that should be mounted").Default("").String()
	execCommand   = exec.Command
    procMounts    = "/proc/mounts"
    mmpmonMap = map[string]string{
            "_fs_": "FS",
            "_nn_": "NodeName",
            "_br_": "ReadBytes",
            "_bw_": "WriteBytes",
            "_rdc_": "Reads",
            "_wc_": "Writes",
            "_oc_": "Opens",
            "_cc_": "Closes",
            "_dir_": "ReadDir",
            "_iu_": "InodeUpdates",
        }
)

type Exporter struct {
	sync.Mutex
	fqdn                      string
    check_mounts              []string
	fs_mount_status           *prometheus.Desc
    read_bytes                *prometheus.Desc
    write_bytes               *prometheus.Desc
    operations           *prometheus.Desc
	scrapeFailures            prometheus.Counter
}

type PerfMetrics struct {
    FS          string
    NodeName    string
    ReadBytes   int64
    WriteBytes  int64
    Reads       int64
    Writes      int64
    Opens       int64
    Closes      int64
    ReadDir     int64
    InodeUpdates    int64
}

/*
type GPFSMount struct {
    Path    string
    Mounted bool
}
*/

func sliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if str == s {
			return true
		}
	}
	return false
}

func getGPFSMounts() ([]string, error) {
//func getGPFSMounts() ([]GPFSMount, error) {
    //var gpfsMounts []GPFSMount
    var gpfsMounts []string
    mounts, err := linuxproc.ReadMounts(procMounts)
    if err != nil {
        return nil, err
    }
    for _, mount := range mounts.Mounts {
        if mount.FSType != "gpfs" {
            continue
        }
        //gpfsMount := GPFSMount{Path: mount.MountPoint, Mounted: true}
        gpfsMount := mount.MountPoint
        gpfsMounts = append(gpfsMounts, gpfsMount)
    }
    return gpfsMounts, err
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
        if ! strings.HasPrefix(l, "_") {
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
        s := ps.Elem() // struct
        for i, h := range headers {
            if field, ok := mmpmonMap[h] ; ok {
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

/*
func mmpmon_parse(out string) ([]PerfMetrics, error) {
    var metrics []PerfMetrics
    lines := strings.Split(out, "\n")
    for _, l := range lines {
        if ! strings.HasPrefix(l, "_") {
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
        for i, h := range headers {
            switch h {
            case "_nn_":
                perf.NodeName = values[i]
            case "_fs_":
                perf.FS = values[i]
            case "_br_":
                if val, err := strconv.ParseInt(values[i], 10, 64); err == nil {
                    perf.ReadBytes = val
                } else {
                    log.Errorf("Error parsing _br_ value %s: %s", values[i], err.Error())
                }
            case "_rdc_":
                if val, err := strconv.ParseInt(values[i], 10, 64); err == nil {
                    perf.Reads = val
                } else {
                    log.Errorf("Error parsing _rdc_ value %s: %s", values[i], err.Error())
                }
            case "_bw_":
                if val, err := strconv.ParseInt(values[i], 10, 64); err == nil {
                    perf.WriteBytes = val
                } else {
                    log.Errorf("Error parsing _bw_ value %s: %s", values[i], err.Error())
                }
            case "_wc_":
                if val, err := strconv.ParseInt(values[i], 10, 64); err == nil {
                    perf.Writes = val
                } else {
                    log.Errorf("Error parsing _wc_ value %s: %s", values[i], err.Error())
                }
            }
        }
        metrics = append(metrics, perf)
    }
    return metrics, nil
}
*/

func NewExporter() *Exporter {
	return &Exporter{
		fs_mount_status: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "fs_mount_status"),
            "Status of GPFS filesystems, 1=mounted 0=not mounted",
            []string{"mount"},
            nil,
		),
		read_bytes: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "read_bytes"),
            "GPFS read bytes",
            []string{"fs","nodename"},
            nil,
        ),
		write_bytes: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "", "write_bytes"),
            "GPFS write bytes",
            []string{"fs","nodename"},
            nil,
        ),
        operations: prometheus.NewDesc(
            prometheus.BuildFQName(namespace, "perf", "operations"),
            "GPFS operations reported by mmpmon",
            []string{"fs","nodename","operation"},
            nil,
        ),
		scrapeFailures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "exporter_scrape_failures_total",
			Help:      "Number of errors while collecting metrics.",
		}),
	}
}

func (e *Exporter) collect(ch chan<- prometheus.Metric) error {
	log.Info("Collecting metrics")
    //ch <- prometheus.MustNewConstMetric(e.up, prometheus.GaugeValue, 0)
    gpfsMounts, err := getGPFSMounts()
    if err != nil {
        return nil
    }
    if len(e.check_mounts) == 1 && e.check_mounts[0] == "" {
        e.check_mounts = gpfsMounts
    }
    for _, mount := range e.check_mounts {
        if sliceContains(gpfsMounts, mount) {
            ch <- prometheus.MustNewConstMetric(e.fs_mount_status, prometheus.GaugeValue, 1, mount)
        } else {
            ch <- prometheus.MustNewConstMetric(e.fs_mount_status, prometheus.GaugeValue, 0, mount)
        }
    }

    mmpmon_out, err := mmpmon()
    if err != nil {
        return err
    }
    perfs, err := mmpmon_parse(mmpmon_out)
    if err != nil {
        return err
    }
    for _, perf := range perfs {
        ch <- prometheus.MustNewConstMetric(e.read_bytes, prometheus.CounterValue, float64(perf.ReadBytes), perf.FS, perf.NodeName)
        ch <- prometheus.MustNewConstMetric(e.write_bytes, prometheus.CounterValue, float64(perf.WriteBytes), perf.FS, perf.NodeName)
        ch <- prometheus.MustNewConstMetric(e.operations, prometheus.CounterValue, float64(perf.Reads), perf.FS, perf.NodeName, "reads")
        ch <- prometheus.MustNewConstMetric(e.operations, prometheus.CounterValue, float64(perf.Writes), perf.FS, perf.NodeName, "writes")
        ch <- prometheus.MustNewConstMetric(e.operations, prometheus.CounterValue, float64(perf.Opens), perf.FS, perf.NodeName, "opens")
        ch <- prometheus.MustNewConstMetric(e.operations, prometheus.CounterValue, float64(perf.Closes), perf.FS, perf.NodeName, "closes")
        ch <- prometheus.MustNewConstMetric(e.operations, prometheus.CounterValue, float64(perf.ReadDir), perf.FS, perf.NodeName, "read_dir")
        ch <- prometheus.MustNewConstMetric(e.operations, prometheus.CounterValue, float64(perf.InodeUpdates), perf.FS, perf.NodeName, "inode_updates")
    }
	/*puns, err := getActivePuns()
	if err != nil {
		return err
	}
	e.active_puns.Set(float64(len(puns)))
	e.puns = puns
	if err := e.getProcessMetrics(); err != nil {
		return err
	}
	if err := e.getApacheMetrics(); err != nil {
		return err
	}*/
	return nil
}

func (e *Exporter) Describe(ch chan<- *prometheus.Desc) {
    ch <- e.fs_mount_status
	ch <- e.read_bytes
    ch <- e.write_bytes
    ch <- e.operations
	e.scrapeFailures.Describe(ch)
}

func (e *Exporter) Collect(ch chan<- prometheus.Metric) {
	e.Lock() // To protect metrics from concurrent collects.
	defer e.Unlock()
	if err := e.collect(ch); err != nil {
		log.Errorf("Error scraping gpfs: %s", err)
		e.scrapeFailures.Inc()
		e.scrapeFailures.Collect(ch)
	}
	return
}

func main() {
	metricsEndpoint := "/metrics"
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gpfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting gpfs_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Starting Server: %s", *listenAddr)

	exporter := NewExporter()
    exporter.check_mounts = strings.Split(*fsMounts, ",")
	prometheus.MustRegister(exporter)
	prometheus.MustRegister(version.NewCollector("gpfs_exporter"))

	http.Handle(metricsEndpoint, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>GPFS Exporter</title></head>
             <body>
             <h1>Apache Exporter</h1>
             <p><a href='` + metricsEndpoint + `'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
