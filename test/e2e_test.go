// Copyright (c) 2022 Alibaba Group Holding Ltd.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// +build conformance

package test

import (
	"flag"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/networking/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"

	"github.com/alibaba/higress/test/e2e/conformance/tests"
	"github.com/alibaba/higress/test/e2e/conformance/utils/flags"
	"github.com/alibaba/higress/test/e2e/conformance/utils/suite"
)

var isWasmPluginTest = flag.Bool("isWasmPluginTest", false, "")
var wasmPluginType = flag.String("wasmPluginType", "GO", "")
var wasmPluginName = flag.String("wasmPluginName", "", "")

func TestHigressConformanceTests(t *testing.T) {
	flag.Parse()

	cfg, err := config.GetConfig()
	require.NoError(t, err)

	client, err := client.New(cfg, client.Options{})
	require.NoError(t, err)

	require.NoError(t, v1.AddToScheme(client.Scheme()))

	cSuite := suite.New(suite.Options{
		Client:               client,
		IngressClassName:     *flags.IngressClassName,
		Debug:                *flags.ShowDebug,
		CleanupBaseResources: *flags.CleanupBaseResources,
		GatewayAddress:       "localhost",
	})

	cSuite.Setup(t)
	var higressTests []suite.ConformanceTest

	for _, testcase := range tests.HigressConformanceTests {
		if !strings.Contains(testcase.ShortName, "Wasm") {
			higressTests = append(higressTests, testcase)
		}
	}

	cSuite.Run(t, higressTests)
}
