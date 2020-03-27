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
	"net/http"
	"net/url"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestConfigDefaults(t *testing.T) {
	config := NewConfig()

	if err := config.Validate(); err != nil {
		t.Errorf("validation for config failed with error: %q", err)
	}
}

func TestValidatedMaxConcurrency(t *testing.T) {
	expectedErr := "config.MaxConcurrency must be greater than 0"
	config := NewConfig(SetMaxConcurrency(-1))

	err := config.Validate()

	if err == nil {
		t.Errorf("expected config to validate max concurrency")
	} else if err.Error() != expectedErr {
		t.Errorf("expected config to validate max concurrency: %q", err)
	}
}

func TestValidateMaxPendingURLS(t *testing.T) {
	expectedErr := "config.MaxPendingURLS must be greater than 0"
	config := NewConfig(SetMaxPendingURLS(0))

	err := config.Validate()

	if err == nil {
		t.Errorf("expected config to validate max pending URLS")
	} else if err.Error() != expectedErr {
		t.Errorf("expected config to validate max pending URLS: %q", err)
	}
}

func TestValidateKeepAlive(t *testing.T) {
	expectedErr := "config.KeepAlive duration should be >= 0s"
	config := NewConfig(SetKeepAlive(time.Duration(-1)))

	err := config.Validate()

	if err == nil {
		t.Errorf("expected config to validate keep alive")
	} else if err.Error() != expectedErr {
		t.Errorf("expected config to validate keep alive: %q", err)
	}
}

func TestValidateTimeout(t *testing.T) {
	expectedErr := "config.Timeout duration should be >= 0s"
	config := NewConfig(SetTimeout(time.Duration(-1)))

	err := config.Validate()

	if err == nil {
		t.Errorf("expected config to validate timeout")
	} else if err.Error() != expectedErr {
		t.Errorf("expected config to validate timeout: %q", err)
	}
}

func TestValidateClient(t *testing.T) {
	expectedErr := "config.Client must be defined"
	config := NewConfig()
	config.Client = nil

	err := config.Validate()

	if err == nil {
		t.Errorf("expected config to validate client")
	} else if err.Error() != expectedErr {
		t.Errorf("expected config to validate client: %q", err)
	}
}

func TestValidateLogger(t *testing.T) {
	expectedErr := "config.Logger must be defined"
	config := NewConfig()
	config.Logger = nil

	err := config.Validate()

	if err == nil {
		t.Errorf("expected config to validate logger")
	} else if err.Error() != expectedErr {
		t.Errorf("expected config to validate logger: %q", err)
	}
}

func TestValidateDomainValidator(t *testing.T) {
	expectedErr := "config.DomainValidator must be defined"
	config := NewConfig()
	config.DomainValidator = nil

	err := config.Validate()
	if err == nil {
		t.Errorf("expected config to validate domain validator")
	} else if err.Error() != expectedErr {
		t.Errorf("expected config to validate domain validators: %q", err)
	}
}

func TestMaxConcurrencyOption(t *testing.T) {
	expectedMaxConcurrency := DefaultMaxConcurrency * 2
	config := NewConfig(SetMaxConcurrency(expectedMaxConcurrency))

	if config.MaxConcurrency != expectedMaxConcurrency {
		t.Errorf(
			"expected option to set max concurrency to %d but it was %d",
			expectedMaxConcurrency,
			config.MaxConcurrency,
		)
	}
}

func TestMaxPendingURLSOption(t *testing.T) {
	expectedMaxPendingURLS := DefaultMaxPendingURLS * 2
	config := NewConfig(SetMaxPendingURLS(expectedMaxPendingURLS))

	if config.MaxPendingURLS != expectedMaxPendingURLS {
		t.Errorf(
			"expected option to set max pending URLS to %d but it was %d",
			expectedMaxPendingURLS,
			config.MaxPendingURLS,
		)
	}
}

func TestCrawlTimeoutOption(t *testing.T) {
	expectedCrawlTimeout := 5 * time.Second
	config := NewConfig(SetCrawlTimeout(expectedCrawlTimeout))

	if config.CrawlTimeout != expectedCrawlTimeout {
		t.Errorf(
			"expected option to set crawl timeout to %d but it was %d",
			expectedCrawlTimeout,
			config.CrawlTimeout,
		)
	}
}

func TestKeepAliveOption(t *testing.T) {
	expectedKeepAlive := 2 * DefaultKeepAlive
	config := NewConfig(SetKeepAlive(expectedKeepAlive))

	if config.KeepAlive != expectedKeepAlive {
		t.Errorf(
			"expected option to set keep alive to %d but it was %d",
			expectedKeepAlive,
			config.KeepAlive,
		)
	}
}

func TestTimoutOption(t *testing.T) {
	expectedTimeout := 2 * DefaultTimeout
	config := NewConfig(SetTimeout(expectedTimeout))

	if config.Timeout != expectedTimeout {
		t.Errorf(
			"expected option to set timeout to %d but it was %d",
			expectedTimeout,
			config.Timeout,
		)
	}
}

func TestClientOption(t *testing.T) {
	config := NewConfig(SetClient(http.DefaultClient))

	// The client should be a copy
	if config.Client == http.DefaultClient {
		t.Errorf("expected the config to shallow copy the client")
	}

	// The client should override redirects
	if config.Client.CheckRedirect == nil {
		t.Errorf("expected config to override check redirect")
	}

	// The client should be derived from the client provided
	if config.Client.Transport != http.DefaultClient.Transport {
		t.Errorf("expected config to use the provided client")
	}
}

func TestLoggerOption(t *testing.T) {
	expectedLogger := zap.NewNop()

	config := NewConfig(SetLogger(expectedLogger))

	if config.Logger != expectedLogger {
		t.Errorf("expected config to use provided logger")
	}
}

func TestDomainValidatorOption(t *testing.T) {
	// override validator returns false
	testURL := &url.URL{
		Scheme: "https",
		Host:   "github.com",
	}

	expectedDomainValidator := DomainValidatorFunc(func(a, b *url.URL) bool {
		return false
	})

	config := NewConfig(SetDomainValidator(expectedDomainValidator))

	checkDefaultResult := ValidateHosts(testURL, testURL)
	if !checkDefaultResult {
		t.Errorf("expected the default host check to return true")
	}

	checkConfigResult := config.DomainValidator.Validate(testURL, testURL)
	if checkConfigResult {
		t.Errorf("expected the config host check to return false")
	}
}

func TestClienNilOption(t *testing.T) {
	config := NewConfig(SetClient(nil))

	if config.Client == nil {
		t.Errorf("expected default client when client option is nil")
	}
}

func TestLoggerNilOption(t *testing.T) {
	config := NewConfig(SetLogger(nil))

	if config.Logger == nil {
		t.Errorf("expected default logger when logger option is nil")
	}
}

func TestDomainValidatorNilOption(t *testing.T) {
	config := NewConfig(SetDomainValidator(nil))

	if config.DomainValidator == nil {
		t.Errorf("expected default domain validator when option is nil")
	}
}
