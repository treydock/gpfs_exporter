# GPFS Prometheus exporter

[![Build Status](https://circleci.com/gh/treydock/gpfs_exporter/tree/master.svg?style=shield)](https://circleci.com/gh/treydock/gpfs_exporter)
[![GitHub release](https://img.shields.io/github/v/release/treydock/gpfs_exporter?include_prereleases&sort=semver)](https://github.com/treydock/gpfs_exporter/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/treydock/gpfs_exporter/total)
[![Go Report Card](https://goreportcard.com/badge/github.com/treydock/gpfs_exporter)](https://goreportcard.com/report/github.com/treydock/gpfs_exporter)
[![codecov](https://codecov.io/gh/treydock/gpfs_exporter/branch/master/graph/badge.svg)](https://codecov.io/gh/treydock/gpfs_exporter)

# GPFS Prometheus exporter

The GPFS exporter collects metrics from the GPFS filesystem.
The exporter supports the `/metrics` endpoint to gather GPFS metrics and metrics about the exporter.

## Collectors

Collectors are enabled or disabled via `--collector.<name>` and `--no-collector.<name>` flags.

Name | Description | Default
-----|-------------|--------
mmgetstate | Collect state via mmgetstate | Enabled
mmpmon| Collect metrics from `mmpmon` using `fs_io_s` | Enabled
mount | Check status of GPFS mounts. | Enabled
config | Collect configs via 'mmdiag --config' | Enabled
verbs | Test if GPFS is using verbs interface | Disabled
mmhealth | Test node health through `mmhealth` | Disabled
waiter | Collect waiters via 'mmdiag --waiters' | Disabled
mmdf | Collect filesystem space for inodes, block and metadata. | Disabled
mmces | Collect state of CES | Disabled
mmrepquota | Collect fileset quota information | Disabled
mmlssnapshot | Collect GPFS snapshot information | Disabled
mmlsfileset | Collect GPFS fileset information | Disabled
mmlsqos | Collect GPFS I/O performance values of a file system, when you enable Quality of Service | Disabled
mmlspool | Collect GPFS pool data | Disabled

### mount

The default behavior of the `mount` collector is to collect mount statuses on GPFS mounts in /proc/mounts or /etc/fstab. The `--collector.mount.mounts` flag can be used to adjust which mount points to check.

### mmhealth

The mmhealth statuses and events collected can be filtered with the following flags that all take a regex.

* `--collector.mmhealth.ignored-component` - The component regex to ignore.
* `--collector.mmhealth.ignored-entityname` - The entity name regex to ignore.
* `--collector.mmhealth.ignored-entitytype` - The entity type regex to ignore.
* `--collector.mmhealth.ignored-event` - The event regex to ignore.

### waiter

The waiter's seconds are stored in Histogram buckets defined by `--collector.waiter.buckets` which is a comma separated list of durations that are converted to seconds so `1s,5s,30s,1m` would have buckets of `[]float64{1,5,30,60}`.

The flag `--collector.waiter.exclude` defines a regular expression of waiter names to exclude.

The flag `--collector.waiter.log-reason` can enable logging of waiter reasons. The reason can produce very high cardinality so it is not included in metrics.

### mmdf

Due to the time it can take to execute mmdf that is an executable provided that can be used to collect mmdf via cron. See `gpfs_mmdf_exporter`.

Flags:

* `--output` - This is expected to be a path collected by the Prometheus node_exporter textfile collector
* `--collector.mmdf.filesystems` - A comma separated list of filesystems to collect. Default is to collect all filesystems listed by `mmlsfs`.

### mmces

The command used to collect CES states needs a specific node name.
The `--collector.mmces.nodename` flag can be used to specify which CES node to check.
The default is FQDN of those running the exporter.

### mmrepquota

* `--collector.mmrepquota.filesystems` - A comma separated list of filesystems to collect. Default is to collect all filesystems.
* `--collector.mmrepquota.quota-types` - Comma seperated list of filesystem types to collect (`fileset` for FILESET, `user` for USR, `group` for GRP). Default is FILESET only. Ex: `fileset,user` collects FILESET and USR.

### mmlssnapshot

* `--collector.mmlssnapshot.filesystems` - A comma separated list of filesystems to collect. Default is to collect all filesystems listed by `mmlsfs`.
* `--collector.mmlssnapshot.get-size` - Pass this flag to collect snapshot sizes. This operation could take a long time depending on filesystem size, consider using `gpfs_mmlssnapshot_exporter` instead.

The exporter `gpfs_mmlssnapshot_exporter` is provided to allow snapshot collection, including size (with `--collector.mmlssnapshot.get-size`) to be collected with cron rather than a Prometheus scrape through the normal exporter.

### mmlsfileset

* `--collector.mmlsfileset.filesystems` - A comma separated list of filesystems to collect. Default is to collect all filesystems listed by `mmlsfs`.

**NOTE**: This collector does not collect used inodes. To get used inodes look at using the [mmrepquota](#mmrepquota) collector.

### mmlsqos

Displays the I/O performance values of a file system, when you enable Quality of Service for I/O operations (QoS) with the mmchqos command.

Flags:
* `--collector.mmlsqos.filesystems` - A comma separated list of filesystems to collect. Default is to collect all filesystems listed by `mmlsfs`.
* `--collector.mmlsqos.timeout` - Count of seconds for running mmlsqos command before timeout error will be raised. Default value is 60 seconds.
* `--collector.mmlsqos.seconds` - Displays the I/O performance values for the previous number of seconds. The valid range of seconds is 1-999. The default value is 60 seconds.

### mmlspool

Collects GPFS pool data

Flags:
* `--collector.mmlspool.filesystems` - A comma separated list of filesystems to collect. Default is to collect all filesystems listed by `mmlsfs`.
* `--collector.mmlspool.timeout` - Count of seconds for running mmlspool command before timeout error will be raised. Default value is 30 seconds.

## Sudo

Ensure the user running `gpfs_exporter` can execute GPFS commands necessary to collect metrics.
The following sudo config assumes `gpfs_exporter` is running as `gpfs_exporter`.

```
Defaults:gpfs_exporter !syslog
Defaults:gpfs_exporter !requiretty
# mmgetstate collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmgetstate -Y
# mmpmon collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmpmon -s -p
# config collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmdiag --config -Y
# mmhealth collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmhealth node show -Y
# verbs collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmfsadm test verbs status
# mmdf/mmlssnapshot collector if filesystems not specified
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlsfs all -Y -T
# waiter collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmdiag --waiters -Y
# mmces collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmces state show *
# mmdf collector, each filesystem must be listed
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmdf project -Y
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmdf scratch -Y
# mmrepquota collector, filesystems not specified
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmrepquota -j -Y -a
# mmrepquota collector, filesystems specified
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmrepquota -j -Y project scratch
# mmlssnapshot collector, each filesystem must be listed
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlssnapshot project -s all -Y
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlssnapshot ess -s all -Y
# mmlsfileset collector, each filesystem must be listed
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlsfileset project -Y
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlsfileset ess -Y
# mmlsqos collector, each filesystem must be listed
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlsqos mmfs1 -Y
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlsqos ess -Y
# mmlspool collector, each filesystem must be listed
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlspool mmfs1
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlspool ess
```

## Install

Download the [latest release](https://github.com/treydock/gpfs_exporter/releases)

Add the user that will run `gpfs_exporter`

```
groupadd -r gpfs_exporter
useradd -r -d /var/lib/gpfs_exporter -s /sbin/nologin -M -g gpfs_exporter -M gpfs_exporter
```

Install compiled binaries after extracting tar.gz from release page.

```
cp /tmp/gpfs_exporter /usr/local/bin/gpfs_exporter
```

Add sudo rules, see [sudo section](#sudo)

Add systemd unit file and start service. Modify the `ExecStart` with desired flags.

```
cp systemd/gpfs_exporter.service /etc/systemd/system/gpfs_exporter.service
systemctl daemon-reload
systemctl start gpfs_exporter
```

## Build from source

To produce the `gpfs_exporter`, `gpfs_mmdf_exporter`, and `gpfs_mmlssnapshot_exporter` binaries:

```
make build
```

Or

```
go get github.com/treydock/gpfs_exporter/cmd/gpfs_exporter
go get github.com/treydock/gpfs_exporter/cmd/gpfs_mmdf_exporter
go get github.com/treydock/gpfs_exporter/cmd/gpfs_mmlssnapshot_exporter
```

## TLS and basic auth

`gpfs_exporter` supports TLS and basic auth using [exporter-toolkit](https://github.com/prometheus/exporter-toolkit). To use TLS and/or basic auth, users need to use `--web.config.file` CLI flag as follows

```
gpfs_exporter --web.config.file=web-config.yaml
```

A sample `web-config.yaml` file can be fetched from [exporter-toolkit repository](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-config.yml). The reference of the `web-config.yaml` file can be consulted in the [docs](https://github.com/prometheus/exporter-toolkit/blob/master/docs/web-configuration.md).

## Grafana

There is an example [GPFS Performance](https://grafana.com/grafana/dashboards/14844) dashboard.  See the description on that dashboard for additional information on labels needed to utilize that dashboard.

## Prometheus Configuration

This is an example scrape config with some metrics excluded for HPC compute nodes with label `role=compute`:

```yaml
- job_name: gpfs
  scrape_timeout: 2m
  scrape_interval: 3m
  relabel_configs:
  - source_labels: [__address__]
    regex: "([^.]+)..*"
    replacement: "$1"
    target_label: host
  metric_relabel_configs:
  - source_labels: [__name__,role]
    regex: gpfs_(mount|health|verbs)_status;compute
    action: drop
  - source_labels: [__name__,collector,role]
    regex: gpfs_exporter_(collect_error|collector_duration_seconds);(mmhealth|mount|verbs);compute
    action: drop
  - source_labels: [__name__,role]
    regex: "^(go|process|promhttp)_.*;compute"
    action: drop
  file_sd_configs:
  - files:
    - "/etc/prometheus/file_sd_config.d/gpfs_*.yaml"
```

An example scrape target configuration:

```yaml
- targets:
  - c0001.example.com:9303
  labels:
    host: c0001
    cluster: example
    environment: production
    role: compute
```
