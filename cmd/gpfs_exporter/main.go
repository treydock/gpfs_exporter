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

package main

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/treydock/gpfs_exporter/collectors"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddr             = kingpin.Flag("web.listen-address", "Address to listen on for web interface and telemetry.").Default(":9303").String()
	disableExporterMetrics = kingpin.Flag("web.disable-exporter-metrics", "Exclude metrics about the exporter (promhttp_*, process_*, go_*)").Default("false").Bool()
)

func gpfsHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		registry := prometheus.NewRegistry()

		gpfsCollector := collectors.NewGPFSCollector()
		gpfsCollector.Lock()
		defer gpfsCollector.Unlock()
		for key, collector := range gpfsCollector.Collectors {
			log.Debugf("Enabled collector %s", key)
			registry.MustRegister(collector)
		}

		gatherers := prometheus.Gatherers{registry}
		if !*disableExporterMetrics {
			gatherers = append(gatherers, prometheus.DefaultGatherer)
		}

		// Delegate http serving to Prometheus client library, which will call collector.Collect.
		h := promhttp.HandlerFor(gatherers, promhttp.HandlerOpts{})
		h.ServeHTTP(w, r)
	}
}

func main() {
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("gpfs_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()

	log.Infoln("Starting gpfs_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Starting Server: %s", *listenAddr)

	http.Handle("/metrics", gpfsHandler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		//nolint:errcheck
		w.Write([]byte(`<html>
             <head><title>GPFS Exporter</title></head>
             <body>
             <h1>GPFS Metrics Exporter</h1>
             <p><a href='/metrics'>Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
