// Copyright (c) 2025 OpenBao a Series of LF Projects, LLC
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package benchmarktests

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/openbao/openbao/api/v2"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

const (
	NamespaceType   = "namespace"
	NamespaceMethod = "POST"
)

func init() {
	// "Register" this test to the main test registry
	TestList[NamespaceType] = func() BenchmarkBuilder {
		return &NamespaceTest{}
	}
}

type NamespaceTest struct {
	pathPrefix      string
	header          http.Header
	config          *NamespaceTestConfig
	namespacePrefix string
	namespaceData   string
	plugin          string
	capabilities    []string
	logger          hclog.Logger
}

type NamespaceTestConfig struct {
	NamespacePrefix string `hcl:"namespace_prefix,optional"`
}

func (n *NamespaceTest) ParseConfig(body hcl.Body) error {
	testConfig := &struct {
		Config *NamespaceTestConfig `hcl:"config,block"`
	}{
		Config: &NamespaceTestConfig{
			NamespacePrefix: "benchmark",
		},
	}

	diags := gohcl.DecodeBody(body, nil, testConfig)
	if diags.HasErrors() {
		return fmt.Errorf("error decoding to struct: %v", diags)
	}
	n.config = testConfig.Config
	return nil
}

func (n *NamespaceTest) Target(client *api.Client) vegeta.Target {
	namespacePath, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}

	namespacePath = n.namespacePrefix + "-" + namespacePath

	return vegeta.Target{
		Method: NamespaceMethod,
		URL:    client.Address() + n.pathPrefix + "/" + namespacePath,
		Body:   []byte(`{"source":"benchmark-` + n.namespaceData + `"}`),
		Header: n.header,
	}
}

func (n *NamespaceTest) GetTargetInfo() TargetInfo {
	return TargetInfo{
		method:     NamespaceMethod,
		pathPrefix: n.pathPrefix,
	}
}

func (n *NamespaceTest) Cleanup(client *api.Client) error {
	n.logger.Trace("cleaning namespaces under " + n.pathPrefix)

	resp, err := client.Logical().List("sys/namespaces")
	if err != nil {
		return fmt.Errorf("error listing namespaces: %w", err)
	}

	for _, pathRaw := range resp.Data["keys"].([]interface{}) {
		path := pathRaw.(string)
		if !strings.HasPrefix(path, n.namespacePrefix) {
			continue
		}

		info := resp.Data["key_info"].(map[string]interface{})[path].(map[string]interface{})
		if valueRaw, present := info["source"]; present {
			value := valueRaw.(string)
			expected := "benchmark-" + n.namespaceData
			if value != expected {
				continue
			}
		}

		if _, err := client.Logical().Delete("sys/namespaces/" + path); err != nil {
			return fmt.Errorf("error cleaning up %v: %w", path, err)
		}
	}

	return nil
}

func (n *NamespaceTest) Setup(client *api.Client, namespaceName string, topLevelConfig *TopLevelTargetConfig) (BenchmarkBuilder, error) {
	n.logger = targetLogger.Named("namespaces")

	var err error
	var namespaceData = namespaceName
	if topLevelConfig.RandomMounts {
		namespaceData, err = uuid.GenerateUUID()
		if err != nil {
			log.Fatalf("can't create UUID")
		}
	}

	headers := http.Header{"X-Vault-Token": []string{client.Token()}, "X-Vault-Namespace": []string{client.Headers().Get("X-Vault-Namespace")}}
	return &NamespaceTest{
		pathPrefix:      "/v1/sys/namespaces",
		header:          headers,
		namespacePrefix: n.config.NamespacePrefix,
		namespaceData:   namespaceData,
		logger:          n.logger,
	}, nil
}

func (n *NamespaceTest) Flags(fs *flag.FlagSet) {}
