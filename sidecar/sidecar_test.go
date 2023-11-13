// Copyright 2013 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package sidecar

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-kit/log"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func Test_sidecarService_UpdateConfigReload(t *testing.T) {
	testDir, err := os.MkdirTemp("", "snmp-config")
	if err != nil {
		t.Error(err)
		return
	}

	fmt.Println("test dir:", testDir)
	defer os.RemoveAll(testDir)

	configFile := filepath.Join(testDir, "snmp.yml")
	templateConfigYaml, err := os.ReadFile(filepath.Join("..", "snmp.yml"))
	if err != nil {
		t.Error(err)
		return
	}

	// 准备一下文件
	err = os.WriteFile(configFile, templateConfigYaml, 0o666)
	if err != nil {
		t.Error(err)
		return
	}

	s := &sidecarService{
		logger:     log.NewLogfmtLogger(os.Stdout),
		configFile: configFile,
	}

	require.Equal(t, time.Time{}, s.GetLastUpdateTs(), "should not be updated")

	t.Run("success", func(t *testing.T) {

		cmd := &UpdateConfigCmd{
			Yaml: `
auths:
  test_v1:
    community: public
    security_level: noAuthNoPriv
    auth_protocol: MD5
    priv_protocol: DES
    version: 1
`,
		}

		reloadCh := make(chan chan error)
		go func() {
			ch := <-reloadCh
			ch <- nil
		}()
		require.NoError(t, s.UpdateConfigReload(context.TODO(), cmd, reloadCh))
		require.NotEqual(t, time.Time{}, s.GetLastUpdateTs(), "GetLastUpdateTs() still zero")
	})

}

func Test_sidecarService_UpdateConfigReload_FailRecover(t *testing.T) {

	testDir, err := os.MkdirTemp("", "snmp-config")
	require.NoError(t, err)

	fmt.Println("test dir:", testDir)
	defer os.RemoveAll(testDir)

	configFile := filepath.Join(testDir, "snmp.yml")
	templateConfigYaml, err := os.ReadFile(filepath.Join("..", "snmp.yml"))
	require.NoError(t, err)

	// 预先准备一下文件
	err = os.WriteFile(configFile, templateConfigYaml, 0o666)
	require.NoError(t, err)

	s := &sidecarService{
		logger:     log.NewLogfmtLogger(os.Stdout),
		configFile: configFile,
	}

	require.Equal(t, time.Time{}, s.GetLastUpdateTs(), "should not be updated")

	t.Run("bad yaml", func(t *testing.T) {

		cmd := &UpdateConfigCmd{
			Yaml: `
modules:
  http_2xx:
    prober: http
    blah blah
    http:
      preferred_ip_protocol: "ip4"
`,
		}
		// parse yaml 的时候出现错误
		reloadCh := make(chan chan error)
		go func() {
			ch := <-reloadCh
			ch <- nil
		}()
		err = s.UpdateConfigReload(context.TODO(), cmd, reloadCh)
		require.Error(t, err, "UpdateConfigReload should return err")
		require.Equal(t, time.Time{}, s.GetLastUpdateTs(), "GetLastUpdateTs() should not be updated")

		afterUpdateConfigYaml, err := os.ReadFile(filepath.Join("..", "snmp.yml"))
		require.NoError(t, err)
		require.Equal(t, templateConfigYaml, afterUpdateConfigYaml, "UpdateConfigReload fail should keep old file unchanged")

	})

	t.Run("reload error happen", func(t *testing.T) {

		cmd := &UpdateConfigCmd{
			Yaml: `
auths:
  test_v1:
    community: public
    security_level: noAuthNoPriv
    auth_protocol: MD5
    priv_protocol: DES
    version: 1
`,
		}
		// blackbox reload 时发生错误
		reloadCh := make(chan chan error)
		go func() {
			ch := <-reloadCh
			ch <- errors.New("on purpose")
		}()
		err = s.UpdateConfigReload(context.TODO(), cmd, reloadCh)
		require.Error(t, err, "UpdateConfigReload should return err")
		require.Equal(t, time.Time{}, s.GetLastUpdateTs(), "GetLastUpdateTs() should not be updated")

		afterUpdateConfigYaml, err := os.ReadFile(filepath.Join("..", "snmp.yml"))
		require.NoError(t, err)
		require.Equal(t, templateConfigYaml, afterUpdateConfigYaml, "UpdateConfigReload fail should keep old file unchanged")
	})

}
