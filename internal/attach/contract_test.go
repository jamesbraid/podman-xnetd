// SPDX-License-Identifier: Apache-2.0
package attach

import (
	"go.podman.io/common/libnetwork/types"

	"github.com/jamesbraid/xnetd/internal/config"
	"github.com/jamesbraid/xnetd/internal/proto"
)

var (
	_ func(*config.Config) (*Attacher, error)                                = New
	_ func(*Attacher, proto.Request) (map[string]types.StatusBlock, error)   = (*Attacher).Attach
	_ func(*Attacher, proto.Request) error                                   = (*Attacher).Detach
	_ func(*Attacher, int, string) (string, error)                           = (*Attacher).PinNetns
	_ func(*Attacher, string) error                                          = (*Attacher).UnpinNetns
	_ func(proto.Request, map[string]types.StatusBlock) error                = WriteResolvConf
)
