// Copyright (c) 2025 OpenBao a Series of LF Projects, LLC
// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package benchmarktests

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/openbao/openbao/api/v2"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

const (
	ACLPolicyReadType    = "acl_policy_read"
	ACLPolicyListType    = "acl_policy_list"
	ACLPolicyWriteType   = "acl_policy_write"
	ACLPolicyReadMethod  = "GET"
	ACLPolicyListMethod  = "LIST"
	ACLPolicyWriteMethod = "POST"
)

func init() {
	// "Register" this test to the main test registry
	TestList[ACLPolicyReadType] = func() BenchmarkBuilder {
		return &ACLPolicyTest{action: "read"}
	}
	TestList[ACLPolicyListType] = func() BenchmarkBuilder {
		return &ACLPolicyTest{action: "list"}
	}
	TestList[ACLPolicyWriteType] = func() BenchmarkBuilder {
		return &ACLPolicyTest{action: "write"}
	}
}

type ACLPolicyTest struct {
	pathPrefix   string
	header       http.Header
	config       *ACLPolicyTestConfig
	action       string
	policies     int
	pathLength   int
	paths        int
	capabilities []string
	logger       hclog.Logger
}

type ACLPolicyTestConfig struct {
	Policies     int      `hcl:"policies,optional"`
	PathLength   int      `hcl:"path_length,optional"`
	Paths        int      `hcl:"paths,optional"`
	Capabilities []string `hcl:"capabilities,optional"`
}

func (a *ACLPolicyTest) ParseConfig(body hcl.Body) error {
	testConfig := &struct {
		Config *ACLPolicyTestConfig `hcl:"config,block"`
	}{
		Config: &ACLPolicyTestConfig{
			Policies:     10,
			PathLength:   25,
			Paths:        1,
			Capabilities: []string{"create", "read", "update", "delete", "list", "sudo"},
		},
	}

	diags := gohcl.DecodeBody(body, nil, testConfig)
	if diags.HasErrors() {
		return fmt.Errorf("error decoding to struct: %v", diags)
	}
	a.config = testConfig.Config
	return nil
}

func (a *ACLPolicyTest) read(client *api.Client) vegeta.Target {
	policyNum := int(1 + rand.Int31n(int32(a.policies)))
	return vegeta.Target{
		Method: ACLPolicyReadMethod,
		URL:    client.Address() + a.pathPrefix + "/policy-" + strconv.Itoa(policyNum),
		Header: a.header,
	}
}

func (a *ACLPolicyTest) list(client *api.Client) vegeta.Target {
	return vegeta.Target{
		Method: ACLPolicyListMethod,
		URL:    client.Address() + a.pathPrefix,
		Header: a.header,
	}
}

func (a *ACLPolicyTest) draftPolicy(paths int, pathLength int, capabilities []string) map[string]interface{} {
	var policy string
	for i := 0; i < paths; i++ {
		// Hopefully ensure unique paths.
		path := fmt.Sprintf("%v", i) + strings.Repeat("a", pathLength)
		path = path[0:pathLength]

		policy += `path "` + path + `" {
  capabilities = ["` + strings.Join(capabilities, `", "`) + `"]
}
`
	}

	data := map[string]interface{}{
		"policy": policy,
	}

	return data
}

func (a *ACLPolicyTest) write(client *api.Client) vegeta.Target {
	policyNum := int(1 + rand.Int31n(int32(a.policies)))

	policy := a.draftPolicy(a.paths, a.pathLength, a.capabilities)
	body, err := json.Marshal(policy)
	if err != nil {
		panic("failed to marshal body: " + err.Error())
	}

	return vegeta.Target{
		Method: ACLPolicyWriteMethod,
		URL:    client.Address() + a.pathPrefix + "/policy-" + strconv.Itoa(policyNum),
		Body:   body,
		Header: a.header,
	}
}

func (a *ACLPolicyTest) Target(client *api.Client) vegeta.Target {
	switch a.action {
	case "write":
		return a.write(client)
	case "list":
		return a.list(client)
	default:
		return a.read(client)
	}
}

func (a *ACLPolicyTest) GetTargetInfo() TargetInfo {
	var method string
	switch a.action {
	case "write":
		method = ACLPolicyWriteMethod
	case "list":
		method = ACLPolicyListMethod
	default:
		method = ACLPolicyReadMethod
	}
	return TargetInfo{
		method:     method,
		pathPrefix: a.pathPrefix,
	}
}

func (a *ACLPolicyTest) Cleanup(client *api.Client) error {
	a.logger.Trace("cleaning policies under " + a.pathPrefix)
	for i := 1; i <= a.policies; i++ {
		_, err := client.Logical().Delete(strings.TrimPrefix(a.pathPrefix, "/v1") + "/policy-" + strconv.Itoa(i))
		if err != nil {
			return fmt.Errorf("failed to clean up policy (%v): %w", i, err)
		}
	}
	return nil
}

func (a *ACLPolicyTest) Setup(client *api.Client, mountName string, topLevelConfig *TopLevelTargetConfig) (BenchmarkBuilder, error) {
	var err error
	var policyPath = mountName
	a.logger = targetLogger.Named("acl-policies")

	if topLevelConfig.RandomMounts {
		policyPath, err = uuid.GenerateUUID()
		if err != nil {
			log.Fatalf("can't create UUID")
		}
	}

	a.logger.Trace("setting up policies under " + policyPath)

	for i := 1; i <= a.config.Policies; i++ {
		policy := a.draftPolicy(a.config.Paths, a.config.PathLength, a.config.Capabilities)
		_, err := client.Logical().Write("sys/policies/acl/"+policyPath+"/policy-"+strconv.Itoa(i), policy)
		if err != nil {
			return nil, fmt.Errorf("failed to create policy (%v): %w", i, err)
		}
	}

	headers := http.Header{"X-Vault-Token": []string{client.Token()}, "X-Vault-Namespace": []string{client.Headers().Get("X-Vault-Namespace")}}
	return &ACLPolicyTest{
		pathPrefix:   "/v1/sys/policies/acl/" + policyPath,
		action:       a.action,
		header:       headers,
		policies:     a.config.Policies,
		pathLength:   a.config.PathLength,
		paths:        a.config.Paths,
		capabilities: a.config.Capabilities,
		logger:       a.logger,
	}, nil
}

func (a *ACLPolicyTest) Flags(fs *flag.FlagSet) {}
