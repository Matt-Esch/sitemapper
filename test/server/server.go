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

package server

import (
	"bytes"
	"io/ioutil"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// A Resource is a file loaded from disk
type Resource struct {
	FileName string
	Content  []byte
	ModTime  time.Time
}

// HandleRequest returns the resource for a given http GET or HEAD request
func (res *Resource) HandleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		handleMethodNotAllowed(w, r)
		return
	}

	reader := bytes.NewReader(res.Content)
	http.ServeContent(w, r, res.FileName, res.ModTime, reader)
}

func handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	http.Error(w, "not found", http.StatusNotFound)
}

// LoadResource loads a Resource from a specified file path
func LoadResource(path string) (*Resource, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	fileContent, err := ioutil.ReadAll(file)
	if err != nil {
		return nil, err
	}

	_, fileName := filepath.Split(path)

	return &Resource{
		FileName: fileName,
		Content:  fileContent,
		ModTime:  fileInfo.ModTime(),
	}, nil
}

// LoadAllResources recursively walks a directory searching for files that have
// a supported mime type for http and loads the contents. A URL path is
// generated from the path relative to the specified directory. The retuned map
// maps the generated URLs to the loaded resources.
func LoadAllResources(dir string) (map[string]*Resource, error) {
	resources := map[string]*Resource{}
	err := filepath.Walk(dir,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}
			if mime.TypeByExtension(filepath.Ext(path)) != "" {
				resource, err := LoadResource(path)
				if err != nil {
					return err
				}

				route, err := filepath.Rel(dir, path)
				if err != nil {
					return err
				}

				resources["/"+route] = resource
			}
			return nil
		})

	if err != nil {
		return nil, err
	}

	return resources, nil
}

// DirectoryHandler produces an http handler function that can serve the
// contents of a specified directory. The handler will only serve files that
// have a matching mime type for their extension. The handler assumes that the
// path / translates to /index.html and that paths without an extension also
// map to the .html extension.
func DirectoryHandler(dir string) (http.HandlerFunc, error) {
	resources, err := LoadAllResources(dir)
	if err != nil {
		return nil, err
	}

	return func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if path == "" || path == "/" {
			path = "/index.html"
		}
		if filepath.Ext(path) == "" {
			path = path + ".html"
		}
		resource := resources[path]
		if resource == nil {
			handleNotFound(w, r)
		} else {
			resource.HandleRequest(w, r)
		}
	}, nil
}
