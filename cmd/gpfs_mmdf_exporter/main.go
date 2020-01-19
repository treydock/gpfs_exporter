package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"reflect"
	"strconv"
	"strings"
	//"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	execCommand = exec.Command
	output      = kingpin.Flag("output", "Path to node exporter collected file").Required().String()
    filesystem  = kingpin.Flag("filesystems", "Comma separated list of filesystems to query").Required().String()
    mappedSections = []string{"inode","fsTotal"}
    KbToBytes      = []string{"fsSize","freeBlocks"}
	dfMap       = map[string]string{
		"inode:usedInodes":      "InodesUsed",
		"inode:freeInodes":      "InodesFree",
		"inode:allocatedInodes": "InodesAllocated",
		"inode:maxInodes":       "InodesTotal",
        "fsTotal:fsSize": "FSTotal",
        "fsTotal:freeBlocks": "FSFree",
	}
)

type DFMetrics struct {
	InodesUsed      int64
	InodesFree      int64
	InodesAllocated int64
	InodesTotal     int64
    FSTotal          int64
    FSFree          int64
}

func GetMetrics(fs string) (DFMetrics, error) {
    out, err := mmdf(fs)
    if err != nil {
        return DFMetrics{}, err
    }
    dfMetric, err := parse_mmdf(out)
    if err != nil {
        return DFMetrics{}, err
    }
	return dfMetric, nil
}

func mmlsfs() (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmlsfs", "all", "-Y", "-T")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func parse_mmlsfs(out string) ([]string, error) {
	var filesystems []string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		items := strings.Split(line, ":")
		if len(items) < 7 {
			continue
		}
		if items[2] == "HEADER" {
			continue
		}
		filesystems = append(filesystems, items[6])
	}
	return filesystems, nil
}

func mmdf(fs string) (string, error) {
	cmd := execCommand("sudo", "/usr/lpp/mmfs/bin/mmdf", fs, "-Y")
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return out.String(), nil
}

func sliceContains(slice []string, str string) bool {
	for _, s := range slice {
		if str == s {
			return true
		}
	}
	return false
}

func parse_mmdf(out string) (DFMetrics, error) {
    var dfMetrics DFMetrics
	headers := make(map[string][]string)
	values := make(map[string][]string)
	lines := strings.Split(out, "\n")
	for _, l := range lines {
        if ! strings.HasPrefix(l, "mmdf") {
            continue
        }
		items := strings.Split(l, ":")
		if len(items) < 3 {
			continue
		}
        if ! sliceContains(mappedSections, items[1]) {
            continue
        }
		if items[2] == "HEADER" {
			for _, i := range items {
				headers[items[1]] = append(headers[items[1]], i)
			}
		} else {
			for _, i := range items {
				values[items[1]] = append(values[items[1]], i)
			}
		}
    }
    ps := reflect.ValueOf(&dfMetrics) // pointer to struct - addressable
	s := ps.Elem()                  // struct
	for k, vals := range headers {
		for i, v := range vals {
			mapKey := fmt.Sprintf("%s:%s", k, v)
            value := values[k][i]
			if field, ok := dfMap[mapKey]; ok {
				f := s.FieldByName(field)
				if f.Kind() == reflect.String {
					f.SetString(value)
				} else if f.Kind() == reflect.Int64 {
					if val, err := strconv.ParseInt(value, 10, 64); err == nil {
                        if sliceContains(KbToBytes, v) {
                            val = val * 1024
                        }
						f.SetInt(val)
					} else {
						log.Errorf("Error parsing %s value %s: %s", mapKey, value, err.Error())
					}
				}
			}
		}
	}
	return dfMetrics, nil
}

type Collector struct {
    fs               string
	inodes_used      *prometheus.Desc
	inodes_free      *prometheus.Desc
	inodes_allocated *prometheus.Desc
	inodes_total     *prometheus.Desc
	fs_total         *prometheus.Desc
    fs_free         *prometheus.Desc
	metadata         *prometheus.Desc
}

func NewCollector(fs string) *Collector {
	return &Collector{
        fs: fs,
		inodes_used:      prometheus.NewDesc("gpfs_fs_inodes_used", "GPFS filesystem inodes used", []string{"fs"}, nil),
		inodes_free:      prometheus.NewDesc("gpfs_fs_inodes_free", "GPFS filesystem inodes free", []string{"fs"}, nil),
		inodes_allocated: prometheus.NewDesc("gpfs_fs_inodes_allocated", "GPFS filesystem inodes allocated", []string{"fs"}, nil),
		inodes_total:     prometheus.NewDesc("gpfs_fs_inodes_total", "GPFS filesystem inodes total in bytes", []string{"fs"}, nil),
		fs_total:         prometheus.NewDesc("gpfs_fs_total", "GPFS filesystem total size in bytes", []string{"fs"}, nil),
		fs_free:         prometheus.NewDesc("gpfs_fs_free", "GPFS filesystem free size in bytes", []string{"fs"}, nil),
		metadata:         prometheus.NewDesc("gpfs_fs_metadata", "GPFS filesystem metadata", []string{"fs"}, nil),
	}
}

func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.inodes_used
	ch <- c.inodes_free
	ch <- c.inodes_allocated
	ch <- c.inodes_total
	ch <- c.fs_total
    ch <- c.fs_free
	ch <- c.metadata
}

func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	metrics, err := GetMetrics(c.fs)
	if err != nil {
		log.Fatal(err)
	}
	ch <- prometheus.MustNewConstMetric(c.inodes_used, prometheus.GaugeValue, float64(metrics.InodesUsed), c.fs)
	ch <- prometheus.MustNewConstMetric(c.inodes_free, prometheus.GaugeValue, float64(metrics.InodesFree), c.fs)
	ch <- prometheus.MustNewConstMetric(c.inodes_allocated, prometheus.GaugeValue, float64(metrics.InodesAllocated), c.fs)
	ch <- prometheus.MustNewConstMetric(c.inodes_total, prometheus.GaugeValue, float64(metrics.InodesTotal), c.fs)
	ch <- prometheus.MustNewConstMetric(c.fs_total, prometheus.GaugeValue, float64(metrics.FSTotal), c.fs)
	ch <- prometheus.MustNewConstMetric(c.fs_free, prometheus.GaugeValue, float64(metrics.FSFree), c.fs)
}

func collect(fs string) {
	registry := prometheus.NewRegistry()
	registry.MustRegister(NewCollector(fs))
    outputPath := fmt.Sprintf("%s_fs_%s", *output, fs)
	err := prometheus.WriteToTextfile(outputPath, registry)
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gpfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
    filesystems := strings.Split(*filesystem, ",")
    for _, fs := range filesystems {
	    collect(fs)
    }
}
