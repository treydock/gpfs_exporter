# GPFS Prometheus exporter

[![Build Status](https://circleci.com/gh/treydock/gpfs_exporter/tree/master.svg?style=shield)](https://circleci.com/gh/treydock/gpfs_exporter)
[![GitHub release](https://img.shields.io/github/v/release/treydock/gpfs_exporter?include_prereleases&sort=semver)](https://github.com/treydock/gpfs_exporter/releases/latest)
![GitHub All Releases](https://img.shields.io/github/downloads/treydock/gpfs_exporter/total)
[![codecov](https://codecov.io/gh/treydock/gpfs_exporter/branch/master/graph/badge.svg)](https://codecov.io/gh/treydock/gpfs_exporter)

# GPFS Prometheus exporter

The GPFS exporter collects metrics from the GPFS filesystem.
The exporter supports the `/metrics` endpoint to gather GPFS metrics an metrics about the exporter.

## Collectors

Collectors are enabled or disabled via `--collector.<name>` and `--no-collector.<name>` flags.

Name | Description | Default
-----|-------------|--------
mmpmon| Collect metrics from `mmpmon` using `fs_io_s` | Enabled
mount | Check status of GPFS mounts. | Enabled
verbs | Test if GPFS is using verbs interface | Disabled
mmhealth | Test node health through `mmhealth` | Disabled
mmdiag | Test mmdiag waiters | Disabled
mmdf | Collect filesystem space for inodes, block and metadata. | Disabled
mmces | Collect state of CES | Disabled

### mount

The default behavior of the `mount` collector is to collect mount statuses on GPFS mounts in /proc/mounts or /etc/fstab. The `--collector.mount.mounts` flag can be used to adjust which mount points to check.

### mmdf

Due to the time it can take to execute mmdf that is an executable provided that can be used to collect mmdf via cron. See `gpfs_mmdf_exporter`.

Flags:

* `--output` - This is expected to be a path collected by the Prometheus node_exporter textfile collector
* `--collector.mmdf.filesystems` - A comma separated list of filesystems to collect. Default is to collect all filesystems listed by `mmlsfs`.

### mmces

The command used to collect CES states needs a specific node name.
The `--collector.mmces.nodename` flag can be used to specify which CES node to check.
The default is FQDN of those running the exporter.

## Sudo

Ensure the user running `gpfs_exporter` can execute GPFS commands necessary to collect metrics.
The following sudo config assumes `gpfs_exporter` is running as `gpfs_exporter`.

```
Defaults:gpfs_exporter !syslog
Defaults:gpfs_exporter !requiretty
# mmpmon collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmpmon -s -p
# mmhealth collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmhealth node show -Y
# verbs collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmfsadm test verbs status
# mmdf collector if filesystems not specified
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmlsfs all -Y -T
# mmdiag collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmdiag --waiters -Y
# mmces collector
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmces state show *
# mmdf collector, each filesystem must be listed
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmdf project -Y
gpfs_exporter ALL=(ALL) NOPASSWD:/usr/lpp/mmfs/bin/mmdf scratch -Y
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

To produce the `gpfs_exporter` and `gpfs_mmdf_exporter` binaries:

```
make build
```

Or

```
go get github.com/treydock/gpfs_exporter/cmd/gpfs_exporter
go get github.com/treydock/gpfs_exporter/cmd/gpfs_mmdf_exporter
```
