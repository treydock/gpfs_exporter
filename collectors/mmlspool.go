// Copyright 2020 Trey Dockendorf
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package collectors

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	poolFilesystems = kingpin.Flag("collector.mmlspool.filesystems", "Filesystems to query with mmlspool, comma separated. Defaults to all filesystems.").Default("").String()
	mmlspoolTimeout = kingpin.Flag("collector.mmlspool.timeout", "Timeout for mmlspool execution").Default("30").Int()
	MmlspoolExec    = mmlspool
)

type PoolMetric struct {
	FS              string
	PoolName        string
	PoolTotal       float64
	PoolFree        float64
	PoolFreePercent float64
	Meta            bool
	MetaTotal       float64
	MetaFree        float64
	MetaFreePercent float64
}

type MmlspoolCollector struct {
	PoolTotal       *prometheus.Desc
	PoolFree        *prometheus.Desc
	PoolFreePercent *prometheus.Desc
	MetaTotal       *prometheus.Desc
	MetaFree        *prometheus.Desc
	MetaFreePercent *prometheus.Desc
	logger          *slog.Logger
}

func init() {
	registerCollector("mmlspool", false, NewMmlspoolCollector)
}

func NewMmlspoolCollector(logger *slog.Logger) Collector {
	return &MmlspoolCollector{
		PoolTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "pool", "total_bytes"),
			"GPFS pool total size in bytes", []string{"fs", "pool"}, nil),
		PoolFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "pool", "free_bytes"),
			"GPFS pool free size in bytes", []string{"fs", "pool"}, nil),
		PoolFreePercent: prometheus.NewDesc(prometheus.BuildFQName(namespace, "pool", "free_percent"),
			"GPFS pool free percent", []string{"fs", "pool"}, nil),
		MetaTotal: prometheus.NewDesc(prometheus.BuildFQName(namespace, "pool", "metadata_total_bytes"),
			"GPFS pool total metadata in bytes", []string{"fs", "pool"}, nil),
		MetaFree: prometheus.NewDesc(prometheus.BuildFQName(namespace, "pool", "metadata_free_bytes"),
			"GPFS pool free metadata in bytes", []string{"fs", "pool"}, nil),
		MetaFreePercent: prometheus.NewDesc(prometheus.BuildFQName(namespace, "pool", "metadata_free_percent"),
			"GPFS pool free percent", []string{"fs", "pool"}, nil),
		logger: logger,
	}
}

func (c *MmlspoolCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.PoolTotal
	ch <- c.PoolFree
	ch <- c.PoolFreePercent
	ch <- c.MetaTotal
	ch <- c.MetaFree
	ch <- c.MetaFreePercent
}

func (c *MmlspoolCollector) Collect(ch chan<- prometheus.Metric) {
	wg := &sync.WaitGroup{}
	var filesystems []string
	if *poolFilesystems == "" {
		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmlsfsTimeout)*time.Second)
		defer cancel()
		var mmlsfsTimeout float64
		var mmlsfsError float64
		mmlfsfs_filesystems, err := mmlfsfsFilesystems(ctx, c.logger)
		if err == context.DeadlineExceeded {
			mmlsfsTimeout = 1
			c.logger.Error("Timeout executing mmlsfs")
		} else if err != nil {
			mmlsfsError = 1
			c.logger.Error("Error collecting metrics", "err", err)
		}
		ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, mmlsfsTimeout, "mmlspool-mmlsfs")
		ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, mmlsfsError, "mmlspool-mmlsfs")
		filesystems = mmlfsfs_filesystems
	} else {
		filesystems = strings.Split(*poolFilesystems, ",")
	}
	for _, fs := range filesystems {
		c.logger.Debug("Collecting mmlspool metrics", "fs", fs)
		wg.Add(1)
		collectTime := time.Now()
		go func(fs string) {
			defer wg.Done()
			label := fmt.Sprintf("mmlspool-%s", fs)
			timeout := 0
			errorMetric := 0
			metrics, err := c.mmlspoolCollect(fs)
			if err == context.DeadlineExceeded {
				c.logger.Error(fmt.Sprintf("Timeout executing %s", label))
				timeout = 1
			} else if err != nil {
				c.logger.Error("Error collecting metrics", "err", err, "fs", fs)
				errorMetric = 1
			}
			ch <- prometheus.MustNewConstMetric(collectError, prometheus.GaugeValue, float64(errorMetric), label)
			ch <- prometheus.MustNewConstMetric(collecTimeout, prometheus.GaugeValue, float64(timeout), label)
			ch <- prometheus.MustNewConstMetric(collectDuration, prometheus.GaugeValue, time.Since(collectTime).Seconds(), label)
			if err == nil {
				for _, pool := range metrics {
					ch <- prometheus.MustNewConstMetric(c.PoolTotal, prometheus.GaugeValue, pool.PoolTotal, fs, pool.PoolName)
					ch <- prometheus.MustNewConstMetric(c.PoolFree, prometheus.GaugeValue, pool.PoolFree, fs, pool.PoolName)
					ch <- prometheus.MustNewConstMetric(c.PoolFreePercent, prometheus.GaugeValue, pool.PoolFreePercent, fs, pool.PoolName)
					if pool.Meta {
						ch <- prometheus.MustNewConstMetric(c.MetaTotal, prometheus.GaugeValue, pool.MetaTotal, fs, pool.PoolName)
						ch <- prometheus.MustNewConstMetric(c.MetaFree, prometheus.GaugeValue, pool.MetaFree, fs, pool.PoolName)
						ch <- prometheus.MustNewConstMetric(c.MetaFreePercent, prometheus.GaugeValue, pool.MetaFreePercent, fs, pool.PoolName)
					}
				}
			}
			ch <- prometheus.MustNewConstMetric(lastExecution, prometheus.GaugeValue, float64(time.Now().Unix()), label)
		}(fs)
	}
	wg.Wait()
}

func (c *MmlspoolCollector) mmlspoolCollect(fs string) ([]PoolMetric, error) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*mmlspoolTimeout)*time.Second)
	defer cancel()
	out, err := MmlspoolExec(fs, ctx)
	if err != nil {
		return nil, err
	}
	metrics, err := parse_mmlspool(fs, out, c.logger)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func mmlspool(fs string, ctx context.Context) (string, error) {
	cmd := execCommand(ctx, *sudoCmd, "/usr/lpp/mmfs/bin/mmlspool", fs)
	var out bytes.Buffer
	cmd.Stdout = &out
	err := cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", ctx.Err()
	} else if err != nil {
		return "", err
	}
	return out.String(), nil
}

func parse_mmlspool(fs string, out string, logger *slog.Logger) ([]PoolMetric, error) {
	pools := []PoolMetric{}
	headers := []string{}
	lines := strings.Split(out, "\n")
	headerParsed := false
	for _, l := range lines {
		// Header parsing must use the original output line
		rawItems := strings.Fields(l)
		if len(rawItems) == 0 {
			continue
		}
		if rawItems[0] == "Name" {
			headers = parse_mmlspool_headers(rawItems)
			headerParsed = true
			logger.Debug("headers", "headers", fmt.Sprintf("%v", headers), "line", l)
			continue
		}
		// Skip lines that don't look like data rows
		if len(rawItems) < 2 {
			continue
		}
		// If headers haven't been parsed yet, skip
		if !headerParsed {
			continue
		}

		// Replace beginning of percent ( N%)
		line := strings.Replace(l, "(", "", -1)
		// Replace percent N %) with just N
		line = strings.Replace(line, "%)", "", -1)
		// Replace '8 MB' with just '8'
		line = strings.Replace(line, " MB", "", -1)
		// Replace '1024 KB' with just '1024' - for GPFS 5.x compatibility
		line = strings.Replace(line, " KB", "", -1)
		items := strings.Fields(line)
		logger.Debug("items", "items", fmt.Sprintf("%v", items), "line", line)
		// Check the item len is same as header
		if len(items) < len(headers) {
			return nil, fmt.Errorf("mmlspool output column mismatch")
		}

		pool := PoolMetric{
			FS: fs,
		}
		for i, item := range items {
			if i >= len(headers) {
				continue
			}
			field := headers[i]
			switch field {
			case "Name":
				pool.PoolName = item
			case "Meta":
				if item == "yes" {
					pool.Meta = true
				}
			case "TotalData":
				poolTotal, err := ParseFloat(item, false, logger)
				if err != nil {
					return nil, err
				}
				pool.PoolTotal = poolTotal * 1024
			case "FreeData":
				poolFree, err := ParseFloat(item, false, logger)
				if err != nil {
					return nil, err
				}
				pool.PoolFree = poolFree * 1024
			case "FreeDataPercent":
				poolFreePercent, err := ParseFloat(item, false, logger)
				if err != nil {
					return nil, err
				}
				pool.PoolFreePercent = poolFreePercent
			case "TotalMeta":
				metaTotal, err := ParseFloat(item, false, logger)
				if err != nil {
					return nil, err
				}
				pool.MetaTotal = metaTotal * 1024
			case "FreeMeta":
				metaFree, err := ParseFloat(item, false, logger)
				if err != nil {
					return nil, err
				}
				pool.MetaFree = metaFree * 1024
			case "FreeMetaPercent":
				metaFreePercent, err := ParseFloat(item, false, logger)
				if err != nil {
					return nil, err
				}
				pool.MetaFreePercent = metaFreePercent
			}
		}
		pools = append(pools, pool)
	}
	// If headers were parsed but no pools were found, return error
	if headerParsed && len(pools) == 0 {
		return nil, fmt.Errorf("no valid pool data found")
	}
	return pools, nil
}

func parse_mmlspool_headers(items []string) []string {
	skip := 0
	headers := []string{}
	for i := 0; i < len(items); i++ {
		if skip > 0 {
			skip--
			continue
		}
		item := items[i]
		if item == "Total" && items[i+1] == "Data" {
			item = "TotalData"
			skip = 3
		}
		if item == "Free" && items[i+1] == "Data" {
			item = "FreeData"
			skip = 3
		}
		if item == "Total" && items[i+1] == "Meta" {
			item = "TotalMeta"
			skip = 3
		}
		if item == "Free" && items[i+1] == "Meta" {
			item = "FreeMeta"
			skip = 3
		}
		headers = append(headers, item)
		if strings.HasPrefix(item, "Free") {
			percentItem := fmt.Sprintf("%sPercent", item)
			headers = append(headers, percentItem)
		}
	}
	return headers
}
