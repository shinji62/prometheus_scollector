# Goal #
Use bosun.org's mature machine statistics gatherer,
scollector (`bosun.org/scollector`) with prometheus.io's nice slim&sleek Hadoop-free storage.

# Usage #

```shell
go get github.com/shinji62/prometheus_scollector
prometheus_scollector -http=0.0.0.0:9107
scollector -h=<the-collector-machine>:9107
```


Now you can scrape this collector adding this configuration into prometheus.conf
```
	job: {
		name: "scollector"
		target_group: {
			target: "http://the-collector-machine:9107/metrics"
		}
	}
```

# Testing #
For testing purpose you need scollector binary.

You can use docker and alias
```shell
alias scollector="docker run --rm diyan/scollector"
```

Run the test
```
ginkgo -r

```
