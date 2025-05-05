// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package benchmarktests

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/openbao/openbao/api/v2"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

const (
	KVV2ReadTestType    = "kvv2_read"
	KVV2ListTestType    = "kvv2_list"
	KVV2WriteTestType   = "kvv2_write"
	KVV2ReadTestMethod  = "GET"
	KVV2ListTestMethod  = "LIST"
	KVV2WriteTestMethod = "POST"

	MAX_UPGRADE_RETRY = 100
)

func init() {
	TestList[KVV2ReadTestType] = func() BenchmarkBuilder {
		return &KVV2Test{action: "read"}
	}
	TestList[KVV2WriteTestType] = func() BenchmarkBuilder {
		return &KVV2Test{action: "write"}
	}
	TestList[KVV2ListTestType] = func() BenchmarkBuilder {
		return &KVV2Test{action: "list"}
	}
}

type KVV2Test struct {
	pathPrefix string
	header     http.Header
	config     *KVV2SecretTestConfig
	action     string
	numKVs     int
	kvSize     int
	detailed   bool
	logger     hclog.Logger
}

type KVV2SecretTestConfig struct {
	KVSize   int  `hcl:"kvsize,optional"`
	NumKVs   int  `hcl:"numkvs,optional"`
	Detailed bool `hcl:"detailed,optional"`
}

func (k *KVV2Test) ParseConfig(body hcl.Body) error {
	testConfig := &struct {
		Config *KVV2SecretTestConfig `hcl:"config,block"`
	}{
		Config: &KVV2SecretTestConfig{
			KVSize:   1,
			NumKVs:   1000,
			Detailed: false,
		},
	}

	diags := gohcl.DecodeBody(body, nil, testConfig)
	if diags.HasErrors() {
		return fmt.Errorf("error decoding to struct: %v", diags)
	}
	k.config = testConfig.Config
	return nil
}

func (k *KVV2Test) read(client *api.Client) vegeta.Target {
	secnum := int(1 + rand.Int31n(int32(k.numKVs)))
	return vegeta.Target{
		Method: "GET",
		URL:    client.Address() + k.pathPrefix + "/data/secret-" + strconv.Itoa(secnum),
		Header: k.header,
	}
}

func (k *KVV2Test) list(client *api.Client) vegeta.Target {
	path := "metadata"
	if k.detailed {
		path = "detailed-metadata"
	}

	return vegeta.Target{
		Method: "LIST",
		URL:    client.Address() + k.pathPrefix + "/" + path,
		Header: k.header,
	}
}

func (k *KVV2Test) write(client *api.Client) vegeta.Target {
	secnum := int(1 + rand.Int31n(int32(k.numKVs)))
	value := strings.Repeat("a", k.kvSize)
	return vegeta.Target{
		Method: "POST",
		URL:    client.Address() + k.pathPrefix + "/data/secret-" + strconv.Itoa(secnum),
		Header: k.header,
		Body:   []byte(`{"data": {"foo": "` + value + `"}}`),
	}
}

func (k *KVV2Test) Target(client *api.Client) vegeta.Target {
	switch k.action {
	case "write":
		return k.write(client)
	case "list":
		return k.list(client)
	default:
		return k.read(client)
	}
}

func (k *KVV2Test) GetTargetInfo() TargetInfo {
	var method string
	switch k.action {
	case "write":
		method = KVV2WriteTestMethod
	case "list":
		method = KVV2ListTestMethod
	default:
		method = KVV2ReadTestMethod
	}
	return TargetInfo{
		method:     method,
		pathPrefix: k.pathPrefix,
	}
}

func (k *KVV2Test) Cleanup(client *api.Client) error {
	k.logger.Trace(cleanupLogMessage(k.pathPrefix))
	_, err := client.Logical().Delete(strings.Replace(k.pathPrefix, "/v1/", "/sys/mounts/", 1))
	if err != nil {
		return fmt.Errorf("error cleaning up mount: %v", err)
	}
	return nil
}

func (k *KVV2Test) Setup(client *api.Client, mountName string, topLevelConfig *TopLevelTargetConfig) (BenchmarkBuilder, error) {
	var err error
	mountPath := mountName
	switch k.action {
	case "write":
		k.logger = targetLogger.Named(KVV2WriteTestType)
	case "list":
		k.logger = targetLogger.Named(KVV2ListTestType)
	default:
		k.logger = targetLogger.Named(KVV2ReadTestType)
	}

	if topLevelConfig.RandomMounts {
		mountPath, err = uuid.GenerateUUID()
		if err != nil {
			log.Fatalf("can't create UUID")
		}
	}

	k.logger.Trace(mountLogMessage("secrets", "kvv2", mountPath))
	err = client.Sys().Mount(mountPath, &api.MountInput{
		Type: "kv",
		Options: map[string]string{
			"version": "2",
		},
	})
	if err != nil {
		return nil, fmt.Errorf("error mounting kv secrets engine: %v", err)
	}

	setupLogger := k.logger.Named(mountPath)

	secval := map[string]interface{}{
		"data": map[string]interface{}{
			"foo": 1,
		},
	}

	// TODO: Find more deterministic way of avoiding this
	// Avoid error of the form:
	// * Upgrading from non-versioned to versioned data. This backend will be unavailable for a brief period and will resume service shortly.
	for i := 1; i <= MAX_UPGRADE_RETRY; i++ {
		_, err = client.Logical().Read(mountPath + "/config")
		if err == nil {
			break
		}
		if !strings.Contains(err.Error(), "Upgrading from non-versioned to versioned data.") {
			return nil, fmt.Errorf("cannot read KVv2 configuration: %w", err)
		}

		time.Sleep(time.Duration(i) * 10 * time.Millisecond)
	}

	setupLogger.Trace("seeding secrets")
	for i := 1; i <= k.config.NumKVs; i++ {
		_, err = client.Logical().Write(mountPath+"/data/secret-"+strconv.Itoa(i), secval)
		if err != nil {
			return nil, fmt.Errorf("error writing kv secret: %v", err)
		}
	}

	return &KVV2Test{
		pathPrefix: "/v1/" + mountPath,
		header:     http.Header{"X-Vault-Token": []string{client.Token()}, "X-Vault-Namespace": []string{client.Headers().Get("X-Vault-Namespace")}},
		numKVs:     k.config.NumKVs,
		kvSize:     k.config.KVSize,
		detailed:   k.config.Detailed,
		logger:     k.logger,
		action:     k.action,
	}, nil
}

func (k *KVV2Test) Flags(fs *flag.FlagSet) {}
