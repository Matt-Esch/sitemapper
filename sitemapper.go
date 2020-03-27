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

// Package sitemapper provides a parallel site crawler for producing site maps
package sitemapper

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"go.uber.org/atomic"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

// hrefAttr is used for matching the 'href' attribute in an 'a' tag.
var hrefAttr = []byte("href")

// CrawlDomain crawls a domain provided as a string URL. It wraps a call to
// CrawlDomainWithURL.
func CrawlDomain(rootURL string, opts ...Option) (*SiteMap, error) {
	root, rootErr := url.Parse(rootURL)
	if rootErr != nil {
		return nil, rootErr
	}
	return CrawlDomainWithURL(root, opts...)
}

// CrawlDomainWithURL crawls a domain provided as a URL and returns the
// resulting sitemap.
func CrawlDomainWithURL(root *url.URL, opts ...Option) (*SiteMap, error) {
	config := NewConfig(opts...)

	crawler, crawlerError := NewDomainCrawler(root, config)
	if crawlerError != nil {
		return nil, crawlerError
	}

	return crawler.Crawl()
}

// DomainCrawler contains the state of a domain web crawler. The domain crawler
// exposes a Crawl method which proudces a site map.
type DomainCrawler struct {
	root                 *url.URL
	config               *Config
	siteMap              *SiteMap
	pendingURLS          chan *url.URL
	pendingURLSRemaining *sync.WaitGroup
	accessedPageCount    atomic.Uint64
	timedOut             atomic.Bool
}

// NewDomainCrawler creates a new DomainCrawler from the root url and given
// configuration.
func NewDomainCrawler(root *url.URL, config *Config) (*DomainCrawler, error) {
	configError := config.Validate()
	if configError != nil {
		return nil, configError
	}

	siteMap := NewSiteMap(root, config.DomainValidator)

	pendingURLS := make(chan *url.URL, config.MaxPendingURLS)
	pendingURLS <- root

	var pendingURLSRemaining sync.WaitGroup
	pendingURLSRemaining.Add(1)

	return &DomainCrawler{
		root:                 root,
		config:               config,
		siteMap:              siteMap,
		pendingURLS:          pendingURLS,
		pendingURLSRemaining: &pendingURLSRemaining,
	}, nil
}

// Crawl reads all links in the domain with the specified concurrency and
// returns a site map. Note that Crawl is not thread safe and each caller must
// create a separate DomainCrawler.
func (crawler *DomainCrawler) Crawl() (*SiteMap, error) {
	maxConcurrency := crawler.config.MaxConcurrency
	crawlTimeout := crawler.config.CrawlTimeout

	for i := 0; i < maxConcurrency; i++ {
		go crawler.drainURLS()
	}

	// The timeout mechanism signals to the goroutines to stop reading
	// more URLs after the specified timeout. The function doesn't
	// return until the goroutines have drained the URLs.
	if crawlTimeout > 0 {
		go func() {
			time.Sleep(crawlTimeout)
			crawler.timedOut.Store(true)
		}()
	}

	crawler.pendingURLSRemaining.Wait()
	close(crawler.pendingURLS)

	if crawler.accessedPageCount.Load() == 0 {
		return nil, fmt.Errorf("unable to access url %s", crawler.root.String())
	}

	return crawler.siteMap, nil
}

// drainURLS reads from the the pending URLS channel and crawls the page for
// more links
func (crawler *DomainCrawler) drainURLS() {
	client := crawler.config.Client
	logger := crawler.config.Logger

	for pageURL := range crawler.pendingURLS {
		logger.Debug("crawling page for links",
			zap.String("url", pageURL.String()),
		)

		if crawler.timedOut.Load() {
			logger.Debug("skipping url due to timeout",
				zap.String("url", pageURL.String()),
			)
		} else {
			linkReader := NewLinkReader(pageURL, client)
			crawler.realAllLinks(linkReader)
			linkReader.Close()
		}

		crawler.pendingURLSRemaining.Done()
	}
}

// readAllLinks pushes all previously unseen links from the given linkReader
// into the domain crawler's pending URL channel for crawling.
func (crawler *DomainCrawler) realAllLinks(linkReader *LinkReader) {
	logger := crawler.config.Logger

	for {
		hrefString, hrefErr := linkReader.Read()

		if hrefErr != nil {
			if hrefErr != io.EOF {
				// TODO: If we error while reading a page we could schedule
				// it for retry. We would then need to configure some sort
				// of max attempts and perhaps some sort of backoff to
				// prevent spamming the page with requests.
				logger.Warn("error reading link from channel",
					zap.String("page", linkReader.URL()),
					zap.Error(hrefErr),
				)
			}
			break
		}

		crawler.accessedPageCount.Add(1)

		hrefURL, hrefParseErr := url.Parse(hrefString)
		if hrefParseErr != nil {
			logger.Warn("error parsing url",
				zap.String("page", linkReader.URL()),
				zap.String("link", hrefString),
				zap.Error(hrefParseErr),
			)

			continue
		}

		// Note that the link must be resolved relative to the current
		// page. URLs such as "?a=123" are rooted in the current path
		hrefResolved := linkReader.pageURL.ResolveReference(hrefURL)

		if crawler.siteMap.appendURL(hrefResolved) {
			logger.Debug("found new page",
				zap.String("page", hrefResolved.String()),
			)
			// Note that if we were to do blocking writes here, the
			// buffered channel could be full and the write would block
			// here. If all goroutines were blocked on writing to the
			// channel this would deadlock.
			select {
			case crawler.pendingURLS <- hrefResolved:
				logger.Debug("page appended to channel",
					zap.String("page", hrefResolved.String()),
				)
				crawler.pendingURLSRemaining.Add(1)
			default:
				// If the buffered channel is full we ran out of memory
				logger.Error("too many pending urls, page will be ignored",
					zap.String("page", hrefResolved.String()),
					zap.String("link", linkReader.URL()),
				)
			}
		}
	}
}

// A DomainValidator provides a Validate functions for comparing two URLs
// for same domain inclusion. This allows for custom behavior such as checking
// scheme (http vs https) or DNS lookup.
type DomainValidator interface {
	Validate(root *url.URL, link *url.URL) bool
}

// DomainValidatorFunc acts as an adapter for allowing the use of ordinary
// functions as domain validators.
type DomainValidatorFunc func(root, link *url.URL) bool

// Validate calls v(root, link).
func (v DomainValidatorFunc) Validate(root, link *url.URL) bool {
	return v(root, link)
}

// ValidateHosts provides a default domain validation function that compares
// the host components of the provided URLs.
func ValidateHosts(root, link *url.URL) bool {
	// Note: We could consider the transport to also be relevant (http vs https)
	return root.Host == link.Host
}

// SiteMap contains the state of a site map.
type SiteMap struct {
	url       *url.URL
	rwl       *sync.RWMutex
	siteURLS  map[string]bool
	validator DomainValidator
}

// NewSiteMap initializes a new SiteMap anchored at the specified URL and
// crawls with the specified HTTP client
func NewSiteMap(url *url.URL, validator DomainValidator) *SiteMap {
	return &SiteMap{
		url:       url,
		rwl:       &sync.RWMutex{},
		siteURLS:  map[string]bool{},
		validator: validator,
	}
}

// appendURL returns true if the url should be crawled. If true is returned
// it is assumed that the caller will crawl this URL and subsequent calls to
// appendURL will return false.
func (s *SiteMap) appendURL(url *url.URL) bool {
	// We shouldn't crawl if the url is not valid or is in an external domain
	if !s.validator.Validate(s.url, url) {
		return false
	}

	urlString := url.String()

	// We could always lock over a normal mutex, but by using a RWMutex
	// we should increase the throughput of checking duplicate urls.
	// It's reasonable to expect many duplicates on a typical page (in the
	// navigation bar for example), so it's a reasonable to expect that many
	// calls to shouldCrawl will not yield write contention.
	s.rwl.RLock()
	maybeCrawl := !s.siteURLS[urlString]
	s.rwl.RUnlock()

	if !maybeCrawl {
		return false
	}

	// Even when we do write to update the urls list, we could have lost out
	// in a race condition, so reading again is necessary after acquiring the
	// write lock.
	s.rwl.Lock()
	crawl := !s.siteURLS[urlString]
	s.siteURLS[urlString] = true
	s.rwl.Unlock()
	return crawl
}

// WriteMap writes the ordered site map to a given writer.
func (s *SiteMap) WriteMap(out io.Writer) {
	s.rwl.RLock()
	defer s.rwl.RUnlock()

	paths := make([]string, 0, len(s.siteURLS))
	for u := range s.siteURLS {
		paths = append(paths, u)
	}
	sort.Strings(paths)

	for _, path := range paths {
		io.WriteString(out, path)
		io.WriteString(out, "\n")
	}
}

// LinkReader is an iterative structure that allows for reading all href tags
// in a given URL. The link reader will make the http request to the specified
// url and allow for reading through all links in the returned page. When there
// are no more links in the page Read returns io.EOF. The consumer is
// responsible for closing the LinkReader when done to ensure and client http
// requests are cleaned up.
type LinkReader struct {
	client   *http.Client
	pageURL  *url.URL
	response *http.Response
	doc      *html.Tokenizer
	done     bool
}

// NewLinkReader returns a LinkReader for the specified URL, fetching the
// content with the specified client
func NewLinkReader(pageURL *url.URL, client *http.Client) *LinkReader {
	return &LinkReader{
		client:  client,
		pageURL: pageURL,
	}
}

// Read returns the next href in the html document
func (u *LinkReader) Read() (string, error) {
	if u.done {
		return "", io.EOF
	}

	if u.doc == nil {
		resp, respErr := u.client.Get(u.pageURL.String())
		if respErr != nil {
			return "", fmt.Errorf("http get error: %q", respErr)
		}

		u.response = resp
		u.doc = html.NewTokenizer(resp.Body)

		// If the response is a redirect we should read the location header
		// It is valid for 201 to return a location header but this should
		// not happen as a response to http GET
		if resp.StatusCode >= 300 && resp.StatusCode <= 399 {
			if err := resp.Body.Close(); err != nil {
				return "", err
			}
			locationURL, err := resp.Location()
			if err != nil {
				return "", err
			}
			u.done = true
			return locationURL.String(), nil
		}
	}

	// Read the href attributes from all a tags using a streaming tokenizer
	for {
		tt := u.doc.Next()
		switch tt {
		case html.ErrorToken:
			if closeErr := u.response.Body.Close(); closeErr != nil {
				return "", closeErr
			}
			return "", u.doc.Err()
		case html.StartTagToken:
			tn, hasAttr := u.doc.TagName()
			if len(tn) == 1 && tn[0] == 'a' && hasAttr {

				// Read the href attribute from the link
				for {
					key, val, moreAttr := u.doc.TagAttr()
					if bytes.Equal(key, hrefAttr) {
						return string(val), nil
					}
					if !moreAttr {
						break
					}
				}
			}
		}
	}
}

// Close cleans up any remaining client response. If all links are read from
// the link reader the body will be automatically closed, however if only the
// first N links are required, the body must be closed by the caller.
func (u *LinkReader) Close() error {
	u.done = true
	if u.response != nil {
		return u.response.Body.Close()
	}

	return nil
}

// URL returns the read-only url string that was used to make the client request
func (u *LinkReader) URL() string {
	return u.pageURL.String()
}
