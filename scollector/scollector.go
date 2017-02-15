package scollector

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"bosun.org/opentsdb"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/shinji62/prometheus_scollector/replaceTags"
	"github.com/shinji62/prometheus_scollector/utils"
	"gopkg.in/inconshreveable/log15.v2"
)

var (
	lastProcessed = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "scollector_last_processed_timestamp_seconds",
			Help: "Unix timestamp of the last processed scollector metric.",
		})

	Log = log15.New()
)

// Most of the ideas are stolen from https://github.com/prometheus/graphite_exporter/blob/master/main.go
// https://github.com/prometheus/graphite_exporter/commit/298611cd340e0a34bc9d2d434f47456c6c201221

type scollectorSample struct {
	Name      string
	Labels    map[string]string
	Help      string
	Value     float64
	Type      prometheus.ValueType
	Timestamp time.Time
}

func (s scollectorSample) NameWithLabels() string {
	labels := make([]string, 0, len(s.Labels))
	for k, v := range s.Labels {
		labels = append(labels, k+"="+v)
	}
	sort.Strings(labels)
	return s.Name + "#" + strings.Join(labels, ";")
}

type ScollectorCollector struct {
	samples        map[string]scollectorSample
	types          map[string]string
	ch             chan scollectorSample
	mu             sync.Mutex
	ReplacingsTags map[string]string
	expiryTimes    *time.Duration
}

func NewScollectorCollector(expiryTimes *time.Duration) *ScollectorCollector {
	c := &ScollectorCollector{
		ch:          make(chan scollectorSample, 0),
		samples:     make(map[string]scollectorSample, 512),
		types:       make(map[string]string, 512),
		expiryTimes: expiryTimes,
	}
	go c.processSamples()
	return c
}

func (c *ScollectorCollector) processSamples() {
	ticker := time.NewTicker(time.Minute).C
	for {
		select {
		case sample := <-c.ch:
			c.mu.Lock()
			c.samples[sample.NameWithLabels()] = sample
			c.mu.Unlock()
		case <-ticker:
			// Garbage collect expired samples.
			ageLimit := time.Now().Add(-*c.expiryTimes)
			c.mu.Lock()
			for k, sample := range c.samples {
				if ageLimit.After(sample.Timestamp) {
					delete(c.samples, k)
				}
			}
			c.mu.Unlock()
		}
	}
}

// Collect implements prometheus.Collector.
func (c *ScollectorCollector) Collect(ch chan<- prometheus.Metric) {
	Log.Debug("Collect", "samples", len(c.samples))
	ch <- lastProcessed
	Log.Debug("Collect", "lastProcessed", lastProcessed)

	c.mu.Lock()
	samples := make([]scollectorSample, 0, len(c.samples))
	for _, sample := range c.samples {
		samples = append(samples, sample)
	}
	c.mu.Unlock()

	ageLimit := time.Now().Add(-*c.expiryTimes)
	for _, sample := range samples {
		if ageLimit.After(sample.Timestamp) {
			Log.Debug("skipping old sample", "limit", ageLimit, "sample", sample)
			continue
		}
		Log.Debug("sending sample", "sample", sample)
		ch <- prometheus.MustNewConstMetric(
			prometheus.NewDesc(sample.Name, sample.Help, []string{}, sample.Labels),
			sample.Type,
			sample.Value,
		)
	}
}

func (c *ScollectorCollector) SetReplacingTags(replaceTags string) {
	c.ReplacingsTags = map[string]string{}
	if tags, err := replacetags.ParseExtraFields(replaceTags); err == nil {
		c.ReplacingsTags = tags
	}
}

// Describe implements prometheus.Collector.
func (c *ScollectorCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- lastProcessed.Desc()
}

var dotReplacer = strings.NewReplacer(".", "_")

func (c *ScollectorCollector) HandleScoll(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	sendErr := func(msg string, code int) {
		Log.Error(msg)
		if code == 0 {
			code = http.StatusInternalServerError
		}
		http.Error(w, msg, code)
	}
	if r.Method != "POST" {
		sendErr("Only POST is allowed", http.StatusMethodNotAllowed)
		return
	}
	if r.Header.Get("Content-Type") != "application/json" {
		sendErr("Only application/json is allowed", http.StatusBadRequest)
		return
	}

	rdr := io.Reader(r.Body)
	if r.Header.Get("Content-Encoding") == "gzip" {
		var err error
		if rdr, err = gzip.NewReader(rdr); err != nil {
			sendErr("Not gzipped: "+err.Error(), http.StatusBadRequest)
			return
		}
	}
	var batch []opentsdb.DataPoint
	if err := json.NewDecoder(rdr).Decode(&batch); err != nil {
		sendErr("cannot decode JSON: "+err.Error(), http.StatusBadRequest)
		return
	}

	Log.Debug("batch", "size", len(batch))
	n := 0
	for _, m := range batch {
		Log.Debug("got", "msg", m)
		name := utils.ClearName(m.Metric, true, 'x')
		if name == "" {
			Log.Warn("bad metric name: " + m.Metric)
			continue
		}

		var v float64
		switch x := m.Value.(type) {
		case float64:
			v = x
		case int:
			v = float64(x)
		case int64:
			v = float64(x)
		case int32:
			v = float64(x)
		case string: // type info
			if x != "" {
				if z, ok := c.types[name]; !ok || z != x {
					c.types[name] = x
				}
			}
			continue
		default:
			Log.Warn("unknown value", "type", fmt.Sprintf("%T", m.Value), "msg", m)
			continue
		}
		typ := prometheus.GaugeValue
		if c.types[name] == "counter" {
			typ = prometheus.CounterValue
		}
		var ts time.Time
		if m.Timestamp >= 1e10 {
			ts = time.Unix(
				m.Timestamp&(1e10-1),
				int64(math.Mod(float64(m.Timestamp), 1e10))*int64(time.Millisecond))
		} else {
			ts = time.Unix(m.Timestamp, 0)
		}
		for k := range m.Tags {

			k2 := utils.ClearName(k, false, 'x')
			if k2 != "" && k2 != k {
				Log.Warn("bad label name " + k)
				m.Tags[k2] = m.Tags[k]
				delete(m.Tags, k)
			}
		}
		if m.Tags["host"] != "" {
			m.Tags["instance"] = m.Tags["host"]
			delete(m.Tags, "host")
		}
		for k := range m.Tags {
			if replace, there := c.ReplacingsTags[k]; there != false {
				m.Tags[replace] = m.Tags[k]
				delete(m.Tags, k)
			}
		}

		c.ch <- scollectorSample{
			Name:      name,
			Labels:    m.Tags,
			Type:      typ,
			Help:      fmt.Sprintf("Scollector metric %s (%s)", m.Metric, c.types[name]),
			Value:     v,
			Timestamp: ts,
		}
		n++
	}
	lastProcessed.Set(float64(time.Now().UnixNano()) / 1e9)
	Log.Info("processed", "messages", n, "samples", len(c.samples), "types", len(c.types))

	w.WriteHeader(http.StatusNoContent)
}
