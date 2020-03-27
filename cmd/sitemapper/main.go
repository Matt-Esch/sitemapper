// Copyright (c) 2020 Matthew Esch
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Matt-Esch/sitemapper"
	"go.uber.org/zap"
)

const concurrency int = 8
const crawlTimeout time.Duration = 0
const timeout time.Duration = 30 * time.Second
const keepAlive time.Duration = sitemapper.DefaultKeepAlive

func main() {
	urlPtr := flag.String("u", "", "url to crawl (required)")
	concPtr := flag.Int("c", concurrency, "maximum concurrency")
	crawlTimeoutPtr := flag.Duration("w", crawlTimeout, "maximum crawl time")
	timeoutPtr := flag.Duration("t", timeout, "http request timeout")
	keepAlivePtr := flag.Duration("k", keepAlive, "http keep alive timeout")
	verbosePtr := flag.Bool("v", false, "enable verbose logging")
	debugPtr := flag.Bool("d", false, "enable debug logs")

	flag.Parse()

	if *urlPtr == "" {
		flag.Usage()
		os.Exit(1)
	}

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        *concPtr,
			MaxIdleConnsPerHost: *concPtr,
			MaxConnsPerHost:     *concPtr,
			IdleConnTimeout:     *keepAlivePtr,
		},
		Timeout: *timeoutPtr,
	}

	logger, loggerErr := newLogger(*verbosePtr, *debugPtr)
	if loggerErr != nil {
		log.Fatalf("error: %s", loggerErr)
	}

	siteMap, siteMapErr := sitemapper.CrawlDomain(
		*urlPtr,
		sitemapper.SetMaxConcurrency(*concPtr),
		sitemapper.SetCrawlTimeout(*crawlTimeoutPtr),
		sitemapper.SetKeepAlive(*keepAlivePtr),
		sitemapper.SetTimeout(*timeoutPtr),
		sitemapper.SetClient(client),
		sitemapper.SetLogger(logger),
	)

	if siteMapErr != nil {
		log.Fatalf("error: %s", siteMapErr)
	}

	siteMap.WriteMap(os.Stdout)
}

func newLogger(verbose bool, debug bool) (*zap.Logger, error) {
	if !verbose && !debug {
		return zap.NewNop(), nil
	}

	config := zap.Config{
		Level:       zap.NewAtomicLevelAt(zap.InfoLevel),
		Development: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         "json",
		EncoderConfig:    zap.NewProductionEncoderConfig(),
		OutputPaths:      []string{"stderr"},
		ErrorOutputPaths: []string{"stderr"},
	}

	if debug {
		config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}

	return config.Build()
}
