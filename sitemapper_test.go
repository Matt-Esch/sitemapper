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
	"bytes"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	testServer "github.com/Matt-Esch/sitemapper/test/server"
	"go.uber.org/zap"
)

var directoryHandler http.HandlerFunc = nil

// The expected site map string for the example
var expectedSiteMap = []string{
	"/",
	"/about",
	"/hidden",
	"/hidden?t=0",
	"/images",
	"/rectangle",
	"/secret",
	"/square",
}

// The expected site map when the links are truncated by max pending
var expectedTruncatedSiteMap = []string{
	"/",
	"/about",
	"/images",
	"/secret",
}

func TestCrawlExample(t *testing.T) {
	testServer := newTestServer()
	defer testServer.Close()

	resolvedSiteMap, resolveSiteMapErr := expectedSiteMapString(
		testServer.URL,
		expectedSiteMap,
	)
	if resolveSiteMapErr != nil {
		t.Fatalf(
			"error creating resolved expected site map: %q",
			resolveSiteMapErr,
		)
	}

	sitemap, err := CrawlDomain(
		testServer.URL,
		SetClient(testServer.Client()),
		SetLogger(zap.NewNop()),
	)

	if err != nil {
		t.Fatalf("error reading example site map: %q", err)
	}

	var siteMapBuf bytes.Buffer
	sitemap.WriteMap(&siteMapBuf)

	siteMapString := siteMapBuf.String()

	if siteMapString != resolvedSiteMap {
		t.Errorf(
			"unexpected site map produced.\n\n\n"+
				"Got:\n\n%s\n\nExpected:\n\n%s",
			siteMapString,
			resolvedSiteMap,
		)
	}
}

func TestDropTruncatedURLS(t *testing.T) {
	testServer := newTestServer()
	defer testServer.Close()

	resolvedSiteMap, resolveSiteMapErr := expectedSiteMapString(
		testServer.URL,
		expectedTruncatedSiteMap,
	)
	if resolveSiteMapErr != nil {
		t.Fatalf(
			"error creating resolved expected site map: %q",
			resolveSiteMapErr,
		)
	}

	sitemap, err := CrawlDomain(
		testServer.URL,
		SetMaxConcurrency(1),
		SetMaxPendingURLS(1),
		SetClient(testServer.Client()),
		SetLogger(zap.NewNop()),
	)

	if err != nil {
		t.Fatalf("error reading example site map: %q", err)
	}

	var siteMapBuf bytes.Buffer
	sitemap.WriteMap(&siteMapBuf)

	siteMapString := siteMapBuf.String()

	if siteMapString != resolvedSiteMap {
		t.Errorf(
			"unexpected site map produced.\n\n\n"+
				"Got:\n\n%s\n\nExpected:\n\n%s",
			siteMapString,
			resolvedSiteMap,
		)
	}
}

func TestCrawlTimeout(t *testing.T) {
	testServer := newTestServer()
	defer testServer.Close()

	resolvedSiteMap, resolveSiteMapErr := expectedSiteMapString(
		testServer.URL,
		expectedSiteMap,
	)
	if resolveSiteMapErr != nil {
		t.Fatalf(
			"error creating resolved expected site map: %q",
			resolveSiteMapErr,
		)
	}

	// now run with a timeout
	partialSiteMap, partialSiteMapErr := CrawlDomain(
		testServer.URL+"/slow",
		SetClient(testServer.Client()),
		SetLogger(zap.NewNop()),
		SetCrawlTimeout(100*time.Millisecond),
	)

	if partialSiteMapErr != nil {
		t.Fatalf("error reading partial site map: %q", partialSiteMapErr)
	}

	var partialSiteMapBuf bytes.Buffer
	partialSiteMap.WriteMap(&partialSiteMapBuf)

	partialSiteMapString := partialSiteMapBuf.String()

	if partialSiteMapString == resolvedSiteMap {
		t.Fatalf(
			"full site map produced when expecting partial.\n\n\n"+
				"Got:\n\n%s",
			partialSiteMapString,
		)
	}
}

func TestCrawlError(t *testing.T) {
	testServer := newTestServer()
	testServer.Close()

	expectedError := fmt.Sprintf("unable to access url %s", testServer.URL)

	_, err := CrawlDomain(
		testServer.URL,
		SetClient(testServer.Client()),
		SetLogger(zap.NewNop()),
	)

	if err == nil || err.Error() != expectedError {
		t.Errorf("expected error reading site map got %q", err)
	}
}

func TestInvalidRootURL(t *testing.T) {
	testURL := "%"
	//lint:ignore SA1007 we want to test an invalid URL
	_, expectedError := url.Parse(testURL)

	_, err := CrawlDomain(testURL, SetLogger(zap.NewNop()))

	if err == nil || err.Error() != expectedError.Error() {
		t.Errorf("expected error reading site map got %q", err)
	}
}

func TestInvalidConfiguration(t *testing.T) {
	expectedError := "config.MaxConcurrency must be greater than 0"

	_, err := CrawlDomain(
		"http://localhost",
		SetLogger(zap.NewNop()),
		SetMaxConcurrency(0),
	)

	if err == nil || err.Error() != expectedError {
		t.Errorf("expected error reading site map got %q", err)
	}
}

func BenchmarkCrawlExample(b *testing.B) {
	testServer := newTestServer()
	defer testServer.Close()

	resolvedSiteMap, resolveSiteMapErr := expectedSiteMapString(
		testServer.URL,
		expectedSiteMap,
	)
	if resolveSiteMapErr != nil {
		b.Errorf(
			"error creating resolved expected site map: %q",
			resolveSiteMapErr,
		)
	}

	for n := 0; n < b.N; n++ {
		sitemap, err := CrawlDomain(
			testServer.URL,
			SetClient(testServer.Client()),
			SetLogger(zap.NewNop()),
		)

		if err != nil {
			b.Errorf("error reading example site map: %q", err)
		}

		var siteMapBuf bytes.Buffer
		sitemap.WriteMap(&siteMapBuf)

		siteMapString := siteMapBuf.String()

		if siteMapString != resolvedSiteMap {
			b.Errorf(
				"unexpected site map produced.\n\n\n"+
					"Got:\n\n%s\n\nExpected:\n\n%s",
				siteMapString,
				resolvedSiteMap,
			)
		}
	}

}

func newTestServer() *httptest.Server {
	mux := http.NewServeMux()

	// Tests where redirects point to third party sites
	rh := http.RedirectHandler(
		"http://picsum.org",
		http.StatusTemporaryRedirect,
	)

	// Tests where redirects point internally. In this example the link on
	// the homepage /secret should redirect to /hidden. /hidden should be
	// present in the site map
	ih := http.RedirectHandler("/hidden", http.StatusMovedPermanently)

	ch := cachedDirectoryHandler()

	sh := http.HandlerFunc(func(res http.ResponseWriter, req *http.Request) {
		time.Sleep(time.Second)
		req.URL.Path = "/"
		ch(res, req)
	})

	mux.Handle("/picsum", rh)
	mux.Handle("/secret", ih)
	mux.Handle("/", ch)
	mux.Handle("/slow", sh)

	return httptest.NewServer(mux)
}

// expectedSiteMapString takes the expected paths and prefixes with the given
// root URL, producing a single expected site map string.
func expectedSiteMapString(rootURL string, paths []string) (string, error) {
	root, err := url.Parse(rootURL)
	if err != nil {
		return "", err
	}

	fullString := make([]string, len(paths)+1)
	for i, path := range paths {
		pathURL, pathURLErr := url.Parse(path)
		if pathURLErr != nil {
			return "", pathURLErr
		}
		fullString[i] = root.ResolveReference(pathURL).String()
	}
	// An extra new line is required on the end
	fullString[len(paths)] = ""

	return strings.Join(fullString, "\n"), nil
}

func cachedDirectoryHandler() http.HandlerFunc {
	if directoryHandler == nil {
		var err error = nil
		directoryHandler, err = testServer.DirectoryHandler(
			"./test/server/resources",
		)

		if err != nil {
			log.Fatalf("error loading example directory: %q", err)
		}
	}

	return directoryHandler
}
