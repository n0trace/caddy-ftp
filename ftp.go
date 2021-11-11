// Copyright 2015 Matthew Holt and The Caddy Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package h4fode implements an h4foder middleware for Caddy. The initial
// enhancements related to Accept-h4foding, minimum content length, and
// buffer/writer pools were adapted from https://github.com/xi2/httpgzip
// then modified heavily to accommodate modular h4foders and fix bugs.
// Code borrowed from that repository is Copyright (c) 2015 The Httpgzip Authors.
package ftp

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/modules/caddyhttp"
	"github.com/jlaffaye/ftp"
)

func init() {
	caddy.RegisterModule(HTTP4Ftp{})
}

type HTTP4Ftp struct {
	Addr         string        `json:"addr"`
	User         string        `json:"user"`
	Pass         string        `json:"pass"`
	DialTimeout  time.Duration `json:"dial_timeout"`
	DisabledEPSV bool          `json:"disable_epsv"`
	DisabledMLSD bool          `json:"disable_mlsd"`
	DisableUTF8  bool          `json:"disable_utf8"`
}

// Validate implements caddy.Validator.
func (h4f *HTTP4Ftp) Validate() (err error) {
	if h4f.DialTimeout == 0 {
		h4f.DialTimeout = 5 * time.Second
	}

	return nil
}

// CaddyModule returns the Caddy module information.
func (h4f HTTP4Ftp) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID: "http.handlers.HTTP4Ftp",
		New: func() caddy.Module {
			return new(HTTP4Ftp)
		},
	}
}

type ResponseWriter struct {
	http.ResponseWriter
	buf        *bytes.Buffer
	statusCode int
}

// Here we are implementing a Write() function from ResponseWriter with our custom instructions.
func (rw *ResponseWriter) Write(p []byte) (int, error) {
	return rw.buf.Write(p)
}

func (rw *ResponseWriter) WriteHeader(statusCode int) {
	rw.statusCode = statusCode
}

func (h4f HTTP4Ftp) ServeHTTP(w http.ResponseWriter, r *http.Request, next caddyhttp.Handler) (err error) {
	path := r.URL.Path
	var resp *ftp.Response
	var conn *ftp.ServerConn
	if conn, err = ftp.Dial(h4f.Addr,
		ftp.DialWithDisabledEPSV(h4f.DisabledEPSV),
		ftp.DialWithDisabledMLSD(h4f.DisabledMLSD),
		ftp.DialWithDisabledUTF8(h4f.DisableUTF8),
		ftp.DialWithTimeout(h4f.DialTimeout)); err != nil {
		err = fmt.Errorf("dial ftp server error: %w", err)
		return
	}

	if h4f.User != "" {
		if err = conn.Login(h4f.User, h4f.Pass); err != nil {
			err = fmt.Errorf("ftp login error: %w", err)
			return
		}
	}

	if resp, err = conn.Retr(path); err != nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte("Not Found"))
		return
	}

	defer resp.Close()
	w.WriteHeader(http.StatusOK)
	w.Header().Add("Content-Type", "application/octet-stream")
	_, err = io.Copy(w, resp)
	return err
}
