# SiteMapper [![GoDoc][doc-img]][doc] [![Build Status][ci-img]][ci] [![Coverage Status][cov-img]][cov]

Parallel web crawler implemented in Golang for producing site maps

## Installation

`go get -u github.com/Matt-Esch/sitemapper`

## Quick Start

You can use the package to read a site map from a given URL or you can compile
and use the provided binary.

### Basic Usage

```go
package main

import (
  "log"
  "os"

  "github.com/Matt-Esch/sitemapper"
)

func main() {
  siteMap, err := sitemapper.CrawlDomain("https://monzo.com")
    if err != nil {
      log.Fatalf("Error: %s", err)
    }

    siteMap.WriteMap(os.Stdout)
}
```

### Binary usage

The package provides a binary to run the crawler from the command line

```bash
go install github.com/Matt-Esch/sitemapper/cmd/sitemapper
sitemapper -u "http://todomvc.com"

http://todomvc.com
http://todomvc.com/
http://todomvc.com/examples/angular-dart/web
http://todomvc.com/examples/angular-dart/web/
http://todomvc.com/examples/angular2
http://todomvc.com/examples/angular2/
http://todomvc.com/examples/angularjs
http://todomvc.com/examples/angularjs/
http://todomvc.com/examples/angularjs_require
http://todomvc.com/examples/angularjs_require/

...

```

For a list of options use `sitemapper -h`

```
  -c int
        maximum concurrency (default 8)
  -d    enable debug logs
  -k duration
        http keep alive timeout (default 30s)
  -t duration
        http request timeout (default 30s)
  -u string
        url to crawl (required)
  -v    enable verbose logging
  -w duration
        maximum crawl time
```


## Brief implementation outline

  - The bulk of the implementation is found in `./sitemapper.go`

  - Tests and benchmarks are defined in `./sitemapper_test.go`

  - A test server is defined in `./test/server` and is used to create a
    crawlable website that listens on localhost on a random port. This website
    adds various traps such as pointing to external domains in order to test
    the crawler.

  - The binary to run the web crawler from the command line is defined under
    `./cmds/sitemapper/main.go`


## Design choices and limitations:

  - The web crawler is a parallel web crawler with bounded concurrency. A
    channel of URLs is consumed by a fixed number of go routines. These go
    routines make an http GET request to the received URL, parse it for a tags,
    and push previously unseen URLs into the URL channel for further
    consumption.

  - The web crawler populates the site map with new URLs before making a request
    to the new URL. This means that non-existent pages (404) and non-web page
    links (i.e. links to PDFs) will appear in the site map.

  - By default the logic for checking "same domain" considers just the "host"
    portion of the URL. The scheme (http/https) is ignored when checking same
    domain constraints even though this would be considered cross origin.
    It can be quite difficult to define a universally acceptable definition of
    "same domain", where some may resort to DNS lookup as the most accurate.
    For that reason, a sensible default is provided but it can be overridden by
    the caller.


## License

Released under the [MIT License](LICENSE.txt).

[doc-img]: https://godoc.org/github.com/Matt-Esch/sitemapper?status.svg
[doc]: https://godoc.org/github.com/Matt-Esch/sitemapper
[ci-img]: https://travis-ci.com/Matt-Esch/sitemapper.svg?branch=master
[ci]: https://travis-ci.com/Matt-Esch/sitemapper
[cov-img]: https://codecov.io/gh/Matt-Esch/sitemapper/branch/master/graph/badge.svg
[cov]: https://codecov.io/gh/Matt-Esch/zap
