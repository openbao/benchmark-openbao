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
	MountType   = "mount"
	MountMethod = "POST"
)

func init() {
	// "Register" this test to the main test registry
	TestList[MountType] = func() BenchmarkBuilder {
		return &MountTest{}
	}
}

type MountTest struct {
	pathPrefix   string
	header       http.Header
	config       *MountTestConfig
	mountType    string
	mountPrefix  string
	plugin       string
	capabilities []string
	logger       hclog.Logger
}

type MountTestConfig struct {
	MountType string `hcl:"mount_type,optional"`
	Plugin    string `hcl:"plugin,optional"`
}

func (m *MountTest) ParseConfig(body hcl.Body) error {
	testConfig := &struct {
		Config *MountTestConfig `hcl:"config,block"`
	}{
		Config: &MountTestConfig{
			MountType: "secret",
			Plugin:    "kv-v2",
		},
	}

	diags := gohcl.DecodeBody(body, nil, testConfig)
	if diags.HasErrors() {
		return fmt.Errorf("error decoding to struct: %v", diags)
	}
	m.config = testConfig.Config
	return nil
}

func (m *MountTest) Target(client *api.Client) vegeta.Target {
	mountPath, err := uuid.GenerateUUID()
	if err != nil {
		panic(err)
	}

	mountPath = m.plugin + "-" + mountPath

	return vegeta.Target{
		Method: MountMethod,
		URL:    client.Address() + m.pathPrefix + "/" + mountPath,
		Body:   []byte(`{"type":"` + m.plugin + `"}`),
		Header: m.header,
	}
}

func (m *MountTest) GetTargetInfo() TargetInfo {
	return TargetInfo{
		method:     MountMethod,
		pathPrefix: m.pathPrefix,
	}
}

func (m *MountTest) Cleanup(client *api.Client) error {
	m.logger.Trace("cleaning mounts under " + m.pathPrefix)

	switch m.mountType {
	case "secret":
		mounts, err := client.Sys().ListMounts()
		if err != nil {
			return fmt.Errorf("error listing mounts: %w", err)
		}

		for path, info := range mounts {
			if info.Type != m.plugin {
				continue
			}

			if !strings.HasPrefix(path, m.mountPrefix) {
				continue
			}

			if err := client.Sys().Unmount(path); err != nil {
				return fmt.Errorf("error cleaning up %v: %w", path, err)
			}
		}
	case "auth":
		mounts, err := client.Sys().ListAuth()
		if err != nil {
			return fmt.Errorf("error listing auth mounts: %w", err)
		}

		for path, info := range mounts {
			if info.Type != m.plugin {
				continue
			}

			if !strings.HasPrefix(path, m.mountPrefix) {
				continue
			}

			if err := client.Sys().DisableAuth(path); err != nil {
				return fmt.Errorf("error cleaning up %v: %w", path, err)
			}
		}
	}

	return nil
}

func (m *MountTest) Setup(client *api.Client, mountName string, topLevelConfig *TopLevelTargetConfig) (BenchmarkBuilder, error) {
	var err error
	var mountPath = mountName
	m.logger = targetLogger.Named("mounts")

	if topLevelConfig.RandomMounts {
		mountPath, err = uuid.GenerateUUID()
		if err != nil {
			log.Fatalf("can't create UUID")
		}
	}

	var table string
	switch m.config.MountType {
	case "secret":
		table = "mounts"
	case "auth":
		table = "auth"
	default:
		return nil, fmt.Errorf("unknown mount type: %v", m.config.MountType)
	}

	headers := http.Header{"X-Vault-Token": []string{client.Token()}, "X-Vault-Namespace": []string{client.Headers().Get("X-Vault-Namespace")}}
	return &MountTest{
		pathPrefix:  "/v1/sys/" + table + "/" + mountPath,
		header:      headers,
		mountPrefix: mountPath,
		mountType:   m.config.MountType,
		plugin:      m.config.Plugin,
		logger:      m.logger,
	}, nil
}

func (m *MountTest) Flags(fs *flag.FlagSet) {}
