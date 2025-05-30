// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

package benchmarktests

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-uuid"
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/gohcl"
	"github.com/openbao/openbao/api/v2"
	vegeta "github.com/tsenart/vegeta/v12/lib"
)

// Constants for test
const (
	RedisDynamicSecretTestType         = "redis_dynamic_secret"
	RedisDynamicSecretTestMethod       = "GET"
	RedisDynamicSecretDBUsernameEnvVar = VaultBenchmarkEnvVarPrefix + "REDIS_USERNAME"
	RedisDynamicSecretDBPasswordEnvVar = VaultBenchmarkEnvVarPrefix + "REDIS_PASSWORD"
)

func init() {
	TestList[RedisDynamicSecretTestType] = func() BenchmarkBuilder { return &RedisDynamicSecret{} }
}

type RedisDynamicSecret struct {
	pathPrefix string
	roleName   string
	header     http.Header
	config     *RedisDynamicSecretTestConfig
	logger     hclog.Logger
}

type RedisDynamicSecretTestConfig struct {
	DBConfig   *RedisDBConfig          `hcl:"db_connection,block"`
	RoleConfig *RedisDynamicRoleConfig `hcl:"role,block"`
}

type RedisDynamicRoleConfig struct {
	Name               string `hcl:"name,optional"`
	DBName             string `hcl:"db_name,optional"`
	DefaultTTL         string `hcl:"default_ttl,optional"`
	MaxTTL             string `hcl:"max_ttl,optional"`
	CreationStatements string `hcl:"creation_statements"`
}

// ParseConfig parses the passed in hcl.Body into Configuration structs for use during
// test configuration in Vault. Any default configuration definitions for required
// parameters will be set here.
func (r *RedisDynamicSecret) ParseConfig(body hcl.Body) error {
	// provide defaults
	testConfig := &struct {
		Config *RedisDynamicSecretTestConfig `hcl:"config,block"`
	}{
		Config: &RedisDynamicSecretTestConfig{
			DBConfig: &RedisDBConfig{
				Name:         "benchmark-redis-db",
				PluginName:   "redis-database-plugin",
				AllowedRoles: []string{"my-*-role"},
				Username:     os.Getenv(RedisDynamicSecretDBUsernameEnvVar),
				Password:     os.Getenv(RedisDynamicSecretDBPasswordEnvVar),
			},
			RoleConfig: &RedisDynamicRoleConfig{
				Name:   "my-dynamic-role",
				DBName: "benchmark-redis-db",
			},
		},
	}

	diags := gohcl.DecodeBody(body, nil, testConfig)
	if diags.HasErrors() {
		return fmt.Errorf("error decoding to struct: %v", diags)
	}
	r.config = testConfig.Config

	if r.config.DBConfig.Username == "" {
		return fmt.Errorf("no redis username provided but required")
	}

	if r.config.DBConfig.Password == "" {
		return fmt.Errorf("no redis password provided but required")
	}

	return nil
}

func (r *RedisDynamicSecret) Target(client *api.Client) vegeta.Target {
	return vegeta.Target{
		Method: RedisDynamicSecretTestMethod,
		URL:    fmt.Sprintf("%s%s/creds/%s", client.Address(), r.pathPrefix, r.roleName),
		Header: r.header,
	}
}

func (r *RedisDynamicSecret) Cleanup(client *api.Client) error {
	r.logger.Trace(cleanupLogMessage(r.pathPrefix))
	_, err := client.Logical().Delete(strings.Replace(r.pathPrefix, "/v1/", "/sys/mounts/", 1))
	if err != nil {
		return fmt.Errorf("error cleaning up mount: %v", err)
	}
	return nil
}

func (r *RedisDynamicSecret) GetTargetInfo() TargetInfo {
	return TargetInfo{
		method:     RedisDynamicSecretTestMethod,
		pathPrefix: r.pathPrefix,
	}
}

func (r *RedisDynamicSecret) Setup(client *api.Client, mountName string, topLevelConfig *TopLevelTargetConfig) (BenchmarkBuilder, error) {
	var err error
	secretPath := mountName
	r.logger = targetLogger.Named(RedisDynamicSecretTestType)

	if topLevelConfig.RandomMounts {
		secretPath, err = uuid.GenerateUUID()
		if err != nil {
			log.Fatalf("can't create UUID")
		}
	}

	// Create Database Secret Mount
	r.logger.Trace(mountLogMessage("secrets", "database", secretPath))
	err = client.Sys().Mount(secretPath, &api.MountInput{
		Type: "database",
	})
	if err != nil {
		return nil, fmt.Errorf("error mounting db secrets engine: %v", err)
	}

	setupLogger := r.logger.Named(secretPath)

	// Decode DB Config struct into mapstructure to pass with request
	setupLogger.Trace(parsingConfigLogMessage("db"))
	dbData, err := structToMap(r.config.DBConfig)
	if err != nil {
		return nil, fmt.Errorf("error parsing db config from struct: %v", err)
	}

	// Set up db
	setupLogger.Trace(writingLogMessage("redis db config"), "name", r.config.DBConfig.Name)
	dbPath := filepath.Join(secretPath, "config", r.config.DBConfig.Name)
	_, err = client.Logical().Write(dbPath, dbData)
	if err != nil {
		return nil, fmt.Errorf("error writing redis db config: %v", err)
	}

	setupLogger.Trace(parsingConfigLogMessage("role"))
	roleData, err := structToMap(r.config.RoleConfig)
	if err != nil {
		return nil, fmt.Errorf("error parsing role config from struct: %v", err)
	}

	// Set Up Role
	setupLogger.Trace(writingLogMessage("redis role"), "name", r.config.RoleConfig.Name)
	rolePath := filepath.Join(secretPath, "roles", r.config.RoleConfig.Name)
	_, err = client.Logical().Write(rolePath, roleData)
	if err != nil {
		return nil, fmt.Errorf("error writing redis role %q: %v", r.config.RoleConfig.Name, err)
	}

	return &RedisDynamicSecret{
		pathPrefix: "/v1/" + secretPath,
		header:     generateHeader(client),
		roleName:   r.config.RoleConfig.Name,
		config:     r.config,
		logger:     r.logger,
	}, nil
}

func (r *RedisDynamicSecret) Flags(fs *flag.FlagSet) {}
