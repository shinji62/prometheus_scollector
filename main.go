// Copyright 2015 The Prometheus Authors
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
	"flag"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/shinji62/prometheus_scollector/scollector"
	"gopkg.in/inconshreveable/log15.v2"
)

var (
	Log          = log15.New()
	sampleExpiry = flag.Duration("scollector.sample-expiry", 5*time.Minute, "How long a sample is valid for.")
)

var (
	flagAddr       = flag.String("http", "0.0.0.0:9107", "address to listen on")
	flagScollPref  = flag.String("scollector.prefix", "/api/put", "HTTP path prefix for listening for scollector-sent data")
	flagVerbose    = flag.Bool("v", false, "verbose logging")
	flagReplaceTag = flag.String("replace-label", "", "Comma separated of replaced and replacing tag ex old:newTag,one:two")
)

func main() {
	hndl := log15.CallerFileHandler(log15.StderrHandler)
	Log.SetHandler(hndl)

	flag.Parse()
	if !*flagVerbose {
		Log.SetHandler(log15.LvlFilterHandler(log15.LvlInfo, hndl))
	}

	c := scollector.NewScollectorCollector(sampleExpiry)
	c.SetReplacingTags(*flagReplaceTag)
	prometheus.MustRegister(c)

	http.HandleFunc(*flagScollPref, c.HandleScoll)
	http.Handle("/metrics", prometheus.Handler())
	Log.Info("Serving on " + *flagAddr)
	http.ListenAndServe(*flagAddr, nil)
}
