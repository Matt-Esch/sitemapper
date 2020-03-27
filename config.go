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

package sitemapper

import (
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// DefaultMaxConcurrency sets the number of goroutines to be used to crawl
// pages. This default is used to configure the transport of the default
// http client so that there are enough connections to support the number of
// goroutines used.
const DefaultMaxConcurrency = 8

// DefaultMaxPendingURLS limits the size of the URLS list. This prevents us from
// increasing the URLS list faster than we can drain it. This wouldn't normally
// expect to happen, but there could be cases where URLs are poorly designed
// and contain data that changes on every page load.
const DefaultMaxPendingURLS = 8192

// DefaultCrawlTimeout limits the total amount of time spent crawling. When 0
// there is no limit.
const DefaultCrawlTimeout = time.Duration(0)

// DefaultTimeout is the default timeout used by the http client if no other
// timeout is specified.
const DefaultTimeout = time.Second * 10

// DefaultKeepAlive is the default keepalive timeout for client connections.
const DefaultKeepAlive = time.Second * 30

// Config is a stuct of crawler configuration options.
type Config struct {
	MaxConcurrency  int
	MaxPendingURLS  int
	CrawlTimeout    time.Duration
	KeepAlive       time.Duration
	Timeout         time.Duration
	Client          *http.Client
	Logger          *zap.Logger
	DomainValidator DomainValidator
}

// NewConfig creates a config from the specified options, and provides
// defaults for options which are not specified
func NewConfig(options ...Option) *Config {
	config := &Config{
		MaxConcurrency:  DefaultMaxConcurrency,
		MaxPendingURLS:  DefaultMaxPendingURLS,
		CrawlTimeout:    DefaultCrawlTimeout,
		KeepAlive:       DefaultKeepAlive,
		Timeout:         DefaultTimeout,
		Client:          nil,
		Logger:          nil,
		DomainValidator: nil,
	}

	// Options are applied first to inform client options if none is set
	for _, opt := range options {
		opt.apply(config)
	}

	if config.Client == nil {
		config.Client = &http.Client{
			// Following redirects is disabled by default because they
			// could redirect outside the current domain
			CheckRedirect: overrideRedirect,
			Transport: &http.Transport{
				MaxIdleConns:        config.MaxConcurrency,
				MaxIdleConnsPerHost: config.MaxConcurrency,
				MaxConnsPerHost:     config.MaxConcurrency,
				IdleConnTimeout:     config.KeepAlive,
			},
			Timeout: config.Timeout,
		}
	}

	if config.Logger == nil {
		logger, loggerErr := zap.NewProduction(zap.IncreaseLevel(zap.WarnLevel))
		if loggerErr != nil {
			logger = zap.NewNop()
		}
		config.Logger = logger
	}

	if config.DomainValidator == nil {
		config.DomainValidator = DomainValidatorFunc(ValidateHosts)
	}

	return config
}

// Validate checks the configuration options for validation issues.
func (config *Config) Validate() error {
	if config.MaxConcurrency <= 0 {
		return fmt.Errorf("config.MaxConcurrency must be greater than 0")
	}

	if config.MaxPendingURLS <= 0 {
		return fmt.Errorf("config.MaxPendingURLS must be greater than 0")
	}

	if config.KeepAlive < time.Duration(0) {
		return fmt.Errorf("config.KeepAlive duration should be >= 0s")
	}

	if config.Timeout < time.Duration(0) {
		return fmt.Errorf("config.Timeout duration should be >= 0s")
	}

	if config.Client == nil {
		return fmt.Errorf("config.Client must be defined")
	}

	if config.Logger == nil {
		return fmt.Errorf("config.Logger must be defined")
	}

	if config.DomainValidator == nil {
		return fmt.Errorf("config.DomainValidator must be defined")
	}

	return nil
}

// Option is used to configure configuration options that are not required
type Option interface {
	apply(config *Config)
}

type optionFunc func(config *Config)

func (o optionFunc) apply(config *Config) {
	o(config)
}

// SetMaxConcurrency sets the number of goroutines that will be used. This is
// also used to configure the default http client with enough open connections
// to support this number of goroutines.
func SetMaxConcurrency(maxConcurrency int) Option {
	return optionFunc(func(config *Config) {
		config.MaxConcurrency = maxConcurrency
	})
}

// SetMaxPendingURLS sets the maximum number of URLs that can persist in the
// queue for crawling. This will set the size of the channel of URLs being
// processed by the goroutines. This helps prevent cases where the number of
// URLs runs away indefinitely due to dynamic urls in page links.
func SetMaxPendingURLS(maxPendingURLS int) Option {
	return optionFunc(func(config *Config) {
		config.MaxPendingURLS = maxPendingURLS
	})
}

// SetCrawlTimeout sets the maximum time spent crawling URLs. When the timeout
// is zero or negative, no timeout is applied and the caller will wait for
// completion. If the timeout fires, the caller will receive the partial site
// map.
func SetCrawlTimeout(crawlTimeout time.Duration) Option {
	return optionFunc(func(config *Config) {
		config.CrawlTimeout = crawlTimeout
	})
}

// SetKeepAlive sets the http client connection keep alive timeout when the
// default http client is used.
func SetKeepAlive(keepAlive time.Duration) Option {
	return optionFunc(func(config *Config) {
		config.KeepAlive = keepAlive
	})
}

// SetTimeout sets the http client request timeout when the default http
// client is used.
func SetTimeout(timeout time.Duration) Option {
	return optionFunc(func(config *Config) {
		config.Timeout = timeout
	})
}

// SetClient overrides the default client config. Note that if the client is
// set, KeepAlive and Timeout will not be effective and the keep alive and
// timeout options set for the client will take precendence.
func SetClient(client *http.Client) Option {
	return optionFunc(func(config *Config) {
		if client == nil {
			config.Client = nil
			return
		}

		// If a custom client is used, we need to make sure that redirects
		// are not followed without mutating the original client
		var overrideClient = *client
		overrideClient.CheckRedirect = overrideRedirect
		config.Client = &overrideClient
	})
}

// SetLogger overrides the default logger. The default logger is configured
// to write warning and error logs to stderr.
func SetLogger(logger *zap.Logger) Option {
	return optionFunc(func(config *Config) {
		config.Logger = logger
	})
}

// SetDomainValidator overrides the default domain validator. The default
// validator is configured to compare the host component of the URLs only, not
// the scheme or any DNS lookups.
func SetDomainValidator(validator DomainValidator) Option {
	return optionFunc(func(config *Config) {
		config.DomainValidator = validator
	})
}

// overrideRedirect is used to prevent the http client following external
// redirects.
func overrideRedirect(req *http.Request, via []*http.Request) error {
	return http.ErrUseLastResponse
}
