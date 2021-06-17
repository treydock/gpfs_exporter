## 1.5.1 / 2021-06-17

* Fix `mmdf` collector to still write last collection metric during errors

## 1.5.0 / 2021-06-07

* Add `gpfs_mmdiag_waiter_info` metric

## 1.4.0 / 2021-05-27

* Add config collector, enabled by default

## 1.3.0 / 2021-04-23

### Changes

* Update to Go 1.16
* Add mmlsfileset collector

## 1.2.0 / 2021-04-15

### Changes

* Add mmlssnapshot collector

## 1.1.2 / 2021-04-12

### Bug fixes

* Do not produce errors if no metadata is reported by mmdf

## 1.1.1 / 2021-03-31

### Bug fixes

* Fix possible index out of range parsing errors with mmdf collector

## 1.1.0 / 2021-01-02

### Changes

* Allow mmhealth items to be filtered out via CLI flags
* Allow mmces services to be filtered out via CLI flags

## 1.0.0 / 2020-11-24

### **Breaking Changes**

* Remove --exporter.use-cache flag and all caching logic
* Rename several metrics to standardize naming conventions
  * gpfs_perf_read_bytes to gpfs_perf_read_bytes_total
  * gpfs_perf_write_bytes to gpfs_perf_write_bytes_total
  * gpfs_perf_operations to gpfs_perf_operations_total
  * gpfs_fs_inodes_allocated to gpfs_fs_allocated_inodes
  * gpfs_fs_inodes_free to gpfs_fs_free_inodes
  * gpfs_fs_inodes_total to gpfs_fs_total_inodes
  * gpfs_fs_inodes_used to gpfs_fs_used_inodes
  * gpfs_fs_total_inodes to gpfs_fs_inodes
  * gpfs_fs_total_bytes to gpfs_fs_size_bytes
  * gpfs_fs_metadata_total_bytes to gpfs_fs_metadata_size_bytes
* Removed metrics that can be calculated using other metrics
  * gpfs_fs_metadata_free_percent
  * gpfs_fs_free_percent
* Remove nodename label from gpfs_perf_* metrics, replace with gpfs_perf_info metric
* mmces state metrics will have one metric per possible state, with active state having value 1
* mmhealth status metrics will have one metric per possible status with active status having value 1

### Changes

* Update to Go 1.15 and update all dependencies
* Improved error handling for cron gpfs_mmdf_exporter
* Add mmrepquota collector to collect quota information for filesets

## 0.11.1 / 2020-04-21

* Fix mount collector to avoid false positives

## 0.11.0 / 2020-04-04

* Improve timeout/error handling around mmlsfs and add tests

## 0.10.0 / 2020-04-04

* Simplified timeout and error handling

## 0.9.0 / 2020-03-16

### Changes

* Allow caching of metrics if errors or timeouts occur
* Improved testing

## 0.8.0 / 2020-03-05

### Changes

* Add mmgetstate collector and metrics
* Use promlog for logging

## 0.7.0 / 2020-03-02

### Changes

* Add timeouts to all collectors

## 0.6.0 / 2020-02-25

### Changes

* Update client_golang dependency
* Testing improvements

## 0.5.0 / 2020-02-18

### Changes

* Support excluding waiters

## 0.4.0 / 2020-02-17

### Changes

* Refactor mmdiag waiter metric collection

## 0.3.1 / 2020-02-17

### Fixes

* Avoid duplicate metrics for collector errors

## 0.3.0 / 2020-02-15

### Changes

* Add mmdiag collector with waiters metric
* Add mmces collector with service state metrics

## 0.2.0 / 2020-01-30

### Changes

* Move all metrics to /metrics endpoint, remove /gpfs endpoint
* Add --web.disable-exporter-metrics flag

## 0.1.0 / 2020-01-29

### Changes

* Rename gpfs_mmhealth_state to gpfs_mmhealth_status
* Add status label to mmhealth status metric

## 0.0.1 / 2020-01-29

### Changes

* Initial Release

