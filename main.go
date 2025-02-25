// Copyright 2025 Pierre-Henri Symoneaux
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// 	http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	"github.com/ovh/kmip-go/kmipclient"
	"github.com/phsym/kmip-explorer/internal"

	"flag"

	"github.com/google/uuid"
	_ "github.com/joho/godotenv/autoload"
)

var (
	// The following values will be set automatically by goreleaser during the CI/CD pipeline execution
	// see: https://goreleaser.com/cookbooks/using-main.version/ and https://goreleaser.com/customization/builds/
	// The default ldflags are '-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}'.
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	addr  = flag.String("addr", os.Getenv("KMIP_ADDR"), "Address and port of the KMIP Server")
	cert  = flag.String("cert", os.Getenv("KMIP_CERT"), "Path to the client certificate")
	key   = flag.String("key", os.Getenv("KMIP_KEY"), "Path to the client private key")
	ca    = flag.String("ca", os.Getenv("KMIP_CA"), "Server's CA (optional)")
	noCcv = flag.Bool("no-ccv", false, "Do not add client correlation value to requests")
	vers  = flag.Bool("version", false, "Display version information")
)

func main() {
	if inf, ok := debug.ReadBuildInfo(); ok {
		version = inf.Main.Version
	}
	flag.Parse()
	if *vers {
		fmt.Printf("Version: %s\nCommit: %s\nBuild Date: %s\nGo Verison: %s\nOS: %s\nArch: %s\n", version, commit, date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
		return
	}
	if *addr == "" || *cert == "" || *key == "" {
		fmt.Fprintln(os.Stderr, "Missing one of arguments --addr, --cert or --key")
		flag.PrintDefaults()
		return
	}
	// tview.Styles.PrimitiveBackgroundColor = tcell.ColorNone
	client := newClient()
	exp := internal.NewExplorer(client, version)
	if err := exp.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
}

func newClient() *kmipclient.Client {
	middlewares := []kmipclient.Middleware{}
	if !*noCcv {
		middlewares = append(middlewares, kmipclient.CorrelationValueMiddleware(uuid.NewString))
	}
	client, err := kmipclient.Dial(
		*addr,
		kmipclient.WithRootCAFile(*ca),
		kmipclient.WithClientCertFiles(*cert, *key),
		kmipclient.WithMiddlewares(middlewares...),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	return client
}
