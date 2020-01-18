package main

import (
    "encoding/json"
	"net/http"

    "github.com/treydock/gpfs_exporter/collector"
    "github.com/treydock/gpfs_exporter/config"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	listenAddr    = kingpin.Flag("listen", "Address on which to expose metrics.").Default(":9303").String()
    configPath    = kingpin.Flag("config", "Path to config").Default("").String()
    configTargets = &config.Targets{}
    defaultCollectors  = []string{"mmpmon","mount"}
)

func gpfsHandler() http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        registry := prometheus.NewRegistry()

        target := r.URL.Query().Get("target")
        if target == "" {
            target = "default"
        }   

        configTarget, err := configTargets.GetTarget(target)
        if err != nil {
            log.Error(err.Error())
            http.Error(w, err.Error(), 404)
            return
        }

        if configTarget.Collectors == nil {
            configTarget.Collectors = defaultCollectors
        }

        jsonTarget , _ := json.Marshal(configTarget)
        log.Debugln("Target config:", string(jsonTarget))
        collector := collector.New(configTarget)
        registry.MustRegister(collector)

        gatherers := prometheus.Gatherers{
            prometheus.DefaultGatherer,
            registry,
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

    if *configPath != "" {
        if err := configTargets.LoadConfig(*configPath) ; err != nil {
            log.Fatalf("Error parsing config file %s: %s", *configPath, err.Error())
        }
    }

	log.Infoln("Starting gpfs_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())
	log.Infof("Starting Server: %s", *listenAddr)

	http.Handle("/metrics", promhttp.Handler())
    http.Handle("/gpfs", gpfsHandler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
             <head><title>GPFS Exporter</title></head>
             <body>
             <h1>Metrics Exporter</h1>
             <p><a href='/metrics'>Metrics</a></p>
             <h1>GPFS Exporter</h1>
             <p><a href='/gpfs'>GPFS Metrics</a></p>
             </body>
             </html>`))
	})
	log.Fatal(http.ListenAndServe(*listenAddr, nil))
}
